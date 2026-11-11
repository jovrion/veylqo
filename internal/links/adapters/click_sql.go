package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// sqlClicks is the shared click-count store. Dialects differ only in
// placeholders. A click is a public code and a count (ADR-0009). This
// type has no field for an IP, user agent, referrer, token, or destination.
type sqlClicks struct {
	db    *sql.DB
	add   string
	count string
}

// SQLiteClicks is the click store for the small-install engine. The
// schema must already be applied. This type does not import a driver.
type SQLiteClicks struct {
	sqlClicks
}

// PostgresClicks is the click store for Postgres. The schema must already
// be applied. This type does not import a driver.
type PostgresClicks struct {
	sqlClicks
}

// NewSQLiteClicks returns a store that uses db. A nil db fails on the
// first call. Placeholders are '?'. SQLite has one writer; this type does
// not claim to match Postgres under concurrent writes.
func NewSQLiteClicks(db *sql.DB) *SQLiteClicks {
	return &SQLiteClicks{sqlClicks: sqlClicks{
		db: db,
		add: `INSERT INTO clicks (code, click_count) VALUES (?, ?)
ON CONFLICT(code) DO UPDATE SET click_count = click_count + excluded.click_count`,
		count: `SELECT click_count FROM clicks WHERE code = ?`,
	}}
}

// NewPostgresClicks returns a store that uses db. A nil db fails on the
// first call. Placeholders are $1 and $2.
func NewPostgresClicks(db *sql.DB) *PostgresClicks {
	return &PostgresClicks{sqlClicks: sqlClicks{
		db: db,
		add: `INSERT INTO clicks (code, click_count) VALUES ($1, $2)
ON CONFLICT (code) DO UPDATE SET click_count = clicks.click_count + EXCLUDED.click_count`,
		count: `SELECT click_count FROM clicks WHERE code = $1`,
	}}
}

func (s *sqlClicks) Add(ctx context.Context, code link.Code, n int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("click repository: nil")
	}
	if code.String() == "" {
		return errors.New("click repository: empty code")
	}
	if n <= 0 {
		return errors.New("click repository: n must be positive")
	}
	if _, err := s.db.ExecContext(ctx, s.add, code.String(), n); err != nil {
		return fmt.Errorf("click repository: add: %w", err)
	}
	return nil
}

func (s *sqlClicks) Count(ctx context.Context, code link.Code) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s == nil || s.db == nil {
		return 0, errors.New("click repository: nil")
	}
	var n int64
	err := s.db.QueryRowContext(ctx, s.count, code.String()).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("click repository: count: %w", err)
	}
	return n, nil
}
