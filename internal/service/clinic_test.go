package service

import (
	"context"
	"errors"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// ---- fakes ----

type fakeTx struct {
	calls int
	err   error
}

func (f *fakeTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return fn(ctx)
}

type fakePets struct {
	all              []*model.Pet
	one              *model.Pet
	types            []*model.PetType
	err              error
	saved, deleted   []*model.Pet
	gotID            int
	gotPage, gotSize int
}

func (f *fakePets) FindAll(ctx context.Context) ([]*model.Pet, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakePets) FindAllPage(ctx context.Context, page, size int) (store.Page[*model.Pet], error) {
	f.gotPage, f.gotSize = page, size
	return store.Page[*model.Pet]{}, f.err
}
func (f *fakePets) FindByID(ctx context.Context, id int) (*model.Pet, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakePets) FindPetTypes(ctx context.Context) ([]*model.PetType, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.types, nil
}
func (f *fakePets) Save(ctx context.Context, p *model.Pet) error {
	f.saved = append(f.saved, p)
	return f.err
}
func (f *fakePets) Delete(ctx context.Context, p *model.Pet) error {
	f.deleted = append(f.deleted, p)
	return f.err
}

type fakeOwners struct {
	all               []*model.Owner
	one               *model.Owner
	err               error
	saved, deleted    []*model.Owner
	gotID             int
	gotLastName       string
	allPageCalls      int
	lastNamePageCalls int
	gotPage, gotSize  int
}

func (f *fakeOwners) FindAll(ctx context.Context) ([]*model.Owner, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeOwners) FindAllPage(ctx context.Context, page, size int) (store.Page[*model.Owner], error) {
	f.allPageCalls++
	f.gotPage, f.gotSize = page, size
	return store.Page[*model.Owner]{}, f.err
}
func (f *fakeOwners) FindByLastName(ctx context.Context, lastName string) ([]*model.Owner, error) {
	f.gotLastName = lastName
	if f.err != nil {
		return nil, f.err
	}
	if lastName == "Davis" {
		return f.all, nil
	}
	return []*model.Owner{}, nil
}
func (f *fakeOwners) FindByLastNamePage(ctx context.Context, lastName string, page, size int) (store.Page[*model.Owner], error) {
	f.lastNamePageCalls++
	f.gotLastName = lastName
	f.gotPage, f.gotSize = page, size
	return store.Page[*model.Owner]{}, f.err
}
func (f *fakeOwners) FindByID(ctx context.Context, id int) (*model.Owner, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakeOwners) Save(ctx context.Context, o *model.Owner) error {
	f.saved = append(f.saved, o)
	return f.err
}
func (f *fakeOwners) Delete(ctx context.Context, o *model.Owner) error {
	f.deleted = append(f.deleted, o)
	return f.err
}

type fakeVets struct {
	all            []*model.Vet
	one            *model.Vet
	err            error
	saved, deleted []*model.Vet
	allCalls       int
}

func (f *fakeVets) FindAll(ctx context.Context) ([]*model.Vet, error) {
	f.allCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeVets) FindByID(ctx context.Context, id int) (*model.Vet, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakeVets) Save(ctx context.Context, v *model.Vet) error {
	f.saved = append(f.saved, v)
	return f.err
}
func (f *fakeVets) Delete(ctx context.Context, v *model.Vet) error {
	f.deleted = append(f.deleted, v)
	return f.err
}

type fakeVisits struct {
	all            []*model.Visit
	one            *model.Visit
	err            error
	saved, deleted []*model.Visit
	gotPetID       int
}

func (f *fakeVisits) FindAll(ctx context.Context) ([]*model.Visit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeVisits) FindByID(ctx context.Context, id int) (*model.Visit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakeVisits) FindByPetID(ctx context.Context, petID int) ([]*model.Visit, error) {
	f.gotPetID = petID
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeVisits) Save(ctx context.Context, v *model.Visit) error {
	f.saved = append(f.saved, v)
	return f.err
}
func (f *fakeVisits) Delete(ctx context.Context, v *model.Visit) error {
	f.deleted = append(f.deleted, v)
	return f.err
}

type fakeSpecialties struct {
	all            []*model.Specialty
	one            *model.Specialty
	err            error
	saved, deleted []*model.Specialty
	gotNames       []string
}

func (f *fakeSpecialties) FindAll(ctx context.Context) ([]*model.Specialty, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeSpecialties) FindByID(ctx context.Context, id int) (*model.Specialty, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakeSpecialties) FindSpecialtiesByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error) {
	f.gotNames = names
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakeSpecialties) Save(ctx context.Context, s *model.Specialty) error {
	f.saved = append(f.saved, s)
	return f.err
}
func (f *fakeSpecialties) Delete(ctx context.Context, s *model.Specialty) error {
	f.deleted = append(f.deleted, s)
	return f.err
}

type fakePetTypes struct {
	all            []*model.PetType
	one            *model.PetType
	err            error
	findCalls      int
	saved, deleted []*model.PetType
}

func (f *fakePetTypes) FindAll(ctx context.Context) ([]*model.PetType, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.all, nil
}
func (f *fakePetTypes) FindByID(ctx context.Context, id int) (*model.PetType, error) {
	f.findCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.one, nil
}
func (f *fakePetTypes) Save(ctx context.Context, p *model.PetType) error {
	f.saved = append(f.saved, p)
	return f.err
}
func (f *fakePetTypes) Delete(ctx context.Context, p *model.PetType) error {
	f.deleted = append(f.deleted, p)
	return f.err
}

type env struct {
	tx          *fakeTx
	pets        *fakePets
	owners      *fakeOwners
	vets        *fakeVets
	visits      *fakeVisits
	specialties *fakeSpecialties
	petTypes    *fakePetTypes
	svc         *ClinicService
}

func newEnv(err error) *env {
	e := &env{
		tx: &fakeTx{},
		pets: &fakePets{
			all:   []*model.Pet{{}, {}, {}},
			one:   &model.Pet{},
			types: []*model.PetType{{}, {}},
			err:   err,
		},
		owners:      &fakeOwners{all: []*model.Owner{{}, {}}, one: &model.Owner{}, err: err},
		vets:        &fakeVets{all: []*model.Vet{{}, {}, {}}, one: &model.Vet{}, err: err},
		visits:      &fakeVisits{all: []*model.Visit{{}, {}}, one: &model.Visit{}, err: err},
		specialties: &fakeSpecialties{all: []*model.Specialty{{}, {}, {}}, one: &model.Specialty{}, err: err},
		petTypes:    &fakePetTypes{all: []*model.PetType{{}, {}, {}}, one: &model.PetType{}, err: err},
	}
	e.svc = NewClinic(e.tx, e.pets, e.vets, e.owners, e.visits, e.specialties, e.petTypes)
	return e
}

var errDB = errors.New("db down")

// ---- tests ----

func TestClinicService_ImplementsClinic(t *testing.T) {
	var c Clinic = NewClinic(nil, nil, nil, nil, nil, nil, nil)
	if c == nil {
		t.Fatal("expected non-nil Clinic")
	}
}

type findCase struct {
	name string
	call func(s *ClinicService) (bool, error)
}

func findCases() []findCase {
	ctx := context.Background()
	return []findCase{
		{"FindAllPets", func(s *ClinicService) (bool, error) { r, err := s.FindAllPets(ctx); return r == nil, err }},
		{"FindPetByID", func(s *ClinicService) (bool, error) { r, err := s.FindPetByID(ctx, 7); return r == nil, err }},
		{"FindPetTypes", func(s *ClinicService) (bool, error) { r, err := s.FindPetTypes(ctx); return r == nil, err }},
		{"FindVisitByID", func(s *ClinicService) (bool, error) { r, err := s.FindVisitByID(ctx, 1); return r == nil, err }},
		{"FindAllVisits", func(s *ClinicService) (bool, error) { r, err := s.FindAllVisits(ctx); return r == nil, err }},
		{"FindVisitsByPetID", func(s *ClinicService) (bool, error) { r, err := s.FindVisitsByPetID(ctx, 7); return r == nil, err }},
		{"FindVetByID", func(s *ClinicService) (bool, error) { r, err := s.FindVetByID(ctx, 1); return r == nil, err }},
		{"FindAllVets", func(s *ClinicService) (bool, error) { r, err := s.FindAllVets(ctx); return r == nil, err }},
		{"FindVets", func(s *ClinicService) (bool, error) { r, err := s.FindVets(ctx); return r == nil, err }},
		{"FindAllOwners", func(s *ClinicService) (bool, error) { r, err := s.FindAllOwners(ctx); return r == nil, err }},
		{"FindOwnerByID", func(s *ClinicService) (bool, error) { r, err := s.FindOwnerByID(ctx, 1); return r == nil, err }},
		{"FindOwnerByLastName", func(s *ClinicService) (bool, error) { r, err := s.FindOwnerByLastName(ctx, "Davis"); return r == nil, err }},
		{"FindPetTypeByID", func(s *ClinicService) (bool, error) { r, err := s.FindPetTypeByID(ctx, 1); return r == nil, err }},
		{"FindAllPetTypes", func(s *ClinicService) (bool, error) { r, err := s.FindAllPetTypes(ctx); return r == nil, err }},
		{"FindSpecialtyByID", func(s *ClinicService) (bool, error) { r, err := s.FindSpecialtyByID(ctx, 1); return r == nil, err }},
		{"FindAllSpecialties", func(s *ClinicService) (bool, error) { r, err := s.FindAllSpecialties(ctx); return r == nil, err }},
	}
}

func TestFind_Success(t *testing.T) {
	for _, tc := range findCases() {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(nil)
			isNil, err := tc.call(e.svc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if isNil {
				t.Fatal("expected non-nil result")
			}
			if e.tx.calls != 0 {
				t.Errorf("reads must not open a transaction, got %d", e.tx.calls)
			}
		})
	}
}

func TestFindPetByID_PassesIDAndReturnsSamePet(t *testing.T) {
	e := newEnv(nil)
	got, err := e.svc.FindPetByID(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if got != e.pets.one || e.pets.gotID != 7 {
		t.Errorf("unexpected pet or id: %v %d", got, e.pets.gotID)
	}
}

func TestFindOwnerByID_PassesID(t *testing.T) {
	e := newEnv(nil)
	got, err := e.svc.FindOwnerByID(context.Background(), 1)
	if err != nil || got != e.owners.one || e.owners.gotID != 1 {
		t.Fatalf("got %v %v id=%d", got, err, e.owners.gotID)
	}
}

func TestFindOwnerByLastName(t *testing.T) {
	tests := []struct {
		lastName string
		want     int
	}{
		{"Davis", 2},
		{"Daviss", 0},
	}
	for _, tc := range tests {
		t.Run(tc.lastName, func(t *testing.T) {
			e := newEnv(nil)
			got, err := e.svc.FindOwnerByLastName(context.Background(), tc.lastName)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("expected non-nil (possibly empty) collection")
			}
			if len(got) != tc.want {
				t.Errorf("len = %d, want %d", len(got), tc.want)
			}
			if e.owners.gotLastName != tc.lastName {
				t.Errorf("lastName passed = %q", e.owners.gotLastName)
			}
		})
	}
}

func TestFindVisitsByPetID_PassesPetID(t *testing.T) {
	e := newEnv(nil)
	got, err := e.svc.FindVisitsByPetID(context.Background(), 7)
	if err != nil || len(got) != 2 || e.visits.gotPetID != 7 {
		t.Fatalf("got len=%d err=%v petID=%d", len(got), err, e.visits.gotPetID)
	}
}

func TestFindVets_SameAsFindAllVets(t *testing.T) {
	e := newEnv(nil)
	a, err1 := e.svc.FindAllVets(context.Background())
	b, err2 := e.svc.FindVets(context.Background())
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("element %d differs", i)
		}
	}
	if e.vets.allCalls != 2 {
		t.Errorf("expected 2 repo calls, got %d", e.vets.allCalls)
	}
}

func TestFindPets(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"ok", nil, false},
		{"repo error", errDB, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(tc.err)
			_, err := e.svc.FindPets(context.Background(), 2, 5)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tc.wantErr && !errors.Is(err, tc.err) {
				t.Errorf("error not wrapped: %v", err)
			}
			if e.pets.gotPage != 2 || e.pets.gotSize != 5 {
				t.Errorf("page/size = %d/%d", e.pets.gotPage, e.pets.gotSize)
			}
		})
	}
}

func TestFindOwners(t *testing.T) {
	davis := "Davis"
	tests := []struct {
		name             string
		lastName         *string
		err              error
		wantAllPage      int
		wantLastNamePage int
	}{
		{"nil lastName means no filter", nil, nil, 1, 0},
		{"lastName filters", &davis, nil, 0, 1},
		{"nil lastName error", nil, errDB, 1, 0},
		{"lastName error", &davis, errDB, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(tc.err)
			_, err := e.svc.FindOwners(context.Background(), tc.lastName, 0, 3)
			if tc.err == nil && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("expected wrapped %v, got %v", tc.err, err)
			}
			if e.owners.allPageCalls != tc.wantAllPage || e.owners.lastNamePageCalls != tc.wantLastNamePage {
				t.Errorf("calls all=%d lastName=%d", e.owners.allPageCalls, e.owners.lastNamePageCalls)
			}
			if tc.lastName != nil && e.owners.gotLastName != *tc.lastName {
				t.Errorf("lastName = %q", e.owners.gotLastName)
			}
			if e.owners.gotPage != 0 || e.owners.gotSize != 3 {
				t.Errorf("page/size = %d/%d", e.owners.gotPage, e.owners.gotSize)
			}
		})
	}
}

func TestFindSpecialtiesByNameIn(t *testing.T) {
	names := []string{"radiology", "surgery", "dentistry"}
	tests := []struct {
		name     string
		err      error
		wantLen  int
		wantNil  bool
		wantWrap error
	}{
		{"found", nil, 3, false, nil},
		{"not found swallowed", store.ErrNotFound, 0, true, nil},
		{"other error propagates", errDB, 0, true, errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(tc.err)
			got, err := e.svc.FindSpecialtiesByNameIn(context.Background(), names)
			if tc.wantWrap == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantWrap != nil && !errors.Is(err, tc.wantWrap) {
				t.Fatalf("expected %v, got %v", tc.wantWrap, err)
			}
			if (got == nil) != tc.wantNil || len(got) != tc.wantLen {
				t.Errorf("got %v", got)
			}
			if len(e.specialties.gotNames) != 3 {
				t.Errorf("names not passed: %v", e.specialties.gotNames)
			}
		})
	}
}

type deleteCase struct {
	name    string
	call    func(s *ClinicService) error
	deleted func(e *env) int
}

func deleteCases() []deleteCase {
	ctx := context.Background()
	return []deleteCase{
		{"DeletePet", func(s *ClinicService) error { return s.DeletePet(ctx, &model.Pet{}) }, func(e *env) int { return len(e.pets.deleted) }},
		{"DeleteVisit", func(s *ClinicService) error { return s.DeleteVisit(ctx, &model.Visit{}) }, func(e *env) int { return len(e.visits.deleted) }},
		{"DeleteVet", func(s *ClinicService) error { return s.DeleteVet(ctx, &model.Vet{}) }, func(e *env) int { return len(e.vets.deleted) }},
		{"DeleteOwner", func(s *ClinicService) error { return s.DeleteOwner(ctx, &model.Owner{}) }, func(e *env) int { return len(e.owners.deleted) }},
		{"DeletePetType", func(s *ClinicService) error { return s.DeletePetType(ctx, &model.PetType{}) }, func(e *env) int { return len(e.petTypes.deleted) }},
		{"DeleteSpecialty", func(s *ClinicService) error { return s.DeleteSpecialty(ctx, &model.Specialty{}) }, func(e *env) int { return len(e.specialties.deleted) }},
	}
}

func TestDelete(t *testing.T) {
	for _, tc := range deleteCases() {
		t.Run(tc.name+"/ok", func(t *testing.T) {
			e := newEnv(nil)
			if err := tc.call(e.svc); err != nil {
				t.Fatal(err)
			}
			if tc.deleted(e) != 1 {
				t.Error("repo Delete not called")
			}
			if e.tx.calls != 1 {
				t.Errorf("expected 1 transaction, got %d", e.tx.calls)
			}
		})
		t.Run(tc.name+"/repo error", func(t *testing.T) {
			e := newEnv(errDB)
			if err := tc.call(e.svc); !errors.Is(err, errDB) {
				t.Fatalf("expected wrapped errDB, got %v", err)
			}
		})
		t.Run(tc.name+"/tx error", func(t *testing.T) {
			e := newEnv(nil)
			e.tx.err = errDB
			if err := tc.call(e.svc); !errors.Is(err, errDB) {
				t.Fatalf("expected tx error, got %v", err)
			}
			if tc.deleted(e) != 0 {
				t.Error("repo should not be called when tx fails")
			}
		})
		t.Run(tc.name+"/nil tx", func(t *testing.T) {
			e := newEnv(nil)
			e.svc.tx = nil
			if err := tc.call(e.svc); err != nil {
				t.Fatal(err)
			}
			if tc.deleted(e) != 1 {
				t.Error("repo Delete not called")
			}
		})
	}
}

type saveCase struct {
	name    string
	nilCall func(s *ClinicService) error
	saved   func(e *env) int
}

func saveCases() []saveCase {
	ctx := context.Background()
	return []saveCase{
		{"SaveVisit",
			func(s *ClinicService) error { return s.SaveVisit(ctx, nil) },
			func(e *env) int { return len(e.visits.saved) }},
		{"SaveVet",
			func(s *ClinicService) error { return s.SaveVet(ctx, nil) },
			func(e *env) int { return len(e.vets.saved) }},
		{"SaveOwner",
			func(s *ClinicService) error { return s.SaveOwner(ctx, nil) },
			func(e *env) int { return len(e.owners.saved) }},
		{"SavePetType",
			func(s *ClinicService) error { return s.SavePetType(ctx, nil) },
			func(e *env) int { return len(e.petTypes.saved) }},
		{"SaveSpecialty",
			func(s *ClinicService) error { return s.SaveSpecialty(ctx, nil) },
			func(e *env) int { return len(e.specialties.saved) }},
		{"SavePet",
			func(s *ClinicService) error { return s.SavePet(ctx, nil) },
			func(e *env) int { return len(e.pets.saved) }},
	}
}

func TestSave_Nil(t *testing.T) {
	for _, tc := range saveCases() {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(nil)
			if err := tc.nilCall(e.svc); err == nil {
				t.Fatal("expected error for nil entity")
			}
			if tc.saved(e) != 0 || e.tx.calls != 0 {
				t.Error("repo/tx must not be called for nil entity")
			}
		})
	}
}

func TestSavePet_NilType(t *testing.T) {
	e := newEnv(nil)
	pet := &model.Pet{}
	pet.Type = nil
	err := e.svc.SavePet(context.Background(), pet)
	if err == nil {
		t.Fatal("expected error when pet has no type")
	}
	if e.tx.calls == 1 && !errors.Is(err, store.ErrIntegrity) {
		t.Errorf("expected ErrIntegrity, got %v", err)
	}
	if len(e.pets.saved) != 0 {
		t.Error("pet must not be saved without type")
	}
}

type fakeValidator struct{ err error }

func (f fakeValidator) Validate() error { return f.err }

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want error
	}{
		{"no validator", 42, nil},
		{"valid", fakeValidator{}, nil},
		{"invalid", fakeValidator{err: errDB}, errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validate(tc.in); !errors.Is(err, tc.want) {
				t.Errorf("got %v want %v", err, tc.want)
			}
		})
	}
}