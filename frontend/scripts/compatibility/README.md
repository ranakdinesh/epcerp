# Optional historical compatibility tests

These are migration parity checks, not the normal application build/test path.
They require Go only to run the frozen reference at commit
`a18ae634c00fd194fe00ed7bbe9f303bad9b39b9`. Each script extracts `backend/` from
that Git commit into a private temporary directory and removes that directory on
exit. No Go application source or runtime is kept in the current working tree.
A full clone containing that commit is needed (fetch history if using a shallow clone).

Run a `verify-*.sh` script here only with the isolated QA database prefix it
requires. The active Node migration CLI creates the schema. Cloud and email are
disabled except provider tests with injected synthetic responses. Never use an
operational database. These checks preserve the original independent evidence
for password/encryption compatibility and detailed purchasing workflows.

Normal frontend tests, native integration checks, builds, document processing
and deployments use Node/TypeScript and do not require Go.
