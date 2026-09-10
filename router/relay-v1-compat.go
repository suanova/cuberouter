package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// bareV1RoutePatterns 把 /v1 下已注册的路由路径去掉前缀，得到它们在裸路径下的等价模式，
// 供 NoRoute 在收到漏写 /v1 的请求时补回前缀。":param"、"*path" 段原样保留。
func bareV1RoutePatterns(routes []gin.RouteInfo) []string {
	seen := make(map[string]struct{}, len(routes))
	patterns := make([]string, 0, len(routes))
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/v1/") {
			continue
		}
		pattern := strings.TrimPrefix(route.Path, "/v1")
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		patterns = append(patterns, pattern)
	}
	return patterns
}

// relayV1CompatRedirect 把漏写 /v1 的 API 请求 307 到同名的 /v1 路径。
// 307 保持方法与请求体，且不会被客户端缓存。两种请求放行给原有链路：
// 浏览器导航（Accept 含 text/html）应看到 dashboard SPA，WebSocket 升级不跟随重定向。
func relayV1CompatRedirect(patterns []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.GetHeader("Accept"), "text/html") || c.GetHeader("Upgrade") != "" {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !matchesBareV1Pattern(patterns, path) {
			c.Next()
			return
		}
		target := "/v1" + path
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		// 相对 Location，避免反代后把客户端指向错误的 host。
		c.Redirect(http.StatusTemporaryRedirect, target)
		c.Abort()
	}
}

func matchesBareV1Pattern(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if matchesRoutePattern(pattern, path) {
			return true
		}
	}
	return false
}

// matchesRoutePattern 按 gin 的路由语法逐段比较：字面量相等，":name" 匹配一段，"*name" 匹配剩余全部。
func matchesRoutePattern(pattern string, path string) bool {
	if !strings.HasPrefix(pattern, "/") || !strings.HasPrefix(path, "/") {
		return false
	}
	patternSegments := strings.Split(pattern[1:], "/")
	pathSegments := strings.Split(path[1:], "/")
	for index, segment := range patternSegments {
		if strings.HasPrefix(segment, "*") {
			return true
		}
		if index >= len(pathSegments) {
			return false
		}
		if strings.HasPrefix(segment, ":") {
			continue
		}
		if segment != pathSegments[index] {
			return false
		}
	}
	return len(patternSegments) == len(pathSegments)
}
