package router

import (
	"bytes"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// docsSite is a pre-built VitePress static site read from disk at startup.
type docsSite struct {
	root     string
	prefix   string // mount path, e.g. "/docs/user"
	index    []byte // <root>/index.html
	notFound []byte // <root>/404.html
}

// withBasePath rewrites the site's own absolute references so a prefixed
// deployment resolves them. VitePress bakes its `base` option into the built
// HTML and CSS (see docs-site/*/docs/.vitepress/config.ts), and that value is
// root-absolute, which a reverse proxy publishing the app under a URL prefix
// will not route. Without a prefix configured the bytes are returned untouched.
func (s docsSite) withBasePath(b []byte) []byte {
	base := common.BasePath()
	if base == "" {
		return b
	}
	return bytes.ReplaceAll(b, []byte(s.prefix+"/"), []byte(base+s.prefix+"/"))
}

// SetDocsRouter serves the documentation sites bundled in the image (see the
// root Dockerfile):
//
//	/docs/user/...   user documentation
//	/docs/admin/...  admin documentation
//
// The static output lives under DOCS_SITE_DIR (default /app/docs inside the
// image). When a site is absent (e.g. local dev without a docs build), its
// requests get a plain 404.
func SetDocsRouter(r *gin.Engine) {
	root := os.Getenv("DOCS_SITE_DIR")
	if root == "" {
		root = "/app/docs"
	}
	registerDocsSite(r, "/docs/user", loadDocsSite(filepath.Join(root, "user")))
	registerDocsSite(r, "/docs/admin", loadDocsSite(filepath.Join(root, "admin")))
}

func registerDocsSite(r *gin.Engine, prefix string, site docsSite) {
	site.prefix = prefix
	r.GET(prefix, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, common.WithBasePath(prefix+"/"))
	})
	r.GET(prefix+"/*filepath", middleware.RouteTag("web"), serveDocsSite(site))
}

func loadDocsSite(dir string) docsSite {
	site := docsSite{root: dir}
	if b, err := os.ReadFile(filepath.Join(dir, "index.html")); err == nil {
		site.index = b
	}
	if b, err := os.ReadFile(filepath.Join(dir, "404.html")); err == nil {
		site.notFound = b
	}
	return site
}

func serveDocsSite(site docsSite) gin.HandlerFunc {
	return func(c *gin.Context) {
		if site.index == nil {
			c.Status(http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		if rel == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", site.withBasePath(site.index))
			return
		}
		// filepath.Join cleans the path; rejecting anything that escapes the
		// site root blocks ../ traversal (including %2e%2e, which gin decodes
		// before routing).
		full := filepath.Join(site.root, rel)
		if !strings.HasPrefix(full, site.root+string(os.PathSeparator)) {
			serveDocsNotFound(c, site)
			return
		}
		if info, err := os.Stat(full); err == nil {
			if info.IsDir() {
				if b, err := os.ReadFile(filepath.Join(full, "index.html")); err == nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", site.withBasePath(b))
					return
				}
			} else {
				serveDocsAsset(c, full, site)
				return
			}
		}
		// VitePress emits extensionless pages as flat files: /guide/token
		// resolves to guide/token.html, not guide/token/index.html.
		if page, err := os.ReadFile(full + ".html"); err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", site.withBasePath(page))
			return
		}
		serveDocsNotFound(c, site)
	}
}

// serveDocsAsset answers a request for a real file under the site root. Text
// assets can carry the site's baked-in base path and are rewritten in memory;
// everything else -- fonts, images, archives -- is streamed from disk by
// c.File, which also sets the Content-Type and honours range requests.
//
// The rewrite only engages when a URL prefix is configured, so a root-mounted
// deployment keeps serving every asset through c.File exactly as before.
func serveDocsAsset(c *gin.Context, full string, site docsSite) {
	if common.BasePath() != "" {
		if contentType := mime.TypeByExtension(filepath.Ext(full)); strings.HasPrefix(contentType, "text/") {
			if b, err := os.ReadFile(full); err == nil {
				c.Data(http.StatusOK, contentType, site.withBasePath(b))
				return
			}
			serveDocsNotFound(c, site)
			return
		}
	}
	c.File(full)
}

func serveDocsNotFound(c *gin.Context, site docsSite) {
	if site.notFound != nil {
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", site.withBasePath(site.notFound))
		return
	}
	c.Status(http.StatusNotFound)
}
