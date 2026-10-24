# Auth SQL ownership

- `migrations/`: ten historical Auth migration pairs and the forward schema move
  at version 89. Original SQL/version numbers are preserved; the platform runner
  combines these embedded assets with the remaining global history.
- `queries/`: the seven Auth sqlc query files. Runtime table references are schema
  qualified, e.g. `auth.users` and `auth.auth_departments`.

Generated pgx code stays in `adapters/repository/sqlc/`; SQLC-to-domain conversion
stays in repository DTOs. Domain/application layers never depend on SQL assets.
Run generation from `backend/` using pinned sqlc v1.30.0 and the central sqlc.yaml.

The original migration 1 includes the platform ownership marker for historical
reasons. That marker stays in public. Do not split, renumber or rewrite applied
versions to make the file layout appear cleaner. New schema changes append a
globally unique version to the coordinated history and sqlc schema manifest.
