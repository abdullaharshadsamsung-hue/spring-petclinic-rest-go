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

// birthDateLayout is the ISO-8601 calendar date layout used by
// java.time.LocalDate.toString(), used when reporting rejected values.
const birthDateLayout = "2006-01-02"

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
// It ports PetAgeValidator.isValid together with the @PetAgeValidation
// constraint:
//   - a nil birth date is valid (null is left to other constraints such as
//     @NotNull, exactly as in the source);
//   - a date after today is rejected with PetAgeFutureMessage;
//   - a date before today minus 50 years is rejected with PetAgeTooOldMessage.
//
// Both bounds are inclusive: a birth date equal to today, or exactly 50
// years before today, is accepted (isAfter / isBefore are strict).
//
// Only the calendar date matters: birthDate is interpreted in its own
// location (dates decoded from "yyyy-MM-dd" JSON are UTC midnight) and
// "today" is now's calendar date in now's location. Pass time.Now() in
// production; the parameter exists so tests can pin the clock.
//
// MIGRATION_NOTE: context.disableDefaultConstraintViolation() +
// buildConstraintViolationWithTemplate(...) is modelled by returning the
// specific message directly; the default message is never reported.
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

// ValidatePetBirthDate runs the pet age rule on birthDate and, when it
// fails, records a field error on errs under field (normally "birthDate").
// It reports whether the date was valid. A nil birthDate is valid and
// leaves errs untouched.
//
// MIGRATION_NOTE: Jakarta Bean Validation discovers @PetAgeValidation by
// reflection and runs PetAgeValidator implicitly on @Valid request bodies.
// Go has no equivalent.
// INTEGRATION: call ValidatePetBirthDate(errs, "birthDate", dto.BirthDate,
// time.Now()) from the pet/owner-pet create and update handlers when
// validating PetDto / PetFieldsDto bodies, then return errs via HandleError
// if errs.HasErrors().
func ValidatePetBirthDate(errs *ValidationError, field string, birthDate *time.Time, now time.Time) bool {
	msg, ok := validatePetAge(birthDate, now)
	if ok {
		return true
	}
	if errs != nil {
		errs.Add(field, birthDate.Format(birthDateLayout), msg)
	}
	return false
}
