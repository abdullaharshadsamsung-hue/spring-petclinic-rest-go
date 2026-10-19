package api

// This file is the port of PetRestControllerV2: the paginated pet listing
// GET {contextPath}/api/v2/pets?page=&size=.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). The
// @CrossOrigin(exposedHeaders = "errors, content-type") behaviour is provided by
// withRootCORS. Spring's PageRequest.of(page, size, Sort.by("id")) becomes the
// (page, size) arguments of service.Clinic.FindPets; the repository orders by
// pets.id ascending.
//
// DELIBERATE DEVIATION: page < 0 or size < 1 made PageRequest.of throw
// IllegalArgumentException (HTTP 500) in the source. Here they are rejected
// up front with a 400 validation problem, as the migration plan agreed (and
// consistent with the owner v2 endpoint).

import (
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// Pet v2 endpoint constants.
const (
	petV2RoleOwnerAdmin  = "OWNER_ADMIN"
	petV2DefaultPage     = 0
	petV2DefaultPageSize = 20
)

// PetV2PageDTO is the JSON body of GET /api/v2/pets (PetPageDto).
type PetV2PageDTO struct {
	Content       []*model.Pet `json:"content"`
	Page          int          `json:"page"`
	Size          int          `json:"size"`
	TotalElements int64        `json:"totalElements"`
	TotalPages    int          `json:"totalPages"`
}

// PetV2Handler serves the v2 pet endpoints.
type PetV2Handler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewPetV2Handler returns a PetV2Handler backed by clinic. authorize may be
// nil, in which case no role checks are applied (security disabled).
func NewPetV2Handler(clinic service.Clinic, authorize Authorizer) *PetV2Handler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &PetV2Handler{clinic: clinic, authorize: authorize}
}

// Register mounts GET {contextPath}/api/v2/pets on mux.
func (h *PetV2Handler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/v2/pets"
	mux.Handle("GET "+base, withRootCORS(h.authorize(http.HandlerFunc(h.listPetsPage), petV2RoleOwnerAdmin)))
}

// listPetsPage handles GET /api/v2/pets. It answers 200 with a PetPage on
// success; page defaults to 0 and size to 20.
func (h *PetV2Handler) listPetsPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	verr := NewValidationError()
	page, pageOK := petV2ParseInt(verr, q.Get("page"), "page", petV2DefaultPage, 0)
	size, sizeOK := petV2ParseInt(verr, q.Get("size"), "size", petV2DefaultPageSize, 1)
	if !pageOK || !sizeOK {
		HandleError(w, r, verr)
		return
	}

	result, err := h.clinic.FindPets(r.Context(), page, size)
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing pets page: %w", err))
		return
	}
	petTypeWriteJSON(w, http.StatusOK, petPageToDTO(result, page, size))
}

// petV2ParseInt parses an optional 32-bit integer query parameter, applying
// def when absent and enforcing value >= minimum. Violations are added to verr.
func petV2ParseInt(verr *ValidationError, raw, field string, def, minimum int) (int, bool) {
	if raw == "" {
		return def, true
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		verr.Add(field, raw, "must be a valid integer")
		return 0, false
	}
	if int(n) < minimum {
		verr.Add(field, n, fmt.Sprintf("must be greater than or equal to %d", minimum))
		return 0, false
	}
	return int(n), true
}

// petPageToDTO is the port of PetMapper.toPetPageDto. Pets are serialized via
// their model JSON representation (id, name, birthDate, type, ownerId, visits).
//
// MIGRATION_NOTE / MANUAL REVIEW: the generic store.Page layout is read
// reflectively (Content/Items, Number/Page, Size, TotalElements/Total,
// TotalPages), the same approach as the owner v2 handler. Replace with direct
// field access once store.Page is final. Missing values fall back to the
// requested page/size and a computed totalPages. Nil pets are skipped.
func petPageToDTO(p any, reqPage, reqSize int) PetV2PageDTO {
	dto := PetV2PageDTO{Content: []*model.Pet{}, Page: reqPage, Size: reqSize}

	v := reflect.ValueOf(p)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return dto
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return dto
	}

	if c, ok := petV2Field(v, "Content", "Items", "Elements", "Data"); ok && c.Kind() == reflect.Slice {
		for i := 0; i < c.Len(); i++ {
			if pet, ok := c.Index(i).Interface().(*model.Pet); ok && pet != nil {
				dto.Content = append(dto.Content, pet)
			}
		}
	}
	if n, ok := petV2IntField(v, "Number", "Page", "PageNumber"); ok {
		dto.Page = int(n)
	}
	if n, ok := petV2IntField(v, "Size", "PageSize"); ok {
		dto.Size = int(n)
	}
	if n, ok := petV2IntField(v, "TotalElements", "Total", "TotalCount"); ok {
		dto.TotalElements = n
	} else {
		dto.TotalElements = int64(len(dto.Content))
	}
	if n, ok := petV2IntField(v, "TotalPages"); ok {
		dto.TotalPages = int(n)
	} else if dto.Size > 0 {
		dto.TotalPages = int(math.Ceil(float64(dto.TotalElements) / float64(dto.Size)))
	}
	return dto
}

// petV2Field returns the first exported field of v matching one of names.
func petV2Field(v reflect.Value, names ...string) (reflect.Value, bool) {
	for _, name := range names {
		if f := v.FieldByName(name); f.IsValid() && f.CanInterface() {
			return f, true
		}
	}
	return reflect.Value{}, false
}

// petV2IntField returns the first integer field of v matching one of names.
func petV2IntField(v reflect.Value, names ...string) (int64, bool) {
	f, ok := petV2Field(v, names...)
	if !ok {
		return 0, false
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return f.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(f.Uint()), true
	default:
		return 0, false
	}
}
