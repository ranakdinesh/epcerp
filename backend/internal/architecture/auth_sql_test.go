package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAuthSQLAssetsCannotContainRuntimeLogic(t *testing.T) {
	root := filepath.Join("..", "modules", "auth", "sql")
	if _, err := os.Stat(filepath.Join("..", "modules", "auth", "adapters", "sql")); !os.IsNotExist(err) {
		t.Fatal("Auth queries must have one owner under auth/sql/queries")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "migrations", "embed.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		name, _ := strconv.Unquote(imported.Path.Value)
		if name != "embed" {
			t.Fatalf("migration asset provider imports executable dependency %s", name)
		}
	}
	for _, declaration := range file.Decls {
		if _, ok := declaration.(*ast.FuncDecl); ok {
			t.Fatal("migration asset provider must not execute runtime or business logic")
		}
	}
	if !allowed("auth", "sql", "embed") || allowed("auth", "sql", "os") || allowed("auth", "domain", prefix+"auth/sql/migrations") {
		t.Fatal("SQL assets must not relax core architecture boundaries")
	}
}
