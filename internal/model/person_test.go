package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPersonValidate(t *testing.T) {
	tests := []struct {
		name          string
		person        *Person
		wantFirst     bool
		wantLast      bool
		wantNil       bool
		wantViolCount int
	}{
		{"firstName empty, lastName smith", &Person{FirstName: "", LastName: "smith"}, true, false, false, 1},
		{"lastName empty, firstName set", &Person{FirstName: "George", LastName: ""}, false, true, false, 1},
		{"firstName zero value (null), lastName set", &Person{LastName: "smith"}, true, false, false, 1},
		{"lastName zero value (null), firstName set", &Person{FirstName: "George"}, false, true, false, 1},
		{"both empty", &Person{FirstName: "", LastName: ""}, true, true, false, 2},
		{"both zero value", &Person{}, true, true, false, 2},
		{"nil person", nil, true, true, false, 2},
		{"both non-empty", &Person{FirstName: "George", LastName: "Franklin"}, false, false, true, 0},
		{"whitespace-only names pass", &Person{FirstName: " ", LastName: " "}, false, false, true, 0},
		{"id not validated (nil id)", &Person{BaseEntity: BaseEntity{ID: nil}, FirstName: "a", LastName: "b"}, false, false, true, 0},
		{"id not validated (zero id)", &Person{BaseEntity: BaseEntity{ID: IntPtr(0)}, FirstName: "a", LastName: "b"}, false, false, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.person.Validate()
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := errors.Is(err, ErrFirstNameEmpty); got != tt.wantFirst {
				t.Errorf("errors.Is(ErrFirstNameEmpty) = %v, want %v", got, tt.wantFirst)
			}
			if got := errors.Is(err, ErrLastNameEmpty); got != tt.wantLast {
				t.Errorf("errors.Is(ErrLastNameEmpty) = %v, want %v", got, tt.wantLast)
			}
			count := 0
			if u, ok := err.(interface{ Unwrap() []error }); ok {
				count = len(u.Unwrap())
			} else {
				count = 1
			}
			if count != tt.wantViolCount {
				t.Errorf("violation count = %d, want %d", count, tt.wantViolCount)
			}
		})
	}
}

func TestPersonValidationMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		property string
	}{
		{"firstName", ErrFirstNameEmpty, "firstName"},
		{"lastName", ErrLastNameEmpty, "lastName"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			parts := strings.SplitN(msg, ": ", 2)
			if len(parts) != 2 {
				t.Fatalf("unexpected message format %q", msg)
			}
			if parts[0] != tt.property {
				t.Errorf("property = %q, want %q", parts[0], tt.property)
			}
			if parts[1] != "must not be empty" {
				t.Errorf("message = %q, want %q", parts[1], "must not be empty")
			}
		})
	}
}

func TestPersonFirstNameAccessors(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"George", "George"},
		{"empty", ""},
		{"untrimmed", "  George  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Person{}
			p.SetFirstName(tt.value)
			if got := p.GetFirstName(); got != tt.value {
				t.Errorf("GetFirstName() = %q, want %q", got, tt.value)
			}
		})
	}
}

func TestPersonLastNameAccessors(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"smith", "smith"},
		{"empty", ""},
		{"untrimmed", " smith "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Person{}
			p.SetLastName(tt.value)
			if got := p.GetLastName(); got != tt.value {
				t.Errorf("GetLastName() = %q, want %q", got, tt.value)
			}
		})
	}
}

func TestPersonDefaults(t *testing.T) {
	p := &Person{}
	if p.GetFirstName() != "" || p.GetLastName() != "" {
		t.Error("expected empty defaults")
	}
	if !p.IsNew() || p.GetID() != nil {
		t.Error("expected new person with nil id")
	}
	var np *Person
	if np.GetFirstName() != "" || np.GetLastName() != "" {
		t.Error("nil person getters should return empty")
	}
}

func TestPersonSettersDoNotValidate(t *testing.T) {
	p := &Person{FirstName: "a", LastName: "b"}
	p.SetFirstName("")
	p.SetLastName("")
	if p.FirstName != "" || p.LastName != "" {
		t.Error("setters should store empty values as given")
	}
	err := p.Validate()
	if !errors.Is(err, ErrFirstNameEmpty) || !errors.Is(err, ErrLastNameEmpty) {
		t.Errorf("expected both violations after explicit Validate, got %v", err)
	}
}

func TestPersonInheritsID(t *testing.T) {
	p := &Person{}
	p.SetID(IntPtr(7))
	if p.GetID() == nil || *p.GetID() != 7 || p.IsNew() {
		t.Error("expected inherited id 7")
	}
}

func TestPersonJSON(t *testing.T) {
	tests := []struct {
		name string
		p    Person
		want string
	}{
		{"new", Person{FirstName: "George", LastName: "smith"}, `{"id":null,"firstName":"George","lastName":"smith"}`},
		{"with id", Person{BaseEntity: BaseEntity{ID: IntPtr(1)}, FirstName: "a", LastName: "b"}, `{"id":1,"firstName":"a","lastName":"b"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.p)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.want {
				t.Errorf("got %s, want %s", b, tt.want)
			}
		})
	}
}