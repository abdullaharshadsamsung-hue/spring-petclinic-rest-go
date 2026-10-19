package api

import "time"

// Pet age validation messages. They are the port of the @PetAgeValidation
// constraint annotation (default message) and of the messages that
// PetAgeValidator emits after disabling the default violation.
const (
	// PetAgeValidationMessage is the constraint's default message
	// (PetAgeValidation.message()). The validator always replaces it with
	// one of the more specific messages below, but it is kept for callers
	// that want a generic description of the rule (e.g. API docs).
	PetAgeValidationMessage = "Birth date must not be in the future or older than 50 years"

	// PetAgeFutureMessage is reported when the birth date is after today.
	PetAgeFutureMessage = "Birth date cannot be in the future"

	// PetAgeTooOldMessage is reported when the birth date is more than
	// petMaxAgeYears years before today.
	PetAgeTooOldMessage = "Birth date cannot be older than 50 years"
)

// petMaxAgeYears is the maximum pet age accepted by the validator.
const petMaxAgeYears = 50

// civilDate is a calendar date without time or zone, mirroring
// java.time.LocalDate for comparison purposes.
type civilDate struct {
	year  int
	month time.Month
	day   int
}

// civilDateOf extracts the calendar date of t in t's own location.
func civilDateOf(t time.Time) civilDate {
	y, m, d := t.Date()
	return civilDate{year: y, month: m, day: d}
}

// before reports whether c is strictly earlier than o.
func (c civilDate) before(o civilDate) bool {
	if c.year != o.year {
		return c.year < o.year
	}
	if c.month != o.month {
		return c.month < o.month
	}
	return c.day < o.day
}

// minusYears subtracts n years with LocalDate.minusYears semantics: if the
// resulting date is invalid (Feb 29 in a non-leap year) the day is clamped
// to the last valid day of the month instead of rolling into the next month
// as time.Time.AddDate would.
func (c civilDate) minusYears(n int) civilDate {
	y := c.year - n
	d := c.day
	// Last day of the target month: day 0 of the following month.
	last := time.Date(y, c.month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > last {
		d = last
	}
	return civilDate{year: y, month: c.month, day: d}
}

// validatePetAge checks a pet's birth date and returns the violation message
// and false when it is invalid, or "" and true when it is acceptable.
//
// It ports the @PetAgeValidation constraint together with PetAgeValidator:
//   - a nil birth date is valid (null is left to other constraints such as
//     @NotNull, exactly as in the source);
//   - a date after today is rejected with PetAgeFutureMessage;
//   - a date before today minus 50 years is rejected with PetAgeTooOldMessage.
//
// Only the calendar date matters: birthDate is interpreted in its own
// location (dates decoded from "yyyy-MM-dd" JSON are UTC midnight) and
// "today" is now's calendar date in now's location. Pass time.Now() in
// production; the parameter exists so tests can pin the clock.
//
// MIGRATION_NOTE: Jakarta Bean Validation discovers the annotation by
// reflection and runs the validator implicitly on @Valid request bodies.
// Go has no equivalent, so request handlers must call validatePetAge
// explicitly on the birthDate of PetFields and Pet DTOs and report the
// message as a field error on "birthDate". Validation groups and payloads
// were unused defaults in the source and have no counterpart here.
func validatePetAge(birthDate *time.Time, now time.Time) (string, bool) {
	if birthDate == nil {
		return "", true
	}

	bd := civilDateOf(*birthDate)
	today := civilDateOf(now)

	if today.before(bd) {
		return PetAgeFutureMessage, false
	}
	if bd.before(today.minusYears(petMaxAgeYears)) {
		return PetAgeTooOldMessage, false
	}
	return "", true
}
