package api

// This file is the port of OwnerRestControllerV1. It serves the owner
// endpoints under {contextPath}/api/owners, including the nested pet and
// visit endpoints.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") becomes the
// injected Authorizer middleware. @CrossOrigin(exposedHeaders = "errors, content-type")
// becomes withRootCORS. Bean Validation becomes validateOwnerFields,
// validatePetV1Input and validateVisitFields. The MapStruct mappers are
// replaced by JSON round-trips through validated input structs, plus the
// existing petToV1DTO and visitToV1DTO helpers.

import (
	"encoding/json"
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

// ownerRoleOwnerAdmin is the role required by every owner endpoint.
const ownerRoleOwnerAdmin = "OWNER_ADMIN"

var (
	ownerNamePattern      = regexp.MustCompile(`^[\p{L}]+([ '-][\p{L}]+){0,2}\.?$`)
	ownerTelephonePattern = regexp.MustCompile(`^[0-9]*$`)
)

// ownerFieldsInput mirrors the OwnerFields schema. Only these five fields
// are ever copied onto an Owner entity.
type ownerFieldsInput struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Address   *string `json:"address"`
	City      *string `json:"city"`
	Telephone *string `json:"telephone"`
}

// OwnerHandler serves the /api/owners endpoints.
type OwnerHandler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewOwnerHandler returns an OwnerHandler backed by clinic. authorize may be
// nil. In that case no role checks are applied (security disabled).
func NewOwnerHandler(clinic service.Clinic, authorize Authorizer) *OwnerHandler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &OwnerHandler{clinic: clinic, authorize: authorize}
}

// Register mounts all owner routes on mux under contextPath.
func (h *OwnerHandler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/owners"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, withRootCORS(h.authorize(fn, ownerRoleOwnerAdmin)))
	}
	route("GET "+base, h.listOwners)
	route("POST "+base, h.addOwner)
	route("GET "+base+"/{ownerId}", h.getOwner)
	route("PUT "+base+"/{ownerId}", h.updateOwner)
	route("DELETE "+base+"/{ownerId}", h.deleteOwner)
	route("POST "+base+"/{ownerId}/pets", h.addPetToOwner)
	route("GET "+base+"/{ownerId}/pets/{petId}", h.getOwnersPet)
	route("PUT "+base+"/{ownerId}/pets/{petId}", h.updateOwnersPet)
	route("POST "+base+"/{ownerId}/pets/{petId}/visits", h.addVisitToOwner)
}

// listOwners handles GET /api/owners[?lastName=]. It answers 404 with an
// empty body when no owner matches.
func (h *OwnerHandler) listOwners(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var (
		owners []*model.Owner
		err    error
	)
	if q.Has("lastName") {
		owners, err = h.clinic.FindOwnerByLastName(r.Context(), q.Get("lastName"))
	} else {
		owners, err = h.clinic.FindAllOwners(r.Context())
	}
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing owners: %w", err))
		return
	}
	if len(owners) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, owners)
}

// getOwner handles GET /api/owners/{ownerId}.
func (h *OwnerHandler) getOwner(w http.ResponseWriter, r *http.Request) {
	id, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	owner, found, err := h.findOwner(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, owner)
}

// addOwner handles POST /api/owners. It answers 201 with
// Location: /api/owners/{id}.
func (h *OwnerHandler) addOwner(w http.ResponseWriter, r *http.Request) {
	var in ownerFieldsInput
	if !petTypeDecodeBody(w, r, &in) {
		return
	}
	if verr := validateOwnerFields(&in); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	owner := &model.Owner{}
	if err := applyOwnerFields(owner, &in); err != nil {
		HandleError(w, r, err)
		return
	}
	if err := h.clinic.SaveOwner(r.Context(), owner); err != nil {
		HandleError(w, r, fmt.Errorf("saving owner: %w", err))
		return
	}
	// MIGRATION_NOTE: UriComponentsBuilder.newInstance() builds a path-only
	// URI with no context path. That behaviour is kept here.
	if id := owner.GetID(); id != nil {
		w.Header().Set("Location", "/api/owners/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, owner)
}

// updateOwner handles PUT /api/owners/{ownerId}. It replaces the five scalar
// fields, leaves the pets untouched and answers 204.
func (h *OwnerHandler) updateOwner(w http.ResponseWriter, r *http.Request) {
	id, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	var in ownerFieldsInput
	if !petTypeDecodeBody(w, r, &in) {
		return
	}
	if verr := validateOwnerFields(&in); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	current, found, err := h.findOwner(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := applyOwnerFields(current, &in); err != nil {
		HandleError(w, r, err)
		return
	}
	if err := h.clinic.SaveOwner(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating owner %d: %w", id, err))
		return
	}
	// MIGRATION_NOTE: Spring passed an OwnerDto along with the 204, but a
	// 204 response carries no body.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// deleteOwner handles DELETE /api/owners/{ownerId}. The repository cascades
// the delete to the owner's pets and their visits.
//
// MIGRATION_NOTE: in the source, @Transactional covered both the lookup and
// the delete. Here the delete runs in its own transaction inside
// ClinicService.DeleteOwner and the lookup runs outside it. REQUIRES MANUAL
// REVIEW if one transaction around both is strictly needed.
func (h *OwnerHandler) deleteOwner(w http.ResponseWriter, r *http.Request) {
	id, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	owner, found, err := h.findOwner(r, id)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := h.clinic.DeleteOwner(r.Context(), owner); err != nil {
		HandleError(w, r, fmt.Errorf("deleting owner %d: %w", id, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addPetToOwner handles POST /api/owners/{ownerId}/pets. It answers 201 with
// Location: /api/pets/{id}.
func (h *OwnerHandler) addPetToOwner(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	var in petV1Input
	if !petTypeDecodeBody(w, r, &in) {
		return
	}
	if verr := validatePetV1Input(&in); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	_, found, err := h.findOwner(r, ownerID)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pet, err := petFromV1Input(&in)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	oid := ownerID
	pet.OwnerID = &oid
	// The client-supplied type name is discarded (source: setName(null)).
	// SavePet re-resolves the type from its id and replaces pet.Type.
	if err := h.clinic.SavePet(r.Context(), pet); err != nil {
		HandleError(w, r, fmt.Errorf("saving pet for owner %d: %w", ownerID, err))
		return
	}
	if id := pet.GetID(); id != nil {
		w.Header().Set("Location", "/api/pets/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, petToV1DTO(pet))
}

// getOwnersPet handles GET /api/owners/{ownerId}/pets/{petId}. It answers
// 404 when the owner is missing or the pet does not belong to that owner.
func (h *OwnerHandler) getOwnersPet(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	petID, ok := ownerParsePathID(w, r, "petId")
	if !ok {
		return
	}
	_, found, err := h.findOwner(r, ownerID)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pet, found, err := h.findPet(r, petID)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	// owner.getPet(petId) looked only inside the owner's own pets. The same
	// check is done here by comparing the pet's owner id.
	if !found || pet.GetOwnerID() == nil || *pet.GetOwnerID() != ownerID {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	petTypeWriteJSON(w, http.StatusOK, petToV1DTO(pet))
}

// updateOwnersPet handles PUT /api/owners/{ownerId}/pets/{petId}.
//
// MIGRATION_NOTE: kept from the source on purpose. The handler checks that
// both the owner and the pet exist, but NOT that the pet belongs to the owner.
func (h *OwnerHandler) updateOwnersPet(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerParsePathID(w, r, "ownerId")
	if !ok {
		return
	}
	petID, ok := ownerParsePathID(w, r, "petId")
	if !ok {
		return
	}
	var in petV1Input
	if !petTypeDecodeBody(w, r, &in) {
		return
	}
	if verr := validatePetV1Input(&in); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	_, found, err := h.findOwner(r, ownerID)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	current, found, err := h.findPet(r, petID)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	incoming, err := petFromV1Input(&in)
	if err != nil {
		HandleError(w, r, err)
		return
	}
	current.SetBirthDate(copyDatePtr(incoming.GetBirthDate()))
	current.Name = incoming.Name
	current.SetType(incoming.GetType())
	if err := h.clinic.SavePet(r.Context(), current); err != nil {
		HandleError(w, r, fmt.Errorf("updating pet %d: %w", petID, err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addVisitToOwner handles POST /api/owners/{ownerId}/pets/{petId}/visits.
// ownerId is ignored and nothing is checked for existence first. A missing
// pet shows up as an FK violation, which HandleError maps.
func (h *OwnerHandler) addVisitToOwner(w http.ResponseWriter, r *http.Request) {
	if _, ok := ownerParsePathID(w, r, "ownerId"); !ok {
		return
	}
	petID, ok := ownerParsePathID(w, r, "petId")
	if !ok {
		return
	}
	var in visitV1FieldsDTO
	if !petTypeDecodeBody(w, r, &in) {
		return
	}
	if verr := validateVisitFields(in.Description); verr.HasErrors() {
		HandleError(w, r, verr)
		return
	}
	visit := &model.Visit{}
	if in.Date != nil {
		visit.SetDate(copyDatePtr(in.Date))
	} else {
		// The Visit entity defaulted its date to today.
		visit.SetDate(ownerDatePtr(model.Today()))
	}
	visit.SetDescription(*in.Description)
	pid := petID
	visit.PetID = &pid
	if err := h.clinic.SaveVisit(r.Context(), visit); err != nil {
		HandleError(w, r, fmt.Errorf("saving visit for pet %d: %w", petID, err))
		return
	}
	if id := visit.GetID(); id != nil {
		w.Header().Set("Location", "/api/visits/"+strconv.Itoa(*id))
	}
	petTypeWriteJSON(w, http.StatusCreated, visitToV1DTO(visit))
}

// findOwner turns not-found errors into found=false (the source returned null).
func (h *OwnerHandler) findOwner(r *http.Request, id int) (*model.Owner, bool, error) {
	owner, err := h.clinic.FindOwnerByID(r.Context(), id)
	if err != nil {
		var orf *sqlstore.ObjectRetrievalFailureError
		if errors.Is(err, store.ErrNotFound) || errors.As(err, &orf) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("finding owner %d: %w", id, err)
	}
	if owner == nil {
		return nil, false, nil
	}
	return owner, true, nil
}

// findPet turns not-found errors into found=false (the source returned null).
func (h *OwnerHandler) findPet(r *http.Request, id int) (*model.Pet, bool, error) {
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

// ownerParsePathID parses a path parameter as an int32 >= 0. If parsing
// fails it writes a 400 and returns false.
func ownerParsePathID(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		HandleError(w, r, NewValidationError(FieldError{
			Field: name, RejectedValue: raw, DefaultMessage: "must be a valid integer",
		}))
		return 0, false
	}
	if id < 0 {
		HandleError(w, r, NewValidationError(FieldError{
			Field: name, RejectedValue: id, DefaultMessage: "must be greater than or equal to 0",
		}))
		return 0, false
	}
	return int(id), true
}

// validateOwnerFields applies the OwnerFields schema constraints.
func validateOwnerFields(in *ownerFieldsInput) *ValidationError {
	verr := NewValidationError()
	ownerCheckString(verr, "firstName", in.FirstName, 30, ownerNamePattern)
	ownerCheckString(verr, "lastName", in.LastName, 30, ownerNamePattern)
	ownerCheckString(verr, "address", in.Address, 255, nil)
	ownerCheckString(verr, "city", in.City, 80, nil)
	ownerCheckString(verr, "telephone", in.Telephone, 20, ownerTelephonePattern)
	return verr
}

// ownerCheckString checks that a field is present, between 1 and maxLen
// characters long, and matches pattern when one is given.
func ownerCheckString(verr *ValidationError, field string, v *string, maxLen int, pattern *regexp.Regexp) {
	if v == nil {
		verr.Add(field, nil, "must not be null")
		return
	}
	n := utf8.RuneCountInString(*v)
	if n < 1 || n > maxLen {
		verr.Add(field, *v, fmt.Sprintf("size must be between 1 and %d", maxLen))
	}
	if pattern != nil && !pattern.MatchString(*v) {
		verr.Add(field, *v, fmt.Sprintf("must match %q", pattern.String()))
	}
}

// applyOwnerFields copies the five validated OwnerFields onto owner. The
// owner's id and pets are left unchanged.
func applyOwnerFields(owner *model.Owner, in *ownerFieldsInput) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("encoding owner fields: %w", err)
	}
	if err := json.Unmarshal(raw, owner); err != nil {
		return fmt.Errorf("applying owner fields: %w", err)
	}
	return nil
}

// petFromV1Input builds a new Pet entity from validated PetFields. Only the
// type id is carried over; the type name is dropped.
func petFromV1Input(in *petV1Input) (*model.Pet, error) {
	fields := struct {
		Name      *string     `json:"name"`
		BirthDate *model.Date `json:"birthDate"`
		Type      struct {
			ID *int `json:"id"`
		} `json:"type"`
	}{Name: in.Name, BirthDate: in.BirthDate}
	if in.Type != nil {
		fields.Type.ID = in.Type.ID
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encoding pet fields: %w", err)
	}
	pet := &model.Pet{}
	if err := json.Unmarshal(raw, pet); err != nil {
		return nil, fmt.Errorf("decoding pet fields: %w", err)
	}
	return pet, nil
}

// ownerDatePtr turns either a model.Date or a *model.Date into a *model.Date.
func ownerDatePtr(v any) *model.Date {
	switch d := v.(type) {
	case model.Date:
		return &d
	case *model.Date:
		return d
	default:
		return nil
	}
}
