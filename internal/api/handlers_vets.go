package api

// This file is the port of VetRestControllerV1: CRUD endpoints for
// veterinarians under {contextPath}/api/vets.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.VET_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). The
// @CrossOrigin(exposedHeaders = "errors, content-type") behaviour is provided
// by withRootCORS. Bean Validation (@Valid on the generated VetsApi) is done
// by validateVetDTO. The MapStruct VetMapper is replaced by the unexported
// vetToDTO / vetFromDTO helpers below.

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"unicode/utf8"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store"
	"migrated-app/internal/store/sqlstore"
)

// vetRoleVetAdmin is the role required by every vet endpoint.
const vetRoleVetAdmin = "VET_ADMIN"

const (
	vetNameMaxLen          = 30
	vetSpecialtyNameMaxLen = 80
)

// vetNamePattern is the OpenAPI person-name pattern for firstName/lastName.
var vetNamePattern = regexp.MustCompile(`^[a-zA-Z]+(([' -][a-zA-Z ])?[a-zA-Z]*)*$`)

// VetDTO is a veterinarian as exposed by the REST API (the "Vet" schema in
// openapi.yml). It replaces the OpenAPI-generated VetDto.
type VetDTO struct {
	ID          *int            `json:"id"`
	FirstName   string          `json:"firstName"`
	LastName    string          `json:"lastName"`
	Specialties []*SpecialtyDTO `json:"specialties"`
}

// VetHandler serves the /api/vets endpoints.
type VetHandler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewVetHandler returns a VetHandler backed by clinic. authorize may be nil,
// in which case no role checks are applied (security disabled).
func NewVetHandler(clinic service.Clinic, authorize Authorizer) *VetHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &VetHandler{clinic: clinic, authorize: authorize}
}

// Register mounts all vet routes on mux under contextPath.
func (h *VetHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/vets"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, vetRoleVetAdmin)))
	}
	route("GET "+base, h.listVets)
	route("GET "+base+"/{vetId}", h.getVet)
	route("POST "+base, h.addVet)
	route("PUT "+base+"/{vetId}", h.updateVet)
	route("DELETE "+base+"/{vetId}", h.deleteVet)
}

// listVets handles GET /api/vets. An empty list yields 404 with empty body.
func (h *VetHandler) listVets(w http.ResponseWriter, r *http.Request) {
	vets, err := h.clinic.FindAllVets(r.Context())
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing vets: %w", err))
		return
	}
	dtos := vetsToDTOs(vets)
	if len(dtos) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, dtos)
}

// getVet handles GET /api/vets/{vetId}.
func (h *VetHandler) getVet(w http.ResponseWriter, r *http.Request) {
	id, ok := vetParseID(w, r)
	if !ok {
		return
	}
	vet, found, err := h.findVet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, vetToDTO(vet))
}

// addVet handles POST /api/vets: 201 + Location: /api/vets/{id}. Specialties
// are resolved by name; unknown names are silently dropped. Any id in the
// body is copied (as MapStruct did) and the save then becomes an update.
func (h *VetHandler) addVet(w http.ResponseWriter, r *http.Request) {
	var dto VetDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := validateVetDTO(&dto); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	vet := vetFromDTO(&dto)
	if vet.NrOfSpecialties() > 0 {
		if err := h.resolveSpecialties(r, vet); err != nil {
			HandleError(w, r, err)
			return
		}
	}
	if err := h.clinic.SaveVet(r.Context(), vet); err != nil {
		HandleError(w, r, fmt.Errorf("saving vet: %w", err))
		return
	}
	// MIGRATION_NOTE: UriComponentsBuilder.newInstance().path(...) yields a
	// path-only URI without context path, scheme or host; preserved as-is.
	if vet.ID != nil {
		w.Header().Set("Location", "/api/vets/"+strconv.Itoa(*vet.ID))
	}
	petTypeWriteJSON(w, http.StatusCreated, vetToDTO(vet))
}

// updateVet handles PUT /api/vets/{vetId}: replaces names and specialty set.
func (h *VetHandler) updateVet(w http.ResponseWriter, r *http.Request) {
	id, ok := vetParseID(w, r)
	if !ok {
		return
	}
	var dto VetDTO
	if !petTypeDecodeBody(w, r, &dto) {
		return
	}
	if verr := validateVetDTO(&dto); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	current, found, err := h.findVet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current.FirstName = dto.FirstName
	current.LastName = dto.LastName
	current.ClearSpecialties()
	for _, spec := range SpecialtiesFromDTOs(dto.Specialties) {
		if spec != nil {
			current.AddSpecialty(*spec)
		}
	}
	if current.NrOfSpecialties() > 0 {
		if err := h.resolveSpecialties(r, current); err != nil {
			HandleError(w, r, err)
			return
		}
	}
	if err := h.clinic.SaveVet(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating vet %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: the source passed a body with 204; a 204 cannot carry
	// one, so only the Content-Type header is kept.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deleteVet handles DELETE /api/vets/{vetId}.
//
// MIGRATION_NOTE: @Transactional on the controller method is replaced by the
// transactional Clinic.DeleteVet (which removes vet_specialties rows and the
// vet atomically); the preceding lookup runs outside that transaction.
func (h *VetHandler) deleteVet(w http.ResponseWriter, r *http.Request) {
	id, ok := vetParseID(w, r)
	if !ok {
		return
	}
	vet, found, err := h.findVet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.clinic.DeleteVet(r.Context(), vet); err != nil {
		HandleError(w, r, fmt.Errorf("deleting vet %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveSpecialties replaces vet's specialties with the stored specialties
// whose names match; unknown names are dropped.
func (h *VetHandler) resolveSpecialties(r *http.Request, vet *model.Vet) error {
	seen := make(map[string]struct{})
	names := make([]string, 0, vet.NrOfSpecialties())
	for _, s := range vet.GetSpecialties() {
		if _, dup := seen[s.Name]; dup {
			continue
		}
		seen[s.Name] = struct{}{}
		names = append(names, s.Name)
	}
	found, err := h.clinic.FindSpecialtiesByNameIn(r.Context(), names)
	if err != nil {
		return fmt.Errorf("resolving vet specialties: %w", err)
	}
	resolved := make([]model.Specialty, 0, len(found))
	for _, s := range found {
		if s != nil {
			resolved = append(resolved, *s)
		}
	}
	vet.SetSpecialties(resolved)
	return nil
}

// findVet maps the service's not-found error to found=false, the Go
// equivalent of the source's null return.
func (h *VetHandler) findVet(r *http.Request, id int) (*model.Vet, bool, error) {
	vet, err := h.clinic.FindVetByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding vet %d: %w", id, err)
	}
	if vet == nil {
		return nil, false, nil
	}
	return vet, true, nil
}

// vetParseID parses {vetId} as a 32-bit integer >= 0, writing a 400 problem
// response and returning false otherwise.
func vetParseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("vetId")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "vetId", RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "vetId", RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// validateVetDTO applies the Vet schema constraints.
func validateVetDTO(dto *VetDTO) *ValidationError {
	verr := NewValidationError()
	validateVetName(verr, "firstName", dto.FirstName)
	validateVetName(verr, "lastName", dto.LastName)
	if dto.Specialties == nil {
		verr.Add("specialties", nil, "must not be null")
	}
	for i, s := range dto.Specialties {
		field := fmt.Sprintf("specialties[%d].name", i)
		if s == nil {
			verr.Add(fmt.Sprintf("specialties[%d]", i), nil, "must not be null")
			continue
		}
		n := utf8.RuneCountInString(s.Name)
		switch {
		case n == 0:
			verr.Add(field, s.Name, "must not be empty")
		case n > vetSpecialtyNameMaxLen:
			verr.Add(field, s.Name, fmt.Sprintf("size must be between 1 and %d", vetSpecialtyNameMaxLen))
		}
	}
	return verr
}

func validateVetName(verr *ValidationError, field, value string) {
	n := utf8.RuneCountInString(value)
	switch {
	case n == 0:
		verr.Add(field, value, "must not be empty")
		return
	case n > vetNameMaxLen:
		verr.Add(field, value, fmt.Sprintf("size must be between 1 and %d", vetNameMaxLen))
	}
	if !vetNamePattern.MatchString(value) {
		verr.Add(field, value, fmt.Sprintf("must match \"%s\"", vetNamePattern.String()))
	}
}

// vetToDTO converts a Vet entity to its DTO; specialties come sorted by name.
func vetToDTO(v *model.Vet) *VetDTO {
	if v == nil {
		return nil
	}
	specs := v.GetSpecialties()
	dtos := make([]*SpecialtyDTO, len(specs))
	for i := range specs {
		dtos[i] = SpecialtyToDTO(&specs[i])
	}
	return &VetDTO{
		ID:          copyIntPtr(v.ID),
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Specialties: dtos,
	}
}

// vetsToDTOs converts vets element-wise, keeping nil entries.
func vetsToDTOs(vets []*model.Vet) []*VetDTO {
	out := make([]*VetDTO, len(vets))
	for i, v := range vets {
		out[i] = vetToDTO(v)
	}
	return out
}

// vetFromDTO converts a VetDTO to a new Vet entity (MapStruct toVet).
func vetFromDTO(dto *VetDTO) *model.Vet {
	v := model.NewVet()
	v.ID = copyIntPtr(dto.ID)
	v.FirstName = dto.FirstName
	v.LastName = dto.LastName
	for _, s := range SpecialtiesFromDTOs(dto.Specialties) {
		if s != nil {
			v.AddSpecialty(*s)
		}
	}
	return v
}
