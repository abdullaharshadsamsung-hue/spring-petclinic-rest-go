# Spring PetClinic REST (Go)

A Go port of **Spring PetClinic REST**, a REST API for a veterinary clinic. It manages owners, pets, pet types, visits, vets, specialties and users. It has role-based access control and serves an OpenAPI description with a Swagger UI.

This branch is the result of an automated migration from **Java / Spring Boot** to **Go (standard library)**.

| Item | Status |
|---|---|
| Modules migrated | 58 / 82 (the other 24 were excluded by plan, see [Known limitations](#known-limitations)) |
| Build | Passed |
| Target unit tests (`go test ./...`) | Passed |
| Behavior compared with the original Spring app | Passed |
| Migration confidence | 94%. The migration tool estimated this itself. It is **not** a measured result. |

Several modules still need review by a person. See [Manual review required](#manual-review-required).

---

## Tech stack

- **Language:** Go 1.25
- **HTTP:** `net/http` from the standard library (routing in `internal/api/router.go` and `cmd/server/router.go`)
- **Persistence:** `database/sql`, with one repository implementation in `internal/store/sqlstore`
- **Databases:** PostgreSQL (production target) and SQLite in-memory (default for development and tests)
- **API docs:** embedded `src/main/resources/openapi.yml`, served with an HTML shell that loads `swagger-ui-dist`
- **Auth:** HTTP Basic authentication with roles (`internal/auth`)
- **Container:** `Dockerfile`, `docker-compose.yml`

---

## Prerequisites

- **Go 1.25.** The migration was built and tested with the `golang:1.25-alpine` image.
- **PostgreSQL.** The documented setup below runs against Postgres (this is how the migration was verified). The commands below start one with Docker if you don't already have one.
- **Docker** (with Docker Compose for the Compose workflow). Needed to start the Postgres container below and for the Compose workflow.

---

## Database

| | Original (Spring) | This project (Go) |
|---|---|---|
| H2 (in-memory) | Supported, default | **Removed**. SQLite in-memory replaces it as the default. |
| HSQLDB | Supported | **Removed** |
| MySQL | Supported | **Dropped for now** |
| PostgreSQL | Supported | **Supported**. This is the production target. |
| SQLite (in-memory) | — | **New** default for development and tests |

These changes were made for the following reasons:

- **H2 and HSQLDB:** Go has no driver for either database. SQLite in-memory now handles the in-memory development and test role.
- **MySQL:** Support is deferred. The SQL already uses `?` placeholders, so adding MySQL later only needs a third dialect folder. Use `src/main/resources/db/mysql/schema.sql` as the starting point when that happens.

The original resource files are still in the repository, for example:

- `src/main/resources/application-*.properties`
- `src/main/resources/db/h2/`
- `src/main/resources/db/hsqldb/`
- `src/main/resources/db/mysql/`

Their presence does **not** mean those databases are supported. Check the Go code before relying on any of them. `cmd/server/schema.go` and `internal/config/config.go` are the places to look.

No separate schema setup command is needed. The schema is handled by the server at boot, and the logic lives in `cmd/server/schema.go`. You only need a reachable Postgres database (see Getting started).

---

## Getting started

Run all commands from the root of a fresh clone, in the same shell (the exported variables are used by later steps).

### 1. Download dependencies and build

```sh
go mod download && go build -o bin/server ./cmd/server
```

### 2. Start a Postgres database

If you don't already have Postgres running, start one with Docker:

```sh
docker run -d --name petclinic-db \
  -e POSTGRES_USER=petclinic \
  -e POSTGRES_PASSWORD=petclinic \
  -e POSTGRES_DB=petclinic \
  -p 5432:5432 postgres:16
```

### 3. Configure the environment

The server reads `DATABASE_URL`, `PORT` and `JWT_SECRET`. See [Environment variables](#environment-variables) for details.

```sh
export DATABASE_URL="postgres://petclinic:petclinic@localhost:5432/petclinic?sslmode=disable"
export PORT=8080
export JWT_SECRET="change-me"
```

If you use your own Postgres, adjust `DATABASE_URL` accordingly. To use the SQLite in-memory database instead, check `internal/config/config.go` for how the database is selected when `DATABASE_URL` is unset.

### 4. Database setup

You don't need to run a separate step. The server creates the schema at startup (see `cmd/server/schema.go`).

### 5. Run

```sh
./bin/server
```

The server listens on **port 8080**.

### 6. Try a request

```sh
curl -i http://localhost:8080/petclinic/api/owners
```

Before relying on this request, check the following:

- **Base path:** The routes are defined in `internal/api/router.go` and documented in `src/main/resources/openapi.yml`. Adjust the base path to match the `servers` entry and route definitions there.
- **Authentication:** If security is enabled, send Basic auth credentials with `curl -u <user>:<password>`. The seeded users are in `src/main/resources/db/postgres/data.sql`.
- **Unknown routes:** A path that doesn't exist returns a `404` problem response.

### Alternative: Docker Compose

```sh
docker compose up --build
```

Check `docker-compose.yml` for the service definitions, the exposed ports and the environment values passed to the server.

---

## Running tests

With the Postgres database running and the variables from step 3 exported:

```sh
go test ./...
```

During migration, all target unit tests passed. The test run used an environment that had a Postgres instance available and `DATABASE_URL`, `PORT` and `JWT_SECRET` set.

---

## Environment variables

| Variable | Purpose | Notes |
|---|---|---|
| `DATABASE_URL` | Database connection string | Point it at Postgres, e.g. `postgres://petclinic:petclinic@localhost:5432/petclinic?sslmode=disable`. SQLite in-memory is the default for development and tests. Check `internal/config/config.go` for the accepted format and the behavior when the variable is unset. |
| `PORT` | HTTP listen port | Set to `8080` in the documented setup. |
| `JWT_SECRET` | Secret value read by the server configuration | The original app uses HTTP Basic auth, so check `internal/config/config.go` for how this value is actually used. Never commit a real value. |

---

## Architecture overview

```
cmd/server/            Entry point and wiring
  main.go              Startup: config, DB, router, HTTP server
  router.go            Top-level route wiring
  schema.go            Schema setup at boot
internal/config/       Configuration from environment (config.go)
internal/api/          HTTP layer (replaces the Spring @RestControllers)
  router.go            Route table
  handlers_*.go        v1 handlers: owners, pets, pettypes, specialties, users, vets, visits
  handlers_*_v2.go     v2 handlers (owners, pets) with page/size parameters
  mapping.go, mapper/  Model <-> DTO mapping (replaces the MapStruct mappers)
  pettype_mapping.go, specialty_mapping.go
  problem.go           RFC 7807 problem responses (replaces ExceptionControllerAdvice)
  validate.go          Request validation (includes the pet-age rule)
internal/apidocs/      Serves the embedded openapi.yml and the Swagger UI shell
internal/auth/         Basic auth, the security on/off switch, roles
internal/model/        Domain entities (BaseEntity, NamedEntity, Person, Owner, Pet, ...)
internal/service/      Business logic (clinic.go, user.go), including Validate()
internal/store/        Repository interfaces (store.go, *_repository.go)
  sqlstore/            The single database/sql implementation
src/main/resources/    Original resources; openapi.yml is embedded and served
```

A request flows through the layers in this order:

1. `internal/api` (handler)
2. `internal/auth` (role check)
3. `internal/service` (validation and business rules)
4. `internal/store` interfaces
5. `internal/store/sqlstore` (database)

Errors are returned as problem documents built by `internal/api/problem.go`.

---

## Migration notes

These are the deliberate differences from the original Spring codebase.

### Databases

- **H2 and HSQLDB are no longer supported.** SQLite in-memory replaces H2 as the default.
  - *Why:* Go has no driver for HSQLDB or H2. SQLite takes over the in-memory development and test role, and Postgres is the production target.
- **MySQL support is dropped for now.** The `mysql` profile and DDL are not carried over.
  - *Why:* This is deferred. The SQL already uses `?` placeholders, so adding MySQL later only needs a third dialect folder. `src/main/resources/db/mysql/schema.sql` is the source when that happens.
- **The `jdbc`, `jpa` and `spring-data-jpa` profiles are gone.** A single `database/sql` repository implementation replaces them. It follows default spring-data-jpa semantics.
  - *Why:* Those profiles were alternate implementations of the same interfaces, chosen by Spring profile. Go gets one implementation.
- **The Postgres schema uses named `CREATE INDEX IF NOT EXISTS` indexes** instead of unnamed `CREATE INDEX`.
  - *Why:* The original unnamed `CREATE INDEX` creates a new duplicate index on every boot.
- **The `lastName` search is always case-sensitive.** On HSQLDB, the original search was case-insensitive.
  - *Why:* This matches the behavior of H2 (the original default) and Postgres.

### HTTP status codes and errors

- **Out-of-range `page` or `size` on the v2 endpoints returns `400`** (a validation problem) instead of `500`.
  - *Why:* The spec documents `400`. The `500` was accidental: the `ConstraintViolationException` fell into the catch-all handler.
- **An unknown route returns a `404` problem** (title `NoResourceFoundException`) instead of `500`.
  - *Why:* This is a planned deviation. In the original, `NoResourceFoundException` fell into the catch-all handler.
- **A role denial with security enabled returns a `403` problem** instead of `500`.
  - *Why:* A security failure must not look like a server error.
- **An owner telephone that is valid per the spec but not exactly 10 digits returns a `400` validation problem** instead of `500`.
  - *Why:* The service-level `Validate()` replaces JPA pre-persist validation, which threw a `500`.
- **`204` update responses carry headers only, with no body.**
  - *Why:* Go's `net/http` forbids a body on `204`. Spring's body was dropped by the container anyway.

### Write semantics

- **`POST /specialties`, `/vets` and `/visits` with an `id` in the body always insert a new row** and return the generated id.
  - *Why:* JDBC updated a nonexistent row and returned the fake id, and JPA merge behavior varied. Always inserting is consistent.
- **A visit created through `VisitFields` without a date defaults to today** instead of failing with a `404` integrity problem.
  - *Why:* This matches the intent of the entity's constructor, which defaults the date to today.

### Endpoints and tooling

- **The `/oops` endpoint is removed** from the router and from the served OpenAPI spec.
  - *Why:* It had no controller in the original.
- **The H2 web console (`/h2-console`) is no longer available.**
  - *Why:* It was development tooling only, and H2 is not used. A JVM-only database console has no Go counterpart (inferred).
- **Swagger UI is an embedded HTML shell that loads `swagger-ui-dist`**, instead of springdoc's bundled UI.
  - *Why:* springdoc is Spring-specific. The spec is served from the embedded `openapi.yml`.
- **The Spring cache starter and `CallMonitoringAspect` are not migrated.**
  - *Why:* The code has no `@Cacheable`, and the aspect was never registered. Neither was active in the original.

---

## Known limitations

**Unsupported databases**

- MySQL, H2 and HSQLDB are not supported. See [Database](#database).
- Their properties and SQL files remain under `src/main/resources/` but are not a supported configuration.

**Removed features**

- The H2 console is gone.
- Spring caching is gone.
- Call monitoring is gone. An `expvar` decorator could be added later if you want metrics.

**Swagger UI**

- The UI loads `swagger-ui-dist` instead of bundling it. Check `internal/apidocs/apidocs.go` to see where it loads from. If the source is external, the UI may not render in offline environments.
- The OpenAPI spec itself is served from the embedded file and is not affected.

**Original source files not translated (24 files, all excluded by plan)**

| Files | Reason |
|---|---|
| 13 | Spring Data JPA alternate implementation, for example `PetTypeRepositoryOverride`, `SpringDataPetTypeRepository` and `PetRepositoryOverride`. These were read only for semantics. |
| 7 | JPA alternate implementation, for example `JpaOwnerRepositoryImpl`, `JpaPetRepositoryImpl` and `JpaPetTypeRepositoryImpl`. These were read only for semantics. |
| 2 | Spring Data visit repository (`SpringDataVisitRepositoryImpl`, `VisitRepositoryOverride`). These were read only to settle default-profile semantics. |
| 1 | `CallMonitoringAspect`. It was inactive in the original (no `@Component`, never registered). |
| 1 | `BindingErrorsResponse`. It was dead code apart from a discarded `addAllErrors` call. Validation errors now come from `internal/api/problem.go`. |

No commands failed during migration, and no component was classified as impossible to migrate.

---

## Manual review required

The migration flagged the components below as low confidence. Each one is listed with the Go file or files that now implement it. Review these before you rely on this branch in production.

### Security

- **`DisableSecurityConfig`:** `internal/auth/disable_security.go`
- **`BasicAuthenticationConfig`:** `internal/auth/basic_authentication.go` and `internal/auth/roles.go`

For both, check:

- role mapping
- the `403` behavior
- how security is switched on and off
- what `JWT_SECRET` is used for

### Error handling

- **`ExceptionControllerAdvice`:** `internal/api/problem.go`. Check that every exception-to-status mapping, including the new `400`, `403` and `404` deviations, matches `src/main/resources/openapi.yml`.

### Domain model

- **`Visit`:** `internal/model/visit.go` (has no test file)
- **`Pet`:** `internal/model/pet.go` (has no test file)
- **`Owner`:** `internal/model/owner.go`
- **`Vet`:** `internal/model/vet.go`
- **`Role`:** `internal/model/role.go` (has no test file)

### Repositories (the JDBC implementation is now `internal/store/sqlstore`)

- **`OwnerRepository` and `PetRepository`:** `internal/store/store.go`
- **`JdbcOwnerRepositoryImpl`:** `internal/store/sqlstore/owner.go` (has no test file)
- **`JdbcPetRepositoryImpl`, `JdbcPet`, `JdbcPetRowMapper` and `JdbcPetVisitExtractor`:**
  - `internal/store/sqlstore/pet.go`
  - `internal/store/sqlstore/scan.go`
  - `internal/store/sqlstore/entity_lookup.go`
- **`JdbcPetTypeRepositoryImpl`:** `internal/store/sqlstore/pettype.go` (has no test file)
- **`JdbcVetRepositoryImpl`:** `internal/store/sqlstore/vet.go`
- **`JdbcVisitRepositoryImpl` and `JdbcVisitRowMapper`:** `internal/store/sqlstore/visit.go`
- **`JdbcUserRepositoryImpl`:** `internal/store/sqlstore/user.go` (has no test file)

### Validation

- **`PetAgeValidation` and `PetAgeValidator`:** `internal/api/validate.go`

### Mappers

- **`PetTypeMapper`:** `internal/api/pettype_mapping.go`
- **`OwnerMapper`, `PetMapper` and `VisitMapper`:** `internal/api/mapping.go` and `internal/api/mapper/mapping.go`

Mapping logic exists in two places. Confirm which one is used, and remove or merge the other.

### Services

- **`ClinicServiceImpl`:** `internal/service/clinic.go`, including the `Validate()` rules that replace JPA validation
- **`UserServiceImpl`:** `internal/service/user.go`

### Controllers

- **v1 controllers (`OwnerRestControllerV1`, `PetRestControllerV1`, `PetTypeRestControllerV1`, `SpecialtyRestControllerV1`, `VetRestControllerV1`, `VisitRestControllerV1`, `UserRestControllerV1`):** `internal/api/handlers_*.go`
  - `handlers_pettypes.go`, `handlers_specialties.go` and `handlers_vets.go` have no handler-level tests.
- **v2 controllers (`OwnerRestControllerV2`, `PetRestControllerV2`):**
  - `internal/api/handlers_owners_v2.go`
  - `internal/api/handlers_pets_v2.go`

### Wiring

`cmd/server/router.go` and `internal/api/router.go` both contain routing. Check that the full route table matches `src/main/resources/openapi.yml`, minus the `/oops` endpoint, which was removed on purpose.
