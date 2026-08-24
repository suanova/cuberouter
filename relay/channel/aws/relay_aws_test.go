package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const awsTestModel = "anthropic.claude-3-5-sonnet-20240620-v1:0"

type awsHTTPClientFunc func(*http.Request) (*http.Response, error)

func (f awsHTTPClientFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

type awsNotifyingResponseWriter struct {
	*httptest.ResponseRecorder
	notifyOn []byte
	notified chan int
	once     sync.Once
}

func newAwsNotifyingResponseWriter(notifyOn string) *awsNotifyingResponseWriter {
	return &awsNotifyingResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		notifyOn:         []byte(notifyOn),
		notified:         make(chan int, 1),
	}
}

func (w *awsNotifyingResponseWriter) Write(data []byte) (int, error) {
	return w.ResponseRecorder.Write(data)
}

func (w *awsNotifyingResponseWriter) Flush() {
	w.ResponseRecorder.Flush()
	if bytes.Contains(w.Body.Bytes(), w.notifyOn) {
		w.once.Do(func() {
			w.notified <- w.Body.Len()
		})
	}
}

func newAwsTestClient(httpClient bedrockruntime.HTTPClient) *bedrockruntime.Client {
	return bedrockruntime.New(bedrockruntime.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String("https://bedrock.test"),
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			"access-key", "secret-key", "",
		)),
		HTTPClient: httpClient,
		Retryer:    aws.NopRetryer{},
	})
}

func newAwsTestContext(writer http.ResponseWriter, requestContext context.Context) *gin.Context {
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	return c
}

func newAwsTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		StartTime:          time.Now(),
		IsStream:           true,
		OriginModelName:    awsTestModel,
		RelayFormat:        relaytypes.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: awsTestModel,
		},
	}
}

func newAwsInvokeModelInput() *bedrockruntime.InvokeModelInput {
	return &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(awsTestModel),
		Body:        []byte(`{}`),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}
}

func newAwsStreamInput() *bedrockruntime.InvokeModelWithResponseStreamInput {
	return &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(awsTestModel),
		Body:        []byte(`{}`),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}
}

func writeAwsStreamEvent(writer io.Writer, data string) error {
	payload, err := common.Marshal(struct {
		Bytes []byte `json:"bytes"`
	}{Bytes: []byte(data)})
	if err != nil {
		return err
	}

	return eventstream.NewEncoder().Encode(writer, eventstream.Message{
		Headers: eventstream.Headers{
			{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.EventMessageType)},
			{Name: eventstreamapi.EventTypeHeader, Value: eventstream.StringValue("chunk")},
			{Name: eventstreamapi.ContentTypeHeader, Value: eventstream.StringValue("application/json")},
		},
		Payload: payload,
	})
}

// gin.SetMode writes a package-global that parallel tests would race on, so it
// runs once here instead of inside individual tests.
func init() {
	gin.SetMode(gin.TestMode)
}

func TestDoAwsClientRequest_AppliesRuntimeHeaderOverrideToAnthropicBeta(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName:           "claude-3-5-sonnet-20240620",
		IsStream:                  false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"anthropic-beta": "computer-use-2025-01-24",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "access-key|secret-key|us-east-1",
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}

	requestBody := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
	adaptor := &Adaptor{}

	_, err := doAwsClientRequest(ctx, info, adaptor, requestBody)
	require.NoError(t, err)

	awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	require.True(t, ok)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(awsReq.Body, &payload))

	anthropicBeta, exists := payload["anthropic_beta"]
	require.True(t, exists)

	values, ok := anthropicBeta.([]any)
	require.True(t, ok)
	require.Equal(t, []any{"computer-use-2025-01-24"}, values)
}
