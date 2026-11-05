# epcERP

EPC operations application built with Next.js, TypeScript and PostgreSQL.

- `frontend/src/server/`: authentication, master data, tenders, projects, purchasing and platform services.
- `frontend/migrations/`: the complete owned PostgreSQL migration history.
- `frontend/tools/document-processor/`: private document validation, conversion and extraction helpers.
- `deployment/epcerp/`: Docker configuration and operator tools.

Start with [frontend setup](frontend/README.md) or [Docker setup](deployment/epcerp/README.md).
The ERP has no Go application backend, Go worker or local-model service. PostgreSQL,
MinIO object storage and ClamAV remain required infrastructure. MinIO's optional
vendor-image build uses upstream Go source; no ERP Go source is kept in this tree.

The removed implementation remains recoverable in Git history. Optional historical
compatibility checks are documented in [the test guide](frontend/scripts/compatibility/README.md).
Database volumes, documents, credentials and encryption keys are never Git cleanup targets.

The reviewed release target is `main`; combined development work uses
`codex/development`. [Migration/deployment records](deployment/epcerp/history/) are historical evidence.
