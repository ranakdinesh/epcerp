package config

import (
	"testing"
)

func TestExplicitContainerListener(t *testing.T) {
	s, e := load(nil)
	if e != nil || s.HTTPAddress != "127.0.0.1:8001" {
		t.Fatal("default listener drift")
	}
	s, e = load(map[string]string{"EPCERP_HTTP_ADDRESS": "0.0.0.0:8001"})
	if e != nil || s.HTTPAddress != "0.0.0.0:8001" {
		t.Fatal("container listener rejected", e)
	}
	for _, address := range []string{"", ":8001", "localhost:8001", "0.0.0.0:0", "0.0.0.0:65536", "http://0.0.0.0:8001"} {
		if _, e = load(map[string]string{"EPCERP_HTTP_ADDRESS": address}); e == nil {
			t.Fatal("invalid listener accepted")
		}
	}
}
