package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store"
)

type ownerTestClinic struct {
	service.Clinic
	findAllCalled bool
	lastName      *string
}

func (c *ownerTestClinic) FindAllOwners(_ context.Context) ([]*model.Owner, error) {
	c.findAllCalled = true
	return nil, nil
}

func (c *ownerTestClinic) FindOwnerByLastName(_ context.Context, ln string) ([]*model.Owner, error) {
	c.lastName = &ln
	return nil, nil
}

func (c *ownerTestClinic) FindOwnerByID(_ context.Context, _ int) (*model.Owner, error) {
	return nil, store.ErrNotFound
}

func ownerTestServe(c *ownerTestClinic, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewOwnerHandler(c, nil).Register(mux, "")
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestOwnerListEmptyReturns404(t *testing.T) {
	c := &ownerTestClinic{}
	rec := ownerTestServe(c, "GET", "/api/owners", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
	if !c.findAllCalled {
		t.Fatal("FindAllOwners not called")
	}
}

func TestOwnerListByLastName(t *testing.T) {
	c := &ownerTestClinic{}
	rec := ownerTestServe(c, "GET", "/api/owners?lastName=Davis", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if c.lastName == nil || *c.lastName != "Davis" {
		t.Fatalf("FindOwnerByLastName not called with Davis")
	}
}

func TestOwnerEndpointsStatuses(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"get missing", "GET", "/api/owners/5", "", http.StatusNotFound},
		{"get bad id", "GET", "/api/owners/abc", "", http.StatusBadRequest},
		{"delete missing", "DELETE", "/api/owners/5", "", http.StatusNotFound},
		{"delete bad id", "DELETE", "/api/owners/x", "", http.StatusBadRequest},
		{"post malformed", "POST", "/api/owners", "{not json", http.StatusBadRequest},
		{"get pet owner missing", "GET", "/api/owners/5/pets/3", "", http.StatusNotFound},
		{"get pet bad pet id", "GET", "/api/owners/5/pets/zz", "", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := ownerTestServe(&ownerTestClinic{}, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}