package store

import (
	"errors"
	"strings"
	"testing"
)

var errDB = errors.New("db down")

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
		substr string
	}{
		{"not found", ErrNotFound, ErrNotFound, true, "not found"},
		{"integrity no cause", IntegrityViolation("fk", nil), ErrIntegrity, true, "fk"},
		{"integrity cause", IntegrityViolation("fk", errDB), ErrIntegrity, true, "db down"},
		{"integrity is not notfound", IntegrityViolation("fk", nil), ErrNotFound, false, "integrity"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if errors.Is(tc.err, tc.target) != tc.want {
				t.Fatalf("errors.Is mismatch for %v", tc.err)
			}
			if !strings.Contains(tc.err.Error(), tc.substr) {
				t.Fatalf("%q missing %q", tc.err.Error(), tc.substr)
			}
		})
	}
}

func TestSortOrderString(t *testing.T) {
	tests := []struct {
		o    SortOrder
		want string
	}{
		{SortOrder{Property: "name"}, "name: ASC"},
		{SortOrder{Property: "name", Descending: true}, "name: DESC"},
	}
	for _, tc := range tests {
		if got := tc.o.String(); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

func TestNewPageRequest(t *testing.T) {
	tests := []struct {
		name       string
		page, size int
		sort       []SortOrder
		wantErr    bool
		wantOffset int
	}{
		{"valid", 2, 5, nil, false, 10},
		{"valid sort", 0, 1, []SortOrder{{Property: "name"}}, false, 0},
		{"neg page", -1, 5, nil, true, 0},
		{"zero size", 0, 0, nil, true, 0},
		{"empty sort", 0, 5, []SortOrder{{Property: "  "}}, true, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, err := NewPageRequest(tc.page, tc.size, tc.sort...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
			if err != nil {
				return
			}
			if pr.Offset() != tc.wantOffset || pr.Sort == nil || len(pr.Sort) != len(tc.sort) {
				t.Fatalf("bad request %+v", pr)
			}
		})
	}
}

func TestNewPage(t *testing.T) {
	tests := []struct {
		name      string
		content   []int
		req       PageRequest
		total     int64
		wantPages int
	}{
		{"exact", []int{1, 2}, PageRequest{Page: 0, Size: 2}, 4, 2},
		{"remainder", []int{1}, PageRequest{Page: 2, Size: 2}, 5, 3},
		{"empty", nil, PageRequest{Page: 0, Size: 2}, 0, 0},
		{"zero size", nil, PageRequest{}, 3, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPage(tc.content, tc.req, tc.total)
			if p.Content == nil {
				t.Fatal("nil content")
			}
			if p.TotalPages != tc.wantPages || p.TotalElements != tc.total || p.Number != tc.req.Page || p.Size != tc.req.Size {
				t.Fatalf("bad page %+v", p)
			}
		})
	}
}

type testItem struct{ name string }

func TestNewPageWithPointers(t *testing.T) {
	it := &testItem{name: "cat"}
	p := NewPage([]*testItem{it}, PageRequest{Page: 0, Size: 1}, 1)
	if len(p.Content) != 1 || p.Content[0] != it || p.TotalPages != 1 {
		t.Fatalf("bad page %+v", p)
	}
	nilPage := NewPage[*testItem](nil, PageRequest{Size: 1}, 0)
	if nilPage.Content == nil || len(nilPage.Content) != 0 {
		t.Fatal("content should be empty, not nil")
	}
}

func TestInterfacesUsable(t *testing.T) {
	var (
		_ PetRepository     = nil
		_ OwnerRepository   = nil
		_ PetTypeRepository = nil
	)
}