package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// PostgresRepository is the database/sql link.Repository for Postgres
// (ADR-0005). It does not import a driver. cmd/veylqo blank-imports
// github.com/jackc/pgx/v5/stdlib; tests may register that driver in
// *_test.go.
//
// The caller opens the pool and applies migrations. This type does not
// open a connection and it does not claim to be the small-install engine.
//
// The internal key is the Postgres uuid type, bound as the standard
// uuid.UUID, not text. Timestamps are Unix milliseconds. A zero expiry or
// deletion is NULL, not epoch. destination_hash is SHA-256 of the stored
// destination string; this adapter does not clean.
//
// Add uses the unique index on code and the partial unique index on
// destination_hash (custom = false) so two concurrent inserts cannot both
// commit. Update loads with SELECT FOR UPDATE, calls updateFn, and stores
// only when updateFn returns nil, so two updates cannot both observe the
// pre-update row. updateFn must not call this repository.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository returns a repository that uses db. A nil db fails
// on the first call. The schema must already be applied.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Add stores link. A second public code, or a second non-custom row with
// the same destination string, is a conflict. Comparison is the stored
// string, not a cleaned form. Expired and deleted rows still occupy the
// code and the plain destination.
func (p *PostgresRepository) Add(ctx context.Context, item link.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || p.db == nil {
		return errors.New("link repository: nil")
	}
	if item.ID() == uuid.Nil() || item.Code().String() == "" {
		return errEmptyLink
	}
	return p.withTx(ctx, func(tx *sql.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return insertPostgresLink(ctx, tx, item)
	})
}

// Get returns the row for the public code, including expired and deleted
// links. A miss is ErrNotFound. The returned value is a rehydrated copy.
func (p *PostgresRepository) Get(ctx context.Context, code link.Code) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if p == nil || p.db == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	row := p.db.QueryRowContext(ctx, postgresSelectByCode, code.String())
	item, err := scanPostgresLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return link.Link{}, &link.NotFoundError{}
	}
	if err != nil {
		return link.Link{}, fmt.Errorf("link repository: get: %w", err)
	}
	return item, nil
}

// GetByDestination returns the non-custom row whose stored destination
// string equals destination. Custom rows are skipped. Expired and deleted
// rows are returned. Comparison is exact. No match is ErrNotFound.
func (p *PostgresRepository) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if p == nil || p.db == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	raw := destination.String()
	row := p.db.QueryRowContext(ctx, postgresSelectByDestination, destinationHash(raw), raw)
	item, err := scanPostgresLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return link.Link{}, &link.NotFoundError{}
	}
	if err != nil {
		return link.Link{}, fmt.Errorf("link repository: get destination: %w", err)
	}
	return item, nil
}

// Update loads the row, calls updateFn, and stores the result only when
// updateFn returns nil. The load, the callback, and the store are one
// transaction, and the load takes a row lock. A missing code is
// ErrNotFound and does not call updateFn. A nil updateFn is an error.
func (p *PostgresRepository) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil || p.db == nil {
		return errors.New("link repository: nil")
	}
	if updateFn == nil {
		return errNilUpdate
	}
	return p.withTx(ctx, func(tx *sql.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, err := scanPostgresLink(tx.QueryRowContext(ctx, postgresSelectForUpdate, code.String()))
		if errors.Is(err, sql.ErrNoRows) {
			return &link.NotFoundError{}
		}
		if err != nil {
			return err
		}
		if err := updateFn(&item); err != nil {
			return err
		}
		return updatePostgresLink(ctx, tx, code.String(), item)
	})
}

func (p *PostgresRepository) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

const (
	postgresInsert = `INSERT INTO links (
    id, code, destination, destination_hash, custom, created_at, expires_at, deleted_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	postgresLinkColumns = `id, code, destination, custom, created_at, expires_at, deleted_at`

	postgresSelectByCode = `SELECT ` + postgresLinkColumns + `
FROM links
WHERE code = $1`

	postgresSelectForUpdate = postgresSelectByCode + `
FOR UPDATE`

	postgresSelectByDestination = `SELECT ` + postgresLinkColumns + `
FROM links
WHERE custom = false AND destination_hash = $1 AND destination = $2`

	postgresCodeExists = `SELECT 1 FROM links WHERE code = $1`

	postgresPlainDestinationExists = `SELECT 1 FROM links WHERE custom = false AND destination_hash = $1`

	postgresUpdate = `UPDATE links
SET destination = $1, destination_hash = $2, custom = $3, expires_at = $4, deleted_at = $5
WHERE code = $6`
)

func insertPostgresLink(ctx context.Context, tx *sql.Tx, item link.Link) error {
	code := item.Code().String()
	if err := conflictExists(ctx, tx, postgresCodeExists, code, "code"); err != nil {
		return err
	}
	destination := item.Destination().String()
	hash := destinationHash(destination)
	if !item.Custom() {
		if err := conflictExists(ctx, tx, postgresPlainDestinationExists, hash, "destination"); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, postgresInsert,
		item.ID(),
		code,
		destination,
		hash,
		item.Custom(),
		item.CreatedAt().UTC().UnixMilli(),
		nullableMillis(item.ExpiresAt()),
		nullableMillis(item.DeletedAt()),
	)
	if err != nil {
		if reason, ok := postgresConstraintReason(err); ok {
			return &link.ConflictError{Reason: reason}
		}
		return fmt.Errorf("link repository: add: %w", err)
	}
	return nil
}

func updatePostgresLink(ctx context.Context, tx *sql.Tx, code string, item link.Link) error {
	destination := item.Destination().String()
	result, err := tx.ExecContext(ctx, postgresUpdate,
		destination,
		destinationHash(destination),
		item.Custom(),
		nullableMillis(item.ExpiresAt()),
		nullableMillis(item.DeletedAt()),
		code,
	)
	if err != nil {
		if reason, ok := postgresConstraintReason(err); ok {
			return &link.ConflictError{Reason: reason}
		}
		return fmt.Errorf("link repository: update: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("link repository: update: %w", err)
	}
	if n != 1 {
		return errors.New("link repository: update missed")
	}
	return nil
}

func scanPostgresLink(row rowScanner) (link.Link, error) {
	var (
		id          uuid.UUID
		code        string
		destination string
		custom      bool
		createdAt   int64
		expiresAt   sql.NullInt64
		deletedAt   sql.NullInt64
	)
	if err := row.Scan(&id, &code, &destination, &custom, &createdAt, &expiresAt, &deletedAt); err != nil {
		return link.Link{}, err
	}
	return link.UnmarshalLinkFromDatabase(link.DatabaseLink{
		ID:          id,
		Code:        code,
		Destination: destination,
		Custom:      custom,
		CreatedAt:   time.UnixMilli(createdAt).UTC(),
		ExpiresAt:   timeFromNullMillis(expiresAt),
		DeletedAt:   timeFromNullMillis(deletedAt),
	})
}

// postgresConstraintReason maps a unique-violation to a domain reason.
// It does not import the driver. The driver text used here is the
// SQLSTATE and the index name, never the stored destination.
func postgresConstraintReason(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	text := err.Error()
	if !strings.Contains(text, "SQLSTATE 23505") {
		return "", false
	}
	switch {
	case strings.Contains(text, "links_code"):
		return "code", true
	case strings.Contains(text, "links_destination_hash_plain"), strings.Contains(text, "destination_hash"):
		return "destination", true
	default:
		return "", false
	}
}

var _ link.Repository = (*PostgresRepository)(nil)
