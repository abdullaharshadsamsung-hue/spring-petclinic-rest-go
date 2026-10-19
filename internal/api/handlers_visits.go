package api

// This file is the port of VisitRestControllerV1: CRUD endpoints for visits
// under {contextPath}/api/visits.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). The
// @CrossOrigin(exposedHeaders = "errors, content-type") behaviour is provided
// by withRootCORS. Bean Validation (@Valid on the generated VisitsApi) is done
// by validateVisitFields. The MapStruct VisitMapper is replaced by the
// unexported visitToV1DTO / visitFromV1DTO helpers below.
//
// MIGRATION_NOTE: the handler depends on the small VisitService interface
// rather than the concrete service.Clinic so that it can be wired with any
// implementation exposing the visit operations of ClinicService.
// REQUIRES MANUAL REVIEW: confirm service.Clinic's method names/signatures
// match VisitService (or adapt with a thin wrapper when wiring in main).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
	"migrated-app/internal/store/sqlstore"
)

// visitRoleOwnerAdmin is the role required by every visit endpoint.
const visitRoleOwnerAdmin = "OWNER_ADMIN"

// visitDescriptionMaxLen is the maximum description length (Visit schema).
const visitDescriptionMaxLen = 255

// VisitService is the subset of ClinicService used by the visit endpoints.
type VisitService interface {
	// FindAllVisits returns every visit.
	FindAllVisits(ctx context.Context) ([]*model.Visit, error)
	// FindVisitByID returns the visit with id, or a not-found error.
	FindVisitByID(ctx context.Context, id int) (*model.Visit, error)
	// SaveVisit inserts or updates visit, assigning its id on insert.
	SaveVisit(ctx context.Context, visit *model.Visit) error
	// DeleteVisit removes visit.
	DeleteVisit(ctx context.Context, visit *model.Visit) error
}

// visitV1FieldsDTO mirrors the OpenAPI VisitFields schema.
type visitV1FieldsDTO struct {
	Date        *model.Date `json:"date"`
	Description *string     `json:"description"`
}

// visitV1DTO mirrors the OpenAPI Visit schema.
type visitV1DTO struct {
	ID          *int        `json:"id"`
	Date        *model.Date `json:"date"`
	Description *string     `json:"description"`
	PetID       *int        `json:"petId"`
}

// VisitHandler serves the /api/visits endpoints.
type VisitHandler struct {
	visits    VisitService
	authorize Authorizer
}

// NewVisitHandler returns a VisitHandler backed by visits. authorize may be
// nil, in which case no role checks are applied (security disabled).
func NewVisitHandler(visits VisitService, authorize Authorizer) *VisitHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &VisitHandler{visits: visits, authorize: authorize}
}

// Register mounts all visit routes on mux under contextPath.
func (h *VisitHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/visits"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, visitRoleOwnerAdmin)))
	}
	route("GET "+base, h.listVisits)
	route("GET "+base+"/{visitId}", h.getVisit)
	route("POST "+base, h.addVisit)
	route("PUT "+base+"/{visitId}", h.updateVisit)
	route("DELETE "+base+"/{visitId}", h.deleteVisit)
}

// listVisits handles GET /api/visits. 404 with empty body when none exist.
func (h *VisitHandler) listVisits(w http.ResponseWriter, r *http.Request) {
	visits, err := h.visits.FindAllVisits(r.Context())
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing visits: %w", err))
		return
	}
	if len(visits) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	dtos := make([]*visitV1DTO, 0, len(visits))
	for _, v := range visits {
		dtos = append(dtos, visitToV1DTO(v))
	}
	petTypeWriteJSON(w, http.StatusOK, dtos)
}

// getVisit handles GET /api/visits/{visitId}.
func (h *VisitHandler) getVisit(w http.ResponseWriter, r *http.Request) {
	id, ok := visitParseID(w, r)
	if !ok {
		return
	}
	v, found, err := h.findVisit(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, visitToV1DTO(v))
}

// addVisit handles POST /api/visits: 201 + Location: /api/visits/{id}.
// Any id in the body is ignored; the generated id is returned.
func (h *VisitHandler) addVisit(w http.ResponseWriter, r *http.Request) {
	var dto visitV1DTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	verr := validateVisitFields(dto.Description)
	if dto.PetID == nil {
		verr.Add("petId", nil, "must not be null")
	} else if *dto.PetID < 0 {
		verr.Add("petId", *dto.PetID, "must be greater than or equal to 0")
	}
	if verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	visit := visitFromV1DTO(&dto)
	// A nonexistent pet surfaces as an FK violation, mapped by HandleError.
	if err := h.visits.SaveVisit(r.Context(), visit); err != nil {
		HandleError(w, r, fmt.Errorf("saving visit: %w", err))
		return
	}
	// MIGRATION_NOTE: UriComponentsBuilder.newInstance().path(...) yields a
	// path-only URI without context path; preserved as-is.
	if id := visit.GetID(); id != nil {
		w.Header().Set("Location", "/api/visits/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, visitToV1DTO(visit))
}

// updateVisit handles PUT /api/visits/{visitId}: only date and description
// change; the pet is untouched. 204 with no body on success.
func (h *VisitHandler) updateVisit(w http.ResponseWriter, r *http.Request) {
	id, ok := visitParseID(w, r)
	if !ok {
		return
	}
	var dto visitV1FieldsDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := validateVisitFields(dto.Description); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	current, found, err := h.findVisit(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current.SetDate(copyDatePtr(dto.Date))
	current.SetDescription(*dto.Description)
	if err := h.visits.SaveVisit(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating visit %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: Spring passed a DTO with 204; a 204 carries no body.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deleteVisit handles DELETE /api/visits/{visitId}.
//
// MIGRATION_NOTE: @Transactional on the controller method spanned lookup and
// delete; here they are two service calls (each transactional on its own).
// REQUIRES MANUAL REVIEW if strict single-transaction semantics are needed.
func (h *VisitHandler) deleteVisit(w http.ResponseWriter, r *http.Request) {
	id, ok := visitParseID(w, r)
	if !ok {
		return
	}
	v, found, err := h.findVisit(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.visits.DeleteVisit(r.Context(), v); err != nil {
		HandleError(w, r, fmt.Errorf("deleting visit %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findVisit maps not-found errors to found=false (the source's null return).
func (h *VisitHandler) findVisit(r *http.Request, id int) (*model.Visit, bool, error) {
	v, err := h.visits.FindVisitByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding visit %d: %w", id, err)
	}
	if v == nil {
		return nil, false, nil
	}
	return v, true, nil
}

// visitParseID parses {visitId} as an int32 >= 0, writing a 400 otherwise.
func visitParseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("visitId")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "visitId", RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "visitId", RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// validateVisitFields applies the description constraints (required, 1..255).
func validateVisitFields(description *string) *ValidationError {
	verr := NewValidationError()
	if description == nil {
		verr.Add("description", nil, "must not be null")
		return verr
	}
	n := utf8.RuneCountInString(*description)
	switch {
	case n == 0:
		verr.Add("description", *description, "must not be empty")
	case n > visitDescriptionMaxLen:
		verr.Add("description", *description, fmt.Sprintf("size must be between 1 and %d", visitDescriptionMaxLen))
	}
	return verr
}

// visitToV1DTO maps a Visit to its API representation (toVisitDto).
func visitToV1DTO(v *model.Visit) *visitV1DTO {
	if v == nil {
		return nil
	}
	desc := v.GetDescription()
	return &visitV1DTO{
		ID:          copyIntPtrVisit(v.GetID()),
		Date:        copyDatePtr(v.GetDate()),
		Description: &desc,
		PetID:       copyIntPtrVisit(v.GetPetID()),
	}
}

// visitFromV1DTO maps an API Visit to a new entity (toVisit). The body id is
// ignored so the visit is always inserted.
func visitFromV1DTO(dto *visitV1DTO) *model.Visit {
	v := &model.Visit{}
	v.SetDate(copyDatePtr(dto.Date))
	if dto.Description != nil {
		v.SetDescription(*dto.Description)
	}
	v.PetID = copyIntPtrVisit(dto.PetID)
	return v
}

func copyDatePtr(d *model.Date) *model.Date {
	if d == nil {
		return nil
	}
	c := *d
	return &c
}

func copyIntPtrVisit(p *int) *int {
	if p == nil {
		return nil
	}
	c := *p
	return &c
}
