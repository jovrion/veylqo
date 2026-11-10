package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

const stubDriverName = "veylqo-migrate-stub"

func init() {
	sql.Register(stubDriverName, recordingDriver{})
}

var recorders sync.Map

type recorder struct {
	mu     sync.Mutex
	events []recorded
	conns  int
	failOn string
}

type recorded struct {
	conn  int
	query string
}

func (r *recorder) note(conn int, query string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recorded{conn: conn, query: query})
	if r.failOn != "" && strings.Contains(query, r.failOn) {
		return errStub
	}
	return nil
}

func (r *recorder) queries() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.events))
	for i, event := range r.events {
		out[i] = event.query
	}
	return out
}

func (r *recorder) oneConn() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns != 1 {
		return false
	}
	for _, event := range r.events {
		if event.conn != 1 {
			return false
		}
	}
	return true
}

var errStub = stubError("stub exec failed")

type stubError string

func (e stubError) Error() string { return string(e) }

type recordingDriver struct{}

func (recordingDriver) Open(name string) (driver.Conn, error) {
	v, ok := recorders.Load(name)
	if !ok {
		return nil, stubError("unknown recorder")
	}
	rec := v.(*recorder)
	rec.mu.Lock()
	rec.conns++
	id := rec.conns
	rec.mu.Unlock()
	return &recordingConn{rec: rec, id: id}, nil
}

type recordingConn struct {
	rec *recorder
	id  int
}

func (c *recordingConn) Prepare(query string) (driver.Stmt, error) {
	return &recordingStmt{c: c, query: query}, nil
}

func (c *recordingConn) Close() error { return nil }

func (c *recordingConn) Begin() (driver.Tx, error) {
	return recordingTx{}, nil
}

func (c *recordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return recordingTx{}, nil
}

func (c *recordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if err := c.rec.note(c.id, query); err != nil {
		return nil, err
	}
	return stubResult{}, nil
}

func (c *recordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := c.rec.note(c.id, query); err != nil {
		return nil, err
	}
	return &emptyRows{cols: []string{"version"}}, nil
}

type recordingTx struct{}

func (recordingTx) Commit() error   { return nil }
func (recordingTx) Rollback() error { return nil }

type recordingStmt struct {
	c     *recordingConn
	query string
}

func (s *recordingStmt) Close() error { return nil }

func (s *recordingStmt) NumInput() int { return -1 }

func (s *recordingStmt) Exec([]driver.Value) (driver.Result, error) {
	if err := s.c.rec.note(s.c.id, s.query); err != nil {
		return nil, err
	}
	return stubResult{}, nil
}

func (s *recordingStmt) Query([]driver.Value) (driver.Rows, error) {
	if err := s.c.rec.note(s.c.id, s.query); err != nil {
		return nil, err
	}
	return &emptyRows{cols: []string{"version"}}, nil
}

type stubResult struct{}

func (stubResult) LastInsertId() (int64, error) { return 0, nil }

func (stubResult) RowsAffected() (int64, error) { return 0, nil }

type emptyRows struct {
	cols []string
}

func (r *emptyRows) Columns() []string { return r.cols }

func (r *emptyRows) Close() error { return nil }

func (r *emptyRows) Next([]driver.Value) error { return io.EOF }

func TestPostgresAdvisoryLockWrapsApply(t *testing.T) {
	rec := openStub(t, "")
	if err := Apply(context.Background(), rec.db, Postgres); err != nil {
		t.Fatal(err)
	}
	assertLockWraps(t, rec.recorder, true)
	if !rec.recorder.oneConn() {
		t.Fatal("advisory lock was not held on the migration connection")
	}
}

func TestPostgresUnlocksAfterFailure(t *testing.T) {
	rec := openStub(t, "THIS_FAILS")
	files := fstest.MapFS{
		"migrations/postgres/0001_bad.sql": &fstest.MapFile{Data: []byte("SELECT THIS_FAILS;")},
	}
	err := apply(context.Background(), rec.db, Postgres, files)
	if err == nil {
		t.Fatal("failing migration returned nil error")
	}
	assertLockWraps(t, rec.recorder, false)
}

func TestSQLiteDialectSkipsAdvisoryLock(t *testing.T) {
	rec := openStub(t, "")
	if err := Apply(context.Background(), rec.db, SQLite); err != nil {
		t.Fatal(err)
	}
	for _, query := range rec.recorder.queries() {
		if strings.Contains(query, "pg_advisory") {
			t.Fatalf("sqlite dialect sent %s", query)
		}
	}
}

type stubDB struct {
	db       *sql.DB
	recorder *recorder
}

func openStub(t *testing.T, failOn string) stubDB {
	t.Helper()
	rec := &recorder{failOn: failOn}
	recorders.Store(t.Name(), rec)
	t.Cleanup(func() { recorders.Delete(t.Name()) })
	db, err := sql.Open(stubDriverName, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return stubDB{db: db, recorder: rec}
}

func assertLockWraps(t *testing.T, rec *recorder, wantDDL bool) {
	t.Helper()
	queries := rec.queries()
	lockAt, unlockAt := -1, -1
	sawDDL := false
	for i, query := range queries {
		switch {
		case strings.Contains(query, "pg_advisory_lock"):
			if !strings.Contains(query, "$1") {
				t.Fatalf("lock placeholder = %s", query)
			}
			lockAt = i
		case strings.Contains(query, "pg_advisory_unlock"):
			unlockAt = i
		case strings.Contains(query, "CREATE TABLE"):
			sawDDL = true
		}
	}
	if lockAt < 0 || unlockAt < 0 || lockAt >= unlockAt {
		t.Fatalf("lock order = %q", queries)
	}
	if wantDDL && !sawDDL {
		t.Fatal("lock did not wrap a migration")
	}
	if sawDDL && (lockAt > indexOf(queries, "CREATE TABLE") || unlockAt < indexOf(queries, "CREATE TABLE")) {
		t.Fatalf("ddl outside lock: %q", queries)
	}
}

func indexOf(queries []string, needle string) int {
	for i, query := range queries {
		if strings.Contains(query, needle) {
			return i
		}
	}
	return -1
}
