package service

import (
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskArtifactAccessBindsTaskAndKey(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "task-artifact-access-test-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	access, err := IssueTaskArtifactAccess("task-1", "video-main")
	require.NoError(t, err)
	assert.Len(t, access, 43)
	assert.NotContains(t, access, ".")
	assert.True(t, VerifyTaskArtifactAccess(access, "task-1", "video-main"))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-2", "video-main"))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-1", "video-other"))
	assert.False(t, VerifyTaskArtifactAccess(access+"x", "task-1", "video-main"))

	common.CryptoSecret = "another-node-secret"
	assert.False(t, VerifyTaskArtifactAccess(access, "task-1", "video-main"))
}

func TestBuildTaskArtifactContentURLUsesConfiguredAddressAndPreservesPrefix(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	previousServerAddress := system_setting.ServerAddress
	common.CryptoSecret = "task-artifact-url-test-secret"
	system_setting.TaskPublicAddress = "https://media.example/gateway/prefix/"
	system_setting.ServerAddress = "https://fallback.invalid"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
		system_setting.ServerAddress = previousServerAddress
	})

	contentURL, err := BuildTaskArtifactContentURL("task-public", "video-main")
	require.NoError(t, err)
	parsed, err := url.Parse(contentURL)
	require.NoError(t, err)
	assert.Equal(t, "media.example", parsed.Host)
	assert.Equal(t, "/gateway/prefix/v1/tasks/task-public/artifacts/video-main/content", parsed.Path)
	assert.True(t, VerifyTaskArtifactAccess(
		parsed.Query().Get(TaskArtifactAccessQueryParameter),
		"task-public",
		"video-main",
	))
}

func TestBuildTaskArtifactContentURLFallsBackOnlyToServerAddress(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	previousServerAddress := system_setting.ServerAddress
	common.CryptoSecret = "task-artifact-fallback-test-secret"
	system_setting.TaskPublicAddress = ""
	system_setting.ServerAddress = "https://gateway.example/root"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
		system_setting.ServerAddress = previousServerAddress
	})

	contentURL, err := BuildTaskArtifactContentURL("task-fallback", "audio")
	require.NoError(t, err)
	assert.Contains(t, contentURL, "https://gateway.example/root/v1/tasks/task-fallback/artifacts/audio/content")

	system_setting.TaskPublicAddress = "not-a-url"
	_, err = BuildTaskArtifactContentURL("task-fallback", "audio")
	assert.Error(t, err)
}

func TestBuildTaskVideoContentURLBindsFixedVideoKey(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	previousServerAddress := system_setting.ServerAddress
	common.CryptoSecret = "task-video-url-test-secret"
	system_setting.TaskPublicAddress = "http://127.0.0.1:5000/cuberouter"
	system_setting.ServerAddress = "https://fallback.invalid"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
		system_setting.ServerAddress = previousServerAddress
	})

	contentURL, err := BuildTaskVideoContentURL("task-video")
	require.NoError(t, err)
	parsed, err := url.Parse(contentURL)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:5000", parsed.Host)
	assert.Equal(t, "/cuberouter/v1/videos/task-video/content", parsed.Path)

	// 路线里没有 artifact 段，绑定的是固定的 video key。
	assert.Equal(t, "video", TaskVideoArtifactKey)
	access := parsed.Query().Get(TaskArtifactAccessQueryParameter)
	require.Len(t, access, taskArtifactAccessLength)
	assert.True(t, VerifyTaskArtifactAccess(access, "task-video", TaskVideoArtifactKey))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-video", "image"))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-other", TaskVideoArtifactKey))
}

func TestBuildTaskVideoContentURLRejectsUnusableTaskIDs(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	common.CryptoSecret = "task-video-url-test-secret"
	system_setting.TaskPublicAddress = "https://gateway.example"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
	})

	for _, taskID := range []string{"", "   "} {
		_, err := BuildTaskVideoContentURL(taskID)
		assert.ErrorIs(t, err, ErrTaskArtifactAccessInvalid, taskID)
	}
	_, err := BuildTaskVideoContentURL(strings.Repeat("t", maxTaskArtifactTaskIDLength+1))
	assert.ErrorIs(t, err, ErrTaskArtifactAccessInvalid)

	// 密钥缺失时一律不可签发，调用方回退到 ResultURL。
	common.CryptoSecret = ""
	_, err = BuildTaskVideoContentURL("task-video")
	assert.ErrorIs(t, err, ErrTaskArtifactAccessInvalid)
}

func TestBuildTaskVideoContentURLIsNotBoundToArtifactRoute(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	common.CryptoSecret = "task-video-url-test-secret"
	system_setting.TaskPublicAddress = "https://gateway.example"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
	})

	videoURL, err := BuildTaskVideoContentURL("task-shared")
	require.NoError(t, err)
	artifactURL, err := BuildTaskArtifactContentURL("task-shared", TaskVideoArtifactKey)
	require.NoError(t, err)

	// 同一个 (task, "video") 绑定在两条路由上签名一致：验签只看 task+key，
	// 路由形状由各自的 handler 负责，互不越权到别的 artifact key。
	videoParsed, err := url.Parse(videoURL)
	require.NoError(t, err)
	artifactParsed, err := url.Parse(artifactURL)
	require.NoError(t, err)
	assert.NotEqual(t, videoParsed.Path, artifactParsed.Path)
	assert.Equal(
		t,
		videoParsed.Query().Get(TaskArtifactAccessQueryParameter),
		artifactParsed.Query().Get(TaskArtifactAccessQueryParameter),
	)
}

func TestValidateTaskArtifactBaseURLOnlyAcceptsSafeAbsoluteHTTPURLs(t *testing.T) {
	for _, valid := range []string{
		"http://localhost:3000",
		"https://gateway.example",
		"https://gateway.example/prefix/path/",
	} {
		assert.NoError(t, ValidateTaskArtifactBaseURL(valid), valid)
	}
	for _, invalid := range []string{
		"",
		"/relative",
		"ftp://gateway.example",
		"https://user:secret@gateway.example",
		"https://gateway.example/path?tenant=1",
		"https://gateway.example/path#fragment",
		" https://gateway.example",
		"https://gateway.example ",
	} {
		assert.Error(t, ValidateTaskArtifactBaseURL(invalid), invalid)
	}
}
