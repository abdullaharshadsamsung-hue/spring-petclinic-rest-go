package api

// This file is the port of SpecialtyRestControllerV1: CRUD endpoints for the
// vet Specialty reference data under {contextPath}/api/specialties.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.VET_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). Every route
// requires VET_ADMIN. @CrossOrigin is handled by withRootCORS, shared with the
// pet type handlers.

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store"
	"migrated-app/internal/store/sqlstore"
)

// specialtyRoleVetAdmin is the role required by every specialty endpoint.
const specialtyRoleVetAdmin = "VET_ADMIN"

// specialtyNameMaxLen is the maximum name length (Specialty schema).
const specialtyNameMaxLen = 80

// SpecialtyHandler serves the /api/specialties endpoints.
type SpecialtyHandler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewSpecialtyHandler returns a SpecialtyHandler backed by clinic. authorize
// may be nil, in which case no role checks are applied (security disabled).
func NewSpecialtyHandler(clinic service.Clinic, authorize Authorizer) *SpecialtyHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &SpecialtyHandler{clinic: clinic, authorize: authorize}
}

// Register mounts all specialty routes on mux under contextPath
// (e.g. "/petclinic"; empty for the server root).
func (h *SpecialtyHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/specialties"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, specialtyRoleVetAdmin)))
	}
	route("GET "+base, h.listSpecialties)
	route("GET "+base+"/{specialtyId}", h.getSpecialty)
	route("POST "+base, h.addSpecialty)
	route("PUT "+base+"/{specialtyId}", h.updateSpecialty)
	route("DELETE "+base+"/{specialtyId}", h.deleteSpecialty)
}

// listSpecialties handles GET /api/specialties. 404 with empty body when none exist.
func (h *SpecialtyHandler) listSpecialties(w http.ResponseWriter, r *http.Request) {
	specialties, err := h.clinic.FindAllSpecialties(r.Context())
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing specialties: %w", err))
		return
	}
	if len(specialties) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, SpecialtiesToDTOs(specialties))
}

// getSpecialty handles GET /api/specialties/{specialtyId}.
func (h *SpecialtyHandler) getSpecialty(w http.ResponseWriter, r *http.Request) {
	id, ok := specialtyParseID(w, r)
	if !ok {
		return
	}
	s, found, err := h.findSpecialty(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, SpecialtyToDTO(s))
}

// addSpecialty handles POST /api/specialties: 201 + Location: /api/specialties/{id}.
func (h *SpecialtyHandler) addSpecialty(w http.ResponseWriter, r *http.Request) {
	var dto SpecialtyDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := specialtyValidateName(dto.Name); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	s := SpecialtyFromDTO(&dto)
	// MIGRATION_NOTE: id is readOnly in the spec; per the agreed migration
	// notes a POST always inserts a new row, so any client-supplied id is
	// discarded before saving and the generated id is returned.
	s.SetID(nil)
	if err := h.clinic.SaveSpecialty(r.Context(), s); err != nil {
		HandleError(w, r, fmt.Errorf("saving specialty: %w", err))
		return
	}
	// MIGRATION_NOTE: UriComponentsBuilder.newInstance().path(...) yields a
	// path-only URI without context path, scheme or host; preserved as-is.
	if id := s.GetID(); id != nil {
		w.Header().Set("Location", "/api/specialties/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, SpecialtyToDTO(s))
}

// updateSpecialty handles PUT /api/specialties/{specialtyId}. Only the name
// from the body is applied; the path id is authoritative.
func (h *SpecialtyHandler) updateSpecialty(w http.ResponseWriter, r *http.Request) {
	id, ok := specialtyParseID(w, r)
	if !ok {
		return
	}
	var dto SpecialtyDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := specialtyValidateName(dto.Name); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	current, found, err := h.findSpecialty(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current.SetName(dto.Name)
	if err := h.clinic.SaveSpecialty(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating specialty %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: Spring returns 204 with the DTO as entity; a 204 cannot
	// carry a body, so only the Content-Type header is preserved.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deleteSpecialty handles DELETE /api/specialties/{specialtyId}. The delete of
// the vet_specialties associations and the specialty row runs in one
// transaction in the service/repository.
//
// MIGRATION_NOTE: @Transactional on the controller method is replaced by the
// transactional DeleteSpecialty implementation; the preceding lookup runs
// outside that transaction. A concurrent delete between lookup and delete
// results in a no-op 204, matching the source's observable behaviour.
func (h *SpecialtyHandler) deleteSpecialty(w http.ResponseWriter, r *http.Request) {
	id, ok := specialtyParseID(w, r)
	if !ok {
		return
	}
	s, found, err := h.findSpecialty(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.clinic.DeleteSpecialty(r.Context(), s); err != nil {
		HandleError(w, r, fmt.Errorf("deleting specialty %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findSpecialty maps the service's not-found error to found=false, the Go
// equivalent of the source's null return.
func (h *SpecialtyHandler) findSpecialty(r *http.Request, id int) (*model.Specialty, bool, error) {
	s, err := h.clinic.FindSpecialtyByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding specialty %d: %w", id, err)
	}
	if s == nil {
		return nil, false, nil
	}
	return s, true, nil
}

// specialtyParseID parses the {specialtyId} path value as a 32-bit integer
// >= 0, writing a 400 problem response and returning false otherwise.
func specialtyParseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("specialtyId")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "specialtyId", RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "specialtyId", RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// specialtyValidateName applies the Specialty name constraints
// (required, length 1..80).
func specialtyValidateName(name string) *ValidationError {
	verr := NewValidationError()
	n := utf8.RuneCountInString(name)
	switch {
	case n == 0:
		verr.Add("name", name, "must not be empty")
	case n > specialtyNameMaxLen:
		verr.Add("name", name, fmt.Sprintf("size must be between 1 and %d", specialtyNameMaxLen))
	}
	return verr
}
