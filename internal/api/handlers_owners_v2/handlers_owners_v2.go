package api

// This file is the port of OwnerRestControllerV2: the paginated owner listing
// GET {contextPath}/api/v2/owners?lastName=&page=&size=.
//
// MIGRATION_NOTE: @PreAuthorize("hasRole(@roles.OWNER_ADMIN)") is expressed via
// the injected Authorizer middleware (see handlers_pettypes.go). The
// @CrossOrigin(exposedHeaders = "errors, content-type") behaviour is provided by
// withRootCORS. Spring's PageRequest.of(page, size, Sort.by("id")) becomes the
// (page, size) arguments of service.Clinic.FindOwners; the repository orders
// by id.
//
// DELIBERATE DEVIATION: page < 0 or size < 1 made PageRequest.of throw
// IllegalArgumentException (HTTP 500) in the source. Here they are rejected
// up front with a 400 validation problem, as the migration plan agreed.

import (
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strconv"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// Owner v2 endpoint constants.
const (
	ownerV2RoleOwnerAdmin  = "OWNER_ADMIN"
	ownerV2DefaultPage     = 0
	ownerV2DefaultPageSize = 20
)

// OwnerPageDTO is the JSON body of GET /api/v2/owners (OwnerPageDto).
type OwnerPageDTO struct {
	Content       []*model.Owner `json:"content"`
	Page          int            `json:"page"`
	Size          int            `json:"size"`
	TotalElements int64          `json:"totalElements"`
	TotalPages    int            `json:"totalPages"`
}

// OwnerV2Handler serves the v2 owner endpoints.
type OwnerV2Handler struct {
	clinic    service.Clinic
	authorize Authorizer
}

// NewOwnerV2Handler returns an OwnerV2Handler backed by clinic. authorize may
// be nil, in which case no role checks are applied (security disabled).
func NewOwnerV2Handler(clinic service.Clinic, authorize Authorizer) *OwnerV2Handler {
	if authorize == nil {
		authorize = func(next http.Handler, _ ...string) http.Handler { return next }
	}
	return &OwnerV2Handler{clinic: clinic, authorize: authorize}
}

// Register mounts GET {contextPath}/api/v2/owners on mux.
func (h *OwnerV2Handler) Register(mux *http.ServeMux, contextPath string) {
	base := normalizeContextPath(contextPath) + "/api/v2/owners"
	mux.Handle("GET "+base, withRootCORS(h.authorize(http.HandlerFunc(h.listOwnersPage), ownerV2RoleOwnerAdmin)))
}

// listOwnersPage handles GET /api/v2/owners. It always answers 200 with an
// OwnerPage on success; page defaults to 0 and size to 20.
func (h *OwnerV2Handler) listOwnersPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var lastName *string
	if q.Has("lastName") {
		v := q.Get("lastName")
		lastName = &v
	}

	verr := NewValidationError()
	page, pageOK := ownerV2ParseInt(verr, q.Get("page"), "page", ownerV2DefaultPage, 0)
	size, sizeOK := ownerV2ParseInt(verr, q.Get("size"), "size", ownerV2DefaultPageSize, 1)
	if !pageOK || !sizeOK {
		HandleError(w, r, verr)
		return
	}

	result, err := h.clinic.FindOwners(r.Context(), lastName, page, size)
	if err != nil {
		HandleError(w, r, fmt.Errorf("listing owners page: %w", err))
		return
	}
	petTypeWriteJSON(w, http.StatusOK, ownerPageToDTO(result, page, size))
}

// ownerV2ParseInt parses an optional 32-bit integer query parameter, applying
// def when absent and enforcing value >= minimum. Violations are added to verr.
func ownerV2ParseInt(verr *ValidationError, raw, field string, def, minimum int) (int, bool) {
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

// ownerPageToDTO is the port of OwnerMapper.toOwnerPageDto. Owners are
// serialized through their model JSON representation (id, firstName,
// lastName, address, city, telephone, pets).
//
// MIGRATION_NOTE / MANUAL REVIEW: the field layout of the generic store.Page
// type is read reflectively (Content/Items, Number/Page, Size, TotalElements/
// Total, TotalPages) so this handler stays decoupled from it. Replace with
// direct field access once store.Page is final; values not found fall back to
// the requested page/size and a computed totalPages.
func ownerPageToDTO(p any, reqPage, reqSize int) OwnerPageDTO {
	dto := OwnerPageDTO{Content: []*model.Owner{}, Page: reqPage, Size: reqSize}

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

	if c, ok := ownerV2Field(v, "Content", "Items", "Elements", "Data"); ok && c.Kind() == reflect.Slice {
		for i := 0; i < c.Len(); i++ {
			if o, ok := c.Index(i).Interface().(*model.Owner); ok && o != nil {
				dto.Content = append(dto.Content, o)
			}
		}
	}
	if n, ok := ownerV2IntField(v, "Number", "Page", "PageNumber"); ok {
		dto.Page = int(n)
	}
	if n, ok := ownerV2IntField(v, "Size", "PageSize"); ok {
		dto.Size = int(n)
	}
	if n, ok := ownerV2IntField(v, "TotalElements", "Total", "TotalCount"); ok {
		dto.TotalElements = n
	} else {
		dto.TotalElements = int64(len(dto.Content))
	}
	if n, ok := ownerV2IntField(v, "TotalPages"); ok {
		dto.TotalPages = int(n)
	} else if dto.Size > 0 {
		dto.TotalPages = int(math.Ceil(float64(dto.TotalElements) / float64(dto.Size)))
	}
	return dto
}

// ownerV2Field returns the first exported field of v matching one of names.
func ownerV2Field(v reflect.Value, names ...string) (reflect.Value, bool) {
	for _, name := range names {
		if f := v.FieldByName(name); f.IsValid() && f.CanInterface() {
			return f, true
		}
	}
	return reflect.Value{}, false
}

// ownerV2IntField returns the first integer field of v matching one of names.
func ownerV2IntField(v reflect.Value, names ...string) (int64, bool) {
	f, ok := ownerV2Field(v, names...)
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
