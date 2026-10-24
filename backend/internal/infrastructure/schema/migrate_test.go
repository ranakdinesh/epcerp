package schema

import (
	"context"
	"testing"
)

func TestRejectNonNativeDSNs(t *testing.T) {
	for _, dsn := range []string{"", "postgresql+asyncpg://localhost/client", "postgresql+psycopg://localhost/client", "https://localhost/epcerp"} {
		if _, err := Open(context.Background(), dsn); err == nil {
			t.Fatal("non-PostgreSQL URL accepted")
		}
	}
}
