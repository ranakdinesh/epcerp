# Current system baseline — local TSC increment

29 October 2026. This entry inventories the local TSC increment in this independent
epcERP checkout; it is not a replacement inventory for all historical modules.

- codex/development combines current main/Auth baseline and Tender FMS branch.
- Go migration 94 adds local TSC job mode; immutable snapshots/history and source
  access remain owned by Tender and the reusable document services.
- Go deterministic labelled-fact extraction, local MiniLM reranking, selected-only
  model inputs, prepared per-question source evidence and review UI are implemented.
- Pinned offline INT8 model runs in a private CPU-limited Docker service.
- Native tests, synthetic browser checks and actual local-model integration passed.
- Local Docker was upgraded with a verified database backup. Existing user, role
  and client counts were preserved. No operational data or staff were seeded.

See GO_TSC_LOCAL_RAG_SLICE.md for exact scope, verification and limitations.
Real-tender quality, target-PC capacity, company matching, complete clause/table
understanding, final approval and actual model billing measurement remain unfinished.
Next task: annotate representative real tender packages and benchmark held-out
clause coverage, critical errors, reviewer effort, actual usage and target memory.
