package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"migrated-app/internal/model"
)

// fakeOwnerRepo is an in-memory OwnerRepository used to validate the contract.
type fakeOwnerRepo struct {
	owners map[int]*model.Owner
	nextID int
	fail   error
}

var _ OwnerRepository = (*fakeOwnerRepo)(nil)

func newFake() *fakeOwnerRepo {
	return &fakeOwnerRepo{owners: map[int]*model.Owner{}, nextID: 1}
}

func (f *fakeOwnerRepo) sorted() []*model.Owner {
	ids := make([]int, 0, len(f.owners))
	for id := range f.owners {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := []*model.Owner{}
	for _, id := range ids {
		out = append(out, f.owners[id])
	}
	return out
}

func (f *fakeOwnerRepo) FindByLastName(ctx context.Context, ln string) ([]*model.Owner, error) {
	if f.fail != nil {
		return nil, fmt.Errorf("find: %w", f.fail)
	}
	out := []*model.Owner{}
	for _, o := range f.sorted() {
		if strings.HasPrefix(o.LastName, ln) {
			out = append(out, o)
		}
	}
	return out, nil
}

func paginate(all []*model.Owner, req PageRequest) Page[*model.Owner] {
	off := req.Offset()
	var content []*model.Owner
	if off < int64(len(all)) {
		end := off + int64(req.Size)
		if end > int64(len(all)) {
			end = int64(len(all))
		}
		content = all[off:end]
	}
	return NewPage(content, req, int64(len(all)))
}

func (f *fakeOwnerRepo) FindByLastNamePage(ctx context.Context, ln string, req PageRequest) (Page[*model.Owner], error) {
	all, err := f.FindByLastName(ctx, ln)
	if err != nil {
		return Page[*model.Owner]{}, err
	}
	return paginate(all, req), nil
}

func (f *fakeOwnerRepo) FindByID(ctx context.Context, id int) (*model.Owner, error) {
	if f.fail != nil {
		return nil, fmt.Errorf("find: %w", f.fail)
	}
	o, ok := f.owners[id]
	if !ok {
		return nil, fmt.Errorf("owner %d: %w", id, ErrNotFound)
	}
	return o, nil
}

func (f *fakeOwnerRepo) Save(ctx context.Context, o *model.Owner) error {
	if f.fail != nil {
		return fmt.Errorf("save: %w", f.fail)
	}
	if o.LastName == "" {
		return fmt.Errorf("save: %w", ErrIntegrity)
	}
	if o.IsNew() {
		o.SetID(model.IntPtr(f.nextID))
		f.nextID++
	}
	f.owners[*o.ID] = o
	return nil
}

func (f *fakeOwnerRepo) FindAll(ctx context.Context) ([]*model.Owner, error) {
	if f.fail != nil {
		return nil, fmt.Errorf("find: %w", f.fail)
	}
	return f.sorted(), nil
}

func (f *fakeOwnerRepo) FindAllPage(ctx context.Context, req PageRequest) (Page[*model.Owner], error) {
	all, err := f.FindAll(ctx)
	if err != nil {
		return Page[*model.Owner]{}, err
	}
	return paginate(all, req), nil
}

func (f *fakeOwnerRepo) Delete(ctx context.Context, o *model.Owner) error {
	if f.fail != nil {
		return fmt.Errorf("delete: %w", f.fail)
	}
	if o.IsNew() {
		return ErrNotFound
	}
	if _, ok := f.owners[*o.ID]; !ok {
		return fmt.Errorf("delete: %w", ErrNotFound)
	}
	delete(f.owners, *o.ID)
	return nil
}

func owner(first, last string) *model.Owner {
	o := model.NewOwner()
	o.FirstName = first
	o.LastName = last
	o.Address = "1 St"
	o.City = "Town"
	o.Telephone = "1234567890"
	return o
}

func seeded(t *testing.T) *fakeOwnerRepo {
	t.Helper()
	r := newFake()
	for _, n := range [][2]string{{"George", "Franklin"}, {"Betty", "Davis"}, {"Harold", "Davis"}, {"Jean", "Coleman"}, {"Jeff", "Davison"}, {"Maria", "Escobito"}} {
		if err := r.Save(context.Background(), owner(n[0], n[1])); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestNewPageRequest(t *testing.T) {
	tests := []struct {
		name       string
		page, size int
		wantErr    bool
		offset     int64
	}{
		{"first", 0, 10, false, 0},
		{"third", 2, 5, false, 10},
		{"negative page", -1, 5, true, 0},
		{"zero size", 0, 0, true, 0},
		{"negative size", 0, -3, true, 0},
		{"large", 1 << 30, 1 << 30, false, int64(1<<30) * int64(1<<30)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, err := NewPageRequest(tc.page, tc.size)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				if pr != (PageRequest{}) {
					t.Errorf("expected zero PageRequest, got %+v", pr)
				}
				return
			}
			if pr.Page != tc.page || pr.Size != tc.size {
				t.Errorf("got %+v", pr)
			}
			if pr.Offset() != tc.offset {
				t.Errorf("offset=%d want %d", pr.Offset(), tc.offset)
			}
		})
	}
}

func TestNewPageAndTotalPages(t *testing.T) {
	tests := []struct {
		name      string
		content   []int
		size      int
		total     int64
		wantPages int
	}{
		{"nil content", nil, 10, 0, 0},
		{"exact", []int{1, 2}, 2, 4, 2},
		{"remainder", []int{1}, 3, 7, 3},
		{"single", []int{1}, 10, 1, 1},
		{"unpaged", []int{1}, 0, 5, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPage(tc.content, PageRequest{Page: 1, Size: tc.size}, tc.total)
			if p.Content == nil {
				t.Fatal("content must never be nil")
			}
			if p.Page != 1 || p.Size != tc.size || p.TotalElements != tc.total {
				t.Errorf("got %+v", p)
			}
			if got := p.TotalPages(); got != tc.wantPages {
				t.Errorf("TotalPages=%d want %d", got, tc.wantPages)
			}
		})
	}
}

func TestErrorSentinels(t *testing.T) {
	wrapped := fmt.Errorf("ctx: %w", ErrIntegrity)
	if !errors.Is(wrapped, ErrIntegrity) {
		t.Error("wrapped ErrIntegrity not matched")
	}
	var marker interface{ IntegrityViolation() bool }
	if !errors.As(wrapped, &marker) || !marker.IntegrityViolation() {
		t.Error("IntegrityViolation marker missing")
	}
	if ErrIntegrity.Error() != "store: data integrity violation" {
		t.Errorf("msg=%q", ErrIntegrity.Error())
	}
	if !errors.Is(fmt.Errorf("x: %w", ErrNotFound), ErrNotFound) {
		t.Error("wrapped ErrNotFound not matched")
	}
	if errors.Is(ErrNotFound, ErrIntegrity) || errors.As(ErrNotFound, &marker) {
		t.Error("ErrNotFound must not be an integrity violation")
	}
}

func TestFindByLastName(t *testing.T) {
	tests := []struct {
		prefix string
		want   []string
	}{
		{"Davis", []string{"Davis", "Davis", "Davison"}},
		{"Dav", []string{"Davis", "Davis", "Davison"}},
		{"Franklin", []string{"Franklin"}},
		{"Zzz", []string{}},
		{"avis", []string{}},
		{"", []string{"Franklin", "Davis", "Davis", "Coleman", "Davison", "Escobito"}},
	}
	for _, tc := range tests {
		t.Run("prefix="+tc.prefix, func(t *testing.T) {
			r := seeded(t)
			before := len(r.owners)
			got, err := r.FindByLastName(context.Background(), tc.prefix)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("nil slice")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d want %d", len(got), len(tc.want))
			}
			for i, o := range got {
				if !strings.HasPrefix(o.LastName, tc.prefix) || o.LastName != tc.want[i] {
					t.Errorf("unexpected %q", o.LastName)
				}
			}
			if len(r.owners) != before {
				t.Error("read must not modify")
			}
		})
	}
}

func TestStoreFailures(t *testing.T) {
	boom := errors.New("db down")
	r := seeded(t)
	r.fail = boom
	ctx := context.Background()
	req, _ := NewPageRequest(0, 2)
	calls := map[string]func() error{
		"FindByLastName":     func() error { _, e := r.FindByLastName(ctx, "D"); return e },
		"FindByLastNamePage": func() error { _, e := r.FindByLastNamePage(ctx, "D", req); return e },
		"FindByID":           func() error { _, e := r.FindByID(ctx, 1); return e },
		"Save":               func() error { return r.Save(ctx, owner("a", "b")) },
		"FindAll":            func() error { _, e := r.FindAll(ctx); return e },
		"FindAllPage":        func() error { _, e := r.FindAllPage(ctx, req); return e },
		"Delete":             func() error { return r.Delete(ctx, &model.Owner{Person: model.Person{BaseEntity: model.BaseEntity{ID: model.IntPtr(1)}}}) },
	}
	for name, fn := range calls {
		t.Run(name, func(t *testing.T) {
			if err := fn(); !errors.Is(err, boom) {
				t.Errorf("want wrapped failure, got %v", err)
			}
		})
	}
}

func TestFindByLastNamePage(t *testing.T) {
	tests := []struct {
		name               string
		prefix             string
		page, size         int
		wantLen            int
		wantTotal          int64
		wantPages          int
	}{
		{"first page", "Dav", 0, 2, 2, 3, 2},
		{"last partial", "Dav", 1, 2, 1, 3, 2},
		{"beyond", "Dav", 5, 2, 0, 3, 2},
		{"no match", "Zzz", 0, 2, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := seeded(t)
			req, err := NewPageRequest(tc.page, tc.size)
			if err != nil {
				t.Fatal(err)
			}
			p, err := r.FindByLastNamePage(context.Background(), tc.prefix, req)
			if err != nil {
				t.Fatal(err)
			}
			if p.Content == nil || len(p.Content) != tc.wantLen || p.TotalElements != tc.wantTotal || p.TotalPages() != tc.wantPages {
				t.Fatalf("got %+v", p)
			}
			all, _ := r.FindByLastName(context.Background(), tc.prefix)
			if int64(len(all)) != p.TotalElements {
				t.Error("total mismatch with unpaged")
			}
			for _, o := range p.Content {
				found := false
				for _, a := range all {
					if a == o {
						found = true
					}
				}
				if !found {
					t.Error("page content not subset")
				}
			}
		})
	}
}

func TestFindByID(t *testing.T) {
	r := seeded(t)
	tests := []struct {
		id      int
		wantErr bool
	}{{1, false}, {6, false}, {99, true}, {0, true}}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			o, err := r.FindByID(context.Background(), tc.id)
			if tc.wantErr {
				if !errors.Is(err, ErrNotFound) || o != nil {
					t.Fatalf("want ErrNotFound, got %v %v", o, err)
				}
				return
			}
			if err != nil || o == nil || *o.GetID() != tc.id {
				t.Fatalf("got %v %v", o, err)
			}
		})
	}
}

func TestSave(t *testing.T) {
	ctx := context.Background()
	t.Run("insert", func(t *testing.T) {
		r := seeded(t)
		before, _ := r.FindAll(ctx)
		o := owner("Sam", "Schultz")
		if !o.IsNew() {
			t.Fatal("should be new")
		}
		if err := r.Save(ctx, o); err != nil {
			t.Fatal(err)
		}
		if o.GetID() == nil {
			t.Fatal("id not assigned")
		}
		after, _ := r.FindAll(ctx)
		if len(after) != len(before)+1 {
			t.Error("count not increased")
		}
		got, err := r.FindByID(ctx, *o.ID)
		if err != nil || got.LastName != "Schultz" {
			t.Errorf("not retrievable: %v", err)
		}
	})
	t.Run("update", func(t *testing.T) {
		r := seeded(t)
		before, _ := r.FindAll(ctx)
		o, _ := r.FindByID(ctx, 1)
		upd := owner(o.FirstName, "Changed")
		upd.SetID(model.IntPtr(1))
		if err := r.Save(ctx, upd); err != nil {
			t.Fatal(err)
		}
		after, _ := r.FindAll(ctx)
		if len(after) != len(before) {
			t.Error("duplicate created")
		}
		got, _ := r.FindByID(ctx, 1)
		if got.LastName != "Changed" || *upd.ID != 1 {
			t.Error("update not reflected")
		}
	})
	t.Run("constraint violation", func(t *testing.T) {
		r := newFake()
		err := r.Save(ctx, owner("x", ""))
		if !errors.Is(err, ErrIntegrity) {
			t.Errorf("want ErrIntegrity, got %v", err)
		}
	})
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		name string
		repo func(*testing.T) *fakeOwnerRepo
		want int
	}{
		{"populated", seeded, 6},
		{"empty", func(*testing.T) *fakeOwnerRepo { return newFake() }, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.repo(t).FindAll(context.Background())
			if err != nil || got == nil || len(got) != tc.want {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
}

func TestFindAllPage(t *testing.T) {
	tests := []struct {
		name       string
		page, size int
		wantLen    int
	}{
		{"first", 0, 4, 4},
		{"second", 1, 4, 2},
		{"out of range", 9, 4, 0},
		{"whole", 0, 100, 6},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := seeded(t)
			req, _ := NewPageRequest(tc.page, tc.size)
			p, err := r.FindAllPage(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			all, _ := r.FindAll(context.Background())
			if p.Content == nil || len(p.Content) != tc.wantLen || len(p.Content) > tc.size || p.TotalElements != int64(len(all)) {
				t.Fatalf("got %+v", p)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	r := seeded(t)
	target, _ := r.FindByID(ctx, 2)
	pet := &model.Pet{}
	target.AddPet(pet)
	before, _ := r.FindAll(ctx)
	if err := r.Delete(ctx, target); err != nil {
		t.Fatal(err)
	}
	after, _ := r.FindAll(ctx)
	if len(after) != len(before)-1 {
		t.Error("count not decreased")
	}
	if _, err := r.FindByID(ctx, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("want not found, got %v", err)
	}
	if o, err := r.FindByID(ctx, 3); err != nil || o.LastName != "Davis" {
		t.Error("other owners affected")
	}
	if err := r.Delete(ctx, target); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}