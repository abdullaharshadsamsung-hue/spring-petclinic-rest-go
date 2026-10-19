package model

import "sort"

// PetsTable is the name of the database table that stores Pet rows, as
// mapped by the source's @Table(name = "pets").
const PetsTable = "pets"

// OwnerRef is the view of an owner that a Pet needs: its identifier. The
// concrete *Owner type satisfies it through its embedded Person/BaseEntity.
//
// MIGRATION_NOTE: the source's @ManyToOne Owner association is held through
// this interface (mirroring PetRef in visit.go) so this file does not depend
// on the layout of the separately migrated Owner type. Callers that need the
// full owner type-assert GetOwner() to *Owner. Never store a typed nil
// pointer in it; SetOwner(nil) clears the association.
type OwnerRef interface {
	GetID() *int
}

// Pet is a pet, stored in the "pets" table
// (id, name, birth_date DATE, type_id -> types.id, owner_id -> owners.id).
//
// MIGRATION_NOTE: the JPA @Entity/@Table mapping becomes a plain struct; the
// pets table is created at startup by the embedded schema script
// (src/main/resources/db/postgres/schema.sql). The id and name columns come
// from the embedded NamedEntity (@MappedSuperclass in the source).
//
// MIGRATION_NOTE: JPA cascades are not automatic in Go. CascadeType.ALL on
// Type and on Visits must be honoured explicitly by the repository (save the
// pet type / visits together with the pet, and delete visits when a pet is
// deleted). The owner association has no cascade. FetchType.EAGER on visits
// (and the default eager @ManyToOne for type and owner) means repositories
// should populate Type, Owner/OwnerID and Visits when loading a pet.
type Pet struct {
	NamedEntity
	// BirthDate is the birth_date column (SQL DATE). nil means no date.
	BirthDate *Date `json:"birthDate"`
	// Type is the pet type, joined via the type_id column.
	Type *PetType `json:"type"`
	// OwnerID is the owner_id foreign key column.
	OwnerID *int `json:"ownerId"`
	// Owner is the owning owner, when loaded. It is not serialized, avoiding
	// the Owner -> Pets -> Owner cycle.
	Owner OwnerRef `json:"-"`
	// visits is the internal visit collection (Visit owns the pet_id FK).
	visits []*Visit
}

// petJSON is the wire shape of a Pet, including its visits.
type petJSON struct {
	ID        *int     `json:"id"`
	Name      string   `json:"name"`
	BirthDate *Date    `json:"birthDate"`
	Type      *PetType `json:"type"`
	OwnerID   *int     `json:"ownerId"`
	Visits    []*Visit `json:"visits"`
}

// MarshalJSON encodes the pet with its visits (newest first) and owner id,
// as the source's eager-loaded PetDto did.
func (p Pet) MarshalJSON() ([]byte, error) {
	return json.Marshal(petJSON{
		ID:        p.ID,
		Name:      p.Name,
		BirthDate: p.BirthDate,
		Type:      p.Type,
		OwnerID:   p.GetOwnerID(),
		Visits:    p.GetVisits(),
	})
}

// GetBirthDate returns the pet's birth date, or nil if none is set.
func (p *Pet) GetBirthDate() *Date {
	if p == nil {
		return nil
	}
	return p.BirthDate
}

// SetBirthDate sets the pet's birth date. nil clears it.
func (p *Pet) SetBirthDate(birthDate *Date) {
	p.BirthDate = birthDate
}

// GetType returns the pet's type, or nil if none is set.
func (p *Pet) GetType() *PetType {
	if p == nil {
		return nil
	}
	return p.Type
}

// SetType sets the pet's type (type_id). Persistence of the type cascades
// from the pet in the repository layer.
func (p *Pet) SetType(petType *PetType) {
	p.Type = petType
}

// GetOwner returns the owning owner, or nil if none is loaded.
func (p *Pet) GetOwner() OwnerRef {
	if p == nil {
		return nil
	}
	return p.Owner
}

// SetOwner associates the pet with owner and sets OwnerID to the owner's id.
// A nil owner clears both the association and OwnerID.
func (p *Pet) SetOwner(owner OwnerRef) {
	p.Owner = owner
	if owner == nil {
		p.OwnerID = nil
		return
	}
	p.OwnerID = owner.GetID()
}

// GetOwnerID returns the owner_id foreign key: the associated owner's id
// when an owner with an id is set, otherwise OwnerID.
func (p *Pet) GetOwnerID() *int {
	if p == nil {
		return nil
	}
	if p.Owner != nil {
		if id := p.Owner.GetID(); id != nil {
			return id
		}
	}
	return p.OwnerID
}

// GetVisits returns a snapshot of the pet's visits sorted by date, newest
// first. The returned slice is a fresh copy; modifying it does not affect
// the pet. It is never nil.
//
// MIGRATION_NOTE: Java returns Collections.unmodifiableList; Go has no
// read-only slices, so a defensive copy is returned instead. Java's
// Comparator.comparing(Visit::getDate) would throw on a null date; here
// visits without a date (and nil entries) sort last instead of failing.
func (p *Pet) GetVisits() []*Visit {
	if p == nil {
		return []*Visit{}
	}
	sorted := make([]*Visit, len(p.visits))
	copy(sorted, p.visits)
	sort.SliceStable(sorted, func(i, j int) bool {
		di, dj := sorted[i].GetDate(), sorted[j].GetDate()
		switch {
		case di == nil:
			return false
		case dj == nil:
			return true
		default:
			return di.After(*dj)
		}
	})
	return sorted
}

// SetVisits replaces the pet's visit collection with a new set built from
// visits. Duplicate pointers are collapsed, as building a HashSet does.
// Back-references are not modified, matching the source.
func (p *Pet) SetVisits(visits []*Visit) {
	set := make([]*Visit, 0, len(visits))
	seen := make(map[*Visit]struct{}, len(visits))
	for _, v := range visits {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		set = append(set, v)
	}
	p.visits = set
}

// AddVisit adds visit to the pet and sets the visit's pet back-reference
// (and PetID) to this pet. Adding the same visit twice has no effect on the
// collection, as with the source's Set. A nil visit is ignored.
func (p *Pet) AddVisit(visit *Visit) {
	if visit == nil {
		return
	}
	found := false
	for _, v := range p.visits {
		if v == visit {
			found = true
			break
		}
	}
	if !found {
		p.visits = append(p.visits, visit)
	}
	visit.SetPet(p)
}
