package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"migrated-app/internal/model"
	"migrated-app/internal/store"
)

// UserRepository is the PostgreSQL (database/sql) implementation of
// store.UserRepository. It is the Go port of JdbcUserRepositoryImpl.
//
// MIGRATION_NOTE: @Repository/@Profile("jdbc")/@DependsOnDatabaseInitialization
// are replaced by explicit construction in main after the schema has been
// initialised. SimpleJdbcInsert/BeanPropertySqlParameterSource become an
// explicit INSERT with positional $n arguments. The source used
// EmptyResultDataAccessException as its "not found" signal; here an EXISTS
// query decides between UPDATE and INSERT. The source ran without
// @Transactional (each statement auto-committed); here the user upsert and the
// role replacement run in a single transaction so a failure cannot leave a
// user with a partial set of roles.
type UserRepository struct {
	db *sql.DB
}

// Compile-time check that UserRepository satisfies store.UserRepository.
var _ store.UserRepository = (*UserRepository)(nil)

// NewUserRepository returns a UserRepository backed by db.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

const (
	userExistsSQL  = `SELECT EXISTS (SELECT 1 FROM users WHERE username = $1)`
	userUpdateSQL  = `UPDATE users SET password = $1, enabled = $2 WHERE username = $3`
	userInsertSQL  = `INSERT INTO users (username, password, enabled) VALUES ($1, $2, $3)`
	userDelRoleSQL = `DELETE FROM roles WHERE username = $1`
	userAddRoleSQL = `INSERT INTO roles (username, role) VALUES ($1, $2)`
)

// Save upserts the user keyed by username: when a row exists its password and
// enabled columns are updated, otherwise a new row is inserted. All of the
// user's existing roles are then deleted and one row is inserted per role
// with a non-empty name. Everything runs in one transaction.
func (r *UserRepository) Save(ctx context.Context, user *model.User) (err error) {
	if user == nil {
		return errors.New("sqlstore: save user: user must not be nil")
	}
	if user.Username == "" {
		return errors.New("sqlstore: save user: username must not be empty")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlstore: save user %q: begin tx: %w", user.Username, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = upsertUser(ctx, tx, user); err != nil {
		return err
	}
	if err = replaceUserRoles(ctx, tx, user); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sqlstore: save user %q: commit: %w", user.Username, err)
	}
	return nil
}

// upsertUser updates the existing users row or inserts a new one.
func upsertUser(ctx context.Context, tx *sql.Tx, user *model.User) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, userExistsSQL, user.Username).Scan(&exists); err != nil {
		return fmt.Errorf("sqlstore: save user %q: lookup: %w", user.Username, err)
	}
	if exists {
		if _, err := tx.ExecContext(ctx, userUpdateSQL, user.Password, user.Enabled, user.Username); err != nil {
			return wrapUserWriteErr(user.Username, "update", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, userInsertSQL, user.Username, user.Password, user.Enabled); err != nil {
		return wrapUserWriteErr(user.Username, "insert", err)
	}
	return nil
}

// replaceUserRoles deletes all roles for the user and re-inserts the ones
// with a name. The roles.username column always receives the owning user's
// username, as in the source.
func replaceUserRoles(ctx context.Context, tx *sql.Tx, user *model.User) error {
	if _, err := tx.ExecContext(ctx, userDelRoleSQL, user.Username); err != nil {
		return fmt.Errorf("sqlstore: save user %q: delete roles: %w", user.Username, err)
	}
	for _, role := range user.Roles {
		// MIGRATION_NOTE: Java skipped roles whose name was null; Go strings
		// cannot be null, so empty names (and nil roles) are skipped instead.
		if role == nil || role.Name == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, userAddRoleSQL, user.Username, role.Name); err != nil {
			return wrapUserWriteErr(user.Username, "insert role "+role.Name, err)
		}
	}
	return nil
}

// userSQLStateError matches driver errors exposing a PostgreSQL SQLSTATE
// (e.g. pgx's *pgconn.PgError).
type userSQLStateError interface {
	SQLState() string
}

// wrapUserWriteErr maps constraint violations (SQLSTATE class 23) to
// store.ErrIntegrity and wraps every other error with context.
func wrapUserWriteErr(username, op string, err error) error {
	var se userSQLStateError
	if errors.As(err, &se) && strings.HasPrefix(se.SQLState(), "23") {
		return store.IntegrityViolation(fmt.Sprintf("save user %q: %s", username, op), err)
	}
	return fmt.Errorf("sqlstore: save user %q: %s: %w", username, op, err)
}
