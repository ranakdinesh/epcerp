package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Auth owns Profile in one set of layers, with explicit adapter roles.
func TestAuthAdapterBoundaries(t *testing.T) {
	for _, module := range []string{"auth"} {
		root := filepath.Join("..", "modules", filepath.FromSlash(module))
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
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			inHTTP := len(parts) > 2 && parts[0] == "adapters" && parts[1] == "httpx"
			inRepository := len(parts) > 2 && parts[0] == "adapters" && parts[1] == "repository"
			inDTO := len(parts) > 3 && parts[2] == "dtos"
			inGenerated := inRepository && parts[2] == "sqlc"
			if parts[0] == "adapters" {
				valid := parts[1] == "httpx" || parts[1] == "repository" || (module == "auth" && (parts[1] == "security" || parts[1] == "identity" || parts[1] == "jobs"))
				if !valid {
					t.Errorf("%s: unrecognized Auth adapter responsibility", path)
				}
			}
			queries := map[string]bool{}
			for _, imported := range file.Imports {
				dependency, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				own := prefix + module + "/adapters/"
				if (inHTTP || (len(parts) > 2 && parts[0] == "adapters" && parts[1] == "jobs")) && (strings.HasPrefix(dependency, "github.com/jackc/") || dependency == "database/sql" || strings.HasPrefix(dependency, "github.com/ranakdinesh/epcERP/internal/infrastructure/database") || strings.HasPrefix(dependency, own+"repository") || strings.HasPrefix(dependency, own+"security") || strings.HasPrefix(dependency, own+"media")) {
					t.Errorf("%s: HTTP adapters must receive application services, not persistence/security implementations", path)
				}
				if inHTTP && inDTO && (dependency == "net/http" || strings.Contains(dependency, "/adapters/")) {
					t.Errorf("%s: HTTP DTOs translate pure values and return errors", path)
				}
				if inRepository && strings.HasPrefix(dependency, own+"httpx") {
					t.Errorf("%s: persistence must not depend on HTTP", path)
				}
				if dependency == own+"repository/sqlc" {
					if !inRepository {
						t.Errorf("%s: generated SQLC values belong in persistence", path)
					}
					alias := "sqlc"
					if imported.Name != nil {
						alias = imported.Name.Name
					}
					queries[alias] = true
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if field, ok := node.(*ast.Field); ok && field.Tag != nil && !strings.HasSuffix(path, "_test.go") && (parts[0] == "domain" || (inHTTP && !inDTO)) {
					tag, _ := strconv.Unquote(field.Tag.Value)
					if strings.Contains(tag, "json:") {
						t.Errorf("%s: JSON fields belong in HTTP DTOs", path)
					}
				}
				if call, ok := node.(*ast.CallExpr); ok && !inGenerated {
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "New" {
						if id, ok := selector.X.(*ast.Ident); ok && queries[id.Name] && relative != filepath.Join("adapters", "repository", "store.go") {
							t.Errorf("%s: only repository/store.go initializes SQLC queries", path)
						}
					}
				}
				if parts[0] == "domain" {
					if spec, ok := node.(*ast.TypeSpec); ok && spec.Name.Name == "Error" {
						if fields, ok := spec.Type.(*ast.StructType); ok {
							for _, field := range fields.Fields.List {
								for _, name := range field.Names {
									if name.Name == "Code" || name.Name == "Status" {
										t.Errorf("%s: domain errors must use semantic kinds", path)
									}
								}
							}
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Shared email transport may never grow a dependency on a business module, and
// modules must not embed a second Brevo HTTP integration.
func TestEmailTransportIsCentralized(t *testing.T) {
	file, e := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "infrastructure", "email.go"), nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, imported := range file.Imports {
		dependency, _ := strconv.Unquote(imported.Path.Value)
		if strings.HasPrefix(dependency, prefix) {
			t.Fatal("shared email imports a business module")
		}
	}
	e = filepath.WalkDir(filepath.Join("..", "modules"), func(path string, entry fs.DirEntry, err error) error {
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
			if literal, ok := n.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				value, _ := strconv.Unquote(literal.Value)
				if strings.Contains(value, "api.brevo.com") {
					t.Errorf("%s: Brevo transport belongs in shared infrastructure", path)
				}
			}
			return true
		})
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
