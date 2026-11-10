package link

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDomainDoesNotImportHTTPOrSQL(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	domain := filepath.Join(root, "internal", "links", "domain")
	var files []string
	err := filepath.WalkDir(domain, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk domain: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no domain files found")
	}

	fset := token.NewFileSet()
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("import %s: %v", spec.Path.Value, err)
			}
			if forbiddenDomainImport(path) {
				pos := fset.Position(spec.Path.Pos())
				t.Errorf("domain imports %s at %s:%d", path, filepath.ToSlash(pos.Filename), pos.Line)
			}
		}
	}
}

func forbiddenDomainImport(path string) bool {
	return path == "net/http" || strings.HasPrefix(path, "net/http/") ||
		path == "database/sql" || strings.HasPrefix(path, "database/sql/")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
