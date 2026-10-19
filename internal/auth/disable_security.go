package auth

import (
	"net/http"
	"strings"
)

// SecurityEnableProperty is the configuration key that toggles PetClinic
// security, matching the Spring property petclinic.security.enable.
const SecurityEnableProperty = "petclinic.security.enable"

// IsSecurityDisabled reports whether the raw value of SecurityEnableProperty
// selects the "security disabled" filter chain.
//
// It mirrors @ConditionalOnProperty(havingValue = "false"): the match is
// case-insensitive, and a missing (empty) value does NOT match, because
// matchIfMissing defaults to false. A value that is neither "true" nor
// "false" selects neither chain, as in the source.
func IsSecurityDisabled(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "false")
}

// DisabledSecurityFilterChain returns the middleware installed when security
// is disabled (port of DisableSecurityConfig.filterChain).
//
// Behaviour:
//   - Every request is permitted: no authentication or role checks run.
//   - CSRF protection is off. Go's net/http has no implicit CSRF filter, so
//     nothing needs to be removed.
//   - X-Frame-Options is SAMEORIGIN rather than Spring's default DENY.
//   - The other headers Spring Security still writes by default are kept for
//     parity (see writeDefaultSecurityHeaders).
//
// MIGRATION_NOTE: The Spring @Configuration/@Bean wiring becomes an explicit
// choice in main: wrap the router with this middleware when
// IsSecurityDisabled(cfg value) is true.
func DisabledSecurityFilterChain() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeDefaultSecurityHeaders(w, r)
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			next.ServeHTTP(w, r)
		})
	}
}

// writeDefaultSecurityHeaders writes the response headers that Spring
// Security's HeadersConfigurer adds by default. They are set before the
// handler runs, so a handler can still override them, much as Spring's
// CacheControlHeadersWriter skips headers the application already set.
func writeDefaultSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-XSS-Protection", "0")
	h.Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
	h.Set("Pragma", "no-cache")
	h.Set("Expires", "0")
	// Spring writes HSTS only for secure (HTTPS) requests.
	if r.TLS != nil {
		h.Set("Strict-Transport-Security", "max-age=31536000 ; includeSubDomains")
	}
}
