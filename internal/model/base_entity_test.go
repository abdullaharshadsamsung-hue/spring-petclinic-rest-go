package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetID(t *testing.T) {
	tests := []struct {
		name string
		e    *BaseEntity
		want *int
	}{
		{"fresh entity returns nil", &BaseEntity{}, nil},
		{"id set to 42", func() *BaseEntity { e := &BaseEntity{}; e.SetID(IntPtr(42)); return e }(), IntPtr(42)},
		{"generated id", &BaseEntity{ID: IntPtr(1001)}, IntPtr(1001)},
		{"nil receiver returns nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.e.GetID()
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("GetID() = %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Fatalf("GetID() = %d, want %d", *got, *tt.want)
			}
			// no side effects
			again := tt.e.GetID()
			if again != got {
				t.Fatalf("GetID changed state")
			}
		})
	}
}

// simulated repository persisting entity and assigning generated ID
type idGenerator interface{ NextID() int }

type fakeDB struct{ next int }

func (f *fakeDB) NextID() int { f.next++; return f.next }

func persist(g idGenerator, e *BaseEntity) { e.SetID(IntPtr(g.NextID())) }

func TestGetIDAfterPersist(t *testing.T) {
	db := &fakeDB{next: 9}
	e := &BaseEntity{}
	if !e.IsNew() {
		t.Fatal("expected new before persist")
	}
	persist(db, e)
	if e.GetID() == nil || *e.GetID() != 10 {
		t.Fatalf("got %v, want 10", e.GetID())
	}
	if e.IsNew() {
		t.Fatal("expected not new after persist")
	}
}

func TestSetID(t *testing.T) {
	tests := []struct {
		name    string
		initial *int
		set     *int
		wantNew bool
	}{
		{"non-nil 7", nil, IntPtr(7), false},
		{"nil clears previous id", IntPtr(5), nil, true},
		{"zero stored", nil, IntPtr(0), false},
		{"negative stored", nil, IntPtr(-1), false},
		{"overwrite existing", IntPtr(3), IntPtr(8), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &BaseEntity{ID: tt.initial}
			e.SetID(tt.set)
			got := e.GetID()
			if tt.set == nil {
				if got != nil {
					t.Fatalf("GetID() = %v, want nil", *got)
				}
			} else if got == nil || *got != *tt.set {
				t.Fatalf("GetID() = %v, want %d", got, *tt.set)
			}
			if e.IsNew() != tt.wantNew {
				t.Fatalf("IsNew() = %v, want %v", e.IsNew(), tt.wantNew)
			}
			if e.IsNew() != (e.GetID() == nil) {
				t.Fatal("invariant IsNew == (GetID==nil) violated")
			}
		})
	}
}

func TestIsNew(t *testing.T) {
	tests := []struct {
		name string
		e    *BaseEntity
		want bool
	}{
		{"nil id", &BaseEntity{}, true},
		{"nil receiver", nil, true},
		{"positive id", &BaseEntity{ID: IntPtr(1)}, false},
		{"zero id", &BaseEntity{ID: IntPtr(0)}, false},
		{"negative id", &BaseEntity{ID: IntPtr(-5)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.IsNew(); got != tt.want {
				t.Fatalf("IsNew() = %v, want %v", got, tt.want)
			}
			if tt.e.IsNew() != (tt.e.GetID() == nil) {
				t.Fatal("invariant violated")
			}
		})
	}
}

func TestJSONSerialization(t *testing.T) {
	type owner struct {
		BaseEntity
		Name string `json:"name"`
	}
	tests := []struct {
		name   string
		v      any
		wantID string
	}{
		{"unassigned id null", BaseEntity{}, `"id":null`},
		{"assigned id", BaseEntity{ID: IntPtr(42)}, `"id":42`},
		{"zero id", BaseEntity{ID: IntPtr(0)}, `"id":0`},
		{"embedded", owner{BaseEntity: BaseEntity{ID: IntPtr(3)}, Name: "x"}, `"id":3`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatal(err)
			}
			s := string(b)
			if !strings.Contains(s, tt.wantID) {
				t.Fatalf("json %s missing %s", s, tt.wantID)
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if _, ok := m["new"]; ok {
				t.Fatalf("json %s contains 'new'", s)
			}
			if _, ok := m["isNew"]; ok {
				t.Fatalf("json %s contains 'isNew'", s)
			}
		})
	}
}

func TestIntPtr(t *testing.T) {
	for _, v := range []int{0, 1, -1, 42} {
		p := IntPtr(v)
		if p == nil || *p != v {
			t.Fatalf("IntPtr(%d) = %v", v, p)
		}
	}
	if IntPtr(1) == IntPtr(1) {
		t.Fatal("expected distinct pointers")
	}
}

func TestNoCustomEquality(t *testing.T) {
	a := &BaseEntity{ID: IntPtr(1)}
	b := &BaseEntity{ID: IntPtr(1)}
	if a == b {
		t.Fatal("distinct entities should differ by reference")
	}
}