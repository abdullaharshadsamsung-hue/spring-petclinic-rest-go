package auth

// This file is the port of BasicAuthenticationConfig: the security layer used
// when petclinic.security.enable=true. It provides:
//   - the BCrypt password encoder (PasswordEncoder bean),
//   - JDBC-backed user lookup (configureGlobal / jdbcAuthentication),
//   - the filter chain requiring HTTP Basic auth on every request with CSRF
//     disabled (filterChain bean),
//   - a Require(...) middleware replacing @EnableMethodSecurity/@PreAuthorize.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// Queries used for JDBC authentication. They are the source's
// usersByUsernameQuery / authoritiesByUsernameQuery, rewritten with
// PostgreSQL positional placeholders.
const (
	// UsersByUsernameQuery loads a user's credentials and enabled flag.
	UsersByUsernameQuery = "select username,password,enabled from users where username=$1"
	// AuthoritiesByUsernameQuery loads a user's granted roles.
	AuthoritiesByUsernameQuery = "select username,role from roles where username=$1"
)

// basicRealm is the realm Spring Security's BasicAuthenticationEntryPoint uses
// by default.
const basicRealm = "Realm"

// bcryptDefaultCost matches BCryptPasswordEncoder's default strength (10).
const bcryptDefaultCost = 10

// IsSecurityEnabled reports whether the raw value of SecurityEnableProperty
// selects the Basic authentication chain. It mirrors
// @ConditionalOnProperty(havingValue = "true"): case-insensitive, and a
// missing value does not match.
func IsSecurityEnabled(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

// PasswordEncoder hashes and verifies passwords.
type PasswordEncoder interface {
	// Encode hashes a raw password.
	Encode(raw string) (string, error)
	// Matches reports whether raw matches the encoded hash.
	Matches(raw, encoded string) bool
}

// BCryptPasswordEncoder is a PasswordEncoder backed by bcrypt. Hashes written
// by Spring's BCryptPasswordEncoder ($2a$10$...) verify unchanged.
type BCryptPasswordEncoder struct {
	cost int
}

// NewPasswordEncoder returns the application's password encoder (BCrypt with
// Spring's default strength of 10).
func NewPasswordEncoder() *BCryptPasswordEncoder {
	return &BCryptPasswordEncoder{cost: bcryptDefaultCost}
}

// Encode hashes raw with bcrypt.
func (e *BCryptPasswordEncoder) Encode(raw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(raw), e.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt encode: %w", err)
	}
	return string(h), nil
}

// Matches reports whether raw matches the bcrypt hash encoded. An empty or
// malformed hash never matches.
func (e *BCryptPasswordEncoder) Matches(raw, encoded string) bool {
	if encoded == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(raw)) == nil
}

// UserDetails is an authenticated principal as loaded from the database.
type UserDetails struct {
	Username    string
	Password    string
	Enabled     bool
	Authorities []string
}

// UserDetailsService loads users by username.
type UserDetailsService interface {
	// LoadUserByUsername returns the user and true, or false if the user does
	// not exist (or has no authorities, as in Spring's JdbcDaoImpl).
	LoadUserByUsername(ctx context.Context, username string) (UserDetails, bool, error)
}

// JDBCUserDetailsService is the port of jdbcAuthentication().dataSource(...).
type JDBCUserDetailsService struct {
	db *sql.DB
}

// NewJDBCUserDetailsService returns a UserDetailsService reading the users and
// roles tables from db.
func NewJDBCUserDetailsService(db *sql.DB) *JDBCUserDetailsService {
	return &JDBCUserDetailsService{db: db}
}

// LoadUserByUsername implements UserDetailsService.
func (s *JDBCUserDetailsService) LoadUserByUsername(ctx context.Context, username string) (UserDetails, bool, error) {
	var u UserDetails
	err := s.db.QueryRowContext(ctx, UsersByUsernameQuery, username).Scan(&u.Username, &u.Password, &u.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return UserDetails{}, false, nil
	}
	if err != nil {
		return UserDetails{}, false, fmt.Errorf("load user %q: %w", username, err)
	}

	rows, err := s.db.QueryContext(ctx, AuthoritiesByUsernameQuery, username)
	if err != nil {
		return UserDetails{}, false, fmt.Errorf("load roles for %q: %w", username, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, role string
		if err := rows.Scan(&name, &role); err != nil {
			return UserDetails{}, false, fmt.Errorf("scan role for %q: %w", username, err)
		}
		u.Authorities = append(u.Authorities, role)
	}
	if err := rows.Err(); err != nil {
		return UserDetails{}, false, fmt.Errorf("iterate roles for %q: %w", username, err)
	}
	// Spring's JdbcDaoImpl treats a user with no authorities as not found.
	if len(u.Authorities) == 0 {
		return UserDetails{}, false, nil
	}
	return u, true, nil
}

type principalKey struct{}

// WithPrincipal returns a copy of ctx carrying the authenticated user.
func WithPrincipal(ctx context.Context, u UserDetails) context.Context {
	return context.WithValue(ctx, principalKey{}, u)
}

// PrincipalFromContext returns the authenticated user stored by the Basic
// authentication middleware, if any.
func PrincipalFromContext(ctx context.Context) (UserDetails, bool) {
	u, ok := ctx.Value(principalKey{}).(UserDetails)
	return u, ok
}

// AuthoritiesFromContext returns the roles of the authenticated user, or nil.
func AuthoritiesFromContext(ctx context.Context) []string {
	u, ok := PrincipalFromContext(ctx)
	if !ok {
		return nil
	}
	return u.Authorities
}

// BasicAuthenticationFilterChain returns the middleware installed when
// security is enabled (port of BasicAuthenticationConfig.filterChain):
//   - every request must carry valid HTTP Basic credentials;
//   - unknown, disabled, role-less users and bad passwords get
//     401 with `WWW-Authenticate: Basic realm="Realm"` and an empty body;
//   - CSRF protection is off (net/http has no implicit CSRF filter);
//   - Spring Security's default response headers are written.
//
// MIGRATION_NOTE: the @Configuration/@ConditionalOnProperty wiring becomes an
// explicit choice in main. INTEGRATION: in cmd/server, when
// IsSecurityEnabled(viper value of SecurityEnableProperty) is true, wrap the
// router with BasicAuthenticationFilterChain(NewJDBCUserDetailsService(db),
// NewPasswordEncoder()); CORS middleware must run before it so preflights are
// not challenged.
func BasicAuthenticationFilterChain(users UserDetailsService, encoder PasswordEncoder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeDefaultSecurityHeaders(w, r)
			w.Header().Set("X-Frame-Options", "DENY")

			username, password, ok := r.BasicAuth()
			if !ok {
				unauthorized(w)
				return
			}
			u, found, err := users.LoadUserByUsername(r.Context(), username)
			if err != nil {
				// Spring maps InternalAuthenticationServiceException to the
				// entry point (401) as well.
				log.Error().Err(err).Str("username", username).Msg("authentication lookup failed")
				unauthorized(w)
				return
			}
			if !found {
				// Mitigate user-enumeration timing attacks like Spring's
				// DaoAuthenticationProvider does.
				encoder.Matches(password, dummyHash())
				unauthorized(w)
				return
			}
			if !encoder.Matches(password, u.Password) || !u.Enabled {
				unauthorized(w)
				return
			}
			u.Password = "" // erase credentials, as Spring does after authentication
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), u)))
		})
	}
}

// Require returns middleware that allows the request only if the authenticated
// user has at least one of roles (port of @PreAuthorize("hasRole(...)")).
// A missing role yields 403 with an application/problem+json body.
//
// MIGRATION_NOTE: the Spring source returned 500 for AccessDeniedException
// via the generic exception advice; 403 is a documented deliberate deviation.
// When no principal is present (security disabled), the check is skipped,
// matching the disabled chain where @PreAuthorize is not enforced.
func Require(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := PrincipalFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			for _, role := range roles {
				if HasRole(u.Authorities, role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			forbidden(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", basicRealm))
	w.WriteHeader(http.StatusUnauthorized)
}

func forbidden(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	body := fmt.Sprintf(`{"type":"about:blank","title":"Forbidden","status":403,"detail":"Access Denied","instance":%q}`, r.URL.Path)
	_, _ = w.Write([]byte(body))
}

var (
	dummyHashOnce  sync.Once
	dummyHashValue string
)

// dummyHash returns a lazily computed bcrypt hash used to equalise timing
// for unknown users.
func dummyHash() string {
	dummyHashOnce.Do(func() {
		h, err := bcrypt.GenerateFromPassword([]byte("userNotFoundPassword"), bcryptDefaultCost)
		if err == nil {
			dummyHashValue = string(h)
		}
	})
	return dummyHashValue
}
