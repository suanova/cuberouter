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
	"bytes"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func dshUploadHeader(t *testing.T, filename, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="` + filename + `"`},
		"Content-Type":        {contentType},
	})
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(int64(len(content)) + 1024)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Len(t, form.File["file"], 1)
	return form.File["file"][0]
}

func TestSaveDshAttachmentWritesFilesUnderTheUserDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DSH_UPLOAD_DIR", dir)
	t.Setenv("DSH_UPLOAD_MOUNT", "/data/uploads/")

	attachment, err := SaveDshAttachment(7, dshUploadHeader(t, "../../季度 报告.pdf", "application/pdf", []byte("%PDF-1.7")))
	require.NoError(t, err)

	// 原始名字里的路径穿越被压成纯文件名，落盘名再挂随机前缀。
	assert.Equal(t, "季度 报告.pdf", attachment.Name)
	assert.Equal(t, int64(8), attachment.Size)
	assert.Equal(t, "application/pdf", attachment.MimeType)
	assert.True(t, strings.HasPrefix(attachment.Path, "/data/uploads/7/"))

	storedName := strings.TrimPrefix(attachment.Path, "/data/uploads/7/")
	assert.NotContains(t, storedName, "/")
	assert.True(t, strings.HasSuffix(storedName, "-季度 报告.pdf"))
	content, err := os.ReadFile(filepath.Join(dir, "7", storedName))
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.7", string(content))
}

func TestSaveDshAttachmentTruncatesLongNamesOnRuneBoundaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DSH_UPLOAD_DIR", dir)
	t.Setenv("DSH_UPLOAD_MOUNT", "/data/uploads")

	name := strings.Repeat("季度", 60) + ".pdf"
	attachment, err := SaveDshAttachment(7, dshUploadHeader(t, name, "application/pdf", []byte("pdf")))
	require.NoError(t, err)

	// 超长文件名按 rune 边界截断，保留扩展名且不产生非法 UTF-8。
	assert.LessOrEqual(t, len(attachment.Name), 180)
	assert.True(t, utf8.ValidString(attachment.Name))
	assert.True(t, strings.HasPrefix(attachment.Name, "季度"))
	assert.True(t, strings.HasSuffix(attachment.Name, ".pdf"))
}

func TestValidateDshAttachmentsAcceptsOnlyTheUsersOwnUploads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DSH_UPLOAD_DIR", dir)
	t.Setenv("DSH_UPLOAD_MOUNT", "/data/uploads")

	attachment, err := SaveDshAttachment(7, dshUploadHeader(t, "report.pdf", "application/pdf", []byte("%PDF")))
	require.NoError(t, err)

	t.Run("own upload", func(t *testing.T) {
		require.NoError(t, ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Name: "report.pdf", Path: attachment.Path}}))
	})

	t.Run("no attachments", func(t *testing.T) {
		require.NoError(t, ValidateDshAttachments(7, nil))
	})

	t.Run("another users upload", func(t *testing.T) {
		err := ValidateDshAttachments(8, []relaycommon.TaskAttachment{{Path: attachment.Path}})
		require.ErrorContains(t, err, "must come from this user's uploads")
	})

	t.Run("path traversal", func(t *testing.T) {
		err := ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Path: "/data/uploads/7/../../etc/passwd"}})
		require.ErrorContains(t, err, "must come from this user's uploads")
	})

	t.Run("empty path", func(t *testing.T) {
		err := ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Name: "report.pdf"}})
		require.ErrorContains(t, err, "needs a path")
	})

	t.Run("missing file", func(t *testing.T) {
		err := ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Path: "/data/uploads/7/deadbeef-nothing.pdf"}})
		require.ErrorContains(t, err, "missing")
	})

	t.Run("symlink in the user directory", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "outside.txt")
		require.NoError(t, os.WriteFile(target, []byte("secret"), 0o600))
		link := filepath.Join(dir, "7", "link.txt")
		require.NoError(t, os.Symlink(target, link))
		err := ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Path: "/data/uploads/7/link.txt"}})
		require.ErrorContains(t, err, "missing")
	})

	t.Run("too many attachments", func(t *testing.T) {
		attachments := make([]relaycommon.TaskAttachment, DshUploadMaxFiles+1)
		for index := range attachments {
			attachments[index] = relaycommon.TaskAttachment{Path: attachment.Path}
		}
		err := ValidateDshAttachments(7, attachments)
		require.ErrorContains(t, err, "at most 10 files")
	})

	t.Run("uploads not configured", func(t *testing.T) {
		t.Setenv("DSH_UPLOAD_DIR", "")
		err := ValidateDshAttachments(7, []relaycommon.TaskAttachment{{Path: attachment.Path}})
		require.ErrorContains(t, err, "not configured")
	})
}

func TestSaveDshAttachmentRejectsUnsupportedInputs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DSH_UPLOAD_DIR", dir)
	t.Setenv("DSH_UPLOAD_MOUNT", "/data/uploads")

	cases := []struct {
		name        string
		filename    string
		contentType string
		content     []byte
		message     string
	}{
		{
			name:     "unsupported extension",
			filename: "payload.exe",
			content:  []byte("MZ"),
			message:  "unsupported file type",
		},
		{
			name:     "missing file name",
			filename: ".",
			content:  []byte("x"),
			message:  "needs a name",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := SaveDshAttachment(7, dshUploadHeader(t, testCase.filename, testCase.contentType, testCase.content))
			require.ErrorContains(t, err, testCase.message)
		})
	}

	t.Run("oversized file", func(t *testing.T) {
		header := dshUploadHeader(t, "big.pdf", "application/pdf", []byte("x"))
		header.Size = DshUploadMaxBytes + 1
		_, err := SaveDshAttachment(7, header)
		require.ErrorContains(t, err, "at most 25 MB")
	})

	t.Run("missing shared volume", func(t *testing.T) {
		t.Setenv("DSH_UPLOAD_DIR", "")
		_, err := SaveDshAttachment(7, dshUploadHeader(t, "a.png", "image/png", []byte("png")))
		require.ErrorContains(t, err, "not configured")
	})
}
