package adapters

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// SQLiteRepository is the database/sql link.Repository for the small-install
// engine (ADR-0005). It does not import a driver. cmd/veylqo blank-imports
// modernc.org/sqlite; tests may register that driver in *_test.go.
//
// Open the pool with _txlock=immediate and a busy timeout, and set
// MaxOpenConns to 1. SQLite has one writer. This type does not claim to
// match Postgres under concurrent writes, and it does not open the file.
//
// Timestamps are Unix milliseconds. A zero expiry or deletion is NULL, not
// epoch. The internal UUID is the canonical text form. destination_hash is
// SHA-256 of the stored destination string; this adapter does not clean.
// Update holds one lock across the load, updateFn, and the store, matching
// the memory repository. updateFn must not call this repository.
type SQLiteRepository struct {
	mu sync.Mutex
	db *sql.DB
}

// NewSQLiteRepository returns a repository that uses db. A nil db fails
// on the first call. The schema must already be applied.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Add stores link. A second public code, or a second non-custom row with
// the same destination string, is a conflict. Comparison is the stored
// string, not a cleaned form. Expired and deleted rows still occupy the
// code and the plain destination.
func (s *SQLiteRepository) Add(ctx context.Context, item link.Link) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("link repository: nil")
	}
	if item.ID() == uuid.Nil() || item.Code().String() == "" {
		return errEmptyLink
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		return insertLink(ctx, tx, item)
	})
}

// Get returns the row for the public code, including expired and deleted
// links. A miss is ErrNotFound. The returned value is a rehydrated copy.
func (s *SQLiteRepository) Get(ctx context.Context, code link.Code) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if s == nil || s.db == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	row := s.db.QueryRowContext(ctx, sqliteSelectByCode, code.String())
	item, err := scanLink(row)
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
func (s *SQLiteRepository) GetByDestination(ctx context.Context, destination link.Destination) (link.Link, error) {
	if err := ctx.Err(); err != nil {
		return link.Link{}, err
	}
	if s == nil || s.db == nil {
		return link.Link{}, errors.New("link repository: nil")
	}
	raw := destination.String()
	row := s.db.QueryRowContext(ctx, sqliteSelectByDestination, destinationHash(raw), raw)
	item, err := scanLink(row)
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
// critical section. A missing code is ErrNotFound and does not call
// updateFn. A nil updateFn is an error.
func (s *SQLiteRepository) Update(ctx context.Context, code link.Code, updateFn func(*link.Link) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("link repository: nil")
	}
	if updateFn == nil {
		return errNilUpdate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		item, err := getTx(ctx, tx, code.String())
		if errors.Is(err, sql.ErrNoRows) {
			return &link.NotFoundError{}
		}
		if err != nil {
			return err
		}
		if err := updateFn(&item); err != nil {
			return err
		}
		return updateLink(ctx, tx, code.String(), item)
	})
}

func (s *SQLiteRepository) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
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
	sqliteInsert = `INSERT INTO links (
    id, code, destination, destination_hash, custom, created_at, expires_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	sqliteSelectByCode = `SELECT id, code, destination, custom, created_at, expires_at, deleted_at
FROM links
WHERE code = ?`

	sqliteSelectByDestination = `SELECT id, code, destination, custom, created_at, expires_at, deleted_at
FROM links
WHERE custom = 0 AND destination_hash = ? AND destination = ?`

	sqliteCodeExists = `SELECT 1 FROM links WHERE code = ?`

	sqlitePlainDestinationExists = `SELECT 1 FROM links WHERE custom = 0 AND destination_hash = ?`

	sqliteUpdate = `UPDATE links
SET destination = ?, destination_hash = ?, custom = ?, expires_at = ?, deleted_at = ?
WHERE code = ?`
)

func insertLink(ctx context.Context, tx *sql.Tx, item link.Link) error {
	code := item.Code().String()
	if err := conflictExists(ctx, tx, sqliteCodeExists, code, "code"); err != nil {
		return err
	}
	destination := item.Destination().String()
	hash := destinationHash(destination)
	if !item.Custom() {
		if err := conflictExists(ctx, tx, sqlitePlainDestinationExists, hash, "destination"); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, sqliteInsert,
		item.ID().String(),
		code,
		destination,
		hash,
		boolToInt(item.Custom()),
		item.CreatedAt().UTC().UnixMilli(),
		nullableMillis(item.ExpiresAt()),
		nullableMillis(item.DeletedAt()),
	)
	if err != nil {
		if reason, ok := constraintReason(err); ok {
			return &link.ConflictError{Reason: reason}
		}
		return fmt.Errorf("link repository: add: %w", err)
	}
	return nil
}

func conflictExists(ctx context.Context, tx *sql.Tx, query, value, reason string) error {
	var one int
	err := tx.QueryRowContext(ctx, query, value).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("link repository: add: %w", err)
	}
	return &link.ConflictError{Reason: reason}
}

func getTx(ctx context.Context, tx *sql.Tx, code string) (link.Link, error) {
	item, err := scanLink(tx.QueryRowContext(ctx, sqliteSelectByCode, code))
	if err != nil {
		return link.Link{}, err
	}
	return item, nil
}

func updateLink(ctx context.Context, tx *sql.Tx, code string, item link.Link) error {
	destination := item.Destination().String()
	result, err := tx.ExecContext(ctx, sqliteUpdate,
		destination,
		destinationHash(destination),
		boolToInt(item.Custom()),
		nullableMillis(item.ExpiresAt()),
		nullableMillis(item.DeletedAt()),
		code,
	)
	if err != nil {
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLink(row rowScanner) (link.Link, error) {
	var (
		idRaw       string
		code        string
		destination string
		custom      int
		createdAt   int64
		expiresAt   sql.NullInt64
		deletedAt   sql.NullInt64
	)
	if err := row.Scan(&idRaw, &code, &destination, &custom, &createdAt, &expiresAt, &deletedAt); err != nil {
		return link.Link{}, err
	}
	id, err := uuid.Parse(idRaw)
	if err != nil {
		return link.Link{}, errors.New("link repository: corrupt id")
	}
	return link.UnmarshalLinkFromDatabase(link.DatabaseLink{
		ID:          id,
		Code:        code,
		Destination: destination,
		Custom:      custom != 0,
		CreatedAt:   time.UnixMilli(createdAt).UTC(),
		ExpiresAt:   timeFromNullMillis(expiresAt),
		DeletedAt:   timeFromNullMillis(deletedAt),
	})
}

func destinationHash(destination string) string {
	sum := sha256.Sum256([]byte(destination))
	return hex.EncodeToString(sum[:])
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableMillis(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().UnixMilli()
}

func timeFromNullMillis(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return time.UnixMilli(v.Int64).UTC()
}

func constraintReason(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	text := err.Error()
	switch {
	case strings.Contains(text, "links.code"):
		return "code", true
	case strings.Contains(text, "destination_hash"):
		return "destination", true
	default:
		return "", false
	}
}

var _ link.Repository = (*SQLiteRepository)(nil)
