package api

// This file is the port of ExceptionControllerAdvice: the global REST error
// handler that turns errors returned by handlers into RFC 7807 problem
// details (Content-Type: application/problem+json).
//
// MIGRATION_NOTE: Spring's @ControllerAdvice/@ExceptionHandler dispatch is
// replaced by an explicit call: handlers that fail call HandleError(w, r, err)
// and return. The "most specific handler wins" rule is implemented by the
// ordered checks in HandleError (validation -> data integrity -> fallback).

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"migrated-app/internal/store/sqlstore"
)

// ProblemContentType is the media type of RFC 7807 problem responses.
const ProblemContentType = "application/problem+json"

const (
	errUnexpected     = "An unexpected error occurred while processing your request"
	errDataIntegrity  = "The requested resource could not be processed due to a data constraint violation"
	errInvalidRequest = "The request contains invalid or missing parameters"

	titleDefault        = "RuntimeException"
	titleDataIntegrity  = "DataIntegrityViolationException"
	titleNotValid       = "MethodArgumentNotValidException"
	titleObjectNotFound = "ObjectRetrievalFailureException"

	defaultValidationMessage = "Validation failed"
)

// ErrIntegrity is the sentinel for data integrity violations (Spring's
// DataIntegrityViolationException), e.g. a foreign key referencing an entity
// that does not exist. Wrap it (fmt.Errorf("...: %w", ErrIntegrity)) or
// return an error implementing IntegrityViolation to get a 404 problem.
var ErrIntegrity = errors.New("data integrity violation")

// IntegrityViolation may be implemented by errors from lower layers (which
// cannot import this package) to mark themselves as data integrity
// violations, the equivalent of subclassing DataIntegrityViolationException.
type IntegrityViolation interface {
	error
	IntegrityViolation() bool
}

// Titled may be implemented by errors to control the problem "title". It is
// the counterpart of e.getClass().getSimpleName() in the source.
type Titled interface {
	Title() string
}

// FieldError describes one failing field of a validated request body.
type FieldError struct {
	// Field is the (JSON/bean) property path of the failing field.
	Field string
	// RejectedValue is the value that failed validation; nil means null.
	RejectedValue any
	// DefaultMessage is the constraint message, e.g. "must not be empty".
	// An empty message is reported as "Validation failed".
	DefaultMessage string
}

// ValidationError reports bean-validation style failures on a request
// argument. It replaces MethodArgumentNotValidException.
type ValidationError struct {
	// Errors holds the failing fields in the order they were detected.
	Errors []FieldError
}

// NewValidationError returns a ValidationError for the given field errors.
func NewValidationError(fieldErrors ...FieldError) *ValidationError {
	return &ValidationError{Errors: fieldErrors}
}

// Add appends a field error.
func (e *ValidationError) Add(field string, rejected any, message string) {
	e.Errors = append(e.Errors, FieldError{Field: field, RejectedValue: rejected, DefaultMessage: message})
}

// HasErrors reports whether any field error was recorded.
func (e *ValidationError) HasErrors() bool { return e != nil && len(e.Errors) > 0 }

// Error implements the error interface.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", fe.Field, defaultMessage(fe)))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Title implements Titled.
func (e *ValidationError) Title() string { return titleNotValid }

// ValidationMessage is one entry of "schemaValidationErrors". It mirrors
// ValidationMessageDto: a message plus additional properties flattened into
// the same JSON object.
type ValidationMessage struct {
	Message        string `json:"message"`
	Field          string `json:"field"`
	RejectedValue  string `json:"rejectedValue"`
	DefaultMessage string `json:"defaultMessage"`
}

// Problem is the RFC 7807 body, including the extra "timestamp" and
// "schemaValidationErrors" properties set by the source.
type Problem struct {
	Type                   string              `json:"type"`
	Title                  string              `json:"title"`
	Status                 int                 `json:"status"`
	Detail                 string              `json:"detail"`
	Timestamp              string              `json:"timestamp"`
	SchemaValidationErrors []ValidationMessage `json:"schemaValidationErrors"`
}

// WriteProblem writes a problem response with the given status, title,
// detail and validation messages. A nil validation list is rendered as [].
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string, validation []ValidationMessage) {
	if validation == nil {
		validation = []ValidationMessage{}
	}
	p := Problem{
		Type:                   requestURL(r),
		Title:                  title,
		Status:                 status,
		Detail:                 detail,
		Timestamp:              time.Now().UTC().Format(time.RFC3339Nano),
		SchemaValidationErrors: validation,
	}
	body, err := json.Marshal(p)
	if err != nil {
		// Cannot happen with these field types; still never swallow it.
		log.Error().Err(err).Msg("encoding problem detail")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Debug().Err(err).Msg("writing problem detail")
	}
}

// HandleError converts err into a problem response:
//   - *ValidationError          -> 400 with per-field schemaValidationErrors
//   - ErrIntegrity / IntegrityViolation -> 404 (message logged, never sent)
//   - anything else             -> 500
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	var verr *ValidationError
	if errors.As(err, &verr) {
		handleValidation(w, r, verr)
		return
	}

	if isIntegrityViolation(err) {
		log.Warn().Msgf("Data integrity violation at %s %s: %s", r.Method, r.URL.RequestURI(), err.Error())
		log.Debug().Err(err).Msg("Data integrity violation stacktrace")
		WriteProblem(w, r, http.StatusNotFound, titleFor(err, titleDataIntegrity), errDataIntegrity, nil)
		return
	}

	// MIGRATION_NOTE: Go errors carry no stack trace; the error chain is logged instead.
	log.Error().Err(err).Msgf("Unexpected error at %s %s", r.Method, r.URL.Path)
	WriteProblem(w, r, http.StatusInternalServerError, titleFor(err, titleDefault), errUnexpected, nil)
}

func handleValidation(w http.ResponseWriter, r *http.Request, verr *ValidationError) {
	// MIGRATION_NOTE: the source also filled a BindingErrorsResponse that was
	// never used in the response; it is intentionally not ported.
	var msgs []ValidationMessage
	if verr.HasErrors() {
		msgs = make([]ValidationMessage, 0, len(verr.Errors))
		for _, fe := range verr.Errors {
			rejected := rejectedString(fe.RejectedValue)
			dm := defaultMessage(fe)
			msgs = append(msgs, ValidationMessage{
				Message:        fmt.Sprintf("Field '%s' %s (rejected value: %s)", fe.Field, dm, rejected),
				Field:          fe.Field,
				RejectedValue:  rejected,
				DefaultMessage: dm,
			})
		}
		log.Debug().Msgf("Validation error at %s %s: %v", r.Method, r.URL.Path, verr.Errors)
	}
	WriteProblem(w, r, http.StatusBadRequest, titleNotValid, errInvalidRequest, msgs)
}

func isIntegrityViolation(err error) bool {
	if errors.Is(err, ErrIntegrity) {
		return true
	}
	var iv IntegrityViolation
	return errors.As(err, &iv) && iv.IntegrityViolation()
}

// titleFor picks the problem title: a Titled error in the chain wins, then
// known store errors, then fallback.
func titleFor(err error, fallback string) string {
	var t Titled
	if errors.As(err, &t) {
		if s := t.Title(); s != "" {
			return s
		}
	}
	var orf *sqlstore.ObjectRetrievalFailureError
	if errors.As(err, &orf) {
		return titleObjectNotFound
	}
	return fallback
}

func defaultMessage(fe FieldError) string {
	if fe.DefaultMessage == "" {
		return defaultValidationMessage
	}
	return fe.DefaultMessage
}

// rejectedString mirrors Objects.toString(value, "null"), dereferencing
// pointers so *string / *int values print their content.
func rejectedString(v any) string {
	if v == nil {
		return "null"
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return "null"
		}
		rv = rv.Elem()
	}
	return fmt.Sprint(rv.Interface())
}

// requestURL reconstructs HttpServletRequest.getRequestURL(): scheme, host
// and path, without the query string.
func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + r.URL.EscapedPath()
}
