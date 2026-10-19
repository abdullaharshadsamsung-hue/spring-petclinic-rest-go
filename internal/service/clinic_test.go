package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// fakeClinic is an in-memory implementation of Clinic used to verify the
// behavioural contract documented on the interface.
type fakeClinic struct {
	nextID  int
	pets    map[int]*model.Pet
	visits  map[int]*model.Visit
	vets    map[int]*model.Vet
	owners  map[int]*model.Owner
	types   map[int]*model.PetType
	specs   map[int]*model.Specialty
	failErr error
}

var _ Clinic = (*fakeClinic)(nil)

var errStorage = errors.New("storage failure")

func newFakeClinic() *fakeClinic {
	return &fakeClinic{
		pets:   map[int]*model.Pet{},
		visits: map[int]*model.Visit{},
		vets:   map[int]*model.Vet{},
		owners: map[int]*model.Owner{},
		types:  map[int]*model.PetType{},
		specs:  map[int]*model.Specialty{},
	}
}

func (f *fakeClinic) assign(p **int) int {
	if *p == nil {
		f.nextID++
		v := f.nextID
		*p = &v
	}
	return **p
}

func notFound(kind string, id int) error {
	return fmt.Errorf("%s %d: %w", kind, id, store.ErrNotFound)
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func values[V any](m map[int]V) []V {
	out := make([]V, 0, len(m))
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}

func (f *fakeClinic) FindPetByID(_ context.Context, id int) (*model.Pet, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	p, ok := f.pets[id]
	if !ok {
		return nil, notFound("pet", id)
	}
	return p, nil
}

func (f *fakeClinic) FindAllPets(context.Context) ([]*model.Pet, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.pets), nil
}

func (f *fakeClinic) FindPets(context.Context, store.PageRequest) (store.Page[*model.Pet], error) {
	if f.failErr != nil {
		return store.Page[*model.Pet]{}, f.failErr
	}
	return store.Page[*model.Pet]{}, nil
}

func (f *fakeClinic) SavePet(_ context.Context, pet *model.Pet) error {
	if f.failErr != nil {
		return f.failErr
	}
	if pet == nil {
		return fmt.Errorf("nil pet: %w", store.ErrIntegrity)
	}
	f.pets[f.assign(&pet.ID)] = pet
	return nil
}

func (f *fakeClinic) DeletePet(_ context.Context, pet *model.Pet) error {
	if f.failErr != nil {
		return f.failErr
	}
	if pet == nil || pet.ID == nil {
		return fmt.Errorf("delete pet: %w", store.ErrNotFound)
	}
	id := *pet.ID
	for vid, v := range f.visits {
		if pid := v.GetPetID(); pid != nil && *pid == id {
			delete(f.visits, vid)
		}
	}
	delete(f.pets, id)
	return nil
}

func (f *fakeClinic) FindVisitsByPetID(_ context.Context, petID int) ([]*model.Visit, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	out := []*model.Visit{}
	for _, v := range values(f.visits) {
		if pid := v.GetPetID(); pid != nil && *pid == petID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeClinic) FindVisitByID(_ context.Context, id int) (*model.Visit, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	v, ok := f.visits[id]
	if !ok {
		return nil, notFound("visit", id)
	}
	return v, nil
}

func (f *fakeClinic) FindAllVisits(context.Context) ([]*model.Visit, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.visits), nil
}

func (f *fakeClinic) SaveVisit(_ context.Context, visit *model.Visit) error {
	if f.failErr != nil {
		return f.failErr
	}
	pid := visit.GetPetID()
	if pid == nil {
		return fmt.Errorf("visit without pet: %w", store.ErrIntegrity)
	}
	if _, ok := f.pets[*pid]; !ok {
		return fmt.Errorf("visit pet %d: %w", *pid, store.ErrIntegrity)
	}
	f.visits[f.assign(&visit.ID)] = visit
	return nil
}

func (f *fakeClinic) DeleteVisit(_ context.Context, visit *model.Visit) error {
	if f.failErr != nil {
		return f.failErr
	}
	if visit == nil || visit.ID == nil {
		return fmt.Errorf("delete visit: %w", store.ErrNotFound)
	}
	delete(f.visits, *visit.ID)
	return nil
}

func (f *fakeClinic) FindVetByID(_ context.Context, id int) (*model.Vet, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	v, ok := f.vets[id]
	if !ok {
		return nil, notFound("vet", id)
	}
	return v, nil
}

func (f *fakeClinic) FindVets(ctx context.Context) ([]*model.Vet, error) { return f.FindAllVets(ctx) }

func (f *fakeClinic) FindAllVets(context.Context) ([]*model.Vet, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.vets), nil
}

func (f *fakeClinic) SaveVet(_ context.Context, vet *model.Vet) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.vets[f.assign(&vet.ID)] = vet
	return nil
}

func (f *fakeClinic) DeleteVet(_ context.Context, vet *model.Vet) error {
	if f.failErr != nil {
		return f.failErr
	}
	if vet == nil || vet.ID == nil {
		return fmt.Errorf("delete vet: %w", store.ErrNotFound)
	}
	delete(f.vets, *vet.ID)
	return nil
}

func (f *fakeClinic) FindOwnerByID(_ context.Context, id int) (*model.Owner, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	o, ok := f.owners[id]
	if !ok {
		return nil, notFound("owner", id)
	}
	return o, nil
}

func (f *fakeClinic) FindAllOwners(context.Context) ([]*model.Owner, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.owners), nil
}

func (f *fakeClinic) FindOwners(context.Context, string, store.PageRequest) (store.Page[*model.Owner], error) {
	if f.failErr != nil {
		return store.Page[*model.Owner]{}, f.failErr
	}
	return store.Page[*model.Owner]{}, nil
}

func (f *fakeClinic) SaveOwner(_ context.Context, owner *model.Owner) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.owners[f.assign(&owner.ID)] = owner
	return nil
}

func (f *fakeClinic) DeleteOwner(ctx context.Context, owner *model.Owner) error {
	if f.failErr != nil {
		return f.failErr
	}
	if owner == nil || owner.ID == nil {
		return fmt.Errorf("delete owner: %w", store.ErrNotFound)
	}
	for _, p := range values(f.pets) {
		if oid := p.GetOwnerID(); oid != nil && *oid == *owner.ID {
			if err := f.DeletePet(ctx, p); err != nil {
				return err
			}
		}
	}
	delete(f.owners, *owner.ID)
	return nil
}

func (f *fakeClinic) FindOwnerByLastName(_ context.Context, lastName string) ([]*model.Owner, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	out := []*model.Owner{}
	for _, o := range values(f.owners) {
		if strings.HasPrefix(o.LastName, lastName) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeClinic) FindPetTypeByID(_ context.Context, id int) (*model.PetType, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	t, ok := f.types[id]
	if !ok {
		return nil, notFound("pet type", id)
	}
	return t, nil
}

func (f *fakeClinic) FindAllPetTypes(context.Context) ([]*model.PetType, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.types), nil
}

func (f *fakeClinic) FindPetTypes(ctx context.Context) ([]*model.PetType, error) {
	return f.FindAllPetTypes(ctx)
}

func (f *fakeClinic) SavePetType(_ context.Context, pt *model.PetType) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.types[f.assign(&pt.ID)] = pt
	return nil
}

func (f *fakeClinic) DeletePetType(ctx context.Context, pt *model.PetType) error {
	if f.failErr != nil {
		return f.failErr
	}
	if pt == nil || pt.ID == nil {
		return fmt.Errorf("delete pet type: %w", store.ErrNotFound)
	}
	for _, p := range values(f.pets) {
		if t := p.GetType(); t != nil && t.ID != nil && *t.ID == *pt.ID {
			if err := f.DeletePet(ctx, p); err != nil {
				return err
			}
		}
	}
	delete(f.types, *pt.ID)
	return nil
}

func (f *fakeClinic) FindSpecialtyByID(_ context.Context, id int) (*model.Specialty, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	s, ok := f.specs[id]
	if !ok {
		return nil, notFound("specialty", id)
	}
	return s, nil
}

func (f *fakeClinic) FindAllSpecialties(context.Context) ([]*model.Specialty, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return values(f.specs), nil
}

func (f *fakeClinic) SaveSpecialty(_ context.Context, s *model.Specialty) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.specs[f.assign(&s.ID)] = s
	return nil
}

func (f *fakeClinic) DeleteSpecialty(_ context.Context, s *model.Specialty) error {
	if f.failErr != nil {
		return f.failErr
	}
	if s == nil || s.ID == nil {
		return fmt.Errorf("delete specialty: %w", store.ErrNotFound)
	}
	for _, v := range f.vets {
		kept := []model.Specialty{}
		for _, vs := range v.GetSpecialties() {
			if vs.ID == nil || *vs.ID != *s.ID {
				kept = append(kept, vs)
			}
		}
		v.SetSpecialties(kept)
	}
	delete(f.specs, *s.ID)
	return nil
}

func (f *fakeClinic) FindSpecialtiesByNameIn(_ context.Context, names map[string]struct{}) ([]*model.Specialty, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	out := []*model.Specialty{}
	for _, s := range values(f.specs) {
		if _, ok := names[s.Name]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// ---- tests ----

func intp(i int) *int { return &i }

func TestFindByIDNotFoundWrapsErrNotFound(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(Clinic) (any, error)
	}{
		{"pet", func(c Clinic) (any, error) { return c.FindPetByID(ctx, 99) }},
		{"visit", func(c Clinic) (any, error) { return c.FindVisitByID(ctx, 99) }},
		{"vet", func(c Clinic) (any, error) { return c.FindVetByID(ctx, 99) }},
		{"owner", func(c Clinic) (any, error) { return c.FindOwnerByID(ctx, 99) }},
		{"petType", func(c Clinic) (any, error) { return c.FindPetTypeByID(ctx, 99) }},
		{"specialty", func(c Clinic) (any, error) { return c.FindSpecialtyByID(ctx, 99) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call(newFakeClinic())
			if !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestCollectionsEmptyNonNil(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(Clinic) (int, bool, error)
	}{
		{"FindAllPets", func(c Clinic) (int, bool, error) { r, e := c.FindAllPets(ctx); return len(r), r == nil, e }},
		{"FindVisitsByPetID", func(c Clinic) (int, bool, error) { r, e := c.FindVisitsByPetID(ctx, 1); return len(r), r == nil, e }},
		{"FindAllVisits", func(c Clinic) (int, bool, error) { r, e := c.FindAllVisits(ctx); return len(r), r == nil, e }},
		{"FindVets", func(c Clinic) (int, bool, error) { r, e := c.FindVets(ctx); return len(r), r == nil, e }},
		{"FindAllVets", func(c Clinic) (int, bool, error) { r, e := c.FindAllVets(ctx); return len(r), r == nil, e }},
		{"FindAllOwners", func(c Clinic) (int, bool, error) { r, e := c.FindAllOwners(ctx); return len(r), r == nil, e }},
		{"FindOwnerByLastName", func(c Clinic) (int, bool, error) { r, e := c.FindOwnerByLastName(ctx, "X"); return len(r), r == nil, e }},
		{"FindAllPetTypes", func(c Clinic) (int, bool, error) { r, e := c.FindAllPetTypes(ctx); return len(r), r == nil, e }},
		{"FindPetTypes", func(c Clinic) (int, bool, error) { r, e := c.FindPetTypes(ctx); return len(r), r == nil, e }},
		{"FindAllSpecialties", func(c Clinic) (int, bool, error) { r, e := c.FindAllSpecialties(ctx); return len(r), r == nil, e }},
		{"FindSpecialtiesByNameIn", func(c Clinic) (int, bool, error) {
			r, e := c.FindSpecialtiesByNameIn(ctx, map[string]struct{}{})
			return len(r), r == nil, e
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, isNil, err := tt.call(newFakeClinic())
			if err != nil || isNil || n != 0 {
				t.Fatalf("got n=%d nil=%v err=%v", n, isNil, err)
			}
		})
	}
}

func TestStorageFailurePropagates(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(Clinic) error{
		"FindPetByID":   func(c Clinic) error { _, e := c.FindPetByID(ctx, 1); return e },
		"FindAllPets":   func(c Clinic) error { _, e := c.FindAllPets(ctx); return e },
		"FindPets":      func(c Clinic) error { _, e := c.FindPets(ctx, store.PageRequest{}); return e },
		"SavePet":       func(c Clinic) error { return c.SavePet(ctx, &model.Pet{}) },
		"DeletePet":     func(c Clinic) error { return c.DeletePet(ctx, &model.Pet{}) },
		"FindVisits":    func(c Clinic) error { _, e := c.FindVisitsByPetID(ctx, 1); return e },
		"FindVisitByID": func(c Clinic) error { _, e := c.FindVisitByID(ctx, 1); return e },
		"FindAllVisits": func(c Clinic) error { _, e := c.FindAllVisits(ctx); return e },
		"SaveVisit":     func(c Clinic) error { return c.SaveVisit(ctx, &model.Visit{}) },
		"DeleteVisit":   func(c Clinic) error { return c.DeleteVisit(ctx, &model.Visit{}) },
		"FindVetByID":   func(c Clinic) error { _, e := c.FindVetByID(ctx, 1); return e },
		"FindVets":      func(c Clinic) error { _, e := c.FindVets(ctx); return e },
		"FindAllVets":   func(c Clinic) error { _, e := c.FindAllVets(ctx); return e },
		"SaveVet":       func(c Clinic) error { return c.SaveVet(ctx, model.NewVet()) },
		"DeleteVet":     func(c Clinic) error { return c.DeleteVet(ctx, model.NewVet()) },
		"FindOwnerByID": func(c Clinic) error { _, e := c.FindOwnerByID(ctx, 1); return e },
		"FindAllOwners": func(c Clinic) error { _, e := c.FindAllOwners(ctx); return e },
		"FindOwners":    func(c Clinic) error { _, e := c.FindOwners(ctx, "", store.PageRequest{}); return e },
		"SaveOwner":     func(c Clinic) error { return c.SaveOwner(ctx, model.NewOwner()) },
		"DeleteOwner":   func(c Clinic) error { return c.DeleteOwner(ctx, model.NewOwner()) },
		"FindOwnerByLN": func(c Clinic) error { _, e := c.FindOwnerByLastName(ctx, "a"); return e },
		"FindPetType":   func(c Clinic) error { _, e := c.FindPetTypeByID(ctx, 1); return e },
		"FindAllTypes":  func(c Clinic) error { _, e := c.FindAllPetTypes(ctx); return e },
		"FindPetTypes":  func(c Clinic) error { _, e := c.FindPetTypes(ctx); return e },
		"SavePetType":   func(c Clinic) error { return c.SavePetType(ctx, model.NewPetType()) },
		"DeletePetType": func(c Clinic) error { return c.DeletePetType(ctx, model.NewPetType()) },
		"FindSpecialty": func(c Clinic) error { _, e := c.FindSpecialtyByID(ctx, 1); return e },
		"FindAllSpecs":  func(c Clinic) error { _, e := c.FindAllSpecialties(ctx); return e },
		"SaveSpecialty": func(c Clinic) error { return c.SaveSpecialty(ctx, model.NewSpecialty()) },
		"DeleteSpec":    func(c Clinic) error { return c.DeleteSpecialty(ctx, model.NewSpecialty()) },
		"SpecsByName":   func(c Clinic) error { _, e := c.FindSpecialtiesByNameIn(ctx, map[string]struct{}{"x": {}}); return e },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			f := newFakeClinic()
			f.failErr = errStorage
			if err := call(f); !errors.Is(err, errStorage) {
				t.Fatalf("want storage error, got %v", err)
			}
		})
	}
}

func TestSaveAssignsIDAndUpdates(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func(t *testing.T, c Clinic)
	}{
		{"pet", func(t *testing.T, c Clinic) {
			p := &model.Pet{}
			p.Name = "Leo"
			if err := c.SavePet(ctx, p); err != nil || p.ID == nil {
				t.Fatalf("save: %v id=%v", err, p.ID)
			}
			id := *p.ID
			p.Name = "Max"
			if err := c.SavePet(ctx, p); err != nil || *p.ID != id {
				t.Fatalf("update changed id or failed: %v", err)
			}
			got, err := c.FindPetByID(ctx, id)
			if err != nil || got.Name != "Max" || *got.ID != id {
				t.Fatalf("got %+v %v", got, err)
			}
		}},
		{"owner", func(t *testing.T, c Clinic) {
			o := model.NewOwner()
			o.LastName = "Davis"
			if err := c.SaveOwner(ctx, o); err != nil || o.ID == nil {
				t.Fatal("no id")
			}
			o.City = "Madison"
			_ = c.SaveOwner(ctx, o)
			got, err := c.FindOwnerByID(ctx, *o.ID)
			if err != nil || got.City != "Madison" {
				t.Fatalf("got %v %v", got, err)
			}
		}},
		{"vet", func(t *testing.T, c Clinic) {
			s := model.NewSpecialty()
			s.Name = "surgery"
			_ = c.SaveSpecialty(ctx, s)
			v := model.NewVet()
			v.AddSpecialty(*s)
			if err := c.SaveVet(ctx, v); err != nil || v.ID == nil {
				t.Fatal("no id")
			}
			got, err := c.FindVetByID(ctx, *v.ID)
			if err != nil || got.NrOfSpecialties() != 1 {
				t.Fatalf("got %v %v", got, err)
			}
		}},
		{"petType", func(t *testing.T, c Clinic) {
			pt := model.NewPetType()
			pt.Name = "cat"
			if err := c.SavePetType(ctx, pt); err != nil || pt.ID == nil {
				t.Fatal("no id")
			}
			if got, err := c.FindPetTypeByID(ctx, *pt.ID); err != nil || got.Name != "cat" {
				t.Fatalf("got %v %v", got, err)
			}
		}},
		{"specialty", func(t *testing.T, c Clinic) {
			s := model.NewSpecialty()
			s.Name = "dentistry"
			if err := c.SaveSpecialty(ctx, s); err != nil || s.ID == nil {
				t.Fatal("no id")
			}
			if got, err := c.FindSpecialtyByID(ctx, *s.ID); err != nil || got.Name != "dentistry" {
				t.Fatalf("got %v %v", got, err)
			}
		}},
		{"visit", func(t *testing.T, c Clinic) {
			p := &model.Pet{}
			_ = c.SavePet(ctx, p)
			v := &model.Visit{Description: "shot"}
			v.SetPet(p)
			if err := c.SaveVisit(ctx, v); err != nil || v.ID == nil {
				t.Fatalf("save visit: %v", err)
			}
			got, err := c.FindVisitByID(ctx, *v.ID)
			if err != nil || got.Description != "shot" {
				t.Fatalf("got %v %v", got, err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.run(t, newFakeClinic()) })
	}
}

func TestSaveVisitInvalidPetIsIntegrityError(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		visit *model.Visit
	}{
		{"no pet", &model.Visit{Description: "x"}},
		{"unknown pet", &model.Visit{Description: "x", PetID: intp(42)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := newFakeClinic().SaveVisit(ctx, tt.visit); !errors.Is(err, store.ErrIntegrity) {
				t.Fatalf("want ErrIntegrity, got %v", err)
			}
		})
	}
}

func TestDeleteCascades(t *testing.T) {
	ctx := context.Background()
	setup := func(c Clinic) (*model.Owner, *model.PetType, *model.Pet, *model.Visit) {
		o := model.NewOwner()
		_ = c.SaveOwner(ctx, o)
		pt := model.NewPetType()
		_ = c.SavePetType(ctx, pt)
		p := &model.Pet{}
		p.SetType(pt)
		o.AddPet(p)
		_ = c.SavePet(ctx, p)
		v := &model.Visit{Description: "d"}
		p.AddVisit(v)
		_ = c.SaveVisit(ctx, v)
		return o, pt, p, v
	}
	tests := []struct {
		name      string
		del       func(Clinic, *model.Owner, *model.PetType, *model.Pet, *model.Visit) error
		ownerGone bool
		typeGone  bool
		petGone   bool
	}{
		{"DeletePet", func(c Clinic, _ *model.Owner, _ *model.PetType, p *model.Pet, _ *model.Visit) error { return c.DeletePet(ctx, p) }, false, false, true},
		{"DeleteOwner", func(c Clinic, o *model.Owner, _ *model.PetType, _ *model.Pet, _ *model.Visit) error { return c.DeleteOwner(ctx, o) }, true, false, true},
		{"DeletePetType", func(c Clinic, _ *model.Owner, pt *model.PetType, _ *model.Pet, _ *model.Visit) error { return c.DeletePetType(ctx, pt) }, false, true, true},
		{"DeleteVisit", func(c Clinic, _ *model.Owner, _ *model.PetType, _ *model.Pet, v *model.Visit) error { return c.DeleteVisit(ctx, v) }, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFakeClinic()
			o, pt, p, v := setup(c)
			if err := tt.del(c, o, pt, p, v); err != nil {
				t.Fatal(err)
			}
			if _, err := c.FindVisitByID(ctx, *v.ID); !errors.Is(err, store.ErrNotFound) {
				t.Errorf("visit should be gone: %v", err)
			}
			_, err := c.FindPetByID(ctx, *p.ID)
			if errors.Is(err, store.ErrNotFound) != tt.petGone {
				t.Errorf("pet gone mismatch: %v", err)
			}
			_, err = c.FindOwnerByID(ctx, *o.ID)
			if errors.Is(err, store.ErrNotFound) != tt.ownerGone {
				t.Errorf("owner gone mismatch: %v", err)
			}
			_, err = c.FindPetTypeByID(ctx, *pt.ID)
			if errors.Is(err, store.ErrNotFound) != tt.typeGone {
				t.Errorf("type gone mismatch: %v", err)
			}
		})
	}
}

func TestDeleteVetAndSpecialty(t *testing.T) {
	ctx := context.Background()
	c := newFakeClinic()
	s := model.NewSpecialty()
	s.Name = "radiology"
	_ = c.SaveSpecialty(ctx, s)
	v := model.NewVet()
	v.AddSpecialty(*s)
	_ = c.SaveVet(ctx, v)

	if err := c.DeleteSpecialty(ctx, s); err != nil {
		t.Fatal(err)
	}
	if _, err := c.FindSpecialtyByID(ctx, *s.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("specialty not deleted: %v", err)
	}
	got, _ := c.FindVetByID(ctx, *v.ID)
	if got.NrOfSpecialties() != 0 {
		t.Fatal("vet link not removed")
	}
	if err := c.DeleteVet(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := c.FindVetByID(ctx, *v.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("vet not deleted: %v", err)
	}
}

func TestFindVisitsByPetIDOnlyThatPet(t *testing.T) {
	ctx := context.Background()
	c := newFakeClinic()
	p1, p2 := &model.Pet{}, &model.Pet{}
	_ = c.SavePet(ctx, p1)
	_ = c.SavePet(ctx, p2)
	for _, p := range []*model.Pet{p1, p1, p2} {
		v := &model.Visit{Description: "d"}
		v.SetPet(p)
		_ = c.SaveVisit(ctx, v)
	}
	tests := []struct {
		petID int
		want  int
	}{{*p1.ID, 2}, {*p2.ID, 1}, {999, 0}}
	for _, tt := range tests {
		got, err := c.FindVisitsByPetID(ctx, tt.petID)
		if err != nil || len(got) != tt.want {
			t.Fatalf("pet %d: got %d err %v", tt.petID, len(got), err)
		}
		for _, v := range got {
			if *v.GetPetID() != tt.petID {
				t.Fatalf("visit for wrong pet")
			}
		}
	}
}

func TestFindOwnerByLastNamePrefix(t *testing.T) {
	ctx := context.Background()
	c := newFakeClinic()
	for _, ln := range []string{"Davis", "Davidson", "Franklin"} {
		o := model.NewOwner()
		o.LastName = ln
		_ = c.SaveOwner(ctx, o)
	}
	tests := []struct {
		prefix string
		want   int
	}{{"Dav", 2}, {"Franklin", 1}, {"Zed", 0}, {"", 3}}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got, err := c.FindOwnerByLastName(ctx, tt.prefix)
			if err != nil || len(got) != tt.want {
				t.Fatalf("got %d err %v", len(got), err)
			}
			for _, o := range got {
				if !strings.HasPrefix(o.LastName, tt.prefix) {
					t.Fatalf("%q lacks prefix", o.LastName)
				}
			}
		})
	}
}

func TestListingVariantsMatch(t *testing.T) {
	ctx := context.Background()
	c := newFakeClinic()
	_ = c.SaveVet(ctx, model.NewVet())
	_ = c.SavePetType(ctx, model.NewPetType())
	a, _ := c.FindVets(ctx)
	b, _ := c.FindAllVets(ctx)
	if len(a) != 1 || len(a) != len(b) {
		t.Fatalf("vets mismatch %d %d", len(a), len(b))
	}
	x, _ := c.FindPetTypes(ctx)
	y, _ := c.FindAllPetTypes(ctx)
	if len(x) != 1 || len(x) != len(y) {
		t.Fatalf("types mismatch")
	}
	if _, err := c.FindPets(ctx, store.PageRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.FindOwners(ctx, "", store.PageRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestFindSpecialtiesByNameIn(t *testing.T) {
	ctx := context.Background()
	c := newFakeClinic()
	for _, n := range []string{"radiology", "surgery", "dentistry"} {
		s := model.NewSpecialty()
		s.Name = n
		_ = c.SaveSpecialty(ctx, s)
	}
	tests := []struct {
		name  string
		names map[string]struct{}
		want  int
	}{
		{"some match", map[string]struct{}{"surgery": {}, "unknown": {}}, 1},
		{"all match", map[string]struct{}{"surgery": {}, "radiology": {}, "dentistry": {}}, 3},
		{"empty", map[string]struct{}{}, 0},
		{"nil", nil, 0},
		{"none", map[string]struct{}{"x": {}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.FindSpecialtiesByNameIn(ctx, tt.names)
			if err != nil || got == nil || len(got) != tt.want {
				t.Fatalf("got %v err %v", got, err)
			}
			seen := map[*model.Specialty]bool{}
			for _, s := range got {
				if _, ok := tt.names[s.Name]; !ok || seen[s] {
					t.Fatalf("unexpected or duplicate %q", s.Name)
				}
				seen[s] = true
			}
		})
	}
}