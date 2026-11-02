# Locate or connect the library

Read this only when library access, installation, compatibility or package configuration needs work. Running an offline search does not require installing the package.

Prefer the consuming project's installed `citual-ui` package or its configured checkout. The portable status helper reports the installed path without writes:

```sh
node /path/to/skill/scripts/project.mjs status --cwd /path/to/project
```

Verify the selected source has package name `citual-ui`, the `./foundations` export and `recipes/design-foundation.md`. An older package missing them cannot supply the new foundation. On Dinesh's machine, `/Users/dinesh/workplace/spur-ui` is the current refactored checkout; use it only if it exists and has the expected identity. On other machines, use a configured checkout or existing authenticated access to `git@github.com:ranakdinesh/citual-ui.git`. If access fails, report that the reference could not be consulted and use the project's existing components without inventing Citual APIs.

Until the refactor is committed/released, use the local checkout. The current release tag may not include these changes. For remote adoption, record/pin a reviewed commit or release. Inspect the actual exports as well as version metadata; do not silently upgrade an existing package or skill.

For supported React Next.js/Vite and Tailwind 3/4 projects, read the selected library's `recipes/project-setup.md`. Preview configuration changes before applying them. From a local refactored checkout:

```sh
node /path/to/library/scripts/citual-ui.mjs init --cwd /path/to/project --source /path/to/library --dry-run
node /path/to/library/scripts/citual-ui.mjs init --cwd /path/to/project --source /path/to/library
```

For a matching released package the standalone skill can connect it:

```sh
node /path/to/skill/scripts/project.mjs connect --cwd /path/to/project --dry-run
node /path/to/skill/scripts/project.mjs connect --cwd /path/to/project
```

Connect uses the skill's version-pinned private Git dependency when needed. Local checkout selection uses `--library /path/to/library`. Existing version mismatches stop for an explicit reviewed update. Existing setup markers prevent configuration rewrites. Unsupported CommonJS/custom structures need [manual integration](integration.md), not a forced replacement. Dependency installation may update the package manifest/lock; configuration rollback cannot undo arbitrary npm lifecycle behavior. Preserve unrelated project configuration and instructions. Installing a skill alone does not authorize deployment or provide credentials.

For a new project, scaffold the requested framework before connecting. Preserve non-React applications; use relevant design references and adapt in the native stack instead of migrating just to import Citual. If only recommendations were possible, say so; do not claim implementation or verification.
