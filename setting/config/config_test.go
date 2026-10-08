package config

import (
	"bytes"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfigWithMap struct {
	Modes map[string]string `json:"modes"`
	Exprs map[string]string `json:"exprs"`
	Name  string            `json:"name"`
}

type testConfigWithPorts struct {
	Ports []string `json:"ports"`
}

func TestUpdateConfigFromMap_MapReplacement(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
			"model-b": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
			"model-b": "p * 10 + c * 50",
		},
		Name: "billing",
	}

	// Simulate removing model-a: new value only has model-b
	err := UpdateConfigFromMap("test_config", cfg, map[string]string{
		"modes": `{"model-b": "tiered_expr"}`,
		"exprs": `{"model-b": "p * 10 + c * 50"}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if _, ok := cfg.Modes["model-a"]; ok {
		t.Errorf("Modes still contains model-a after it was removed from the update; got %v", cfg.Modes)
	}
	if _, ok := cfg.Exprs["model-a"]; ok {
		t.Errorf("Exprs still contains model-a after it was removed from the update; got %v", cfg.Exprs)
	}

	if cfg.Modes["model-b"] != "tiered_expr" {
		t.Errorf("Modes[model-b] = %q, want %q", cfg.Modes["model-b"], "tiered_expr")
	}
	if cfg.Exprs["model-b"] != "p * 10 + c * 50" {
		t.Errorf("Exprs[model-b] = %q, want %q", cfg.Exprs["model-b"], "p * 10 + c * 50")
	}
}

func TestUpdateConfigFromMap_EmptyMapClearsAll(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
		},
	}

	err := UpdateConfigFromMap("test_config", cfg, map[string]string{
		"modes": `{}`,
		"exprs": `{}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if len(cfg.Modes) != 0 {
		t.Errorf("Modes should be empty after updating with {}, got %v", cfg.Modes)
	}
	if len(cfg.Exprs) != 0 {
		t.Errorf("Exprs should be empty after updating with {}, got %v", cfg.Exprs)
	}
}

func TestUpdateConfigFromMap_ScalarFieldsUnchanged(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{"m": "v"},
		Name:  "old",
	}

	err := UpdateConfigFromMap("test_config", cfg, map[string]string{
		"name": "new",
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if cfg.Name != "new" {
		t.Errorf("Name = %q, want %q", cfg.Name, "new")
	}
	// modes was not in configMap, should remain unchanged
	if cfg.Modes["m"] != "v" {
		t.Errorf("Modes should be unchanged, got %v", cfg.Modes)
	}
}

// TestUpdateConfigFromMap_ReportsUnparsableValue 覆盖历史脏值：写进库里的
// fetch_setting.allowed_ports 曾是数字数组 [80,443]，而字段是 []string，
// json.Unmarshal 失败后旧代码直接 continue —— 字段静默保留默认值，界面上显示的
// 允许端口因此从不生效，且没有任何痕迹。现在这种失败必须留下一条 SysError。
func TestUpdateConfigFromMap_ReportsUnparsableValue(t *testing.T) {
	previousWriter := gin.DefaultErrorWriter
	var logged bytes.Buffer
	gin.DefaultErrorWriter = &logged
	t.Cleanup(func() { gin.DefaultErrorWriter = previousWriter })

	cfg := &testConfigWithPorts{Ports: []string{"80"}}

	// 数字数组解不进 []string：第一个元素就失败，字段保持原值。
	require.NoError(t, UpdateConfigFromMap("fetch_setting", cfg, map[string]string{
		"ports": `[80,443]`,
	}))

	assert.Equal(t, []string{"80"}, cfg.Ports, "解析失败的值不应被部分写入")
	assert.Contains(t, logged.String(), "failed to parse config fetch_setting.ports",
		"解析失败必须留下 SysError，且日志要能定位到具体配置项")
}
