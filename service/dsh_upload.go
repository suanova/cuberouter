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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	// DshUploadMaxBytes 限制单个附件大小。智能体要把它读进上下文，太大既拖慢
	// 一轮任务也容易把工作目录撑爆。
	DshUploadMaxBytes = 25 << 20
	// DshUploadMaxFiles 是一轮对话里附件的上限，前端与插件两侧都按它裁剪。
	DshUploadMaxFiles = 10
	// dshOrganizationUploadsDirName 与 docker-compose.dsh-org.yml 中
	// DSH_UPLOAD_MOUNT=/data/uploads 对应：组织数据卷的 uploads 子目录。
	dshOrganizationUploadsDirName = "uploads"
)

// dshOrganizationDirName 返回组织数据卷名，必须与
// docker-compose.dsh-org.yml 的 ${DSH_DATA_ROOT}/org-${ORG_ID}:/data 一致。
func dshOrganizationDirName(organizationId int) string {
	return "org-" + strconv.Itoa(organizationId)
}

// dshUploadTypes 是允许上传的扩展名白名单。这里不信任客户端的 Content-Type，
// 扩展名只决定「让不让存」，不决定怎么执行。
var dshUploadTypes = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
	".gif":  true,
	".bmp":  true,
	".heic": true,
	".heif": true,
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".xls":  true,
	".xlsx": true,
	".ppt":  true,
	".pptx": true,
	".csv":  true,
	".txt":  true,
	".md":   true,
	".json": true,
	".yaml": true,
	".yml":  true,
	".xml":  true,
	".html": true,
	".zip":  true,
}

// DshAttachment 是一个已落盘的附件。Path 是 harness 容器里能读到的绝对路径，
// 网关只负责把文件放进共享数据卷，再由插件把清单写进任务提示词。
type DshAttachment struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size"`
}

// DshUploadConfig 描述上传文件的落盘位置。每个组织一个 DSH 容器，网关按
// 组织分目录落盘：<Root>/org-<id>/uploads 正是该组织容器的 /data 卷里
// 的 uploads 目录，容器内看到的仍是 <Mount>/<userId>/<file>。
type DshUploadConfig struct {
	// Dir 是网关进程可见的写入目录（组织配置里已是 <Root>/org-<id>/uploads）。
	Dir string
	// Mount 是该目录在 dsh 容器中的挂载路径前缀。
	Mount string
}

// LoadDshUploadConfig 读取共享卷根目录配置。两个变量必须同时设置，避免返回
// 一个智能体读不到的路径。
func LoadDshUploadConfig() (DshUploadConfig, error) {
	root := strings.TrimSpace(os.Getenv("DSH_UPLOAD_ROOT"))
	mount := strings.TrimSpace(os.Getenv("DSH_UPLOAD_MOUNT"))
	if root == "" || mount == "" {
		return DshUploadConfig{}, errors.New("DSH task uploads are not configured")
	}
	if !filepath.IsAbs(root) {
		return DshUploadConfig{}, errors.New("DSH_UPLOAD_ROOT must be an absolute path")
	}
	if !strings.HasPrefix(mount, "/") {
		return DshUploadConfig{}, errors.New("DSH_UPLOAD_MOUNT must be an absolute path")
	}
	return DshUploadConfig{Dir: filepath.Clean(root), Mount: path.Clean(mount)}, nil
}

// LoadDshOrganizationUploadConfig 返回某个组织的上传目录。目录布局与
// docker-compose.dsh-org.yml 的卷挂载一致：<Root>/org-<id> 挂到容器的 /data。
func LoadDshOrganizationUploadConfig(organizationId int) (DshUploadConfig, error) {
	if organizationId <= 0 {
		return DshUploadConfig{}, errors.New("invalid organization")
	}
	root, err := LoadDshUploadConfig()
	if err != nil {
		return DshUploadConfig{}, err
	}
	return DshUploadConfig{
		Dir:   filepath.Join(root.Dir, dshOrganizationDirName(organizationId), dshOrganizationUploadsDirName),
		Mount: root.Mount,
	}, nil
}

func DshUploadAccept() []string {
	accept := make([]string, 0, len(dshUploadTypes))
	for extension := range dshUploadTypes {
		accept = append(accept, extension)
	}
	sort.Strings(accept)
	return accept
}

// SaveDshAttachment 把上传文件写进所属组织的上传子目录，并返回 harness 侧
// 路径。文件名只保留可打印字符，落盘名再挂一个随机前缀，避免同名覆盖与
// 路径穿越。
func SaveDshAttachment(organizationId, userId int, header *multipart.FileHeader) (DshAttachment, error) {
	if userId <= 0 {
		return DshAttachment{}, errors.New("invalid user")
	}
	if header.Size > DshUploadMaxBytes {
		return DshAttachment{}, fmt.Errorf("files can be at most %d MB", DshUploadMaxBytes>>20)
	}
	name, err := sanitizeDshUploadName(header.Filename)
	if err != nil {
		return DshAttachment{}, err
	}
	extension := strings.ToLower(filepath.Ext(name))
	if !dshUploadTypes[extension] {
		return DshAttachment{}, fmt.Errorf("unsupported file type: %s", extension)
	}
	cfg, err := LoadDshOrganizationUploadConfig(organizationId)
	if err != nil {
		return DshAttachment{}, err
	}
	userDir := filepath.Join(cfg.Dir, strconv.Itoa(userId))
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		return DshAttachment{}, fmt.Errorf("could not prepare the upload directory")
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return DshAttachment{}, fmt.Errorf("could not allocate an upload id")
	}
	storedName := hex.EncodeToString(random) + "-" + name
	storedPath := filepath.Join(userDir, storedName)

	source, err := header.Open()
	if err != nil {
		return DshAttachment{}, fmt.Errorf("could not read the uploaded file")
	}
	defer source.Close()
	target, err := os.OpenFile(storedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return DshAttachment{}, fmt.Errorf("could not store the uploaded file")
	}
	written, copyErr := io.Copy(target, io.LimitReader(source, DshUploadMaxBytes+1))
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(storedPath)
		return DshAttachment{}, fmt.Errorf("could not store the uploaded file")
	}
	// multipart 头里的 Size 可以伪造，写盘时再按实际字节数兜一次底。
	if written > DshUploadMaxBytes {
		_ = os.Remove(storedPath)
		return DshAttachment{}, fmt.Errorf("files can be at most %d MB", DshUploadMaxBytes>>20)
	}

	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = mime.TypeByExtension(extension)
	}
	return DshAttachment{
		Name:     name,
		Path:     path.Join(cfg.Mount, strconv.Itoa(userId), storedName),
		MimeType: mimeType,
		Size:     written,
	}, nil
}

// ValidateDshAttachments 校验随任务提交的附件清单。附件只能先经网关上传接口
// 落进「本组织 + 本用户」的目录，因此这里只接受落在该目录里的文件：其它路径
// （包括其它组织同用户目录与 /etc/passwd）在本地不存在，自然被拒。
func ValidateDshAttachments(organizationId, userId int, attachments []relaycommon.TaskAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	if userId <= 0 {
		return errors.New("invalid user")
	}
	if len(attachments) > DshUploadMaxFiles {
		return fmt.Errorf("at most %d files can be attached", DshUploadMaxFiles)
	}
	cfg, err := LoadDshOrganizationUploadConfig(organizationId)
	if err != nil {
		return err
	}
	userPrefix := path.Join(cfg.Mount, strconv.Itoa(userId)) + "/"
	for _, attachment := range attachments {
		raw := strings.TrimSpace(attachment.Path)
		if raw == "" {
			return errors.New("each attachment needs a path")
		}
		// Clean 会压掉 ".." 和重复分隔符，路径穿越在这里就被挡下来。
		cleaned := path.Clean(raw)
		if !strings.HasPrefix(cleaned, userPrefix) {
			return errors.New("attachment paths must come from this user's uploads")
		}
		local := filepath.Join(cfg.Dir, strings.TrimPrefix(cleaned, cfg.Mount))
		stat, statErr := os.Lstat(local)
		if statErr != nil || !stat.Mode().IsRegular() {
			return errors.New("attachment file is missing")
		}
	}
	return nil
}

func sanitizeDshUploadName(raw string) (string, error) {
	// Windows 客户端会把反斜杠当分隔符；先统一再取 basename。
	name := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")
	if name == "" {
		return "", errors.New("the uploaded file needs a name")
	}
	if len(name) > 180 {
		// 保留扩展名，中间部分截断，既不丢类型也不会撞文件系统上限。
		// 按 rune 边界截断：中文名按字节切会留下半个字符。
		extension := filepath.Ext(name)
		budget := 180 - len(extension)
		if budget < 1 {
			budget = 1
		}
		stem := strings.TrimSuffix(name, extension)
		var builder strings.Builder
		used := 0
		for _, char := range stem {
			size := utf8.RuneLen(char)
			if used+size > budget {
				break
			}
			builder.WriteRune(char)
			used += size
		}
		name = builder.String() + extension
	}
	return name, nil
}
