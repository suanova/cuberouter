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
package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHasModelTagMatchesWholeCommaSeparatedLabels 锁定标签匹配契约：标签是运维
// 的显式声明，必须整项相等——前缀、子串都不算命中。这条规则把标签与刚被删除的
// 模型名字符串匹配（正是它把 qwen-image-edit-* 误判成文生图模型）区分开。
func TestHasModelTagMatchesWholeCommaSeparatedLabels(t *testing.T) {
	tests := []struct {
		name string
		tags string
		tag  string
		want bool
	}{
		{name: "single label", tags: "text-to-image", tag: ModelTagTextToImage, want: true},
		{name: "label among others", tags: "hot,text-to-image,new", tag: ModelTagTextToImage, want: true},
		{name: "case and surrounding blanks ignored", tags: " Text-To-Image ", tag: ModelTagTextToImage, want: true},
		{name: "a longer label does not match", tags: "text-to-imagex", tag: ModelTagTextToImage, want: false},
		{name: "the other label does not match", tags: ModelTagImageToImage, tag: ModelTagTextToImage, want: false},
		{name: "empty tags", tags: "", tag: ModelTagTextToImage, want: false},
		{name: "empty tag", tags: ModelTagTextToImage, tag: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasModelTag(tt.tags, tt.tag))
		})
	}
}
