package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeContextPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"/", ""},
		{"  ", ""},
		{"/petclinic", "/petclinic"},
		{"/petclinic/", "/petclinic"},
		{"petclinic", "/petclinic"},
		{" /petclinic/ ", "/petclinic"},
	}
	for _, tt := range tests {
		if got := normalizeContextPath(tt.in); got != tt.want {
			t.Errorf("normalizeContextPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRootRedirect(t *testing.T) {
	methods := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions,
	}
	tests := []struct {
		name        string
		contextPath string
		paths       []string
		wantLoc     string
	}{
		{"root context", "", []string{"/"}, "/swagger-ui/index.html"},
		{"slash context", "/", []string{"/"}, "/swagger-ui/index.html"},
		{"petclinic", DefaultContextPath, []string{"/petclinic/", "/petclinic"}, "/petclinic/swagger-ui/index.html"},
		{"petclinic trailing slash", "petclinic/", []string{"/petclinic/", "/petclinic"}, "/petclinic/swagger-ui/index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterRootRoutes(mux, tt.contextPath)
			for _, p := range tt.paths {
				for _, m := range methods {
					req := httptest.NewRequest(m, p, nil)
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, req)
					if rec.Code != http.StatusFound {
						t.Fatalf("%s %s: status %d, want 302", m, p, rec.Code)
					}
					loc := rec.Header().Get("Location")
					if loc != tt.wantLoc && !strings.HasSuffix(loc, tt.wantLoc) {
						t.Errorf("%s %s: Location %q, want %q", m, p, loc, tt.wantLoc)
					}
				}
			}
		})
	}
}

func TestOnlyRootHandled(t *testing.T) {
	tests := []struct {
		name, contextPath, path string
	}{
		{"root other path", "", "/foo"},
		{"root swagger", "", "/swagger-ui/index.html"},
		{"ctx subpath", "/petclinic", "/petclinic/api/owners"},
		{"ctx outside", "/petclinic", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			NewRootController(tt.contextPath).Register(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code == http.StatusFound {
				t.Errorf("path %q unexpectedly redirected to %q", tt.path, rec.Header().Get("Location"))
			}
			if rec.Code != http.StatusNotFound {
				t.Errorf("path %q: status %d, want 404", tt.path, rec.Code)
			}
		})
	}
}

func TestRootCORS(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		headers        map[string]string
		wantStatus     int
		wantAllowAll   bool
		wantExpose     bool
		wantAllowMeth  string
		wantAllowHdrs  string
		wantMaxAge     string
	}{
		{
			name:       "no origin",
			method:     http.MethodGet,
			wantStatus: http.StatusFound,
		},
		{
			name:         "simple cross-origin GET",
			method:       http.MethodGet,
			headers:      map[string]string{"Origin": "http://example.com"},
			wantStatus:   http.StatusFound,
			wantAllowAll: true,
			wantExpose:   true,
		},
		{
			name:         "cross-origin POST",
			method:       http.MethodPost,
			headers:      map[string]string{"Origin": "http://other.org"},
			wantStatus:   http.StatusFound,
			wantAllowAll: true,
			wantExpose:   true,
		},
		{
			name:   "preflight",
			method: http.MethodOptions,
			headers: map[string]string{
				"Origin":                         "http://example.com",
				"Access-Control-Request-Method":  "POST",
				"Access-Control-Request-Headers": "X-Custom, Content-Type",
			},
			wantStatus:    http.StatusOK,
			wantAllowAll:  true,
			wantAllowMeth: "POST",
			wantAllowHdrs: "X-Custom, Content-Type",
			wantMaxAge:    "1800",
		},
		{
			name:         "OPTIONS without request method is not preflight",
			method:       http.MethodOptions,
			headers:      map[string]string{"Origin": "http://example.com"},
			wantStatus:   http.StatusFound,
			wantAllowAll: true,
			wantExpose:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterRootRoutes(mux, DefaultContextPath)
			req := httptest.NewRequest(tt.method, "/petclinic/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			h := rec.Header()
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusFound {
				if loc := h.Get("Location"); !strings.HasSuffix(loc, "/petclinic/swagger-ui/index.html") {
					t.Errorf("Location %q", loc)
				}
			}
			acao := h.Get("Access-Control-Allow-Origin")
			if tt.wantAllowAll {
				if acao != "*" {
					t.Errorf("Allow-Origin %q, want *", acao)
				}
				if h.Get("Access-Control-Allow-Credentials") == "true" {
					t.Error("credentials must not be allowed")
				}
				if len(h.Values("Vary")) == 0 {
					t.Error("missing Vary header")
				}
			} else if acao != "" {
				t.Errorf("unexpected Allow-Origin %q", acao)
			}
			expose := h.Get("Access-Control-Expose-Headers")
			if tt.wantExpose {
				lower := strings.ToLower(expose)
				if !strings.Contains(lower, "errors") || !strings.Contains(lower, "content-type") {
					t.Errorf("Expose-Headers %q missing errors/content-type", expose)
				}
			}
			if got := h.Get("Access-Control-Allow-Methods"); got != tt.wantAllowMeth {
				t.Errorf("Allow-Methods %q, want %q", got, tt.wantAllowMeth)
			}
			if got := h.Get("Access-Control-Allow-Headers"); got != tt.wantAllowHdrs {
				t.Errorf("Allow-Headers %q, want %q", got, tt.wantAllowHdrs)
			}
			if got := h.Get("Access-Control-Max-Age"); got != tt.wantMaxAge {
				t.Errorf("Max-Age %q, want %q", got, tt.wantMaxAge)
			}
		})
	}
}

func TestRedirectIdempotent(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRootRoutes(mux, DefaultContextPath)
	var first string
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/petclinic/", nil))
		if rec.Code != http.StatusFound {
			t.Fatalf("iteration %d: status %d", i, rec.Code)
		}
		loc := rec.Header().Get("Location")
		if i == 0 {
			first = loc
		} else if loc != first {
			t.Errorf("iteration %d: Location %q differs from %q", i, loc, first)
		}
	}
}