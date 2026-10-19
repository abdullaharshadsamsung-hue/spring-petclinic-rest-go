package main

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"

	"migrated-app/internal/api"
	"migrated-app/internal/apidocs"
	"migrated-app/internal/config"
	"migrated-app/internal/model"
	"migrated-app/internal/service"
	"migrated-app/internal/store/sqlstore"
)

// clinicAdapter satisfies service.Clinic. The pet type, specialty and visit
// operations are backed by the real sqlstore repositories. Any other Clinic
// operation falls through to the embedded (nil) interface and panics, which
// middleware.Recoverer turns into a 500 response.
//
// MIGRATION_NOTE: the concrete service.Clinic constructor (internal/service/
// clinic.go) should replace this adapter once its signature is confirmed.
type clinicAdapter struct {
	service.Clinic
	petTypes    *sqlstore.PetTypeRepository
	specialties *sqlstore.SpecialtyRepository
	visits      *sqlstore.VisitRepository
}

func (c *clinicAdapter) FindAllPetTypes(ctx context.Context) ([]*model.PetType, error) {
	return c.petTypes.FindAll(ctx)
}

func (c *clinicAdapter) FindPetTypeByID(ctx context.Context, id int) (*model.PetType, error) {
	return c.petTypes.FindByID(ctx, id)
}

func (c *clinicAdapter) SavePetType(ctx context.Context, pt *model.PetType) error {
	return c.petTypes.Save(ctx, pt)
}

func (c *clinicAdapter) DeletePetType(ctx context.Context, pt *model.PetType) error {
	return c.petTypes.Delete(ctx, pt)
}

func (c *clinicAdapter) FindAllSpecialties(ctx context.Context) ([]*model.Specialty, error) {
	return c.specialties.FindAll(ctx)
}

func (c *clinicAdapter) FindSpecialtyByID(ctx context.Context, id int) (*model.Specialty, error) {
	return c.specialties.FindByID(ctx, id)
}

func (c *clinicAdapter) FindSpecialtiesByNameIn(ctx context.Context, names []string) ([]*model.Specialty, error) {
	return c.specialties.FindByNameIn(ctx, names)
}

func (c *clinicAdapter) SaveSpecialty(ctx context.Context, s *model.Specialty) error {
	return c.specialties.Save(ctx, s)
}

func (c *clinicAdapter) DeleteSpecialty(ctx context.Context, s *model.Specialty) error {
	return c.specialties.Delete(ctx, s)
}

func (c *clinicAdapter) SaveVisit(ctx context.Context, v *model.Visit) error {
	return c.visits.Save(ctx, v)
}

// visitService adapts the sqlstore VisitRepository to api.VisitService.
type visitService struct {
	repo *sqlstore.VisitRepository
}

func (s *visitService) FindAllVisits(ctx context.Context) ([]*model.Visit, error) {
	return s.repo.FindAll(ctx)
}

func (s *visitService) FindVisitByID(ctx context.Context, id int) (*model.Visit, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *visitService) SaveVisit(ctx context.Context, v *model.Visit) error {
	return s.repo.Save(ctx, v)
}

func (s *visitService) DeleteVisit(ctx context.Context, v *model.Visit) error {
	return s.repo.Delete(ctx, v)
}

// disabledSecurity mirrors auth.DisabledSecurityFilterChain (security
// disabled: every request permitted, Spring's default headers written,
// X-Frame-Options SAMEORIGIN).
//
// MIGRATION_NOTE: implemented here instead of importing internal/auth, whose
// basic-auth code depends on golang.org/x/crypto/bcrypt, which cannot be
// resolved with the current toolchain. Keep in sync with internal/auth.
func disabledSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-XSS-Protection", "0")
		h.Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000 ; includeSubDomains")
		}
		h.Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}

func buildRouter() http.Handler {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("loading config")
	}
	if cfg.DatabaseURL == "" {
		log.Warn().Msg("DATABASE_URL is not set; database-backed endpoints will fail")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("opening database")
	}

	visitRepo := sqlstore.NewVisitRepository(db)
	clinic := &clinicAdapter{
		petTypes:    sqlstore.NewPetTypeRepository(db),
		specialties: sqlstore.NewSpecialtyRepository(db),
		visits:      visitRepo,
	}
	users := service.NewUserService(sqlstore.NewUserRepository(db))

	ctxPath := api.DefaultContextPath
	mux := http.NewServeMux()

	api.RegisterRootRoutes(mux, ctxPath)
	// Security is disabled by default (petclinic.security.enable=false), so
	// no Authorizer is installed.
	api.NewPetTypeHandler(clinic, nil).Register(mux, ctxPath)
	api.NewSpecialtyHandler(clinic, nil).Register(mux, ctxPath)
	api.NewOwnerHandler(clinic, nil).Register(mux, ctxPath)
	api.NewOwnerV2Handler(clinic, nil).Register(mux, ctxPath)
	api.NewPetHandler(clinic, nil).Register(mux, ctxPath)
	api.NewPetV2Handler(clinic, nil).Register(mux, ctxPath)
	api.NewVetHandler(clinic, nil).Register(mux, ctxPath)
	api.NewVisitHandler(&visitService{repo: visitRepo}, nil).Register(mux, ctxPath)
	api.NewUserHandler(users, nil).Register(mux, ctxPath)

	docs, err := apidocs.NewHandler(apidocs.CustomOpenAPI())
	if err != nil {
		log.Fatal().Err(err).Msg("building api docs")
	}
	docs.Register(mux)

	app := disabledSecurity(mux)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/*", app)
	return r
}