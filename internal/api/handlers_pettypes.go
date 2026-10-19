package api

// This file is the port of PetTypeRestControllerV1: CRUD endpoints for the
// PetType reference data under {contextPath}/api/pettypes.
//
// MIGRATION_NOTE: @PreAuthorize role checks are expressed via an injected
// Authorizer middleware so this file does not depend on the concrete auth
// package API. INTEGRATION: pass a middleware built from internal/auth that
// returns 401 (WWW-Authenticate: Basic) for missing/invalid credentials and
// 403 when the user has none of the given roles. When security is disabled,
// pass nil (all requests allowed), mirroring petclinic.security.enable=false.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/rs/zerolog/log"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store"
	"migrated-app/internal/store/sqlstore"
)

// Role names used by the pet type endpoints (the @roles bean constants
// without the ROLE_ prefix, as used by hasRole/hasAnyRole).
const (
	petTypeRoleOwnerAdmin = "OWNER_ADMIN"
	petTypeRoleVetAdmin   = "VET_ADMIN"
)

// petTypeNameMaxLen is the maximum name length (PetTypeFields schema).
const petTypeNameMaxLen = 80

// Authorizer wraps next so that it only runs when the caller holds at least
// one of roles. It is the Go replacement of @PreAuthorize("hasAnyRole(...)").
type Authorizer func(next http.Handler, roles ...string) http.Handler

// PetTypeHandler serves the /api/pettypes endpoints.
type PetTypeHandler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewPetTypeHandler returns a PetTypeHandler backed by clinic. authorize may
// be nil, in which case no role checks are applied.
func NewPetTypeHandler(clinic service.Clinic, authorize Authorizer) *PetTypeHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &PetTypeHandler{clinic: clinic, authorize: authorize}
}

// Register mounts all pet type routes on mux under contextPath
// (e.g. "/petclinic"; empty for the server root).
func (h *PetTypeHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/pettypes"
	readRoles := []string{petTypeRoleOwnerAdmin, petTypeRoleVetAdmin}
	writeRoles := []string{petTypeRoleVetAdmin}

	route := func(pattern string, fn http.HandlerFunc, roles []string) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, roles...)))
	}
	route("GET "+base, h.listPetTypes, readRoles)
	route("GET "+base+"/{petTypeId}", h.getPetType, readRoles)
	route("POST "+base, h.addPetType, writeRoles)
	route("PUT "+base+"/{petTypeId}", h.updatePetType, writeRoles)
	route("DELETE "+base+"/{petTypeId}", h.deletePetType, writeRoles)
}

// listPetTypes handles GET /api/pettypes. 404 with empty body when none exist.
func (h *PetTypeHandler) listPetTypes(w http.ResponseWriter, r *http.Request) {
	petTypes, err := h.clinic.FindAllPetTypes(r.Context())
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing pet types: %w", err))
		return
	}
	if len(petTypes) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, PetTypesToDTOs(petTypes))
}

// getPetType handles GET /api/pettypes/{petTypeId}.
func (h *PetTypeHandler) getPetType(w http.ResponseWriter, r *http.Request) {
	id, ok := petTypeParseID(w, r)
	if !ok {
		return
	}
	pt, found, err := h.findPetType(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, PetTypeToDTO(pt))
}

// addPetType handles POST /api/pettypes: 201 + Location: /api/pettypes/{id}.
func (h *PetTypeHandler) addPetType(w http.ResponseWriter, r *http.Request) {
	var dto PetTypeFieldsDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := petTypeValidateName(dto.Name); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	pt := PetTypeFromFieldsDTO(&dto)
	if err := h.clinic.SavePetType(r.Context(), pt); err != nil {
		HandleError(w, r, fmt.Errorf("saving pet type: %w", err))
		return
	}
	// MIGRATION_NOTE: UriComponentsBuilder.newInstance().path(...) yields a
	// path-only URI without context path, scheme or host; preserved as-is.
	if id := pt.GetID(); id != nil {
		w.Header().Set("Location", "/api/pettypes/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, PetTypeToDTO(pt))
}

// updatePetType handles PUT /api/pettypes/{petTypeId}. Only the name from the
// body is applied; any id in the body is ignored in favour of the path id.
func (h *PetTypeHandler) updatePetType(w http.ResponseWriter, r *http.Request) {
	id, ok := petTypeParseID(w, r)
	if !ok {
		return
	}
	var dto PetTypeDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := petTypeValidateName(dto.Name); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	current, found, err := h.findPetType(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current.SetName(dto.Name)
	if err := h.clinic.SavePetType(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating pet type %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: Spring returns 204 with Content-Type application/json;
	// a 204 cannot carry a body, so the DTO the source passed is never sent.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deletePetType handles DELETE /api/pettypes/{petTypeId}. The cascade (visits
// of the type's pets, the pets, then the type) runs inside one transaction in
// the Clinic/repository implementation.
//
// MIGRATION_NOTE: @Transactional on the controller method is replaced by the
// transactional DeletePetType implementation; the preceding lookup runs
// outside that transaction. Behaviour is otherwise unchanged.
func (h *PetTypeHandler) deletePetType(w http.ResponseWriter, r *http.Request) {
	id, ok := petTypeParseID(w, r)
	if !ok {
		return
	}
	pt, found, err := h.findPetType(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.clinic.DeletePetType(r.Context(), pt); err != nil {
		HandleError(w, r, fmt.Errorf("deleting pet type %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findPetType maps the service's not-found error to found=false, the Go
// equivalent of the source's null return.
func (h *PetTypeHandler) findPetType(r *http.Request, id int) (*model.PetType, bool, error) {
	pt, err := h.clinic.FindPetTypeByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding pet type %d: %w", id, err)
	}
	if pt == nil {
		return nil, false, nil
	}
	return pt, true, nil
}

// petTypeParseID parses the {petTypeId} path value as a 32-bit integer >= 0,
// writing a 400 problem response and returning false otherwise.
func petTypeParseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("petTypeId")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "petTypeId", RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "petTypeId", RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// petTypeValidateName applies the PetTypeFields name constraints
// (required, length 1..80).
func petTypeValidateName(name string) *ValidationError {
	verr := NewValidationError()
	n := utf8.RuneCountInString(name)
	switch {
	case n == 0:
		verr.Add("name", name, "must not be empty")
	case n > petTypeNameMaxLen:
		verr.Add("name", name, fmt.Sprintf("size must be between 1 and %d", petTypeNameMaxLen))
	}
	return verr
}

// petTypeDecodeBody decodes a required JSON request body into dst, writing a
// 400 problem response on a missing or malformed body.
func petTypeDecodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		WriteProblem(w, r, http.StatusBadRequest, "HttpMessageNotReadableException", errInvalidRequest, nil)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if !errors.Is(err, io.EOF) {
			log.Debug().Err(err).Msg("malformed pet type request body")
		}
		WriteProblem(w, r, http.StatusBadRequest, "HttpMessageNotReadableException", errInvalidRequest, nil)
		return false
	}
	return true
}

// petTypeWriteJSON writes v as a JSON response with the given status.
func petTypeWriteJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("encoding pet type response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Debug().Err(err).Msg("writing pet type response")
	}
}
