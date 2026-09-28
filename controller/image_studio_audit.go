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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func studioAudit(job string, user int, p studioParameters, state string) error {
	if !model.ImageStudioAuditEnabled() {
		return nil
	}
	account, err := model.GetUserCache(user)
	if err != nil {
		return fmt.Errorf("audit_account_unavailable")
	}
	hashes := make([]string, 0, len(p.Images))
	for _, image := range p.Images {
		decoded, err := base64.StdEncoding.Strict().DecodeString(image)
		if err != nil {
			return fmt.Errorf("invalid_image")
		}
		hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256(decoded)))
	}
	params, _ := common.Marshal(map[string]any{"mode": p.Mode, "width": p.Width, "height": p.Height, "steps": p.Steps, "cfg": p.CFG, "seed": p.Seed})
	inputHashes, _ := common.Marshal(hashes)
	return model.CreateImageStudioAudit(&model.ImageStudioAudit{
		JobID: job, UserID: user, Username: account.Username, Model: p.Model, State: state,
		UserPrompt: p.UserPrompt, ModelPrompt: p.Prompt, Parameters: string(params), InputHashes: string(inputHashes),
	})
}

func ImageStudioAuditLookup(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	job, hash := c.Query("job_id"), c.Query("output_sha256")
	user, err := strconv.Atoi(c.DefaultQuery("user_id", "0"))
	if err != nil || user < 0 || (job != "" && !studioJobID.MatchString(job)) ||
		(hash != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_audit_filter"})
		return
	}
	rows, err := model.FindImageStudioAudits(job, hash, user)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "audit_unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows, "retention_days": 30})
}
