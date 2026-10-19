package auth

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityEnableProperty(t *testing.T) {
	if SecurityEnableProperty != "petclinic.security.enable" {
		t.Fatalf("SecurityEnableProperty = %q", SecurityEnableProperty)
	}
}

func TestIsSecurityDisabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"lowercase false", "false", true},
		{"uppercase FALSE", "FALSE", true},
		{"mixed case", "False", true},
		{"whitespace padded", "  false ", true},
		{"true", "true", false},
		{"TRUE", "TRUE", false},
		{"missing/empty", "", false},
		{"whitespace only", "   ", false},
		{"arbitrary", "no", false},
		{"zero", "0", false},
		{"falsey prefix", "falsey", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSecurityDisabled(tt.value); got != tt.want {
				t.Errorf("IsSecurityDisabled(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// fakeApp simulates application routing: a known endpoint and 404 otherwise.
func fakeApp(called *bool, body *string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/owners", func(w http.ResponseWriter, r *http.Request) {
		*called = true
		b, _ := io.ReadAll(r.Body)
		*body = string(b)
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	return mux
}

func TestDisableSecurityMiddleware_PermitsAll(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		headers    map[string]string
		wantStatus int
		wantCalled bool
	}{
		{"anonymous GET", http.MethodGet, "/api/owners", "", nil, http.StatusOK, true},
		{"POST without CSRF", http.MethodPost, "/api/owners", `{"firstName":"a"}`, map[string]string{"Content-Type": "application/json"}, http.StatusCreated, true},
		{"PUT without CSRF", http.MethodPut, "/api/owners", `{"x":1}`, nil, http.StatusOK, true},
		{"DELETE without CSRF", http.MethodDelete, "/api/owners", "", nil, http.StatusNoContent, true},
		{"invalid CSRF token", http.MethodPost, "/api/owners", "x", map[string]string{"X-CSRF-TOKEN": "bogus", "X-XSRF-TOKEN": "bogus"}, http.StatusCreated, true},
		{"bad basic auth", http.MethodGet, "/api/owners", "", map[string]string{"Authorization": "Basic Ym9ndXM6Ym9ndXM="}, http.StatusOK, true},
		{"malformed auth", http.MethodGet, "/api/owners", "", map[string]string{"Authorization": "garbage"}, http.StatusOK, true},
		{"bearer token", http.MethodDelete, "/api/owners", "", map[string]string{"Authorization": "Bearer invalid"}, http.StatusNoContent, true},
		{"unknown path", http.MethodGet, "/does/not/exist", "", nil, http.StatusNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var got string
			h := DisabledSecurityFilterChain()(fakeApp(&called, &got))
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Errorf("security rejected request with %d", rec.Code)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
			if called && got != tt.body {
				t.Errorf("body = %q, want %q", got, tt.body)
			}
			if v := rec.Header().Get("WWW-Authenticate"); v != "" {
				t.Errorf("unexpected WWW-Authenticate: %q", v)
			}
			if v := rec.Header().Get("X-Frame-Options"); v != "SAMEORIGIN" {
				t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", v)
			}
		})
	}
}

func TestDisabledSecurityFilterChain_DefaultHeaders(t *testing.T) {
	tests := []struct {
		name     string
		tls      bool
		wantHSTS string
	}{
		{"plain HTTP", false, ""},
		{"HTTPS", true, "max-age=31536000 ; includeSubDomains"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := DisabledSecurityFilterChain()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/vets", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			} else {
				req.TLS = nil
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			want := map[string]string{
				"X-Frame-Options":        "SAMEORIGIN",
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "0",
				"Cache-Control":          "no-cache, no-store, max-age=0, must-revalidate",
				"Pragma":                 "no-cache",
				"Expires":                "0",
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			if vals := rec.Header().Values("X-Frame-Options"); len(vals) != 1 {
				t.Errorf("X-Frame-Options values = %v, want exactly one", vals)
			}
			if got := rec.Header().Get("Strict-Transport-Security"); got != tt.wantHSTS {
				t.Errorf("HSTS = %q, want %q", got, tt.wantHSTS)
			}
		})
	}
}