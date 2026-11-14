package handler

import (
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/0x2E/fusion/internal/web"
	"github.com/gin-gonic/gin"
)

const frontendIndexFile = "index.html"

var headTagPattern = regexp.MustCompile(`(?i)<head[^>]*>`)

func (h *Handler) setupFrontendRoutes(r *gin.Engine, mounts []*gin.RouterGroup) error {
	frontendFS, _, err := web.FrontendFS()
	if err != nil {
		return err
	}

	for _, mount := range mounts {
		mount.StaticFS("/assets", http.FS(frontendFS))
	}

	fileServer := http.FileServer(http.FS(frontendFS))
	r.NoRoute(func(c *gin.Context) {
		serveFrontendRoute(c, frontendFS, fileServer, h.basePath())
	})

	return nil
}

func serveFrontendRoute(c *gin.Context, frontendFS fs.FS, fileServer http.Handler, basePath string) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusNotFound)
		return
	}

	setFrontendSecurityHeaders(c)

	// Both the prefixed and the unprefixed form reach this fallback: proxies
	// forward the prefix unchanged or strip it. Never redirect here — the
	// backend cannot see the browser's URL, so a redirect guessed from the
	// upstream path loops behind path-rewriting proxies (301 loop via nginx).
	relativePath := trimBasePath(c.Request.URL.Path, basePath)

	cleanedPath := path.Clean(relativePath)
	if cleanedPath == "." {
		cleanedPath = "/"
	}

	if isAPIPath(cleanedPath) {
		c.Status(http.StatusNotFound)
		return
	}

	if cleanedPath == "/" || cleanedPath == "/"+frontendIndexFile {
		serveFrontendIndex(c, frontendFS, basePath)
		return
	}

	assetPath := strings.TrimPrefix(cleanedPath, "/")
	if assetPath == "" {
		assetPath = frontendIndexFile
	}

	if frontendFileExists(frontendFS, assetPath) {
		serveFrontendRequestPath(c, fileServer, "/"+assetPath)
		return
	}

	if looksLikeAssetPath(assetPath) {
		c.Status(http.StatusNotFound)
		return
	}

	serveFrontendIndex(c, frontendFS, basePath)
}

func serveFrontendIndex(c *gin.Context, frontendFS fs.FS, basePath string) {
	indexHTML, err := fs.ReadFile(frontendFS, frontendIndexFile)
	if err != nil {
		internalError(c, err, "read frontend index")
		return
	}

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", injectBasePath(indexHTML, basePath))
}

// trimBasePath strips the configured prefix from a request path so SPA routes
// and assets resolve the same whether the proxy forwarded the prefix or
// stripped it. Paths outside the prefix are returned unchanged.
func trimBasePath(requestPath, basePath string) string {
	if basePath == "" {
		return requestPath
	}

	if requestPath == basePath {
		return ""
	}

	if strings.HasPrefix(requestPath, basePath+"/") {
		return strings.TrimPrefix(requestPath, basePath)
	}

	return requestPath
}

// injectBasePath inserts a <base> tag and a metadata tag so the same prebuilt
// bundle works at any mount point: relative asset URLs resolve against <base>
// and runtime code reads the mount point from the meta tag.
func injectBasePath(indexHTML []byte, basePath string) []byte {
	prefix := basePath + "/"
	injection := fmt.Sprintf(
		`<base href="%s"><meta name="fusion-base-path" content="%s">`,
		prefix,
		prefix,
	)

	loc := headTagPattern.FindIndex(indexHTML)
	if loc == nil {
		return append([]byte(injection), indexHTML...)
	}

	injected := make([]byte, 0, len(indexHTML)+len(injection))
	injected = append(injected, indexHTML[:loc[1]]...)
	injected = append(injected, injection...)
	injected = append(injected, indexHTML[loc[1]:]...)

	return injected
}

func setFrontendSecurityHeaders(c *gin.Context) {
	// base-uri must allow the injected <base> while still pinning it to this origin.
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' http: https: data:; font-src 'self' data:; connect-src 'self' http: https:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
}

func serveFrontendRequestPath(c *gin.Context, fileServer http.Handler, requestPath string) {
	originalPath := c.Request.URL.Path
	c.Request.URL.Path = requestPath
	fileServer.ServeHTTP(c.Writer, c.Request)
	c.Request.URL.Path = originalPath
}

func frontendFileExists(frontendFS fs.FS, filePath string) bool {
	info, err := fs.Stat(frontendFS, filePath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func isAPIPath(requestPath string) bool {
	cleanedPath := path.Clean(requestPath)
	if cleanedPath == "." {
		cleanedPath = "/"
	}

	return cleanedPath == "/api" || strings.HasPrefix(cleanedPath, "/api/")
}

func looksLikeAssetPath(filePath string) bool {
	base := path.Base(filePath)
	return strings.Contains(base, ".")
}
