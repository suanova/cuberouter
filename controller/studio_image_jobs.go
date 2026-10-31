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
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/studioqueue"
	"github.com/gin-gonic/gin"
)

var studioQueueOnce sync.Once
var studioQueue *studioqueue.Queue
var studioMaintenanceOnce sync.Once

func StartStudioImageMaintenance() {
	studioMaintenanceOnce.Do(func() {
		go func() {
			if model.PruneStudioImageAudits(time.Now()) != nil {
				common.SysError("Image Studio audit retention failed")
			}
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for now := range ticker.C {
				if model.PruneStudioImageAudits(now) != nil {
					common.SysError("Image Studio audit retention failed")
				}
			}
		}()
	})
}

func imageJobs() *studioqueue.Queue {
	studioQueueOnce.Do(func() {
		studioQueue = studioqueue.New(reserveStudioImage, func(status studioqueue.Status) {
			// The output gate already committed approval. A delayed completion
			// notification must not overwrite a subsequent cancellation.
			if status.State == "completed" {
				return
			}
			if model.UpdateStudioImageAudit(status.ID, map[string]any{"state": status.State, "failure": status.Error}) != nil {
				common.SysError("Image Studio lifecycle audit failed")
			}
		})
		go studioQueue.Run(context.Background())
	})
	return studioQueue
}

func studioJobError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": code}})
}

// Run after UserAuth. Group membership is checked in the main DB for every
// admission and again immediately before execution, never from browser input.
func StudioImageAccess(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !service.StudioImageEnabled() {
		studioJobError(c, 503, "studio_not_configured")
		return
	}
	if c.GetBool("use_access_token") {
		studioJobError(c, 403, "studio_session_required")
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		studioJobError(c, 503, "studio_access_unavailable")
		return
	}
	if user == nil || user.Status != common.UserStatusEnabled || !service.StudioImageGroupAllowed(user.Group) {
		studioJobError(c, 403, "studio_access_denied")
		return
	}
	c.Next()
}

func StudioImageSession(c *gin.Context) {
	q := imageJobs()
	c.JSON(http.StatusOK, gin.H{"epoch": q.Epoch(), "jobs": q.Session(c.GetInt("id"))})
}

func SubmitStudioImageJob(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, studioqueue.MaxRequestBytes))
	var p studioImageSubmission
	if err != nil || common.Unmarshal(body, &p) != nil {
		studioJobError(c, 400, "invalid_studio_request")
		return
	}
	if _, err := validateStudioImageSubmission(&p); err != nil {
		studioJobError(c, 400, err.Error())
		return
	}
	body, err = common.Marshal(p)
	if err != nil {
		studioJobError(c, 400, "invalid_studio_request")
		return
	}
	q := imageJobs()
	r := studioqueue.Request{Owner: c.GetInt("id"), Key: c.GetHeader("Idempotency-Key"), Model: p.Request.Model, Mode: p.Mode, Body: body}
	// Idempotent lookups remain available even when worker capacity disappears.
	_, existing := q.Lookup(r.Owner, r.Key)
	if !existing {
		busy, leaseErr := model.StudioImageAccountBusy(r.Owner, time.Now())
		if leaseErr != nil {
			studioJobError(c, 503, "studio_access_unavailable")
			return
		}
		if busy {
			studioJobError(c, 409, "studio_account_busy")
			return
		}
		user, userErr := model.GetUserById(r.Owner, false)
		if userErr != nil || user == nil {
			studioJobError(c, 503, "studio_access_unavailable")
			return
		}
		middleware.PlaygroundOrganizationFallback()(c)
		group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		if group == "" {
			group = user.Group
		}
		if len(service.ReadyStudioImageChannels(c.Request.Context(), group, r.Model)) == 0 {
			studioJobError(c, 503, "studio_capacity_unavailable")
			return
		}
		var reviewers int64
		if model.DB.Model(&model.Channel{}).Where("type = ? AND status = ?", constant.ChannelTypeStudioReview, common.ChannelStatusEnabled).Count(&reviewers).Error != nil || reviewers == 0 {
			studioJobError(c, 503, "output_check_unavailable")
			return
		}
	}
	status, err := q.Submit(r, c.GetHeader("X-Trial-Epoch"), time.Now())
	if err != nil {
		studioJobError(c, 409, err.Error())
		return
	}
	row, err := studioImageAudit(r, status.ID)
	if err != nil || model.RecordStudioImageAudit(row) != nil {
		q.Cancel(r.Owner, status.ID, time.Now())
		studioJobError(c, 503, "studio_audit_unavailable")
		return
	}
	c.JSON(http.StatusAccepted, status)
}

func GetStudioImageJob(c *gin.Context) {
	status, ok := imageJobs().Lookup(c.GetInt("id"), c.Param("job"))
	if !ok {
		studioJobError(c, 404, "studio_job_not_found")
		return
	}
	c.JSON(http.StatusOK, status)
}

func CancelStudioImageJob(c *gin.Context) {
	status, ok := imageJobs().Cancel(c.GetInt("id"), c.Param("job"), time.Now())
	if !ok {
		studioJobError(c, 404, "studio_job_not_found")
		return
	}
	if status.CancelRequested {
		_ = model.UpdateStudioImageAudit(status.ID, map[string]any{"state": status.State})
	}
	c.JSON(http.StatusOK, status)
}

func GetStudioImageResult(c *gin.Context) {
	if status, ok := imageJobs().Lookup(c.GetInt("id"), c.Param("job")); ok && (status.Error == "output_blocked" || status.Error == "output_check_unavailable") {
		studioJobError(c, 422, status.Error)
		return
	}
	image, ok := imageJobs().Result(c.GetInt("id"), c.Param("job"))
	if !ok {
		studioJobError(c, 404, "studio_result_unavailable")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/png", image)
}

func AckStudioImageResult(c *gin.Context) {
	if !imageJobs().Ack(c.GetInt("id"), c.Param("job")) {
		studioJobError(c, 409, "studio_job_active")
		return
	}
	c.Status(http.StatusNoContent)
}

func GetStudioImageAudit(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	user, _ := strconv.Atoi(c.Query("user_id"))
	rows, err := model.ListStudioImageAudit(before, user, c.Query("before_job_id"))
	if err != nil {
		studioJobError(c, 503, "studio_audit_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// A configured Studio model has exactly one browser generation path. Old tabs
// cannot silently bypass the review gate via the former synchronous endpoint.
func RequireStudioImageQueue(c *gin.Context) {
	if service.StudioImageEnabled() {
		var request struct {
			Model string `json:"model"`
		}
		if common.UnmarshalBodyReusable(c, &request) == nil && slices.Contains(service.StudioImageModels(), request.Model) {
			studioJobError(c, 409, "studio_queue_required")
			return
		}
	}
	c.Next()
}
