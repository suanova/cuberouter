package plugins_test

import "testing"

func TestViduResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "vidu",
		model:     "viduq2",
		requestBody: map[string]any{
			"model": "viduq2",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "move between frames"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/first.png"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/last.png"},
			}}},
			"seconds": 8,
			"size":    "720p",
		},
		wantAction: "first_tail_to_video",
		wantRequest: map[string]any{
			"model":    "viduq2",
			"prompt":   "move between frames",
			"images":   []any{"https://cdn.example/first.png", "https://cdn.example/last.png"},
			"duration": float64(8),
			"size":     "720p",
		},
		wantUsageKeys:       []string{"credits", "duration", "resolution"},
		wantSubmitUsageKeys: []string{"duration", "resolution"},
		wantVendorName:      "vidu",
	})
}

func TestViduReferenceToVideoAllowsQ3Models(t *testing.T) {
	// 参考图生视频模型约束已随上游文档更新(2026-04):
	// viduq3-turbo 等模型支持参考图,上游模型名不得被归一化为 viduq2
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "vidu",
		model:     "viduq3-turbo",
		requestBody: map[string]any{
			"model": "viduq3-turbo",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "unified character across scenes"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/a.png"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/b.png"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/c.png"},
			}}},
			"seconds": 5,
			"size":    "720p",
		},
		wantAction: "reference_to_video",
		wantRequest: map[string]any{
			"model":    "viduq3-turbo",
			"prompt":   "unified character across scenes",
			"images": []any{
				"https://cdn.example/a.png",
				"https://cdn.example/b.png",
				"https://cdn.example/c.png",
			},
			"duration": float64(5),
			"size":     "720p",
		},
		wantUsageKeys:       []string{"credits", "duration", "resolution"},
		wantSubmitUsageKeys: []string{"duration", "resolution"},
		wantVendorName:      "vidu",
	})
}
