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
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// dshUploadCleanupInterval 是扫描上传目录的周期。附件只在任务读取时需要，
	// 小时级延迟足够，也不会和上传写入抢占磁盘。
	dshUploadCleanupInterval = time.Hour
	// 默认保留 7 天，每个用户最多 200 个文件；两个值都可以用环境变量覆盖，
	// 显式设为 0 表示关闭对应的清理规则。
	dshUploadRetentionDaysDefault   = 7
	dshUploadMaxFilesPerUserDefault = 200
)

// DshUploadRetention 描述上传目录的清理策略。
type DshUploadRetention struct {
	// Age 是文件保留时长，<= 0 表示不按时间清理。
	Age time.Duration
	// MaxFiles 是每个用户目录保留的文件数，<= 0 表示不按数量清理。
	MaxFiles int
}

// LoadDshUploadRetention 读取清理策略，缺省值与上传上限保持一致，避免把还在
// 会话里的附件提前删掉。
func LoadDshUploadRetention() DshUploadRetention {
	days := common.GetEnvOrDefault("DSH_UPLOAD_RETENTION_DAYS", dshUploadRetentionDaysDefault)
	retention := DshUploadRetention{
		MaxFiles: common.GetEnvOrDefault("DSH_UPLOAD_MAX_FILES_PER_USER", dshUploadMaxFilesPerUserDefault),
	}
	if days > 0 {
		retention.Age = time.Duration(days) * 24 * time.Hour
	}
	return retention
}

// StartDshUploadCleanup 定时清理上传目录。上传目录是网关侧的磁盘，不清就会
// 一直涨；只在主节点跑，避免多节点重复删除。
func StartDshUploadCleanup() {
	if !common.IsMasterNode {
		return
	}
	cfg, err := LoadDshUploadConfig()
	if err != nil {
		// 没有配置共享卷就没有上传目录，无需清理。
		return
	}
	retention := LoadDshUploadRetention()
	go func() {
		runDshUploadCleanup(cfg.Dir, retention, time.Now())
		ticker := time.NewTicker(dshUploadCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			runDshUploadCleanup(cfg.Dir, retention, time.Now())
		}
	}()
}

func runDshUploadCleanup(dir string, retention DshUploadRetention, now time.Time) {
	removed, err := CleanupDshUploads(dir, retention, now)
	if err != nil {
		common.SysError("failed to clean up dsh uploads: " + err.Error())
		return
	}
	if removed > 0 {
		common.SysLog(fmt.Sprintf("dsh upload cleanup removed %d files", removed))
	}
}

// CleanupDshUploads 从共享卷根目录扫描 org-<id>/uploads/<userId>/，只处理
// 以用户 id 命名的子目录，其它内容（含组织容器的其它数据）原样留下。
func CleanupDshUploads(root string, retention DshUploadRetention, now time.Time) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var cutoff time.Time
	if retention.Age > 0 {
		cutoff = now.Add(-retention.Age)
	}
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !isDshOrganizationDir(entry.Name()) {
			continue
		}
		uploadsDir := filepath.Join(root, entry.Name(), dshOrganizationUploadsDirName)
		userDirs, err := os.ReadDir(uploadsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		for _, userDir := range userDirs {
			if !userDir.IsDir() || !isDshUploadUserDir(userDir.Name()) {
				continue
			}
			count, err := cleanupDshUploadUserDir(filepath.Join(uploadsDir, userDir.Name()), retention.MaxFiles, cutoff)
			removed += count
			if err != nil {
				return removed, err
			}
		}
	}
	return removed, nil
}

func cleanupDshUploadUserDir(userDir string, maxFiles int, cutoff time.Time) (int, error) {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	kept := make([]dshUploadFile, 0, len(entries))
	removed := 0
	for _, entry := range entries {
		// 子目录与符号链接都不属于上传产物，留给人工处理。
		if !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		if !cutoff.IsZero() && info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(userDir, entry.Name())); err != nil && !os.IsNotExist(err) {
				return removed, err
			}
			removed++
			continue
		}
		kept = append(kept, dshUploadFile{name: entry.Name(), modTime: info.ModTime()})
	}
	if maxFiles <= 0 || len(kept) <= maxFiles {
		return removed, nil
	}
	// 超出上限时先删最旧的，刚上传的文件不会在这一轮被清掉。
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].modTime.After(kept[j].modTime)
	})
	for _, file := range kept[maxFiles:] {
		if err := os.Remove(filepath.Join(userDir, file.name)); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

type dshUploadFile struct {
	name    string
	modTime time.Time
}

func isDshUploadUserDir(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// isDshOrganizationDir 只匹配 org-<正整数>，避免把共享卷里的其它目录当成
// 上传根去扫描。
func isDshOrganizationDir(name string) bool {
	if !strings.HasPrefix(name, "org-") {
		return false
	}
	return isDshUploadUserDir(strings.TrimPrefix(name, "org-"))
}
