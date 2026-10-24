# Tender SQL assets

`queries/` is the only handwritten Tender sqlc query source. Runtime references use
the `tender` schema explicitly; generated Go code stays in
`adapters/outbound/internal/db/` relative to the module root.

`migrations/` owns unchanged historical pairs 82, 85 and 87, plus migration 90
which relocates the twelve Tender tables. Its Go provider imports only `embed`
and contains no functions. The platform `backend/sql/migrations` registry combines
these assets with platform and Auth providers in one global version order.

Mixed/shared applied histories 21, 86 and 88 remain platform-owned and unchanged.
The global `backend/sqlc.yaml` manifest must list every up migration in order and
match the embedded bytes. Do not introduce an independent Tender version table,
per-module migration command or pooled search_path override.

See the parent module README for the table inventory, dependencies and upgrade gates.
