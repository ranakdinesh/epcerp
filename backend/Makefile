SQLC_VERSION := v1.30.0

.PHONY: generate check-sqlc check test build verify run-local migrate-local-up migrate-local-version migrate-up migrate-version migrate-down

check-sqlc:
	@test "$$(sqlc version)" = "$(SQLC_VERSION)" || (echo "Use sqlc $(SQLC_VERSION) for reproducible generation"; exit 1)

generate: check-sqlc
	sqlc generate

check: check-sqlc
	sqlc vet
	sqlc diff
	@test -z "$$(gofmt -l cmd internal sql/migrations helpers)" || (echo "Run gofmt on cmd, internal, sql/migrations and helpers"; exit 1)
	go vet ./...
	go test -race ./...

test:
	go test -race ./...

build:
	go build -o /tmp/epcERP-api ./cmd/api
	go build -o /tmp/epcERP-migrate ./cmd/migrate
	go build -o /tmp/epcERP-supplier-document-storage ./cmd/supplier-document-storage
	go build -o /tmp/epcERP-document-worker ./cmd/document-worker
	go build -o /tmp/epcERP-document-package-worker ./cmd/document-package-worker
	go build -o /tmp/epcERP-document-reader ./cmd/document-reader
	go build -o /tmp/epcERP-tsc-worker ./cmd/tsc-worker

run-local:
	./run-local.sh api

migrate-local-up:
	./run-local.sh migrate-up

migrate-local-version:
	./run-local.sh migrate-version

# Provisions and cleans its own PostgreSQL cluster. No external DSN or Python.
verify:
	EPCERP_GO_INTEGRATION=1 go test -race -count=1 -timeout=5m ./internal/integration ./internal/infrastructure/database ./internal/infrastructure/schema

migrate-up:
	go run ./cmd/migrate up

migrate-version:
	go run ./cmd/migrate version

# Explicit bounded rollback only; no default down-all, drop, or force operation.
migrate-down:
	@test -n "$(STEPS)" || (echo "Set STEPS to a positive migration count"; exit 1)
	go run ./cmd/migrate down $(STEPS)
