package controller

import (
	"testing"

	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripChannelsForNonAdmin(t *testing.T) {
	newResult := func() *perfmetrics.QueryResult {
		return &perfmetrics.QueryResult{
			ModelName: "gpt-4o",
			Groups: []perfmetrics.GroupResult{
				{
					Group:     "default",
					AvgTtftMs: 100,
					Channels: []perfmetrics.ChannelResult{
						{ChannelId: 1, ChannelName: "upstream-a"},
						{ChannelId: 2, ChannelName: "upstream-b"},
					},
				},
				{
					Group:     "auto",
					AvgTtftMs: 200,
					Channels: []perfmetrics.ChannelResult{
						{ChannelId: 3, ChannelName: "upstream-c"},
					},
				},
			},
		}
	}

	tests := []struct {
		name string
		role int
		// wantKeepChannels false = 全部 group 的 Channels 被清空（nil/无渠道数据）。
		wantKeepChannels bool
	}{
		{name: "root keeps channels", role: 100, wantKeepChannels: true},
		{name: "admin keeps channels", role: 10, wantKeepChannels: true},
		{name: "ops strips channels", role: 5, wantKeepChannels: false},
		{name: "common user strips channels", role: 1, wantKeepChannels: false},
		{name: "anonymous strips channels", role: 0, wantKeepChannels: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newResult()
			stripChannelsForNonAdmin(tt.role, result)

			for _, group := range result.Groups {
				if tt.wantKeepChannels {
					assert.NotEmpty(t, group.Channels, "group %q channels should survive for role %d", group.Group, tt.role)
				} else {
					assert.Nil(t, group.Channels, "group %q channels should be stripped for role %d", group.Group, tt.role)
				}
			}

			// 仅剥离 Channels，组级聚合与序列数据必须原样保留。
			require.Len(t, result.Groups, 2)
			assert.Equal(t, "default", result.Groups[0].Group)
			assert.Equal(t, int64(100), result.Groups[0].AvgTtftMs)
			assert.Equal(t, "auto", result.Groups[1].Group)
			assert.Equal(t, int64(200), result.Groups[1].AvgTtftMs)
		})
	}
}

// TestStripChannelsForNonAdminNilGroups 空 group 列表不应 panic。
func TestStripChannelsForNonAdminNilGroups(t *testing.T) {
	result := &perfmetrics.QueryResult{ModelName: "gpt-4o"}
	stripChannelsForNonAdmin(0, result)
	assert.Empty(t, result.Groups)
}
