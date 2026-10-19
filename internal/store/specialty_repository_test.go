package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"migrated-app/internal/model"
)

// stubSpecialtyRepo is a minimal in-memory implementation used to verify
// that the SpecialtyRepository contract can be satisfied and that its
// documented error semantics are expressible with the store sentinels.
type stubSpecialtyRepo struct {
	byID map[int]*model.Specialty
}

var _ SpecialtyRepository = (*stubSpecialtyRepo)(nil)

func newStubSpecialtyRepo() *stubSpecialtyRepo {
	return &stubSpecialtyRepo{byID: map[int]*model.Specialty{}}
}

func (r *stubSpecialtyRepo) FindByID(_ context.Context, id int) (*model.Specialty, error) {
	s, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("specialty %d: %w", id, ErrNotFound)
	}
	return s, nil
}

func (r *stubSpecialtyRepo) FindByNameIn(_ context.Context, _ []string) ([]*model.Specialty, error) {
	return []*model.Specialty{}, nil
}

func (r *stubSpecialtyRepo) FindAll(_ context.Context) ([]*model.Specialty, error) {
	out := make([]*model.Specialty, 0, len(r.byID))
	for _, s := range r.byID {
		out = append(out, s)
	}
	return out, nil
}

func (r *stubSpecialtyRepo) Save(_ context.Context, s *model.Specialty) error {
	if s == nil {
		return errors.New("nil specialty")
	}
	r.byID[len(r.byID)+1] = s
	return nil
}

func (r *stubSpecialtyRepo) Delete(_ context.Context, s *model.Specialty) error {
	if s == nil {
		return errors.New("nil specialty")
	}
	for id, v := range r.byID {
		if v == s {
			delete(r.byID, id)
		}
	}
	return nil
}

func TestSpecialtyRepositoryFindByIDNotFound(t *testing.T) {
	var repo SpecialtyRepository = newStubSpecialtyRepo()
	_, err := repo.FindByID(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if errors.Is(err, ErrIntegrity) {
		t.Fatal("not-found error must not match ErrIntegrity")
	}
}

func TestSpecialtyRepositoryEmptyResultsNotNil(t *testing.T) {
	var repo SpecialtyRepository = newStubSpecialtyRepo()
	ctx := context.Background()
	all, err := repo.FindAll(ctx)
	if err != nil || all == nil || len(all) != 0 {
		t.Fatalf("FindAll = %v, %v; want empty non-nil", all, err)
	}
	byName, err := repo.FindByNameIn(ctx, nil)
	if err != nil || byName == nil || len(byName) != 0 {
		t.Fatalf("FindByNameIn = %v, %v; want empty non-nil", byName, err)
	}
}

func TestSpecialtyRepositoryNilArguments(t *testing.T) {
	var repo SpecialtyRepository = newStubSpecialtyRepo()
	ctx := context.Background()
	if err := repo.Save(ctx, nil); err == nil {
		t.Error("Save(nil) should fail")
	}
	if err := repo.Delete(ctx, nil); err == nil {
		t.Error("Delete(nil) should fail")
	}
}

func TestSpecialtyRepositorySaveThenDelete(t *testing.T) {
	repo := newStubSpecialtyRepo()
	ctx := context.Background()
	s := new(model.Specialty)
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	all, _ := repo.FindAll(ctx)
	if len(all) != 1 {
		t.Fatalf("len = %d, want 1", len(all))
	}
	if err := repo.Delete(ctx, s); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	all, _ = repo.FindAll(ctx)
	if len(all) != 0 {
		t.Fatalf("len = %d, want 0", len(all))
	}
}

func TestIntegrityViolationWrapsSentinel(t *testing.T) {
	err := IntegrityViolation("specialty referenced", errors.New("fk"))
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("err = %v, want ErrIntegrity", err)
	}
	if !errors.Is(IntegrityViolation("x", nil), ErrIntegrity) {
		t.Fatal("nil cause should still wrap ErrIntegrity")
	}
}