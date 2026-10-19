package api

import (
	"testing"
	"time"
)

func ptr(t time.Time) *time.Time { return &t }

func TestValidatePetAge(t *testing.T) {
	now := time.Date(2024, time.June, 15, 13, 45, 0, 0, time.UTC)
	tests := []struct {
		name    string
		bd      *time.Time
		now     time.Time
		wantMsg string
		wantOK  bool
	}{
		{"nil birth date valid", nil, now, "", true},
		{"today valid", ptr(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)), now, "", true},
		{"today later time valid", ptr(time.Date(2024, 6, 15, 23, 59, 0, 0, time.UTC)), now, "", true},
		{"five years ago valid", ptr(time.Date(2019, 6, 15, 0, 0, 0, 0, time.UTC)), now, "", true},
		{"tomorrow future", ptr(time.Date(2024, 6, 16, 0, 0, 0, 0, time.UTC)), now, PetAgeFutureMessage, false},
		{"next year future", ptr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), now, PetAgeFutureMessage, false},
		{"next month future", ptr(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)), now, PetAgeFutureMessage, false},
		{"exactly 50 years valid", ptr(time.Date(1974, 6, 15, 0, 0, 0, 0, time.UTC)), now, "", true},
		{"50 years and a day too old", ptr(time.Date(1974, 6, 14, 0, 0, 0, 0, time.UTC)), now, PetAgeTooOldMessage, false},
		{"51 years too old", ptr(time.Date(1973, 6, 15, 0, 0, 0, 0, time.UTC)), now, PetAgeTooOldMessage, false},
		{"leap day clamp boundary valid", ptr(time.Date(1974, 2, 28, 0, 0, 0, 0, time.UTC)),
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC), "", true},
		{"leap day clamp boundary too old", ptr(time.Date(1974, 2, 27, 0, 0, 0, 0, time.UTC)),
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC), PetAgeTooOldMessage, false},
		{"birth date in own location", ptr(time.Date(2024, 6, 15, 23, 0, 0, 0, time.FixedZone("x", 10*3600))), now, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, ok := validatePetAge(tc.bd, tc.now)
			if msg != tc.wantMsg || ok != tc.wantOK {
				t.Fatalf("got (%q,%v), want (%q,%v)", msg, ok, tc.wantMsg, tc.wantOK)
			}
		})
	}
}

func TestValidatePetAgeNoSideEffects(t *testing.T) {
	bd := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	orig := bd
	validatePetAge(&bd, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	if !bd.Equal(orig) {
		t.Fatal("birth date mutated")
	}
}

func TestValidatePetAgeRealClock(t *testing.T) {
	now := time.Now()
	if _, ok := validatePetAge(ptr(now), now); !ok {
		t.Fatal("now should be valid")
	}
	if msg, ok := validatePetAge(ptr(now.AddDate(0, 0, 1)), now); ok || msg != PetAgeFutureMessage {
		t.Fatalf("tomorrow: got %q,%v", msg, ok)
	}
	if msg, ok := validatePetAge(ptr(now.AddDate(-51, 0, 0)), now); ok || msg != PetAgeTooOldMessage {
		t.Fatalf("51y: got %q,%v", msg, ok)
	}
}

func TestMessages(t *testing.T) {
	tests := []struct{ got, want string }{
		{PetAgeValidationMessage, "Birth date must not be in the future or older than 50 years"},
		{PetAgeFutureMessage, "Birth date cannot be in the future"},
		{PetAgeTooOldMessage, "Birth date cannot be older than 50 years"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("got %q want %q", tc.got, tc.want)
		}
	}
	if petMaxAgeYears != 50 {
		t.Errorf("petMaxAgeYears=%d", petMaxAgeYears)
	}
}

func TestCivilDateBefore(t *testing.T) {
	tests := []struct {
		a, b civilDate
		want bool
	}{
		{civilDate{2020, 1, 1}, civilDate{2021, 1, 1}, true},
		{civilDate{2021, 1, 1}, civilDate{2020, 12, 31}, false},
		{civilDate{2020, 1, 31}, civilDate{2020, 2, 1}, true},
		{civilDate{2020, 2, 1}, civilDate{2020, 2, 2}, true},
		{civilDate{2020, 2, 2}, civilDate{2020, 2, 2}, false},
	}
	for _, tc := range tests {
		if got := tc.a.before(tc.b); got != tc.want {
			t.Errorf("%v.before(%v)=%v", tc.a, tc.b, got)
		}
	}
}

func TestCivilDateMinusYears(t *testing.T) {
	tests := []struct {
		in   civilDate
		n    int
		want civilDate
	}{
		{civilDate{2024, 2, 29}, 1, civilDate{2023, 2, 28}},
		{civilDate{2024, 2, 29}, 4, civilDate{2020, 2, 29}},
		{civilDate{2024, 12, 31}, 50, civilDate{1974, 12, 31}},
		{civilDate{2024, 6, 15}, 50, civilDate{1974, 6, 15}},
	}
	for _, tc := range tests {
		if got := tc.in.minusYears(tc.n); got != tc.want {
			t.Errorf("%v.minusYears(%d)=%v want %v", tc.in, tc.n, got, tc.want)
		}
	}
}