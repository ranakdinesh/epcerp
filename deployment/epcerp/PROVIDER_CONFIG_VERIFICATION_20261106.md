# Provider configuration repair — 6 November 2026

Cause: the preceding local frontend recreation used only `.env`, omitting the
existing private `.env.gemini`. The Gemini key and prior cloud approval were
retained on disk but absent from the container. This was deployment configuration
loss; the application still uses the same provider variable names.

Restored the saved private configuration to the existing frontend image on
`codex/development` at application commit `aab8dd3`. All fetched remote branch
tips remain ancestors of this checkout. Existing unrelated concurrency edits
remain in the working tree and are not included in this repair commit.

Verification:

- Local schema remains version 101, dirty=false; no migrations or data reset.
- Frontend image unchanged; database credentials, encryption keys and TSC tender
  and daily budgets compared before/after and preserved.
- No queued/processing TSC tasks or jobs before recreation.
- Authenticated GET of a real draft's TSC options returns HTTP 200,
  `engine=nextjs`, Gemini `available=true`.
- Provider call count unchanged; no document upload, model request or email sent.
- Frontend healthy after recreation.
- Shell syntax, Python compilation and diff whitespace checks passed.
- Wrapper argument checks passed for base-only, Gemini and all optional files,
  including invocation from a different working directory.

`compose.sh` now consistently includes `.env`, optional `.env.gemini`, then
optional `.env.providers`. Setup and provider activation use this wrapper;
operational documentation points to it. No private configuration is committed.

The public server's SSH connection timed out during this verification. Its
provider runtime was not inspected or changed, and the earlier profile release
remains pending there.
