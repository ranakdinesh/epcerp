// Package postgres provisions harness-owned PostgreSQL clusters for Go integration tests.
package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Start accepts no DSN and creates/removes only its own temporary cluster.
func Start(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()
	for _, tool := range []string{"initdb", "pg_ctl", "createdb"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("install PostgreSQL CLI: missing %s", tool)
		}
	}
	base := os.TempDir()
	if info, err := os.Stat("/private/tmp"); err == nil && info.IsDir() {
		base = "/private/tmp"
	}
	dir, err := os.MkdirTemp(base, "epcerp-go-test-")
	if err != nil {
		t.Fatal(err)
	}
	data, sockets := filepath.Join(dir, "data"), filepath.Join(dir, "sockets")
	if err = os.Mkdir(sockets, 0700); err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	started := false
	t.Cleanup(func() {
		if started {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if out, err := exec.CommandContext(ctx, "pg_ctl", "-D", data, "-m", "immediate", "-w", "stop").CombinedOutput(); err != nil {
				t.Errorf("owned PostgreSQL cleanup failed: %s", out)
				return
			}
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("owned fixture cleanup: %v", err)
		}
	})
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// A Windows PostgreSQL server can inherit pg_ctl's output handles.
		// File output lets pg_ctl finish without waiting for the server to close a pipe.
		output, err := os.CreateTemp(dir, "command-*.log")
		if err != nil {
			t.Fatal(err)
		}
		defer output.Close()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout, cmd.Stderr = output, output
		if err := cmd.Run(); err != nil {
			out, _ := os.ReadFile(output.Name())
			t.Fatalf("owned PostgreSQL %s failed: %s", args[0], out)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	run("initdb", "-D", data, "-U", "epcerp_test", "-A", "trust", "--no-locale", "--encoding=UTF8")
	run("pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-o", fmt.Sprintf("-h 127.0.0.1 -p %d -k %s", port, sockets), "-w", "start")
	started = true
	run("createdb", "-h", "127.0.0.1", "-p", strconv.Itoa(port), "-U", "epcerp_test", "epcerp_go_test_native")
	dsn := fmt.Sprintf("postgresql://epcerp_test@127.0.0.1:%d/epcerp_go_test_native?sslmode=disable", port)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal("owned pool unavailable")
	}
	t.Cleanup(pool.Close)
	return dsn, pool
}
