// Package apidocs replaces the springdoc-openapi configuration of the Spring
// PetClinic REST backend (SwaggerConfig). It builds the OpenAPI document
// metadata (info, license, contact, empty components) and serves it as JSON
// and YAML, together with a Swagger UI page.
package apidocs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Route paths. The Spring application ran under the "/petclinic" context path,
// so the springdoc endpoints are exposed with that prefix.
const (
	APIDocsJSONPath   = "/petclinic/v3/api-docs"
	APIDocsYAMLPath   = "/petclinic/v3/api-docs.yaml"
	SwaggerUIPath     = "/petclinic/swagger-ui/index.html"
	SwaggerUIRedirect = "/petclinic/swagger-ui.html"
)

// Contact is the OpenAPI contact object.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License is the OpenAPI license object.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Info is the OpenAPI info object.
type Info struct {
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version"`
}

// Document is the subset of an OpenAPI 3 document produced by this package.
// Paths, Servers, Tags and Components are kept as raw JSON so that a full
// API specification (e.g. converted from openapi.yml) can be merged in
// without modelling every OpenAPI type.
type Document struct {
	OpenAPI    string                     `json:"openapi"`
	Info       Info                       `json:"info"`
	Servers    json.RawMessage            `json:"servers,omitempty"`
	Tags       json.RawMessage            `json:"tags,omitempty"`
	Paths      map[string]json.RawMessage `json:"paths"`
	Components json.RawMessage            `json:"components"`
}

func swaggerContact() *Contact {
	return &Contact{
		Name:  "Vitaliy Fedoriv",
		Email: "vitaliy.fedoriv@gmail.com",
		URL:   "https://github.com/spring-petclinic/spring-petclinic-rest",
	}
}

func swaggerLicense() *License {
	// The Java code set an empty extensions map; an empty map is serialised
	// as nothing, so it is omitted here.
	return &License{
		Name: "Apache 2.0",
		URL:  "http://www.apache.org/licenses/LICENSE-2.0",
	}
}

// CustomOpenAPI returns the base OpenAPI document with the PetClinic API
// metadata and an empty components section (equivalent of the Spring
// SwaggerConfig.customOpenAPI bean).
func CustomOpenAPI() Document {
	return Document{
		OpenAPI: "3.0.1",
		Info: Info{
			Title:          "REST Petclinic backend API documentation",
			Version:        "1.0",
			TermsOfService: "https://github.com/spring-petclinic/spring-petclinic-rest/blob/master/terms.txt",
			Description: "This is the REST API documentation of the Spring Petclinic backend. " +
				"If authentication is enabled, use admin/admin when calling the APIs",
			License: swaggerLicense(),
			Contact: swaggerContact(),
		},
		Paths:      map[string]json.RawMessage{},
		Components: json.RawMessage(`{}`),
	}
}

// MergeSpec overlays servers, tags, paths and components from a full OpenAPI
// specification given as JSON onto doc, keeping doc's Info (springdoc
// semantics: the user-provided bean's info wins). The "/oops" path and the
// "oops" tag are dropped, as that endpoint is not served by the Go port.
func MergeSpec(doc Document, specJSON []byte) (Document, error) {
	var spec struct {
		Servers    json.RawMessage            `json:"servers"`
		Tags       []json.RawMessage          `json:"tags"`
		Paths      map[string]json.RawMessage `json:"paths"`
		Components json.RawMessage            `json:"components"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return doc, fmt.Errorf("apidocs: decode spec: %w", err)
	}
	if len(spec.Servers) > 0 {
		doc.Servers = spec.Servers
	}
	if len(spec.Tags) > 0 {
		kept := make([]json.RawMessage, 0, len(spec.Tags))
		for _, t := range spec.Tags {
			var tag struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(t, &tag); err != nil {
				return doc, fmt.Errorf("apidocs: decode tag: %w", err)
			}
			if tag.Name == "oops" {
				continue
			}
			kept = append(kept, t)
		}
		raw, err := json.Marshal(kept)
		if err != nil {
			return doc, fmt.Errorf("apidocs: encode tags: %w", err)
		}
		doc.Tags = raw
	}
	paths := make(map[string]json.RawMessage, len(spec.Paths))
	for p, v := range spec.Paths {
		if p == "/oops" {
			continue
		}
		paths[p] = v
	}
	doc.Paths = paths
	if len(spec.Components) > 0 && string(spec.Components) != "null" {
		doc.Components = spec.Components
	}
	return doc, nil
}

// Handler serves the OpenAPI document and Swagger UI.
type Handler struct {
	jsonDoc []byte
	yamlDoc []byte
}

// NewHandler pre-renders doc as JSON and YAML.
func NewHandler(doc Document) (*Handler, error) {
	j, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("apidocs: encode json: %w", err)
	}
	var generic any
	dec := json.NewDecoder(bytes.NewReader(j))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("apidocs: decode generic: %w", err)
	}
	var y strings.Builder
	writeYAML(&y, generic, 0)
	return &Handler{jsonDoc: j, yamlDoc: []byte(y.String())}, nil
}

// Register mounts the API documentation routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+APIDocsJSONPath, h.serveJSON)
	mux.HandleFunc("GET "+APIDocsYAMLPath, h.serveYAML)
	mux.HandleFunc("GET "+SwaggerUIPath, serveSwaggerUI)
	mux.HandleFunc("GET "+SwaggerUIRedirect, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, SwaggerUIPath, http.StatusFound)
	})
}

func (h *Handler) serveJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(h.jsonDoc)
}

func (h *Handler) serveYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.oai.openapi")
	_, _ = w.Write(h.yamlDoc)
}

// MIGRATION_NOTE: springdoc bundled swagger-ui via webjars; here the UI assets
// are loaded from the swagger-ui-dist CDN. Vendor them if offline use is needed.
const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
<script>
  window.onload = function () {
    window.ui = SwaggerUIBundle({ url: "` + APIDocsJSONPath + `", dom_id: "#swagger-ui" });
  };
</script>
</body>
</html>
`

func serveSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}

// writeYAML emits a JSON-decoded value as block-style YAML. Strings are
// double-quoted so no YAML special characters need handling.
func writeYAML(b *strings.Builder, v any, indent int) {
	pad := strings.Repeat("  ", indent)
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString(pad + "{}\n")
			return
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(pad + strconv.Quote(k) + ":")
			writeYAMLChild(b, t[k], indent+1)
		}
	case []any:
		if len(t) == 0 {
			b.WriteString(pad + "[]\n")
			return
		}
		for _, item := range t {
			b.WriteString(pad + "-")
			writeYAMLChild(b, item, indent+1)
		}
	default:
		b.WriteString(pad + yamlScalar(t) + "\n")
	}
}

func writeYAMLChild(b *strings.Builder, v any, indent int) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString(" {}\n")
			return
		}
	case []any:
		if len(t) == 0 {
			b.WriteString(" []\n")
			return
		}
	default:
		b.WriteString(" " + yamlScalar(t) + "\n")
		return
	}
	b.WriteString("\n")
	writeYAML(b, v, indent)
}

func yamlScalar(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(t)
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	default:
		return strconv.Quote(fmt.Sprint(t))
	}
}
