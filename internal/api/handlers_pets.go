package api

// This file is the port of PetRestControllerV1: endpoints for pets under
// {contextPath}/api/pets.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). The
// @CrossOrigin(exposedHeaders = "errors, content-type") behaviour is provided
// by withRootCORS. Bean Validation (@Valid on the generated PetsApi) is done
// by validatePetV1Input. The MapStruct PetMapper is replaced by petToV1DTO.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store"
	"migrated-app/internal/store/sqlstore"
)

// petRoleOwnerAdmin is the role required by every pet endpoint.
const petRoleOwnerAdmin = "OWNER_ADMIN"

// petNameMaxLen is the maximum pet name length (Pet schema).
const petNameMaxLen = 30

// petV1DTO is the JSON representation of a pet (PetDto): the model's own
// fields (id, name, birthDate, type, ownerId) plus its visits.
type petV1DTO struct {
	*model.Pet
	Visits []*visitV1DTO `json:"visits"`
}

// petV1TypeInput mirrors the nested PetType schema for validation.
type petV1TypeInput struct {
	ID   *int    `json:"id"`
	Name *string `json:"name"`
}

// petV1Input mirrors the PetDto request body for validation.
type petV1Input struct {
	Name      *string         `json:"name"`
	BirthDate *model.Date     `json:"birthDate"`
	Type      *petV1TypeInput `json:"type"`
}

// PetHandler serves the /api/pets endpoints.
type PetHandler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewPetHandler returns a PetHandler backed by clinic. authorize may be nil,
// in which case no role checks are applied (security disabled).
func NewPetHandler(clinic service.Clinic, authorize Authorizer) *PetHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &PetHandler{clinic: clinic, authorize: authorize}
}

// Register mounts all pet routes on mux under contextPath.
func (h *PetHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/pets"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, petRoleOwnerAdmin)))
	}
	route("GET "+base, h.listPets)
	route("GET "+base+"/{petId}", h.getPet)
	route("PUT "+base+"/{petId}", h.updatePet)
	route("DELETE "+base+"/{petId}", h.deletePet)
}

// listPets handles GET /api/pets. 404 with empty body when there are none.
func (h *PetHandler) listPets(w http.ResponseWriter, r *http.Request) {
	pets, err := h.clinic.FindAllPets(r.Context())
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing pets: %w", err))
		return
	}
	dtos := make([]*petV1DTO, 0, len(pets))
	for _, p := range pets {
		dtos = append(dtos, petToV1DTO(p))
	}
	if len(dtos) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, dtos)
}

// getPet handles GET /api/pets/{petId}.
func (h *PetHandler) getPet(w http.ResponseWriter, r *http.Request) {
	id, ok := petParseID(w, r)
	if !ok {
		return
	}
	pet, found, err := h.findPet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, petToV1DTO(pet))
}

// updatePet handles PUT /api/pets/{petId}: only name, birthDate and type
// change; id, ownerId and visits in the body are ignored. 204 on success.
func (h *PetHandler) updatePet(w http.ResponseWriter, r *http.Request) {
	id, ok := petParseID(w, r)
	if !ok {
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "body", RejectedValue: nil, DefaultMessage: "unreadable request body",
		}))
		return
	}
	var input petV1Input
	var incoming model.Pet
	if err := json.Unmarshal(raw, &input); err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "body", RejectedValue: nil, DefaultMessage: "malformed JSON request body",
		}))
		return
	}
	if verr := validatePetV1Input(&input); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	if err := json.Unmarshal(raw, &incoming); err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "body", RejectedValue: nil, DefaultMessage: "malformed JSON request body",
		}))
		return
	}

	current, found, err := h.findPet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current.SetBirthDate(copyDatePtr(incoming.GetBirthDate()))
	current.Name = incoming.Name
	current.SetType(incoming.GetType())
	if err := h.clinic.SavePet(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating pet %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: Spring returned a PetDto with 204; a 204 carries no body.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deletePet handles DELETE /api/pets/{petId}. The repository cascades the
// delete to the pet's visits.
func (h *PetHandler) deletePet(w http.ResponseWriter, r *http.Request) {
	id, ok := petParseID(w, r)
	if !ok {
		return
	}
	pet, found, err := h.findPet(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.clinic.DeletePet(r.Context(), pet); err != nil {
		HandleError(w, r, fmt.Errorf("deleting pet %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// findPet maps not-found errors to found=false (the source's null return).
func (h *PetHandler) findPet(r *http.Request, id int) (*model.Pet, bool, error) {
	pet, err := h.clinic.FindPetByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding pet %d: %w", id, err)
	}
	if pet == nil {
		return nil, false, nil
	}
	return pet, true, nil
}

// petParseID parses {petId} as an int32 >= 0, writing a 400 otherwise.
func petParseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("petId")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "petId", RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: "petId", RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// validatePetV1Input applies the Pet schema constraints: name required,
// non-blank, at most 30 chars; birthDate required; type required with
// id >= 0 and a non-empty name.
func validatePetV1Input(in *petV1Input) *ValidationError {
	verr := NewValidationError()
	switch {
	case in.Name == nil:
		verr.Add("name", nil, "must not be null")
	case strings.TrimSpace(*in.Name) == "":
		verr.Add("name", *in.Name, "must not be blank")
	case utf8.RuneCountInString(*in.Name) > petNameMaxLen:
		verr.Add("name", *in.Name, fmt.Sprintf("size must be between 1 and %d", petNameMaxLen))
	}
	if in.BirthDate == nil {
		verr.Add("birthDate", nil, "must not be null")
	}
	if in.Type == nil {
		verr.Add("type", nil, "must not be null")
		return verr
	}
	if in.Type.ID == nil {
		verr.Add("type.id", nil, "must not be null")
	} else if *in.Type.ID < 0 {
		verr.Add("type.id", *in.Type.ID, "must be greater than or equal to 0")
	}
	if in.Type.Name == nil {
		verr.Add("type.name", nil, "must not be null")
	} else if *in.Type.Name == "" {
		verr.Add("type.name", *in.Type.Name, "must not be empty")
	}
	return verr
}

// petToV1DTO maps a Pet to its API representation (toPetDto). Nil in, nil out.
func petToV1DTO(p *model.Pet) *petV1DTO {
	if p == nil {
		return nil
	}
	visits := p.GetVisits()
	dtos := make([]*visitV1DTO, 0, len(visits))
	for _, v := range visits {
		dtos = append(dtos, visitToV1DTO(v))
	}
	cp := *p
	cp.OwnerID = copyIntPtrVisit(p.GetOwnerID())
	return &petV1DTO{Pet: &cp, Visits: dtos}
}
