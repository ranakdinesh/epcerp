package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const prefix = "github.com/ranakdinesh/epcERP/internal/modules/"
const loggingPackage = "github.com/ranakdinesh/epcERP/internal/logging"

// Each module owns one set of layers; Profile belongs to Auth.
func owner(parts []string) (module, layer string, layerIndex int) {
	layerIndex = 1
	if len(parts) <= layerIndex {
		return "", "", layerIndex
	}
	return strings.Join(parts[:layerIndex], "/"), parts[layerIndex], layerIndex
}

func allowed(module, layer, dependency string) bool {
	own := prefix + module + "/"
	within := func(layer string) bool {
		return dependency == own+layer || strings.HasPrefix(dependency, own+layer+"/")
	}
	// Shared identity is a pure value package approved for core layers.
	if dependency == "github.com/ranakdinesh/epcERP/internal/identity" {
		return true
	}
	// Adapters may emit through the application-owned contract, never Zerolog.
	if dependency == loggingPackage {
		return layer == "adapters"
	}
	// Pure shared value/image helpers are explicitly approved; infrastructure remains forbidden.
	if dependency == "github.com/ranakdinesh/epcERP/helpers" && (layer == "domain" || layer == "application" || layer == "adapters") {
		return true
	}
	switch layer {
	case "sql":
		// Module SQL providers contain immutable embedded assets, never runtime logic.
		return (module == "auth" || module == "tender_fms") && dependency == "embed"
	case "domain":
		if within("domain") {
			return true
		}
		for _, value := range []string{"errors", "fmt", "strings", "strconv", "time", "math", "math/big", "sort", "slices", "unicode", "unicode/utf8", "regexp", "cmp"} {
			if value == dependency {
				return true
			}
		}
	case "ports", "contracts":
		return dependency == "context" || within("domain")
	case "application":
		return dependency == "context" || dependency == "errors" || within("domain") || within("ports")
	case "adapters":
		if strings.HasPrefix(dependency, prefix) {
			parts := strings.Split(strings.TrimPrefix(dependency, prefix), "/")
			dependencyOwner, dependencyLayer, _ := owner(parts)
			return dependencyOwner == module || dependencyLayer == "contracts"
		}
		return !strings.HasPrefix(dependency, "github.com/ranakdinesh/epcERP/") || dependency == "github.com/ranakdinesh/epcERP/internal/infrastructure" || strings.HasPrefix(dependency, "github.com/ranakdinesh/epcERP/internal/infrastructure/")
	}
	return false
}

// Parse imports with Go's AST, including test files and build-tagged sources.
func TestModuleBoundaries(t *testing.T) {
	root := filepath.Join("..", "modules")
	modules := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 2 && parts[1] == "module.go" {
			return nil
		}
		if len(parts) >= 3 && parts[1] == "composition" {
			return nil
		}
		if len(parts) < 3 {
			t.Errorf("%s: code must belong to a module layer", path)
			return nil
		}
		module, layer, layerIndex := owner(parts)
		if modules[module] == nil {
			modules[module] = map[string]bool{}
		}
		if !strings.HasSuffix(path, "_test.go") {
			modules[module][layer] = true
		}
		if module != "master" && module != "auth" && module != "purchase_fms" && layer == "adapters" && (len(parts) < layerIndex+3 || (parts[layerIndex+1] != "inbound" && parts[layerIndex+1] != "outbound")) {
			t.Errorf("%s: adapter must be inbound or outbound", path)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, "_test.go") && dependency == "testing" {
				continue
			}
			if !allowed(module, layer, dependency) {
				t.Errorf("%s: forbidden import %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) == 0 {
		t.Fatal("no modules checked")
	}
	for module, layers := range modules {
		for _, layer := range []string{"domain", "ports", "application", "adapters"} {
			if !layers[layer] {
				t.Errorf("%s: missing implemented %s layer", module, layer)
			}
		}
	}
}

func TestModuleCompositionFacades(t *testing.T) {
	root := filepath.Join("..", "modules")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	application, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "app", "app.go"), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	for _, imported := range application.Imports {
		dependency, _ := strconv.Unquote(imported.Path.Value)
		bound[dependency] = true
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "module.go")
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("%s: every module requires a module.go composition facade: %v", entry.Name(), err)
			continue
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if dependency == "github.com/ranakdinesh/epcERP/internal/app" {
				t.Errorf("%s: modules must not depend on the core application", path)
			}
		}
		if !bound[prefix+entry.Name()] {
			t.Errorf("%s: module facade must be bound by internal/app/app.go", entry.Name())
		}
	}
}

func TestAPICommandRemainsAThinEntrypoint(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "api", "main.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if lines := len(strings.Split(strings.TrimSpace(string(source)), "\n")); lines > 30 {
		t.Errorf("%s: command entrypoint has %d lines; module binding belongs in internal/app", path, lines)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range parsed.Imports {
		dependency, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(dependency, prefix) {
			t.Errorf("%s: command must bind only the core application, not module internals: %s", path, dependency)
		}
	}
}

func TestBoundaryPolicy(t *testing.T) {
	for _, test := range []struct {
		layer, dependency string
		want              bool
	}{
		{"domain", "github.com/ranakdinesh/epcERP/helpers", true},
		{"domain", "github.com/ranakdinesh/epcERP/helpers/database", false},
		{"adapters", "github.com/ranakdinesh/epcERP/helpers", true},
		{"domain", "net/http", false}, {"domain", "database/sql", false},
		{"domain", "github.com/jackc/pgx/v5", false},
		{"domain", "github.com/rs/zerolog", false},
		{"ports", "github.com/rs/zerolog", false},
		{"application", "github.com/rs/zerolog", false},
		{"domain", "github.com/ranakdinesh/epcERP/internal/infrastructure/database", false},
		{"ports", "github.com/ranakdinesh/epcERP/internal/infrastructure/database", false},
		{"application", "github.com/ranakdinesh/epcERP/internal/infrastructure/database", false},
		{"adapters", "github.com/ranakdinesh/epcERP/internal/infrastructure/database", true},
		{"adapters", loggingPackage, true},
		{"application", loggingPackage, false},
		{"adapters", "github.com/ranakdinesh/epcERP/internal/app", false},
		{"application", "github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware", false},
		{"domain", prefix + "other/domain", false},
		{"ports", prefix + "health/application", false},
		{"application", prefix + "health/adapters/outbound", false},
		{"application", prefix + "auth/application", false},
		{"application", "github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware", false},
		{"adapters", prefix + "auth/adapters/repository", false},
		{"adapters", prefix + "auth/application", false},
		{"adapters", prefix + "auth/contracts", true},
		{"adapters", prefix + "master/contracts", true},
		{"adapters", prefix + "master/contracts", true},
		{"adapters", prefix + "master/adapters", false},
		{"application", prefix + "master/contracts", false},
		{"adapters", prefix + "master/domain", false},
		{"domain", prefix + "master/domain", false},
		{"ports", prefix + "master/adapters", false},
		{"adapters", prefix + "master/adapters", false},
		{"application", prefix + "master/contracts", false},
		{"application", prefix + "health/ports", true},
		{"domain", prefix + "health/domain_fake", false},
		{"domain", "time", true},
	} {
		if got := allowed("health", test.layer, test.dependency); got != test.want {
			t.Errorf("%s -> %s: got %v, want %v", test.layer, test.dependency, got, test.want)
		}
	}
}

func TestZerologIsPrivateToLoggerAdapter(t *testing.T) {
	root := filepath.Join("..", "..")
	allowedPath := filepath.Join(root, "internal", "infrastructure", "logger.go")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if dependency == "github.com/rs/zerolog" && path != allowedPath {
				t.Errorf("%s: Zerolog must remain private to internal/infrastructure/logger.go", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Runtime infrastructure must never coordinate capability business rules.
func TestInfrastructureHasNoBusinessImports(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "infrastructure"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			// The platform auth bridge consumes only Auth's published identity
			// contract; other infrastructure cannot import capability modules.
			platformIdentity := (filepath.Base(path) == "auth.go" || filepath.Base(path) == "auth_test.go") && dependency == prefix+"auth/contracts"
			if strings.HasPrefix(dependency, prefix) && !platformIdentity {
				t.Errorf("%s: infrastructure cannot import business module %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthProfileBoundaries(t *testing.T) {
	for _, check := range []struct {
		layer, dependency string
		want              bool
	}{
		{"domain", prefix + "auth/domain", true},
		{"application", prefix + "auth/ports", true},
		{"application", prefix + "auth/adapters/repository", false},
		{"application", "github.com/ranakdinesh/epcERP/internal/infrastructure", false},
		{"adapters", prefix + "auth/application", true},
	} {
		if allowed("auth", check.layer, check.dependency) != check.want {
			t.Errorf("unexpected boundary: %s -> %s", check.layer, check.dependency)
		}
	}
}

func TestMasterModuleBoundaries(t *testing.T) {
	for _, check := range []struct {
		layer, dependency string
		want              bool
	}{
		{"domain", prefix + "master/domain", true},
		{"domain", prefix + "master/adapters/repository/sqlc", false},
		{"domain", "net/http", false},
		{"application", prefix + "master/ports", true},
		{"application", prefix + "master/adapters", false},
		{"application", "github.com/jackc/pgx/v5", false},
		{"adapters", prefix + "master/application", true},
		{"adapters", prefix + "auth/adapters/repository", false},
	} {
		if got := allowed("master", check.layer, check.dependency); got != check.want {
			t.Errorf("%s -> %s: got %v", check.layer, check.dependency, got)
		}
	}
}

// The approved helper dependency must never pull business modules or infrastructure
// into domain code. This package is restricted to pure standard-library value/image logic.
func TestHelpersRemainPure(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "..", "helpers"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if dependency != "bytes" && dependency != "image" && dependency != "image/color" && dependency != "image/jpeg" && dependency != "image/png" && dependency != "errors" && dependency != "regexp" && dependency != "strings" && dependency != "unicode" && !(strings.HasSuffix(path, "_test.go") && dependency == "testing") {
				t.Errorf("%s: helpers must remain pure value/image functions: %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMasterDomainHasNoTransportFields(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "modules", "master", "domain"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if field, ok := n.(*ast.Field); ok && field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
				t.Errorf("%s: JSON field belongs in an adapter DTO", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Master persistence owns generated queries; HTTP and core layers never use them.
func TestMasterRepositoryBoundaries(t *testing.T) {
	root := filepath.Join("..", "modules", "master", "adapters")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		inRepository := strings.HasPrefix(relative, "repository"+string(filepath.Separator))
		inDTO := strings.HasPrefix(relative, filepath.Join("repository", "dtos")+string(filepath.Separator))
		inHTTP := strings.HasPrefix(relative, "httpx"+string(filepath.Separator))
		inHTTPDTO := strings.HasPrefix(relative, filepath.Join("httpx", "dtos")+string(filepath.Separator))
		queryAliases := map[string]bool{}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if inHTTP && (dependency == prefix+"master/adapters/repository" || strings.HasPrefix(dependency, prefix+"master/adapters/repository/")) {
				t.Errorf("%s: HTTP handlers/DTOs must use domain and application values", path)
			}
			if inHTTP && (strings.HasPrefix(dependency, "github.com/jackc/") || dependency == "database/sql" || dependency == "github.com/ranakdinesh/epcERP/internal/infrastructure/database") {
				t.Errorf("%s: HTTP adapters must not depend on database connections or drivers", path)
			}
			if inHTTPDTO && (strings.Contains(dependency, "/adapters/") || dependency == "net/http" || strings.HasPrefix(dependency, "github.com/jackc/")) {
				t.Errorf("%s: HTTP DTOs must not depend on handlers or persistence", path)
			}
			if dependency == prefix+"master/adapters/repository/sqlc" {
				if !inRepository {
					t.Errorf("%s: generated SQLC types must stay in repositories", path)
				}
				alias := "sqlc"
				if imported.Name != nil {
					alias = imported.Name.Name
				}
				queryAliases[alias] = true
			}
			if inRepository && (dependency == prefix+"master/adapters/httpx" || strings.HasPrefix(dependency, prefix+"master/adapters/httpx/")) {
				t.Errorf("%s: repository must not depend on HTTP adapters", path)
			}
			if inDTO && (dependency == "net/http" || dependency == prefix+"master/adapters/repository") {
				t.Errorf("%s: DTOs only translate values", path)
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			if inHTTP && !inHTTPDTO {
				if field, ok := node.(*ast.Field); ok && field.Tag != nil {
					tag, _ := strconv.Unquote(field.Tag.Value)
					if strings.Contains(tag, "json:") {
						t.Errorf("%s: HTTP exchange types belong in httpx/dtos", path)
					}
				}
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name, ok := selector.X.(*ast.Ident)
			if ok && queryAliases[name.Name] && selector.Sel.Name == "New" && relative != filepath.Join("repository", "store.go") {
				t.Errorf("%s: initialize SQLC queries in repository/store.go", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSharedDocumentInfrastructureHasNoModuleDependencies(t *testing.T) {
	for _, name := range []string{"document_cipher.go", "document_media.go"} {
		path := filepath.Join("..", "infrastructure", name)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(dependency, ".") {
				t.Errorf("%s: document infrastructure must be independent of business modules: %s", path, dependency)
			}
		}
	}
}

func TestIdentityRemainsInfrastructureFree(t *testing.T) {
	root := filepath.Join("..", "identity")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if strings.Contains(dependency, ".") || strings.HasPrefix(dependency, "net/") || strings.HasPrefix(dependency, "database/") {
				t.Errorf("%s: identity values must remain infrastructure-free: %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, layer := range []string{"domain", "ports", "application"} {
		if !allowed("master", layer, "github.com/ranakdinesh/epcERP/internal/identity") {
			t.Errorf("%s must accept shared identity values", layer)
		}
		if allowed("master", layer, "github.com/ranakdinesh/epcERP/internal/infrastructure") {
			t.Errorf("%s must reject authentication infrastructure", layer)
		}
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "infrastructure", "auth.go"), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range parsed.Imports {
		dependency, _ := strconv.Unquote(imported.Path.Value)
		if strings.HasPrefix(dependency, prefix) && dependency != prefix+"auth/contracts" {
			t.Errorf("shared auth adapter must only consume Auth's published contract: %s", dependency)
		}
	}
}
