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

func TestTenderSQLAssetsHaveOneOwnerAndNoRuntimeLogic(t *testing.T) {
	root := filepath.Join("..", "modules", "tender_fms", "sql")
	if _, err := os.Stat(filepath.Join("..", "modules", "tender_fms", "adapters", "outbound", "sql")); !os.IsNotExist(err) {
		t.Fatal("Tender query sources must live only under tender_fms/sql/queries")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "migrations", "embed.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range file.Imports {
		v, _ := strconv.Unquote(i.Path.Value)
		if v != "embed" {
			t.Fatalf("Tender SQL provider imports runtime dependency %s", v)
		}
	}
	for _, d := range file.Decls {
		if _, ok := d.(*ast.FuncDecl); ok {
			t.Fatal("Tender SQL provider must contain assets only")
		}
	}
	if !allowed("tender_fms", "sql", "embed") || allowed("tender_fms", "sql", "os") || allowed("tender_fms", "domain", prefix+"tender_fms/sql/migrations") {
		t.Fatal("SQL assets must not weaken Tender core boundaries")
	}
}
