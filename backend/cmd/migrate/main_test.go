package main

import (
	"testing"
)

func TestRejectUnsafeCommandsBeforeConnecting(t *testing.T) {
	for _, args := range [][]string{nil, {"down"}, {"down", "0"}, {"down", "-1"}, {"down", "7"}, {"drop"}, {"force", "1"}, {"up", "1"}, {"version", "extra"}} {
		if err := run(args, "postgresql://unreachable.invalid/epcerp"); err == nil {
			t.Fatalf("unsafe arguments accepted: %v", args)
		}
	}
	if err := run([]string{"up"}, ""); err == nil {
		t.Fatal("missing independent database URL accepted")
	}
}
