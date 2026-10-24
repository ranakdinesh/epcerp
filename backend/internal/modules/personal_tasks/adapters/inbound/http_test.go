package inbound

import (
	"net/http/httptest"
	"testing"
)

func TestStrictPersonalQuery(t *testing.T) {
	for _, url := range []string{"?user_id=other", "?page=1&page=2", "?page=0", "?page_size=21", "?state=complete", "?role=manager", "?page=100001"} {
		if _, e := Query(httptest.NewRequest("GET", "/"+url, nil)); e == nil {
			t.Fatal("accepted", url)
		}
	}
	q, e := Query(httptest.NewRequest("GET", "/?page=2&page_size=50&role=resolver&state=blocked&search=cable", nil))
	if e != nil || q.Page != 2 || q.Size != 50 || q.Role != "resolver" {
		t.Fatal(q, e)
	}
}
