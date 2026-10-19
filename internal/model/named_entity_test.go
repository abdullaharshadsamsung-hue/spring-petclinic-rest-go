package model

import (
	"testing"
)

func TestNamedEntity_GetName(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *NamedEntity
		want  string
	}{
		{"new entity has zero name", func() *NamedEntity { return &NamedEntity{} }, ""},
		{"nil receiver returns empty", func() *NamedEntity { return nil }, ""},
		{"after SetName dog", func() *NamedEntity { e := &NamedEntity{}; e.SetName("dog"); return e }, "dog"},
		{"after SetName empty", func() *NamedEntity { e := &NamedEntity{Name: "x"}; e.SetName(""); return e }, ""},
		{"no trimming", func() *NamedEntity { e := &NamedEntity{}; e.SetName("  a b  "); return e }, "  a b  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.setup()
			if got := e.GetName(); got != tt.want {
				t.Fatalf("GetName() = %q, want %q", got, tt.want)
			}
			if got := e.GetName(); got != tt.want {
				t.Fatalf("second GetName() = %q, want %q (side effect)", got, tt.want)
			}
		})
	}
}