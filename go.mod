module migrated-app

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/lib/pq v1.10.9
	github.com/rs/zerolog v1.33.0
	golang.org/x/crypto v0.31.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

// Pin golang.org/x/crypto to a release that supports the local Go toolchain
// (go 1.25, GOTOOLCHAIN=local). Newer releases (v0.58.0+) require go >= 1.26,
// so resolving "latest" during module lookup fails.
replace golang.org/x/crypto => golang.org/x/crypto v0.31.0
