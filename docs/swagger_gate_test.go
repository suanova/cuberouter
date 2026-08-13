package docs

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSwaggerInfo_GlobalAnnotations(t *testing.T) {
	if SwaggerInfo.Title != "new-api 用户 API" {
		t.Errorf("title = %q", SwaggerInfo.Title)
	}
	if SwaggerInfo.BasePath != "/api" {
		t.Errorf("basePath = %q", SwaggerInfo.BasePath)
	}
	if SwaggerInfo.Version == "" {
		t.Error("version 为空")
	}
}

// loadSpec reads the generated swagger.json from the docs/ directory.
// `go test ./docs/` runs with cwd = docs/, so the relative path resolves correctly.
func loadSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile("swagger.json")
	if err != nil {
		t.Fatalf("read swagger.json: %v", err)
	}
	var spec map[string]interface{}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	return spec
}

// refName strips the "#/definitions/" prefix from a $ref, returning the bare
// definition name (or the input unchanged if it is not a ref).
func refName(ref string) string {
	const p = "#/definitions/"
	if strings.HasPrefix(ref, p) {
		return ref[len(p):]
	}
	return ref
}

// responseDefNames collects the names of all definitions directly referenced by
// any response schema (i.e. types produced by @Success). Request-body schemas
// referenced via @Param body are intentionally NOT included — passwords in
// login/register request bodies are expected and allowed.
func responseDefNames(spec map[string]interface{}) map[string]bool {
	names := map[string]bool{}
	paths, _ := spec["paths"].(map[string]interface{})
	for _, methods := range paths {
		opMap, ok := methods.(map[string]interface{})
		if !ok {
			continue
		}
		responses, _ := opMap["responses"].(map[string]interface{})
		for _, resp := range responses {
			respMap, ok := resp.(map[string]interface{})
			if !ok {
				continue
			}
			schema, _ := respMap["schema"].(map[string]interface{})
			if ref, ok := schema["$ref"].(string); ok {
				names[refName(ref)] = true
			}
		}
	}
	return names
}

// TestSpec_NoAdminUserPaths asserts that admin-only user endpoints (which were
// intentionally left unannotated) do not leak into the public spec.
// 路径列表与 router/api-router.go 中 adminRoute 分组一一对应。
func TestSpec_NoAdminUserPaths(t *testing.T) {
	paths := loadSpec(t)["paths"].(map[string]interface{})
	forbidden := []string{
		"/user/search", "/user/manage", "/user/export", "/user/columns",
		"/user/{id}", "/user/", "/user/{id}/invitees", "/user/{id}/quota-dates",
		"/user/{id}/reset_passkey", "/user/{id}/2fa", "/user/2fa/stats",
		"/user/topup/complete", "/user/{id}/oauth/bindings",
		"/user/{id}/oauth/bindings/{provider_id}", "/user/{id}/bindings/{binding_type}",
	}
	for p := range paths {
		for _, f := range forbidden {
			if p == f {
				t.Errorf("spec 含 admin 路径 %s", p)
			}
		}
	}
}

// TestSpec_NoPasswordInResponses asserts that no definition reachable from a
// response schema exposes a `password` property (which would risk leaking
// password hashes). Passwords in request bodies (login/register) are allowed —
// they are not response-referenced.
func TestSpec_NoPasswordInResponses(t *testing.T) {
	spec := loadSpec(t)
	defs, _ := spec["definitions"].(map[string]interface{})
	respDefs := responseDefNames(spec)
	for name := range respDefs {
		d, _ := defs[name].(map[string]interface{})
		props, _ := d["properties"].(map[string]interface{})
		if _, ok := props["password"]; ok {
			t.Errorf("响应 definition %s 含 password 字段", name)
		}
	}
}

// TestSpec_HasUserSelfPath asserts that the annotated self-info endpoint is
// present in the spec.
func TestSpec_HasUserSelfPath(t *testing.T) {
	paths := loadSpec(t)["paths"].(map[string]interface{})
	if _, ok := paths["/user/self"]; !ok {
		t.Errorf("spec 缺少 /user/self")
	}
}

// TestSpec_HasImportCsvPath asserts that the CSV channel-model import endpoint
// (annotated in controller/channel_csv_import.go) is present in the public spec.
func TestSpec_HasImportCsvPath(t *testing.T) {
	paths := loadSpec(t)["paths"].(map[string]interface{})
	if _, ok := paths["/channel/{id}/import_models_csv"]; !ok {
		t.Errorf("spec 缺少 /channel/{id}/import_models_csv")
	}
}

// TestSpec_NoVideoOrKlingPaths asserts that out-of-scope video/kling endpoints
// do not leak into the public spec. Video routes live under /v1 (router/video-router.go),
// outside the /api BasePath and not annotated, so this is a leak guard.
func TestSpec_NoVideoOrKlingPaths(t *testing.T) {
	paths := loadSpec(t)["paths"].(map[string]interface{})
	for p := range paths {
		if strings.Contains(p, "video") || strings.Contains(p, "kling") {
			t.Errorf("spec 含超出范围的 video/kling 路径 %s", p)
		}
	}
}
