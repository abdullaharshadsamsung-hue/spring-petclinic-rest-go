package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pdCustomIntegrityErr struct {
	msg   string
	flag  bool
	title string
}

func (e *pdCustomIntegrityErr) Error() string            { return e.msg }
func (e *pdCustomIntegrityErr) IntegrityViolation() bool { return e.flag }
func (e *pdCustomIntegrityErr) Title() string            { return e.title }

type pdTitledErr struct{ title string }

func (e *pdTitledErr) Error() string { return "titled: secret internal" }
func (e *pdTitledErr) Title() string { return e.title }

func pdDecodeProblem(t *testing.T, rec *httptest.ResponseRecorder) (Problem, map[string]any) {
	t.Helper()
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v; body=%s", err, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	return p, raw
}

func pdCheckCommon(t *testing.T, rec *httptest.ResponseRecorder, p Problem, raw map[string]any, wantStatus int, wantType string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status code = %d, want %d", rec.Code, wantStatus)
	}
	if p.Status != rec.Code {
		t.Errorf("body status %d != response status %d", p.Status, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("content-type = %q", ct)
	}
	if p.Type != wantType {
		t.Errorf("type = %q, want %q", p.Type, wantType)
	}
	for _, k := range []string{"type", "title", "status", "detail", "timestamp", "schemaValidationErrors"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	if _, ok := raw["schemaValidationErrors"].([]any); !ok {
		t.Errorf("schemaValidationErrors is not an array: %v", raw["schemaValidationErrors"])
	}
	ts, err := time.Parse(time.RFC3339Nano, p.Timestamp)
	if err != nil {
		t.Errorf("timestamp parse: %v", err)
	} else if d := time.Since(ts); d < -time.Minute || d > time.Minute {
		t.Errorf("timestamp not current: %v", ts)
	}
}

func TestProblemDetailHandleErrorNonValidation(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantTitle  string
		wantDetail string
		secret     string
	}{
		{"generic runtime", errors.New("boom secret-db-password"), 500, titleDefault, errUnexpected, "secret-db-password"},
		{"wrapped generic", fmt.Errorf("ctx: %w", errors.New("internal path /etc/x")), 500, titleDefault, errUnexpected, "/etc/x"},
		{"titled generic", &pdTitledErr{title: "IllegalStateException"}, 500, "IllegalStateException", errUnexpected, "secret internal"},
		{"titled empty falls back", &pdTitledErr{title: ""}, 500, titleDefault, errUnexpected, "secret internal"},
		{"sentinel integrity", ErrIntegrity, 404, titleDataIntegrity, errDataIntegrity, ""},
		{"wrapped integrity", fmt.Errorf("fk owner_id=99 missing: %w", ErrIntegrity), 404, titleDataIntegrity, errDataIntegrity, "owner_id=99"},
		{"interface integrity subclass title", &pdCustomIntegrityErr{msg: "constraint pk_x", flag: true, title: "DuplicateKeyException"}, 404, "DuplicateKeyException", errDataIntegrity, "pk_x"},
		{"interface integrity default title", &pdCustomIntegrityErr{msg: "constraint fk_y", flag: true}, 404, titleDataIntegrity, errDataIntegrity, "fk_y"},
		{"interface flag false -> 500", &pdCustomIntegrityErr{msg: "not really zz", flag: false}, 500, titleDefault, errUnexpected, "zz"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://example.com/petclinic/api/owners?x=1", nil)
			rec := httptest.NewRecorder()
			HandleError(rec, req, tc.err)
			p, raw := pdDecodeProblem(t, rec)
			pdCheckCommon(t, rec, p, raw, tc.wantStatus, "http://example.com/petclinic/api/owners")
			if p.Title != tc.wantTitle {
				t.Errorf("title = %q, want %q", p.Title, tc.wantTitle)
			}
			if p.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", p.Detail, tc.wantDetail)
			}
			if len(p.SchemaValidationErrors) != 0 {
				t.Errorf("expected empty schemaValidationErrors, got %v", p.SchemaValidationErrors)
			}
			if tc.secret != "" && strings.Contains(rec.Body.String(), tc.secret) {
				t.Errorf("body leaks error message: %s", rec.Body.String())
			}
		})
	}
}

func TestProblemDetailHandleErrorNilWritesNothing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	HandleError(rec, req, nil)
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Errorf("expected no response, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestProblemDetailHandleErrorValidation(t *testing.T) {
	n := -1
	var nilInt *int
	s := "abc"
	tests := []struct {
		name string
		err  error
		want []ValidationMessage
	}{
		{
			name: "blank first name",
			err:  NewValidationError(FieldError{Field: "firstName", RejectedValue: "", DefaultMessage: "must not be blank"}),
			want: []ValidationMessage{{Message: "Field 'firstName' must not be blank (rejected value: )", Field: "firstName", RejectedValue: "", DefaultMessage: "must not be blank"}},
		},
		{
			name: "null rejected value",
			err:  NewValidationError(FieldError{Field: "lastName", RejectedValue: nil, DefaultMessage: "must not be null"}),
			want: []ValidationMessage{{Message: "Field 'lastName' must not be null (rejected value: null)", Field: "lastName", RejectedValue: "null", DefaultMessage: "must not be null"}},
		},
		{
			name: "null default message",
			err:  NewValidationError(FieldError{Field: "x", RejectedValue: "v"}),
			want: []ValidationMessage{{Message: "Field 'x' Validation failed (rejected value: v)", Field: "x", RejectedValue: "v", DefaultMessage: "Validation failed"}},
		},
		{
			name: "integer rejected value",
			err:  NewValidationError(FieldError{Field: "age", RejectedValue: -1, DefaultMessage: "must be positive"}),
			want: []ValidationMessage{{Message: "Field 'age' must be positive (rejected value: -1)", Field: "age", RejectedValue: "-1", DefaultMessage: "must be positive"}},
		},
		{
			name: "pointer values and order preserved",
			err: NewValidationError(
				FieldError{Field: "a", RejectedValue: &n, DefaultMessage: "m1"},
				FieldError{Field: "b", RejectedValue: nilInt, DefaultMessage: "m2"},
				FieldError{Field: "c", RejectedValue: &s, DefaultMessage: "m3"},
			),
			want: []ValidationMessage{
				{Message: "Field 'a' m1 (rejected value: -1)", Field: "a", RejectedValue: "-1", DefaultMessage: "m1"},
				{Message: "Field 'b' m2 (rejected value: null)", Field: "b", RejectedValue: "null", DefaultMessage: "m2"},
				{Message: "Field 'c' m3 (rejected value: abc)", Field: "c", RejectedValue: "abc", DefaultMessage: "m3"},
			},
		},
		{name: "no errors", err: NewValidationError(), want: nil},
		{name: "only global errors (no field errors)", err: &ValidationError{}, want: nil},
		{
			name: "wrapped validation",
			err:  fmt.Errorf("binding: %w", NewValidationError(FieldError{Field: "f", RejectedValue: 1, DefaultMessage: "bad"})),
			want: []ValidationMessage{{Message: "Field 'f' bad (rejected value: 1)", Field: "f", RejectedValue: "1", DefaultMessage: "bad"}},
		},
		{
			name: "validation wins over integrity",
			err:  errors.Join(NewValidationError(FieldError{Field: "f", RejectedValue: "z", DefaultMessage: "bad"}), ErrIntegrity),
			want: []ValidationMessage{{Message: "Field 'f' bad (rejected value: z)", Field: "f", RejectedValue: "z", DefaultMessage: "bad"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "http://localhost:9966/petclinic/api/owners/1", nil)
			rec := httptest.NewRecorder()
			HandleError(rec, req, tc.err)
			p, raw := pdDecodeProblem(t, rec)
			pdCheckCommon(t, rec, p, raw, http.StatusBadRequest, "http://localhost:9966/petclinic/api/owners/1")
			if p.Title != titleNotValid {
				t.Errorf("title = %q", p.Title)
			}
			if p.Detail != errInvalidRequest {
				t.Errorf("detail = %q", p.Detail)
			}
			if len(p.SchemaValidationErrors) != len(tc.want) {
				t.Fatalf("got %d entries, want %d: %v", len(p.SchemaValidationErrors), len(tc.want), p.SchemaValidationErrors)
			}
			for i := range tc.want {
				if p.SchemaValidationErrors[i] != tc.want[i] {
					t.Errorf("entry %d = %+v, want %+v", i, p.SchemaValidationErrors[i], tc.want[i])
				}
			}
			arr := raw["schemaValidationErrors"].([]any)
			for i, e := range arr {
				m := e.(map[string]any)
				for _, k := range []string{"message", "field", "rejectedValue", "defaultMessage"} {
					if v, ok := m[k]; !ok || v == nil {
						t.Errorf("entry %d key %q missing or null", i, k)
					}
				}
			}
		})
	}
}

func TestProblemDetailValidationErrorHelpers(t *testing.T) {
	var nilErr *ValidationError
	if nilErr.HasErrors() {
		t.Error("nil should have no errors")
	}
	v := NewValidationError()
	if v.HasErrors() {
		t.Error("empty should have no errors")
	}
	v.Add("name", "", "")
	v.Add("age", 3, "too young")
	if !v.HasErrors() || len(v.Errors) != 2 {
		t.Fatalf("Add failed: %+v", v.Errors)
	}
	want := "validation failed: name: Validation failed; age: too young"
	if v.Error() != want {
		t.Errorf("Error() = %q, want %q", v.Error(), want)
	}
	if v.Title() != titleNotValid {
		t.Errorf("Title() = %q", v.Title())
	}
}

func TestProblemDetailRejectedString(t *testing.T) {
	i := 42
	pi := &i
	var nilPtr *string
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "null"},
		{"string", "x", "x"},
		{"empty", "", ""},
		{"int", -1, "-1"},
		{"bool", true, "true"},
		{"ptr int", &i, "42"},
		{"ptr ptr", &pi, "42"},
		{"nil ptr", nilPtr, "null"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rejectedString(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProblemDetailRequestURL(t *testing.T) {
	tests := []struct {
		name string
		req  func() *http.Request
		want string
	}{
		{"http strips query", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "http://h:8080/a/b?q=1", nil)
		}, "http://h:8080/a/b"},
		{"https", func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "http://secure.io/p", nil)
			r.TLS = &tls.ConnectionState{}
			return r
		}, "https://secure.io/p"},
		{"escaped path", func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "http://h/a%20b", nil)
		}, "http://h/a%20b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := requestURL(tc.req()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProblemDetailWriteProblemNilValidationRenderedAsEmptyArray(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://h/x", nil)
	rec := httptest.NewRecorder()
	WriteProblem(rec, req, http.StatusTeapot, "T", "D", nil)
	p, raw := pdDecodeProblem(t, rec)
	pdCheckCommon(t, rec, p, raw, http.StatusTeapot, "http://h/x")
	if !strings.Contains(rec.Body.String(), `"schemaValidationErrors":[]`) {
		t.Errorf("expected empty array, body=%s", rec.Body.String())
	}
	if p.Title != "T" || p.Detail != "D" {
		t.Errorf("title/detail = %q/%q", p.Title, p.Detail)
	}
}