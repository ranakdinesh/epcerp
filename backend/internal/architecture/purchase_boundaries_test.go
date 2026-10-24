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

func TestPurchaseIsOneModule(t *testing.T) {
	retired := []string{"client_purchase_rules", "group_quotations", "quotation_approvals", "purchase_bo", "purchase_holds", "purchase_indents", "purchase_indent_groups", "purchase_indent_documents", "purchase_workflow", "purchase_sourcing", "purchase_technical", "purchase_orders", "purchase_acceptance", "purchase_advances", "purchase_inspections", "purchase_inspection_reports", "purchase_quality", "purchase_client_clearance", "purchase_dispatch", "purchase_site_receipts", "purchase_grns", "purchase_engineer_confirmations", "purchase_progress"}
	for _, module := range retired {
		if _, e := os.Stat(filepath.Join("..", "modules", module)); !os.IsNotExist(e) {
			t.Errorf("retired purchase module remains: %s", module)
		}
	}
	e := filepath.WalkDir("..", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if e != nil {
			return e
		}
		for _, im := range f.Imports {
			v, _ := strconv.Unquote(im.Path.Value)
			for _, old := range retired {
				if strings.HasPrefix(v, prefix+old+"/") {
					t.Errorf("%s imports retired %s", path, old)
				}
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
func TestPurchaseAdapters(t *testing.T) {
	root := filepath.Join("..", "modules", "purchase_fms")
	e := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if parts[0] != "adapters" {
			return nil
		}
		if len(parts) < 3 || (parts[1] != "httpx" && parts[1] != "repository") {
			t.Errorf("%s must use flat purchase adapters", path)
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			return e
		}
		inHTTP := parts[1] == "httpx"
		inDTO := len(parts) > 3 && parts[2] == "dtos"
		inSQLC := len(parts) > 3 && parts[2] == "sqlc"
		queryAliases := map[string]bool{}
		for _, im := range f.Imports {
			v, _ := strconv.Unquote(im.Path.Value)
			if inHTTP && (strings.HasPrefix(v, "github.com/jackc/") || v == "database/sql" || strings.HasPrefix(v, prefix+"purchase_fms/adapters/repository") || strings.Contains(v, "/infrastructure/database")) {
				t.Errorf("%s: HTTP must receive services, not database/repositories", path)
			}
			if inDTO && inHTTP && (v == "net/http" || strings.Contains(v, "/adapters/")) {
				t.Errorf("%s: HTTP DTOs return values/errors", path)
			}
			if !inHTTP && strings.HasPrefix(v, prefix+"purchase_fms/adapters/httpx") {
				t.Errorf("%s: persistence must not import HTTP", path)
			}
			if v == prefix+"purchase_fms/adapters/repository/sqlc" {
				alias := "sqlc"
				if im.Name != nil {
					alias = im.Name.Name
				}
				queryAliases[alias] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && !inSQLC {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "New" {
					if id, ok := sel.X.(*ast.Ident); ok && queryAliases[id.Name] && filepath.ToSlash(rel) != "adapters/repository/store.go" {
						t.Errorf("%s: only store.go initializes SQLC", path)
					}
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
