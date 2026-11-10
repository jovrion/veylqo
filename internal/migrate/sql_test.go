package migrate

import "testing"

func TestVersionName(t *testing.T) {
	ok := []string{"0001_links.sql", "0002_clicks.sql", "0010_a.sql"}
	bad := []string{
		"1_links.sql",
		"0001_Links.sql",
		"0001.sql",
		"links.sql",
		"0001_links.txt",
		"0001_.sql",
		"0001_link-s.sql",
	}
	for _, name := range ok {
		if !versionName(name) {
			t.Fatalf("versionName(%q) = false", name)
		}
	}
	for _, name := range bad {
		if versionName(name) {
			t.Fatalf("versionName(%q) = true", name)
		}
	}
}

func TestSplitSQL(t *testing.T) {
	got, err := splitSQL(`
-- comment with ;
CREATE TABLE t (note TEXT); /* block ; */
INSERT INTO t (note) VALUES ('a;b');
INSERT INTO t (note) VALUES ('it''s');
SELECT $$ semi; $$;
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CREATE TABLE t (note TEXT)",
		"INSERT INTO t (note) VALUES ('a;b')",
		"INSERT INTO t (note) VALUES ('it''s')",
		"SELECT $$ semi; $$",
	}
	if len(got) != len(want) {
		t.Fatalf("statements = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("statement %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitSQLUnclosed(t *testing.T) {
	if _, err := splitSQL("SELECT 'open"); err == nil {
		t.Fatal("unclosed quote returned nil error")
	}
}

func TestRejectTxnControl(t *testing.T) {
	if err := rejectTxnControl("BEGIN"); err == nil {
		t.Fatal("BEGIN was accepted")
	}
	if err := rejectTxnControl("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
}
