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
	"migrated-app/internal/service"
	"migrated-app/internal/store"
)

type fakeClinic struct {
	service.Clinic
	pets       []*model.Pet
	listErr    error
	byID       map[int]*model.Pet
	findErr    error
	saved      []*model.Pet
	saveErr    error
	deleted    []*model.Pet
	deleteErr  error
	findCalled int
}

func (f *fakeClinic) FindAllPets(ctx context.Context) ([]*model.Pet, error) {
	return f.pets, f.listErr
}

func (f *fakeClinic) FindPetByID(ctx context.Context, id int) (*model.Pet, error) {
	f.findCalled++
	if f.findErr != nil {
		return nil, f.findErr
	}
	p, ok := f.byID[id]
	if !ok {
		return nil, fmt.Errorf("find pet %d: %w", id, store.ErrNotFound)
	}
	return p, nil
}

func (f *fakeClinic) SavePet(ctx context.Context, p *model.Pet) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, p)
	return nil
}

func (f *fakeClinic) DeletePet(ctx context.Context, p *model.Pet) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, p)
	return nil
}

func mkPet(t *testing.T, js string) *model.Pet {
	t.Helper()
	var p model.Pet
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		t.Fatalf("building pet: %v", err)
	}
	return &p
}

func rosy(t *testing.T) *model.Pet {
	return mkPet(t, `{"id":3,"name":"Rosy","type":{"id":2,"name":"dog"},"ownerId":2}`)
}

func jewel(t *testing.T) *model.Pet {
	return mkPet(t, `{"id":4,"name":"Jewel","type":{"id":2,"name":"dog"},"ownerId":3}`)
}

func newMux(c service.Clinic, auth Authorizer) *http.ServeMux {
	mux := http.NewServeMux()
	NewPetHandler(c, auth).Register(mux, "")
	return mux
}

func do(mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
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

func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGetPet(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		findErr  error
		wantCode int
		wantID   float64
		wantName string
	}{
		{"found", "/api/pets/3", nil, http.StatusOK, 3, "Rosy"},
		{"not found", "/api/pets/999", nil, http.StatusNotFound, 0, ""},
		{"invalid id", "/api/pets/abc", nil, http.StatusBadRequest, 0, ""},
		{"negative id", "/api/pets/-1", nil, http.StatusBadRequest, 0, ""},
		{"service error", "/api/pets/3", errors.New("boom"), http.StatusInternalServerError, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClinic{byID: map[int]*model.Pet{3: rosy(t)}, findErr: tc.findErr}
			rec := do(newMux(fc, nil), http.MethodGet, tc.path, "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code=%d want %d body=%s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if len(fc.saved) != 0 || len(fc.deleted) != 0 {
				t.Error("GET must not modify data")
			}
			switch tc.wantCode {
			case http.StatusOK:
				if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
					t.Errorf("content-type=%q", ct)
				}
				var m map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
					t.Fatal(err)
				}
				if m["id"] != tc.wantID || m["name"] != tc.wantName {
					t.Errorf("body=%v", m)
				}
			case http.StatusNotFound:
				if rec.Body.Len() != 0 {
					t.Errorf("expected empty body, got %q", rec.Body.String())
				}
			}
		})
	}
}

func TestListPets(t *testing.T) {
	tests := []struct {
		name     string
		pets     func(t *testing.T) []*model.Pet
		err      error
		wantCode int
		wantIDs  []float64
		wantNms  []string
	}{
		{"two pets", func(t *testing.T) []*model.Pet { return []*model.Pet{rosy(t), jewel(t)} }, nil, http.StatusOK, []float64{3, 4}, []string{"Rosy", "Jewel"}},
		{"order preserved", func(t *testing.T) []*model.Pet { return []*model.Pet{jewel(t), rosy(t)} }, nil, http.StatusOK, []float64{4, 3}, []string{"Jewel", "Rosy"}},
		{"empty", func(t *testing.T) []*model.Pet { return nil }, nil, http.StatusNotFound, nil, nil},
		{"error", func(t *testing.T) []*model.Pet { return nil }, errors.New("db"), http.StatusInternalServerError, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClinic{pets: tc.pets(t), listErr: tc.err}
			rec := do(newMux(fc, nil), http.MethodGet, "/api/pets", "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code=%d want %d", rec.Code, tc.wantCode)
			}
			if tc.wantCode == http.StatusNotFound && rec.Body.Len() != 0 {
				t.Errorf("expected empty body, got %q", rec.Body.String())
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("content-type=%q", ct)
			}
			var arr []map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
				t.Fatal(err)
			}
			if len(arr) != len(tc.wantIDs) {
				t.Fatalf("len=%d", len(arr))
			}
			for i := range arr {
				if arr[i]["id"] != tc.wantIDs[i] || arr[i]["name"] != tc.wantNms[i] {
					t.Errorf("[%d]=%v", i, arr[i])
				}
			}
		})
	}
}

const validBody = `{"id":99,"name":"Rosy I","birthDate":"2010-09-07","type":{"id":2,"name":"dog"},"ownerId":77,"visits":[]}`

func TestUpdatePet(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		saveErr  error
		wantCode int
		wantSave bool
	}{
		{"success", "/api/pets/3", validBody, nil, http.StatusNoContent, true},
		{"null name", "/api/pets/3", `{"name":null,"birthDate":"2010-09-07","type":{"id":2,"name":"dog"}}`, nil, http.StatusBadRequest, false},
		{"missing name", "/api/pets/3", `{"birthDate":"2010-09-07","type":{"id":2,"name":"dog"}}`, nil, http.StatusBadRequest, false},
		{"blank name", "/api/pets/3", `{"name":"  ","birthDate":"2010-09-07","type":{"id":2,"name":"dog"}}`, nil, http.StatusBadRequest, false},
		{"too long name", "/api/pets/3", `{"name":"` + strings.Repeat("a", 31) + `","birthDate":"2010-09-07","type":{"id":2,"name":"dog"}}`, nil, http.StatusBadRequest, false},
		{"missing type", "/api/pets/3", `{"name":"x","birthDate":"2010-09-07"}`, nil, http.StatusBadRequest, false},
		{"missing birthDate", "/api/pets/3", `{"name":"x","type":{"id":2,"name":"dog"}}`, nil, http.StatusBadRequest, false},
		{"malformed json", "/api/pets/3", `{bad`, nil, http.StatusBadRequest, false},
		{"bad id", "/api/pets/xyz", validBody, nil, http.StatusBadRequest, false},
		{"not found", "/api/pets/999", validBody, nil, http.StatusNotFound, false},
		{"save error", "/api/pets/3", validBody, errors.New("db"), http.StatusInternalServerError, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClinic{byID: map[int]*model.Pet{3: rosy(t)}, saveErr: tc.saveErr}
			mux := newMux(fc, nil)
			rec := do(mux, http.MethodPut, tc.path, tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code=%d want %d body=%s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantSave != (len(fc.saved) == 1) {
				t.Fatalf("saved=%d wantSave=%v", len(fc.saved), tc.wantSave)
			}
			if !tc.wantSave {
				return
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("content-type=%q", ct)
			}
			m := toMap(t, fc.saved[0])
			if m["id"] != float64(3) {
				t.Errorf("id from body must be ignored, got %v", m["id"])
			}
			if m["name"] != "Rosy I" {
				t.Errorf("name=%v", m["name"])
			}
			if m["ownerId"] != float64(2) {
				t.Errorf("ownerId must be unchanged, got %v", m["ownerId"])
			}
			if m["birthDate"] == nil {
				t.Error("birthDate not set")
			}
			typ, _ := m["type"].(map[string]any)
			if typ == nil || typ["id"] != float64(2) {
				t.Errorf("type=%v", m["type"])
			}
			get := do(mux, http.MethodGet, "/api/pets/3", "")
			if get.Code != http.StatusOK {
				t.Fatalf("subsequent get code=%d", get.Code)
			}
			var gm map[string]any
			_ = json.Unmarshal(get.Body.Bytes(), &gm)
			if gm["id"] != float64(3) || gm["name"] != "Rosy I" {
				t.Errorf("subsequent get=%v", gm)
			}
		})
	}
}

func TestDeletePet(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		delErr   error
		wantCode int
		wantDel  bool
	}{
		{"success", "/api/pets/3", "", nil, http.StatusNoContent, true},
		{"body ignored", "/api/pets/3", `{"garbage":true}`, nil, http.StatusNoContent, true},
		{"not found", "/api/pets/999", "", nil, http.StatusNotFound, false},
		{"bad id", "/api/pets/x", "", nil, http.StatusBadRequest, false},
		{"delete error", "/api/pets/3", "", errors.New("db"), http.StatusInternalServerError, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClinic{byID: map[int]*model.Pet{3: rosy(t)}, deleteErr: tc.delErr}
			rec := do(newMux(fc, nil), http.MethodDelete, tc.path, tc.body)
			if rec.Code != tc.wantCode {
				t.Fatalf("code=%d want %d", rec.Code, tc.wantCode)
			}
			if tc.wantDel != (len(fc.deleted) == 1) {
				t.Fatalf("deleted=%d", len(fc.deleted))
			}
		})
	}
}

func TestAuthorizationRequired(t *testing.T) {
	var gotRoles []string
	deny := func(next http.Handler, roles ...string) http.Handler {
		gotRoles = roles
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
	}
	tests := []struct{ method, path, body string }{
		{http.MethodGet, "/api/pets", ""},
		{http.MethodGet, "/api/pets/3", ""},
		{http.MethodPut, "/api/pets/3", validBody},
		{http.MethodDelete, "/api/pets/3", ""},
	}
	for _, tc := range tests {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			fc := &fakeClinic{byID: map[int]*model.Pet{3: rosy(t)}}
			rec := do(newMux(fc, deny), tc.method, tc.path, tc.body)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("code=%d", rec.Code)
			}
			if len(fc.saved)+len(fc.deleted) != 0 || fc.findCalled != 0 {
				t.Error("service called despite denial")
			}
			if len(gotRoles) != 1 || gotRoles[0] != "OWNER_ADMIN" {
				t.Errorf("roles=%v", gotRoles)
			}
		})
	}
}

func TestPetToV1DTONil(t *testing.T) {
	if petToV1DTO(nil) != nil {
		t.Error("nil in must give nil out")
	}
	d := petToV1DTO(rosy(t))
	if d == nil || d.Visits == nil || len(d.Visits) != 0 {
		t.Errorf("visits=%v", d)
	}
}