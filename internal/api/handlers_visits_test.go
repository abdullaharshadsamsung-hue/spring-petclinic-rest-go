package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

type stubVisitService struct {
	visits    map[int]*model.Visit
	findErr   error
	saveErr   error
	saved     []*model.Visit
	deleted   []*model.Visit
	listCalls int
}

func newStubVisitService() *stubVisitService {
	return &stubVisitService{visits: map[int]*model.Visit{}}
}

func (s *stubVisitService) put(id int, desc string, petID int) {
	v := &model.Visit{}
	v.SetDescription(desc)
	p := petID
	v.PetID = &p
	s.visits[id] = v
}

func (s *stubVisitService) FindAllVisits(ctx context.Context) ([]*model.Visit, error) {
	s.listCalls++
	var out []*model.Visit
	for _, v := range s.visits {
		out = append(out, v)
	}
	return out, nil
}

func (s *stubVisitService) FindVisitByID(ctx context.Context, id int) (*model.Visit, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	v, ok := s.visits[id]
	if !ok {
		return nil, fmt.Errorf("visit %d: %w", id, store.ErrNotFound)
	}
	return v, nil
}

func (s *stubVisitService) SaveVisit(ctx context.Context, v *model.Visit) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, v)
	return nil
}

func (s *stubVisitService) DeleteVisit(ctx context.Context, v *model.Visit) error {
	s.deleted = append(s.deleted, v)
	return nil
}

func serveVisit(svc VisitService, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewVisitHandler(svc, nil).Register(mux, "")
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestVisitListEmptyIs404(t *testing.T) {
	rec := serveVisit(newStubVisitService(), "GET", "/api/visits", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestVisitListOK(t *testing.T) {
	s := newStubVisitService()
	s.put(1, "shot", 7)
	rec := serveVisit(s, "GET", "/api/visits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got []visitV1DTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Description == nil || *got[0].Description != "shot" || got[0].PetID == nil || *got[0].PetID != 7 {
		t.Fatalf("got %+v", got)
	}
}

func TestVisitGet(t *testing.T) {
	s := newStubVisitService()
	s.put(2, "neutered", 8)
	cases := []struct {
		path   string
		status int
	}{
		{"/api/visits/2", http.StatusOK},
		{"/api/visits/99", http.StatusNotFound},
		{"/api/visits/abc", http.StatusBadRequest},
		{"/api/visits/-1", http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := serveVisit(s, "GET", c.path, "")
		if rec.Code != c.status {
			t.Errorf("%s: status = %d, want %d", c.path, rec.Code, c.status)
		}
	}
}

func TestVisitGetServiceError(t *testing.T) {
	s := newStubVisitService()
	s.findErr = errors.New("db down")
	rec := serveVisit(s, "GET", "/api/visits/1", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestVisitAdd(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"valid", `{"description":"shot","petId":7}`, http.StatusCreated},
		{"null desc", `{"petId":7}`, http.StatusBadRequest},
		{"empty desc", `{"description":"","petId":7}`, http.StatusBadRequest},
		{"long desc", `{"description":"` + strings.Repeat("a", 256) + `","petId":7}`, http.StatusBadRequest},
		{"no pet", `{"description":"x"}`, http.StatusBadRequest},
		{"neg pet", `{"description":"x","petId":-1}`, http.StatusBadRequest},
		{"malformed", `{`, http.StatusBadRequest},
	}
	for _, c := range cases {
		s := newStubVisitService()
		rec := serveVisit(s, "POST", "/api/visits", c.body)
		if rec.Code != c.status {
			t.Errorf("%s: status = %d, want %d", c.name, rec.Code, c.status)
		}
		if c.status == http.StatusCreated && len(s.saved) != 1 {
			t.Errorf("%s: expected one save", c.name)
		}
		if c.status != http.StatusCreated && len(s.saved) != 0 {
			t.Errorf("%s: unexpected save", c.name)
		}
	}
}

func TestVisitAddIntegrityError(t *testing.T) {
	s := newStubVisitService()
	s.saveErr = fmt.Errorf("fk: %w", ErrIntegrity)
	rec := serveVisit(s, "POST", "/api/visits", `{"description":"x","petId":5}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestVisitUpdate(t *testing.T) {
	s := newStubVisitService()
	s.put(3, "old", 8)
	rec := serveVisit(s, "PUT", "/api/visits/3", `{"description":"new"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if s.visits[3].GetDescription() != "new" {
		t.Errorf("description = %q", s.visits[3].GetDescription())
	}
	if p := s.visits[3].GetPetID(); p == nil || *p != 8 {
		t.Errorf("pet changed")
	}
	if rec := serveVisit(s, "PUT", "/api/visits/99", `{"description":"x"}`); rec.Code != http.StatusNotFound {
		t.Errorf("missing: status = %d", rec.Code)
	}
	if rec := serveVisit(s, "PUT", "/api/visits/3", `{"description":""}`); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid: status = %d", rec.Code)
	}
}

func TestVisitDelete(t *testing.T) {
	s := newStubVisitService()
	s.put(4, "x", 1)
	if rec := serveVisit(s, "DELETE", "/api/visits/4", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(s.deleted) != 1 {
		t.Fatalf("deleted = %d", len(s.deleted))
	}
	if rec := serveVisit(s, "DELETE", "/api/visits/99", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing: status = %d", rec.Code)
	}
}

func TestValidateVisitFields(t *testing.T) {
	ok := "fine"
	if validateVisitFields(&ok).HasErrors() {
		t.Error("unexpected errors")
	}
	if !validateVisitFields(nil).HasErrors() {
		t.Error("expected error for nil")
	}
	maxDesc := strings.Repeat("é", 255)
	if validateVisitFields(&maxDesc).HasErrors() {
		t.Error("255 runes should be valid")
	}
}