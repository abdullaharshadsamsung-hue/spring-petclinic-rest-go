// Package api wires the PetClinic HTTP routes onto a net/http ServeMux.
//
// This file holds the port of RootRestControllerV1: any request to the
// application root ({contextPath}/) is redirected to the Swagger UI page.
package api

import (
	"net/http"
	"strings"
)

// DefaultContextPath is the servlet context path the Spring application was
// deployed under (server.servlet.context-path=/petclinic).
const DefaultContextPath = "/petclinic"

// swaggerUIIndex is the Swagger UI page path relative to the context path.
const swaggerUIIndex = "/swagger-ui/index.html"

// rootCORSExposedHeaders mirrors @CrossOrigin(exposedHeaders = "errors, content-type").
const rootCORSExposedHeaders = "errors, content-type"

// RootController redirects requests for the application root to Swagger UI.
type RootController struct {
	contextPath string
}

// NewRootController returns a RootController for the given context path
// (e.g. "/petclinic"). An empty path or "/" means the app is served at the
// server root. A trailing slash is ignored and a missing leading slash added.
func NewRootController(contextPath string) *RootController {
	return &RootController{contextPath: normalizeContextPath(contextPath)}
}

// Register mounts the root redirect on mux. No HTTP method is given in the
// patterns: like @RequestMapping without a method attribute, the redirect
// answers every method (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS).
// Both "{contextPath}/" and "{contextPath}" are handled.
func (c *RootController) Register(mux *http.ServeMux) {
	h := withRootCORS(http.HandlerFunc(c.redirectToSwagger))
	mux.Handle(c.contextPath+"/{$}", h)
	if c.contextPath != "" {
		mux.Handle(c.contextPath, h)
	}
}

// redirectToSwagger sends a 302 Found to {contextPath}/swagger-ui/index.html,
// matching HttpServletResponse.sendRedirect.
func (c *RootController) redirectToSwagger(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, c.contextPath+swaggerUIIndex, http.StatusFound)
}

// RegisterRootRoutes is a convenience wrapper that mounts the root redirect
// for contextPath on mux.
func RegisterRootRoutes(mux *http.ServeMux, contextPath string) {
	NewRootController(contextPath).Register(mux)
}

func normalizeContextPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// withRootCORS applies the class-level @CrossOrigin defaults of the Spring
// controller: any origin allowed, credentials not allowed, max age 1800s,
// all request headers allowed, and the "errors, content-type" headers exposed.
//
// MIGRATION_NOTE: Spring's CorsProcessor also rejects invalid preflights with
// 403; here every preflight is accepted because the configuration allows all
// origins, methods and headers anyway.
func withRootCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		hdr := w.Header()
		hdr.Add("Vary", "Origin")
		hdr.Add("Vary", "Access-Control-Request-Method")
		hdr.Add("Vary", "Access-Control-Request-Headers")
		hdr.Set("Access-Control-Allow-Origin", "*")

		reqMethod := r.Header.Get("Access-Control-Request-Method")
		if r.Method == http.MethodOptions && reqMethod != "" {
			// Preflight request: answer it directly, as Spring does.
			hdr.Set("Access-Control-Allow-Methods", reqMethod)
			if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				hdr.Set("Access-Control-Allow-Headers", reqHeaders)
			}
			hdr.Set("Access-Control-Max-Age", "1800")
			w.WriteHeader(http.StatusOK)
			return
		}

		hdr.Set("Access-Control-Expose-Headers", rootCORSExposedHeaders)
		next.ServeHTTP(w, r)
	})
}
