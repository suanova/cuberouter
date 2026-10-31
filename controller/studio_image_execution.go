/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/studioqueue"
	"github.com/gin-gonic/gin"
)

func reserveStudioImage(r studioqueue.Request, job string) *studioqueue.Reservation {
	user, err := model.GetUserById(r.Owner, false)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled || !service.StudioImageGroupAllowed(user.Group) {
		return &studioqueue.Reservation{Run: func(context.Context, <-chan struct{}) studioqueue.Result {
			return studioqueue.Result{Error: "studio_access_denied"}
		}, Release: func() {}}
	}
	var p studioImageSubmission
	if common.Unmarshal(r.Body, &p) != nil {
		return nil
	}
	upstream := p.Request
	upstream.Mask = "" // Region preservation is owned by CubeRouter's reviewed compositor.
	body, err := common.Marshal(upstream)
	if err != nil {
		return nil
	}
	buffer := &studioImageBuffer{headers: make(http.Header), code: http.StatusOK}
	req, err := http.NewRequest(http.MethodPost, "http://studio.internal/pg/images/"+map[bool]string{true: "edits", false: "generations"}[p.Mode == "edit"], bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	c := &gin.Context{Request: req, Writer: buffer}
	c.Set("id", user.Id)
	c.Set("role", user.Role)
	c.Set("group", user.Group)
	c.Set(common.RequestIdKey, job)
	user.ToBaseUser().WriteContext(c)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, user.Group)
	middleware.PlaygroundOrganizationFallback()(c)
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if allow := model.GetUserModelAllowList(user.Id); allow != nil && !model.IsModelAllowedByMemberAccess(allow, r.Model) {
		return &studioqueue.Reservation{Run: func(context.Context, <-chan struct{}) studioqueue.Result {
			return studioqueue.Result{Error: "studio_model_denied"}
		}, Release: func() {}}
	}
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	accountResource := fmt.Sprintf("account:%d", r.Owner)
	accountAcquired, err := model.AcquireStudioImageLease(accountResource, job, time.Now(), 15*time.Minute)
	if err != nil || !accountAcquired {
		return nil
	}
	for _, channel := range service.ReadyStudioImageChannels(ctx, group, r.Model) {
		resource := fmt.Sprintf("gpu:%d", channel.Id)
		acquired, err := model.AcquireStudioImageLease(resource, job, time.Now(), 15*time.Minute)
		if err != nil || !acquired {
			continue
		}
		execution := &studioImageExecution{request: r, submission: p, id: job, channel: channel, relayContext: c, buffer: buffer, safeToRelease: true}
		return &studioqueue.Reservation{Run: execution.run, Release: execution.release}
	}
	_ = model.ReleaseStudioImageLease(accountResource, job)
	return nil
}

type studioImageExecution struct {
	request       studioqueue.Request
	submission    studioImageSubmission
	id            string
	channel       *model.Channel
	relayContext  *gin.Context
	buffer        *studioImageBuffer
	safeToRelease bool
	png           []byte
	gateError     error
}

func (execution *studioImageExecution) release() {
	if execution.safeToRelease {
		_ = model.ReleaseStudioImageLease(fmt.Sprintf("gpu:%d", execution.channel.Id), execution.id)
		_ = model.ReleaseStudioImageLease(fmt.Sprintf("account:%d", execution.request.Owner), execution.id)
	}
}

func (execution *studioImageExecution) run(execCtx context.Context, cancel <-chan struct{}) (out studioqueue.Result) {
	c, r, p, job, channel, buffer := execution.relayContext, execution.request, execution.submission, execution.id, execution.channel, execution.buffer
	defer func() {
		select {
		case <-cancel:
			_ = model.UpdateStudioImageAudit(job, map[string]any{"state": "cancelled"})
		default:
			if out.Error != "" {
				_ = model.UpdateStudioImageAudit(job, map[string]any{"state": "failed", "failure": out.Error})
			}
		}
	}()
	defer common.CleanupBodyStorage(c)
	defer service.CleanupFileSources(c)
	c.Request = c.Request.WithContext(execCtx)
	c.Set(service.StudioExecutionContext, job)
	row, err := studioImageAudit(r, job)
	if err != nil || model.RecordStudioImageAudit(row) != nil {
		return studioqueue.Result{Error: "studio_audit_unavailable"}
	}
	if model.UpdateStudioImageAudit(job, map[string]any{"channel_id": channel.Id, "state": "running"}) != nil {
		return studioqueue.Result{Error: "studio_audit_unavailable"}
	}

	if p.Request.Mask != "" {
		var first string
		if common.Unmarshal(p.Request.Image, &first) != nil {
			var refs []string
			if common.Unmarshal(p.Request.Image, &refs) != nil || len(refs) == 0 {
				return studioqueue.Result{Error: "studio_region_invalid"}
			}
			first = refs[0]
		}
		_, input, _ := strings.Cut(first, ",")
		_, mask, _ := strings.Cut(p.Request.Mask, ",")
		reference, err := base64.StdEncoding.Strict().DecodeString(input)
		if err != nil {
			return studioqueue.Result{Error: "studio_region_invalid"}
		}
		selection, err := base64.StdEncoding.Strict().DecodeString(mask)
		if err != nil {
			return studioqueue.Result{Error: "studio_region_invalid"}
		}
		c.Set("studio_image_transform", func(candidate []byte) ([]byte, error) {
			return service.CompositeStudioImage(candidate, reference, selection)
		})
	}
	c.Set(service.StudioReviewContext, execution.review)
	service.GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: channel.Id, Source: dto.ChannelPinSource("studio"), Rank: 20, RetryMode: dto.PinRetrySingleAttempt})
	// The pool checked current DB abilities and member restrictions. Apply the
	// same native relay setup to that exact fresh channel, avoiding a cached
	// key/override snapshot that could disagree with its health probe or audit.
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	if middleware.SetupContextForSelectedChannel(c, channel, r.Model) != nil {
		return studioqueue.Result{Error: "studio_channel_denied"}
	}
	select {
	case <-cancel:
		return studioqueue.Result{}
	default:
	}
	watchCtx, stopWatch := context.WithCancel(execCtx)
	defer stopWatch()
	go cancelStudioImageWorker(watchCtx, channel, job, cancel)
	execution.safeToRelease = false
	PlaygroundImage(c)
	// A transport/5xx failure cannot prove GPU completion. Retain the
	// physical lease for quarantine, rather than retrying or overlapping.
	execution.safeToRelease = buffer.code < 500 && execCtx.Err() == nil
	if execution.gateError != nil {
		return studioqueue.Result{Error: execution.gateError.Error()}
	}
	if buffer.code == http.StatusUnprocessableEntity {
		return studioqueue.Result{Error: "output_check_unavailable"}
	}
	if buffer.code != http.StatusOK || buffer.overflow || len(execution.png) == 0 {
		_ = model.UpdateStudioImageAudit(job, map[string]any{"state": "failed"})
		return studioqueue.Result{Error: "studio_generation_failed"}
	}
	select {
	case <-cancel:
		_ = model.UpdateStudioImageAudit(job, map[string]any{"state": "cancelled"})
		return studioqueue.Result{}
	default:
		return studioqueue.Result{PNG: execution.png}
	}
}

func (execution *studioImageExecution) review(ctx context.Context, image []byte) error {
	job := execution.id
	report, err := service.ReviewStudioPNG(ctx, job, image)
	encoded, marshalErr := common.Marshal(report)
	state := "completed"
	if err != nil {
		state = "failed"
	}
	if marshalErr != nil || model.UpdateStudioImageAudit(job, map[string]any{
		"output_hash": fmt.Sprintf("%x", sha256.Sum256(image)), "review": string(encoded), "state": state,
	}) != nil {
		err = errors.New("studio_audit_unavailable")
	}
	execution.gateError = err
	if err == nil {
		execution.png = append([]byte(nil), image...)
	}
	return err
}

func cancelStudioImageWorker(ctx context.Context, channel *model.Channel, job string, cancel <-chan struct{}) {
	select {
	case <-ctx.Done():
		return
	case <-cancel:
	}
	body, err := common.Marshal(map[string]string{"execution_id": job})
	if err != nil {
		return
	}
	endpoint := strings.TrimRight(channel.GetBaseURL(), "/") + "/internal/cancel"
	client, err := service.StudioPrivateClient(endpoint, "", 3*time.Second)
	if err != nil {
		return
	}
	defer client.CloseIdleConnections()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+channel.Key)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 1025))
			response.Body.Close()
			var ack struct {
				Protocol    string `json:"protocol"`
				ExecutionID string `json:"execution_id"`
				State       string `json:"state"`
			}
			if response.StatusCode == http.StatusOK && readErr == nil && len(data) <= 1024 &&
				common.Unmarshal(data, &ack) == nil && ack.Protocol == "qwen-execution-v1" && ack.ExecutionID == job && (ack.State == "cancelling" || ack.State == "fenced") {
				return
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// The private relay has no socket to the browser and has a strict byte bound.
type studioImageBuffer struct {
	headers  http.Header
	body     bytes.Buffer
	code     int
	written  bool
	overflow bool
}

func (w *studioImageBuffer) Header() http.Header { return w.headers }
func (w *studioImageBuffer) WriteHeader(code int) {
	if !w.written {
		w.code = code
	}
}
func (w *studioImageBuffer) WriteHeaderNow() { w.written = true }
func (w *studioImageBuffer) Write(data []byte) (int, error) {
	w.written = true
	if w.body.Len()+len(data) > 32<<20 {
		w.overflow = true
		return 0, errors.New("studio_result_too_large")
	}
	return w.body.Write(data)
}
func (w *studioImageBuffer) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *studioImageBuffer) Status() int                       { return w.code }
func (w *studioImageBuffer) Size() int                         { return w.body.Len() }
func (w *studioImageBuffer) Written() bool                     { return w.written }
func (w *studioImageBuffer) Flush()                            { w.WriteHeaderNow() }
func (w *studioImageBuffer) CloseNotify() <-chan bool          { return make(chan bool) }
func (w *studioImageBuffer) Pusher() http.Pusher               { return nil }
func (w *studioImageBuffer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("studio images do not support socket hijacking")
}
