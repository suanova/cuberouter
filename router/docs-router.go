package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// docsSite is a pre-built VitePress static site read from disk at startup.
type docsSite struct {
	root     string
	index    []byte // <root>/index.html
	notFound []byte // <root>/404.html
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
	r.GET(prefix, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, prefix+"/")
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
			c.Data(http.StatusOK, "text/html; charset=utf-8", site.index)
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
					c.Data(http.StatusOK, "text/html; charset=utf-8", b)
					return
				}
			} else {
				c.File(full)
				return
			}
		}
		// VitePress emits extensionless pages as flat files: /guide/token
		// resolves to guide/token.html, not guide/token/index.html.
		if page, err := os.ReadFile(full + ".html"); err == nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", page)
			return
		}
		serveDocsNotFound(c, site)
	}
}

func serveDocsNotFound(c *gin.Context, site docsSite) {
	if site.notFound != nil {
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", site.notFound)
		return
	}
	c.Status(http.StatusNotFound)
}
