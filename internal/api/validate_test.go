package api

import (
	"testing"
	"time"
)

func petAgeTestDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func petAgeTestPtr(t time.Time) *time.Time { return &t }

func TestValidatePetAgeRules(t *testing.T) {
	date := petAgeTestDate
	ptr := petAgeTestPtr
	now := time.Date(2024, time.June, 15, 13, 45, 0, 0, time.UTC)
	tests := []struct {
		name    string
		bd      *time.Time
		now     time.Time
		wantMsg string
		wantOK  bool
	}{
		{"nil birth date", nil, now, "", true},
		{"future one day", ptr(date(2024, 6, 16)), now, PetAgeFutureMessage, false},
		{"far future", ptr(date(2100, 1, 1)), now, PetAgeFutureMessage, false},
		{"too old 51 years", ptr(date(1973, 6, 15)), now, PetAgeTooOldMessage, false},
		{"valid 10 years", ptr(date(2014, 6, 15)), now, "", true},
		{"today", ptr(date(2024, 6, 15)), now, "", true},
		{"today later time of day", ptr(time.Date(2024, 6, 15, 23, 59, 0, 0, time.UTC)), now, "", true},
		{"exactly 50 years", ptr(date(1974, 6, 15)), now, "", true},
		{"50 years plus 1 day", ptr(date(1974, 6, 14)), now, PetAgeTooOldMessage, false},
		{"leap day clamp exact", ptr(date(1974, 2, 28)), time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), "", true},
		{"leap day clamp minus one", ptr(date(1974, 2, 27)), time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), PetAgeTooOldMessage, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := validatePetAge(tt.bd, tt.now)
			if ok != tt.wantOK || msg != tt.wantMsg {
				t.Fatalf("got (%q,%v), want (%q,%v)", msg, ok, tt.wantMsg, tt.wantOK)
			}
			if msg == PetAgeValidationMessage {
				t.Fatal("default message must never be reported")
			}
		})
	}
}

func TestValidatePetBirthDateRecordsErrors(t *testing.T) {
	date := petAgeTestDate
	ptr := petAgeTestPtr
	now := date(2024, 6, 15)
	tests := []struct {
		name    string
		bd      *time.Time
		nilErrs bool
		wantOK  bool
		wantMsg string
		wantRej string
	}{
		{"nil date", nil, false, true, "", ""},
		{"valid", ptr(date(2020, 1, 1)), false, true, "", ""},
		{"future", ptr(date(2024, 6, 16)), false, false, PetAgeFutureMessage, "2024-06-16"},
		{"too old", ptr(date(1970, 1, 2)), false, false, PetAgeTooOldMessage, "1970-01-02"},
		{"invalid nil errs", ptr(date(2030, 1, 1)), true, false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errs *ValidationError
			if !tt.nilErrs {
				errs = NewValidationError()
			}
			ok := ValidatePetBirthDate(errs, "birthDate", tt.bd, now)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want %v", ok, tt.wantOK)
			}
			if tt.nilErrs {
				return
			}
			if tt.wantOK {
				if errs.HasErrors() {
					t.Fatalf("unexpected errors %v", errs.Errors)
				}
				return
			}
			if len(errs.Errors) != 1 {
				t.Fatalf("want exactly 1 error, got %d", len(errs.Errors))
			}
			fe := errs.Errors[0]
			if fe.Field != "birthDate" || fe.DefaultMessage != tt.wantMsg || fe.RejectedValue != tt.wantRej {
				t.Fatalf("got %+v", fe)
			}
		})
	}
}