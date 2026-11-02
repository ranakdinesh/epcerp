---
name: citual-ui
description: Build or revise web interfaces using Citual UI foundations and workflows. Use for page layout, visual styling, UI controls and screen composition in projects adopting Citual UI, or when explicitly requested. Skip backend-only work, content-only edits and unrelated configuration or tests.
---

# Citual UI

Use this repository as a design/component reference while preserving the consuming project's brand, framework, existing primitives and data layer. Implement requested UI work; retrieval alone is not completion.

Read the relevant project rules, route/component, and theme first. Resolve this skill's directory from this file's location; helper paths below are relative to it. Read only the guidance needed for the current task:

- For styling or first-time foundation mapping: [theming](references/theming.md).
- If the library is missing, outdated or needs connecting: [library access/setup](references/library-access.md). Skip setup when the existing integration suffices.
- For a new screen or substantial composition: [screen workflow](references/build-screen.md). Small edits need only the affected component's source/type contract.
- For installing or invoking the skill itself: [agent compatibility](references/agents.md).

Search narrowly instead of loading the catalog or reference folder into context:

```sh
node /path/to/skill/scripts/search.mjs search --q "<UI intent>" --generic --limit 3
node /path/to/skill/scripts/search.mjs show --id <selected-id>
```

Use `--catalog /path/to/library/registry/agent-catalog.json` when the installed library differs from the bundled snapshot. Verify real exports before importing. Template origins belong in metadata; demo storage and backend behavior are not production integrations.

Verify the changed flow with relevant type/build checks and browser interaction/layout checks. Report the implementation, checks and material limitations. Read further references only if the task requires them.
