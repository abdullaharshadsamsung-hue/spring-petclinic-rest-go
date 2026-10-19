module github.com/spring-petclinic/petclinic

go 1.23

require (
	github.com/jackc/pgx/v5 v5.7.2
	golang.org/x/crypto v0.31.0
	modernc.org/sqlite v1.34.4
)

// Pin golang.org/x/crypto to a release that supports the local Go toolchain
// (go 1.25, GOTOOLCHAIN=local). Newer releases (v0.58.0+) require go >= 1.26,
// so resolving "latest" during module lookup fails.
replace golang.org/x/crypto => golang.org/x/crypto v0.31.0
