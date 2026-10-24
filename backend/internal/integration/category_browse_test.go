package integration

import (
	"context"
	"github.com/go-chi/chi/v5"
	authInfrastructure "github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	eventOut "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/adapters/outbound"
	eventApp "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/application"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"

	masterHTTP "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	masterRepository "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	masterApplication "github.com/ranakdinesh/epcERP/internal/modules/master/application"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"net/http/httptest"
	"testing"
)

func TestCategoryPagedBrowse(t *testing.T) {
	dsn, pool := owned(t)
	apply(t, dsn)
	ctx := context.Background()
	execFixture(t, pool, `INSERT INTO catalogue_units(id,code,name) VALUES('00000000-0000-4000-8000-000000000001','M','Meter')`)
	execFixture(t, pool, `INSERT INTO categories(id,code,name,unit_id) SELECT md5('browse-cat-'||i)::uuid,'CAT'||lpad(i::text,4,'0'),'Category '||lpad(i::text,4,'0'),'00000000-0000-4000-8000-000000000001'::uuid FROM generate_series(1,1000) i`)
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration,is_active) SELECT md5('browse-child-'||i)::uuid,'TYPE'||lpad(i::text,4,'0'),'Type '||lpad(i::text,4,'0'),md5('browse-cat-1')::uuid,'00000000-0000-4000-8000-000000000001'::uuid,'{"fields":[],"revision":1}'::jsonb,i%2=0 FROM generate_series(1,1000) i`)
	execFixture(t, pool, `INSERT INTO category_subcategories(id,code,name,category_id,unit_id,configuration) VALUES(md5('other-child')::uuid,'OTHER','Other child',md5('browse-cat-2')::uuid,'00000000-0000-4000-8000-000000000001','{"fields":[],"revision":1}')`)
	var parentID, childID string
	if e := pool.QueryRow(ctx, `SELECT id::text FROM categories WHERE code='CAT0001'`).Scan(&parentID); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `SELECT id::text FROM category_subcategories WHERE code='TYPE0001'`).Scan(&childID); e != nil {
		t.Fatal(e)
	}
	identities := projectIdentities{"reader": auth.Identity{ID: "fixture", Active: true, Permissions: []string{"settings.categories.read"}}, "denied": auth.Identity{ID: "fixture", Active: true}}
	journal := func(c database.Connection) events.Writer { return eventApp.NewRecorder(eventOut.NewJournal(c)) }
	service := masterApplication.NewCategoryService(masterRepository.NewCategoryUnitOfWork(pool, journal), authInfrastructure.NewAuthBridge(identities), masterRepository.CategoryValues{})
	q := masterDomain.CategoryBrowse{Kind: "category", Page: 1, Size: 20, Sort: "name", Direction: "asc"}
	page, e := service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 1000 || len(page.Rows) != 20 || page.Rows[0].Code != "CAT0001" || page.Rows[0].ChildCount != 1000 || page.Rows[0].UnitName != "Meter" {
		t.Fatalf("category page: %+v %v", page, e)
	}
	q.Page = 2
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Rows[0].Code != "CAT0021" {
		t.Fatal("page boundary", e)
	}
	q.Kind = "subcategory"
	q.CategoryID = parentID
	q.Page = 1
	q.Size = 100
	q.Direction = "desc"
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 1000 || len(page.Rows) != 100 || page.Rows[0].Code != "TYPE1000" || page.Rows[0].Configuration != nil {
		t.Fatal("child page or eager configuration", e)
	}
	for _, row := range page.Rows {
		if row.CategoryID != parentID {
			t.Fatal("leaked other parent")
		}
	}
	q.Status = "Inactive"
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 500 || page.Rows[0].Active {
		t.Fatal("server status filter", e)
	}
	q.Search = "Type 0001"
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 1 || page.Rows[0].Code != "TYPE0001" {
		t.Fatal("search", e)
	}
	q.Search = "%"
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 0 {
		t.Fatal("search wildcard not literal", e)
	}
	q.Search = ""
	q.Page = 100
	page, e = service.Browse(ctx, "reader", q)
	if e != nil || page.Total != 500 || len(page.Rows) != 0 {
		t.Fatal("empty page loses total", e)
	}
	detail, e := service.Detail(ctx, "reader", "subcategory", childID)
	if e != nil || detail.Configuration == nil || detail.Code != "TYPE0001" {
		t.Fatal("lazy detail", e)
	}
	if _, e = service.Browse(ctx, "denied", q); e == nil {
		t.Fatal("browse lacks auth")
	}
	if _, e = service.Detail(ctx, "denied", "subcategory", childID); e == nil {
		t.Fatal("detail lacks auth")
	}
	router := chi.NewRouter()
	masterHTTP.RegisterCategoryRoutes(router, service)
	for _, path := range []string{"/api/v1/categories/browse?kind=subcategory", "/api/v1/categories/browse?size=1000", "/api/v1/categories/browse?page=0", "/api/v1/categories/browse?sort=sql", "/api/v1/categories/browse?size=20&size=50", "/api/v1/categories/browse?unknown=1"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer reader")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 422 {
			t.Fatal(path, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/api/v1/categories/browse?kind=subcategory&category_id="+parentID+"&size=20", nil)
	req.Header.Set("Authorization", "Bearer reader")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var payload masterDomain.CategoryPage
	if e = masterDTO.UnmarshalJSON(w.Body.Bytes(), &payload); e != nil || w.Code != 200 || payload.Total != 1000 || len(payload.Rows) != 20 {
		t.Fatal("http browse", w.Code, e)
	}
	req = httptest.NewRequest("GET", "/api/v1/categories/detail/subcategory/00000000-0000-4000-8000-000000000099", nil)
	req.Header.Set("Authorization", "Bearer reader")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatal("missing detail", w.Code)
	}
}
