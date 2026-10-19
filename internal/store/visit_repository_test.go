package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"migrated-app/internal/model"
)

// fakeVisitRepository is an in-memory implementation of VisitRepository used
// to validate the behavioral contract described by the interface.
type fakeVisitRepository struct {
	rows   map[int]model.Visit
	pets   map[int]bool
	nextID int
	failOn error
}

var _ VisitRepository = (*fakeVisitRepository)(nil)

func newFakeVisitRepository(petIDs ...int) *fakeVisitRepository {
	r := &fakeVisitRepository{rows: map[int]model.Visit{}, pets: map[int]bool{}, nextID: 1}
	for _, id := range petIDs {
		r.pets[id] = true
	}
	return r
}

func copyVisit(v model.Visit) *model.Visit {
	c := v
	if v.ID != nil {
		c.ID = model.IntPtr(*v.ID)
	}
	if v.PetID != nil {
		c.PetID = model.IntPtr(*v.PetID)
	}
	if v.Date != nil {
		d := *v.Date
		c.Date = &d
	}
	c.Pet = nil
	return &c
}

func (r *fakeVisitRepository) Save(ctx context.Context, visit *model.Visit) error {
	if r.failOn != nil {
		return r.failOn
	}
	if visit == nil {
		return errors.New("nil visit")
	}
	pid := visit.GetPetID()
	if pid == nil {
		return errors.New("visit has no persisted pet")
	}
	if !r.pets[*pid] {
		return IntegrityViolation("pet_id references no pet", nil)
	}
	stored := *visit
	stored.PetID = model.IntPtr(*pid)
	if visit.IsNew() {
		id := r.nextID
		r.nextID++
		visit.SetID(model.IntPtr(id))
		stored.ID = model.IntPtr(id)
		r.rows[id] = *copyVisit(stored)
		return nil
	}
	if _, ok := r.rows[*visit.ID]; !ok {
		return fmt.Errorf("visit %d: %w", *visit.ID, ErrNotFound)
	}
	r.rows[*visit.ID] = *copyVisit(stored)
	return nil
}

func (r *fakeVisitRepository) sorted(filter func(model.Visit) bool) []*model.Visit {
	out := []*model.Visit{}
	for _, v := range r.rows {
		if filter(v) {
			out = append(out, copyVisit(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return *out[i].ID < *out[j].ID })
	return out
}

func (r *fakeVisitRepository) FindByPetID(ctx context.Context, petID int) ([]*model.Visit, error) {
	if r.failOn != nil {
		return nil, r.failOn
	}
	return r.sorted(func(v model.Visit) bool { return v.PetID != nil && *v.PetID == petID }), nil
}

func (r *fakeVisitRepository) FindByID(ctx context.Context, id int) (*model.Visit, error) {
	if r.failOn != nil {
		return nil, r.failOn
	}
	v, ok := r.rows[id]
	if !ok {
		return nil, fmt.Errorf("visit %d: %w", id, ErrNotFound)
	}
	return copyVisit(v), nil
}

func (r *fakeVisitRepository) FindAll(ctx context.Context) ([]*model.Visit, error) {
	if r.failOn != nil {
		return nil, r.failOn
	}
	return r.sorted(func(model.Visit) bool { return true }), nil
}

func (r *fakeVisitRepository) Delete(ctx context.Context, visit *model.Visit) error {
	if r.failOn != nil {
		return r.failOn
	}
	if visit == nil || visit.ID == nil {
		return errors.New("nil visit or id")
	}
	delete(r.rows, *visit.ID)
	return nil
}

type petStub struct{ id *int }

func (p petStub) GetID() *int { return p.id }

func mkVisit(petID int, desc string, d model.Date) *model.Visit {
	v := &model.Visit{Description: desc}
	v.SetDate(&d)
	v.SetPet(petStub{id: model.IntPtr(petID)})
	return v
}

func TestVisitRepository_SaveInsert(t *testing.T) {
	ctx := context.Background()
	d := model.NewDate(2013, time.January, 1)
	tests := []struct {
		name      string
		visit     *model.Visit
		wantErr   bool
		integrity bool
	}{
		{"valid new visit", mkVisit(7, "rabies shot", d), false, false},
		{"pet via PetID only", &model.Visit{Description: "x", Date: &d, PetID: model.IntPtr(7)}, false, false},
		{"nonexistent pet", mkVisit(999, "x", d), true, true},
		{"no pet", &model.Visit{Description: "x"}, true, false},
		{"nil visit", nil, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newFakeVisitRepository(7)
			err := r.Save(ctx, tt.visit)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if errors.Is(err, ErrIntegrity) != tt.integrity {
					t.Fatalf("integrity mismatch: %v", err)
				}
				all, _ := r.FindAll(ctx)
				if len(all) != 0 {
					t.Fatalf("no row should be inserted, got %d", len(all))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.visit.IsNew() || tt.visit.GetID() == nil {
				t.Fatal("id not assigned")
			}
			id := *tt.visit.GetID()
			got, err := r.FindByID(ctx, id)
			if err != nil || got == nil || *got.ID != id || got.Description != tt.visit.Description {
				t.Fatalf("FindByID = %+v, %v", got, err)
			}
			if got.Date == nil || !got.Date.Equal(d) {
				t.Fatalf("date = %v", got.Date)
			}
			byPet, _ := r.FindByPetID(ctx, 7)
			if len(byPet) != 1 || *byPet[0].ID != id {
				t.Fatalf("FindByPetID = %v", byPet)
			}
			all, _ := r.FindAll(ctx)
			if len(all) != 1 {
				t.Fatalf("FindAll len = %d", len(all))
			}
		})
	}
}