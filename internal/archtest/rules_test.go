package archtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLayerRule(t *testing.T) {
	t.Run("layer rule", func(t *testing.T) {
		mod := mustScan(t, repoRoot(t))
		if !hasPackage(mod, "/internal/archtest") {
			t.Fatal("layer rule: scan did not find internal/archtest")
		}
		assertNoViolations(t, "layer rule", layerViolations(mod))

		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/veylqo\n\ngo 1.27\n")
		writeFile(t, root, "internal/links/domain/link.go", `package domain

import (
	"fmt"
	"example.com/veylqo/internal/links/app"
	"example.com/veylqo/internal/links/adapters"
	"example.com/veylqo/internal/links/ports"
	"example.com/veylqo/internal/links/service"
	"example.com/veylqo/internal/links/service/wire"
	"example.com/veylqo/internal/links/application"
	"internal/links/app"
	"github.com/external/lib"
	"encoding/json/v2"
)
`)
		writeFile(t, root, "internal/links/domain/code/code.go", `package code

import _ "example.com/veylqo/internal/links/ports"
`)
		writeFile(t, root, "internal/links/app/app.go", `package app

import _ "example.com/veylqo/internal/links/domain"
`)
		scanned := mustScan(t, root)
		got := layerViolations(scanned)
		for _, want := range []string{
			"imports app",
			"imports adapters",
			"imports ports",
			"imports service",
			"imports a third-party module github.com/external/lib",
			"internal/links/domain/code",
		} {
			if !containsAll(got, want) {
				t.Fatalf("layer rule: missing %q in:\n%s", want, strings.Join(got, "\n"))
			}
		}
		for _, gotLine := range got {
			if !strings.Contains(gotLine, "layer rule") {
				t.Fatalf("violation does not name the layer rule: %s", gotLine)
			}
			if strings.Contains(gotLine, "encoding/json/v2") || strings.Contains(gotLine, "application") || strings.Contains(gotLine, " fmt") {
				t.Fatalf("layer rule: false positive: %s", gotLine)
			}
		}
		if len(got) != 8 {
			t.Fatalf("layer rule: got %d violations, want 8:\n%s", len(got), strings.Join(got, "\n"))
		}
		for _, pkg := range scanned.packages {
			if strings.HasSuffix(pkg.importPath, "/internal/links/app") {
				if v := layerViolations(module{path: scanned.path, packages: []scannedPackage{pkg}}); len(v) != 0 {
					t.Fatalf("layer rule: app importing domain is allowed, got %s", v)
				}
			}
		}
	})
}

func TestStdlibOnlyRule(t *testing.T) {
	t.Run("stdlib-only rule", func(t *testing.T) {
		mod := mustScan(t, repoRoot(t))
		assertNoViolations(t, "stdlib-only rule", stdlibOnlyViolations(mod))

		for _, router := range thirdPartyRouters {
			violations := stdlibOnlyViolations(module{packages: []scannedPackage{{
				importPath: "example.com/veylqo/internal/httpx",
				imports:    []scannedImport{{path: router + "/extra", position: "router.go:1"}},
			}}})
			if len(violations) != 1 || !strings.Contains(violations[0], "stdlib-only rule") || !strings.Contains(violations[0], "third-party router") {
				t.Fatalf("stdlib-only rule: router %s: got %v", router, violations)
			}
		}

		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/veylqo\n\ngo 1.27\n")
		writeFile(t, root, "internal/httpx/json.go", `package httpx

import (
	"encoding/json"
	"encoding/json/v2"
)
`)
		writeFile(t, root, "internal/httpx/router.go", `package httpx

import (
	_ "github.com/go-chi/chi/v5"
	_ "github.com/gin-gonic/gin"
	_ "github.com/labstack/echo/v4"
)
`)
		writeFile(t, root, "internal/httpx/win.go", `//go:build windows

package httpx

import _ "github.com/gorilla/mux"
`)
		writeFile(t, root, "internal/httpx/ignore.go", `//go:build ignore

package httpx

import "encoding/json"
`)
		writeFile(t, root, "internal/links/adapters/db.go", `package adapters

import (
	_ "modernc.org/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
)
`)
		writeFile(t, root, "internal/links/adapters/db_test.go", `package adapters

import (
	_ "modernc.org/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
)
`)
		writeFile(t, root, "cmd/other/main.go", `package main

import _ "modernc.org/sqlite"
`)
		writeFile(t, root, "cmd/veylqo/main.go", `package main

import (
	_ "modernc.org/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
)
`)
		got := stdlibOnlyViolations(mustScan(t, root))
		for _, want := range []string{
			"stdlib-only rule: encoding/json v1 imported",
			"third-party router github.com/go-chi/chi",
			"third-party router github.com/gin-gonic/gin",
			"third-party router github.com/labstack/echo",
			"third-party router github.com/gorilla/mux",
			"driver modernc.org/sqlite imported outside cmd/",
			"driver github.com/jackc/pgx/v5/stdlib imported outside cmd/",
			"cmd/other",
		} {
			if !containsAll(got, want) {
				t.Fatalf("stdlib-only rule: missing %q in:\n%s", want, strings.Join(got, "\n"))
			}
		}
		for _, line := range got {
			if !strings.Contains(line, "stdlib-only rule") {
				t.Fatalf("violation does not name the stdlib-only rule: %s", line)
			}
			if strings.Contains(line, "encoding/json/v2") || strings.Contains(line, "/cmd/veylqo/") || strings.Contains(line, "db_test.go") || strings.Contains(line, "ignore.go") {
				t.Fatalf("stdlib-only rule: false positive: %s", line)
			}
		}
		// json v1, three routers, windows mux, two adapter drivers, one cmd/other driver.
		if len(got) != 8 {
			t.Fatalf("stdlib-only rule: got %d violations, want 8:\n%s", len(got), strings.Join(got, "\n"))
		}
	})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from " + file)
		}
		dir = parent
	}
}

func mustScan(t *testing.T, root string) module {
	t.Helper()
	mod, err := scan(root)
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	return mod
}

func assertNoViolations(t *testing.T, rule string, violations []string) {
	t.Helper()
	if len(violations) != 0 {
		t.Fatalf("%s: current module has violations:\n%s", rule, strings.Join(violations, "\n"))
	}
}

func hasPackage(mod module, suffix string) bool {
	for _, pkg := range mod.packages {
		if strings.HasSuffix(pkg.importPath, suffix) {
			return true
		}
	}
	return false
}

func containsAll(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
