package apidocs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const (
	wantTitle   = "REST Petclinic backend API documentation"
	wantVersion = "1.0"
	wantToS     = "https://github.com/spring-petclinic/spring-petclinic-rest/blob/master/terms.txt"
	wantDesc    = "This is the REST API documentation of the Spring Petclinic backend. If authentication is enabled, use admin/admin when calling the APIs"
)

func TestCustomOpenAPIMetadata(t *testing.T) {
	doc := CustomOpenAPI()
	if doc.Info.License == nil || doc.Info.Contact == nil {
		t.Fatalf("license/contact must be non-nil")
	}
	tests := []struct {
		name, got, want string
	}{
		{"openapi", doc.OpenAPI, "3.0.1"},
		{"title", doc.Info.Title, wantTitle},
		{"version", doc.Info.Version, wantVersion},
		{"termsOfService", doc.Info.TermsOfService, wantToS},
		{"description", doc.Info.Description, wantDesc},
		{"license.name", doc.Info.License.Name, "Apache 2.0"},
		{"license.url", doc.Info.License.URL, "http://www.apache.org/licenses/LICENSE-2.0"},
		{"contact.name", doc.Info.Contact.Name, "Vitaliy Fedoriv"},
		{"contact.email", doc.Info.Contact.Email, "vitaliy.fedoriv@gmail.com"},
		{"contact.url", doc.Info.Contact.URL, "https://github.com/spring-petclinic/spring-petclinic-rest"},
		{"components", string(doc.Components), "{}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
	if doc.Paths == nil || len(doc.Paths) != 0 {
		t.Errorf("paths should be empty non-nil map")
	}
}

func TestCustomOpenAPIRepeatable(t *testing.T) {
	a, b := CustomOpenAPI(), CustomOpenAPI()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("repeated calls differ")
	}
	a.Info.Contact.Name = "x"
	if CustomOpenAPI().Info.Contact.Name != "Vitaliy Fedoriv" {
		t.Fatalf("shared mutable state")
	}
}

func TestCustomOpenAPIJSONShape(t *testing.T) {
	raw, err := json.Marshal(CustomOpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	comp, ok := m["components"].(map[string]any)
	if !ok || len(comp) != 0 {
		t.Errorf("components = %v", m["components"])
	}
	info := m["info"].(map[string]any)
	lic := info["license"].(map[string]any)
	for k := range lic {
		if strings.HasPrefix(k, "x-") {
			t.Errorf("unexpected extension %q", k)
		}
	}
	if len(lic) != 2 {
		t.Errorf("license fields = %v", lic)
	}
}

func TestMergeSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
		check   func(t *testing.T, d Document)
	}{
		{
			name: "merges and drops oops",
			spec: `{"servers":[{"url":"http://x"}],"tags":[{"name":"pet"},{"name":"oops"}],
				"paths":{"/pets":{"get":{}},"/oops":{"get":{}}},"components":{"schemas":{"A":{}}}}`,
			check: func(t *testing.T, d Document) {
				if _, ok := d.Paths["/oops"]; ok {
					t.Error("/oops kept")
				}
				if _, ok := d.Paths["/pets"]; !ok {
					t.Error("/pets missing")
				}
				var tags []map[string]any
				_ = json.Unmarshal(d.Tags, &tags)
				if len(tags) != 1 || tags[0]["name"] != "pet" {
					t.Errorf("tags = %s", d.Tags)
				}
				if !strings.Contains(string(d.Components), "schemas") {
					t.Errorf("components = %s", d.Components)
				}
				if !strings.Contains(string(d.Servers), "http://x") {
					t.Errorf("servers = %s", d.Servers)
				}
				if d.Info.Title != wantTitle {
					t.Error("info overwritten")
				}
			},
		},
		{
			name: "null components keeps empty",
			spec: `{"components":null}`,
			check: func(t *testing.T, d Document) {
				if string(d.Components) != "{}" {
					t.Errorf("components = %s", d.Components)
				}
				if d.Paths == nil {
					t.Error("paths nil")
				}
			},
		},
		{name: "invalid json", spec: `{`, wantErr: true},
		{name: "invalid tag", spec: `{"tags":[1]}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := MergeSpec(CustomOpenAPI(), []byte(tc.spec))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tc.check != nil {
				tc.check(t, d)
			}
		})
	}
}

func TestHandlerRoutes(t *testing.T) {
	h, err := NewHandler(CustomOpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)

	tests := []struct {
		name, method, path string
		wantStatus         int
		wantCT             string
		contains           []string
	}{
		{"json", "GET", APIDocsJSONPath, 200, "application/json",
			[]string{wantTitle, `"version":"1.0"`, `"components":{}`}},
		{"yaml", "GET", APIDocsYAMLPath, 200, "application/vnd.oai.openapi",
			[]string{`"title": "` + wantTitle + `"`, `"version": "1.0"`, `"components": {}`, `"name": "Vitaliy Fedoriv"`}},
		{"ui", "GET", SwaggerUIPath, 200, "text/html; charset=utf-8",
			[]string{"swagger-ui", APIDocsJSONPath}},
		{"redirect", "GET", SwaggerUIRedirect, http.StatusFound, "", nil},
		{"post not allowed", "POST", APIDocsJSONPath, http.StatusMethodNotAllowed, "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d", rec.Code)
			}
			if tc.wantCT != "" && rec.Header().Get("Content-Type") != tc.wantCT {
				t.Errorf("ct = %q", rec.Header().Get("Content-Type"))
			}
			if tc.wantStatus == http.StatusFound && rec.Header().Get("Location") != SwaggerUIPath {
				t.Errorf("location = %q", rec.Header().Get("Location"))
			}
			body := rec.Body.String()
			for _, s := range tc.contains {
				if !strings.Contains(body, s) {
					t.Errorf("body missing %q:\n%s", s, body)
				}
			}
		})
	}
}

func TestJSONEndpointDecodes(t *testing.T) {
	h, _ := NewHandler(CustomOpenAPI())
	rec := httptest.NewRecorder()
	h.serveJSON(rec, httptest.NewRequest("GET", APIDocsJSONPath, nil))
	var d Document
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Info.Description != wantDesc || d.Info.TermsOfService != wantToS {
		t.Errorf("info = %+v", d.Info)
	}
}

func TestYAMLScalar(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{"a", `"a"`},
		{true, "true"},
		{json.Number("12"), "12"},
		{3, `"3"`},
	}
	for _, tc := range tests {
		if got := yamlScalar(tc.in); got != tc.want {
			t.Errorf("yamlScalar(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWriteYAML(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"empty map", map[string]any{}, "{}\n"},
		{"empty list", []any{}, "[]\n"},
		{"nested", map[string]any{"b": []any{"x", map[string]any{}}, "a": json.Number("1")},
			"\"a\": 1\n\"b\":\n  - \"x\"\n  - {}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeYAML(&b, tc.in, 0)
			if b.String() != tc.want {
				t.Errorf("got %q, want %q", b.String(), tc.want)
			}
		})
	}
}