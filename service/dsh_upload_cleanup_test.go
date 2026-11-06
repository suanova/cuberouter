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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDshUploadFile(t *testing.T, dir, name string, modTime time.Time) string {
	t.Helper()
	filePath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(filePath, modTime, modTime))
	return filePath
}

func TestCleanupDshUploadsRemovesExpiredAndExcessFiles(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "1")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	otherUserDir := filepath.Join(dir, "2")
	require.NoError(t, os.MkdirAll(otherUserDir, 0o700))
	notesDir := filepath.Join(dir, "notes")
	require.NoError(t, os.MkdirAll(notesDir, 0o700))

	now := time.Now()
	expired := writeDshUploadFile(t, userDir, "expired.pdf", now.Add(-10*24*time.Hour))
	secondNewest := writeDshUploadFile(t, userDir, "second.pdf", now.Add(-2*24*time.Hour))
	newer := writeDshUploadFile(t, userDir, "newer.pdf", now.Add(-time.Hour))
	newest := writeDshUploadFile(t, userDir, "newest.pdf", now)
	otherUser := writeDshUploadFile(t, otherUserDir, "keep.pdf", now.Add(-time.Hour))
	note := writeDshUploadFile(t, notesDir, "keep.txt", now.Add(-30*24*time.Hour))

	removed, err := CleanupDshUploads(dir, DshUploadRetention{Age: 7 * 24 * time.Hour, MaxFiles: 2}, now)
	require.NoError(t, err)
	// 一个按时间过期，一个按数量超出。
	assert.Equal(t, 2, removed)

	assert.NoFileExists(t, expired)
	assert.NoFileExists(t, secondNewest)
	assert.FileExists(t, newer)
	assert.FileExists(t, newest)
	// 其它用户目录和非数字目录不参与清理。
	assert.FileExists(t, otherUser)
	assert.FileExists(t, note)
}

func TestCleanupDshUploadsKeepsFilesWhenRulesAreDisabled(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "1")
	require.NoError(t, os.MkdirAll(userDir, 0o700))
	old := writeDshUploadFile(t, userDir, "old.pdf", time.Now().Add(-365*24*time.Hour))

	removed, err := CleanupDshUploads(dir, DshUploadRetention{}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
	assert.FileExists(t, old)
}

func TestCleanupDshUploadsIgnoresAMissingDirectory(t *testing.T) {
	removed, err := CleanupDshUploads(filepath.Join(t.TempDir(), "absent"), DshUploadRetention{Age: time.Hour}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}
