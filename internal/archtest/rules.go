// Package archtest enforces the layer rule (ADR-0002) and the
// stdlib-only rule (ADR-0003). It discovers files with go/build and
// reads import paths with go/parser. It does not typecheck.
package archtest

import (
	"bufio"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// forbiddenLayers are the internal/links packages domain must not import.
var forbiddenLayers = []string{"app", "adapters", "ports", "service"}

// thirdPartyRouters are the routers rejected by the stdlib-only rule.
// ADR-0003 names chi, gin, and echo; the rest are other third-party routers.
var thirdPartyRouters = []string{
	"github.com/go-chi/chi",
	"github.com/pressly/chi",
	"github.com/gin-gonic/gin",
	"github.com/labstack/echo",
	"github.com/gofiber/fiber",
	"github.com/gorilla/mux",
	"github.com/julienschmidt/httprouter",
}

// driverModules are the only third-party drivers ADR-0003 allows, and
// only as production imports in cmd/veylqo. Adapter *_test.go files are
// ignored for this rule so go test can register a driver.
var driverModules = []string{
	"github.com/jackc/pgx",
	"modernc.org/sqlite",
}

type module struct {
	path     string
	packages []scannedPackage
}

type scannedPackage struct {
	importPath string
	imports    []scannedImport
}

type scannedImport struct {
	path     string
	position string
	testFile bool
}

func scan(root string) (module, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return module{}, err
	}
	modulePath, err := modulePathFrom(root)
	if err != nil {
		return module{}, err
	}
	var packages []scannedPackage
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if path != root && (name == "vendor" || name == "testdata" || name == "node_modules" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		pkg, scanErr := scanDir(root, modulePath, path)
		if scanErr != nil {
			return scanErr
		}
		if pkg != nil {
			packages = append(packages, *pkg)
		}
		return nil
	})
	if err != nil {
		return module{}, err
	}
	return module{path: modulePath, packages: packages}, nil
}

func modulePathFrom(root string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			path := strings.Trim(fields[1], `"`)
			if path == "" {
				return "", fmt.Errorf("empty module path in %s", root)
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("module path not found in %s", filepath.Join(root, "go.mod"))
}

func scanDir(root, modulePath, dir string) (*scannedPackage, error) {
	hasGo, err := dirHasGoFiles(dir)
	if err != nil || !hasGo {
		return nil, err
	}
	pkg, err := build.Default.ImportDir(dir, build.ImportComment)
	if err != nil {
		if _, ok := err.(*build.NoGoError); !ok {
			return nil, fmt.Errorf("go/build %s: %w", dir, err)
		}
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return nil, err
	}
	importPath := modulePath
	if rel != "." {
		importPath = modulePath + "/" + filepath.ToSlash(rel)
	}
	scanned := scannedPackage{importPath: importPath}
	add := func(names []string, testFile bool) error {
		for _, name := range names {
			imports, parseErr := parseImports(filepath.Join(dir, name), testFile)
			if parseErr != nil {
				return parseErr
			}
			scanned.imports = append(scanned.imports, imports...)
		}
		return nil
	}
	if pkg != nil {
		if err := add(pkg.GoFiles, false); err != nil {
			return nil, err
		}
		if err := add(pkg.CgoFiles, false); err != nil {
			return nil, err
		}
		if err := add(pkg.TestGoFiles, true); err != nil {
			return nil, err
		}
		if err := add(pkg.XTestGoFiles, true); err != nil {
			return nil, err
		}
		for _, name := range pkg.IgnoredGoFiles {
			ignored, ignoreErr := taggedIgnore(filepath.Join(dir, name))
			if ignoreErr != nil {
				return nil, ignoreErr
			}
			if ignored {
				continue
			}
			// go/build hid this file for the current GOOS/GOARCH. The
			// layer and stdlib-only rules still apply. Test files stay
			// test files so the driver exception matches *_test.go.
			if err := add([]string{name}, strings.HasSuffix(name, "_test.go")); err != nil {
				return nil, err
			}
		}
	}
	if len(scanned.imports) == 0 && pkg != nil && len(pkg.GoFiles)+len(pkg.CgoFiles)+len(pkg.TestGoFiles)+len(pkg.XTestGoFiles)+len(pkg.IgnoredGoFiles) == 0 {
		return nil, nil
	}
	return &scanned, nil
}

func dirHasGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

func parseImports(filename string, testFile bool) ([]scannedImport, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("go/parser %s: %w", filename, err)
	}
	imports := make([]scannedImport, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("import path %s: %w", spec.Path.Value, err)
		}
		pos := fset.Position(spec.Path.Pos())
		imports = append(imports, scannedImport{
			path:     path,
			position: fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line),
			testFile: testFile,
		})
	}
	return imports, nil
}

// taggedIgnore reports whether the file is excluded by a build tag of
// "ignore". Those files are not production code.
func taggedIgnore(path string) (ignored bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		isBuild := strings.HasPrefix(line, "//go:build ") || strings.HasPrefix(line, "// +build ")
		if line == "" || (strings.HasPrefix(line, "//") && !isBuild) {
			continue
		}
		if isBuild {
			fields := strings.FieldsFunc(line, func(r rune) bool {
				switch r {
				case ' ', '\t', '&', '|', '!', '(', ')':
					return true
				default:
					return false
				}
			})
			if slices.Contains(fields, "ignore") {
				return true, nil
			}
			continue
		}
		break
	}
	return false, scanner.Err()
}

func layerViolations(mod module) []string {
	var violations []string
	for _, pkg := range mod.packages {
		if !isDomainPackage(pkg.importPath) {
			continue
		}
		for _, imp := range pkg.imports {
			if layer := forbiddenLayer(imp.path); layer != "" {
				violations = append(violations, fmt.Sprintf("layer rule: domain package %s imports %s at %s", pkg.importPath, layer, imp.position))
				continue
			}
			if thirdParty(imp.path, mod.path) {
				violations = append(violations, fmt.Sprintf("layer rule: domain package %s imports a third-party module %s at %s", pkg.importPath, imp.path, imp.position))
			}
		}
	}
	return violations
}

func stdlibOnlyViolations(mod module) []string {
	var violations []string
	for _, pkg := range mod.packages {
		for _, imp := range pkg.imports {
			if imp.path == "encoding/json" {
				violations = append(violations, fmt.Sprintf("stdlib-only rule: encoding/json v1 imported at %s", imp.position))
			}
			if router := thirdPartyRouter(imp.path); router != "" {
				violations = append(violations, fmt.Sprintf("stdlib-only rule: third-party router %s imported at %s", router, imp.position))
			}
			if !imp.testFile && !isCmdVeylqo(pkg.importPath) && isDriver(imp.path) {
				violations = append(violations, fmt.Sprintf("stdlib-only rule: driver %s imported outside cmd/ at %s (only cmd/veylqo production files may import a driver)", imp.path, imp.position))
			}
		}
	}
	return violations
}

func forbiddenLayer(importPath string) string {
	for _, layer := range forbiddenLayers {
		if pathUnder(importPath, "internal/links/"+layer) {
			return layer
		}
	}
	return ""
}

func isDomainPackage(importPath string) bool {
	return pathUnder(importPath, "internal/links/domain")
}

func isCmdVeylqo(importPath string) bool {
	return pathUnder(importPath, "cmd/veylqo")
}

func pathUnder(importPath, suffix string) bool {
	if importPath == suffix || strings.HasPrefix(importPath, suffix+"/") {
		return true
	}
	slash := "/" + suffix
	return strings.HasSuffix(importPath, slash) || strings.Contains(importPath, slash+"/")
}

func thirdParty(importPath, modulePath string) bool {
	if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
		return false
	}
	if stdlibImport(importPath) {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(first, ".")
}

func stdlibImport(importPath string) bool {
	if importPath == "C" {
		return true
	}
	first, _, _ := strings.Cut(importPath, "/")
	if strings.Contains(first, ".") {
		return false
	}
	pkg, err := build.Default.Import(importPath, "", build.FindOnly)
	if err != nil {
		return false
	}
	return pkg.Goroot
}

func thirdPartyRouter(importPath string) string {
	for _, router := range thirdPartyRouters {
		if importPath == router || strings.HasPrefix(importPath, router+"/") {
			return router
		}
	}
	return ""
}

func isDriver(importPath string) bool {
	for _, driver := range driverModules {
		if importPath == driver || strings.HasPrefix(importPath, driver+"/") {
			return true
		}
	}
	return false
}
