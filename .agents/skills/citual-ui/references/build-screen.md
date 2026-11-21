# Compose and implement a screen

Read this for a new screen or substantial screen composition. For a small style/control edit, inspect just the relevant component instead.

## Retrieve a screen design, then its components

```sh
node /path/to/citual-ui-skill/scripts/plan.mjs --q "Build an analytics dashboard with metrics and recent activity"
node /path/to/citual-ui-skill/scripts/search.mjs search --q "sidebar admin shell" --kind layout --limit 2
node /path/to/citual-ui-skill/scripts/search.mjs show --id record-manager
```

`plan` returns a screen composition, visual layout candidates, interactive candidates, exact imports, source/recipe paths, limitations and alternative patterns. The eight recipes cover dashboards, record management, preferences, onboarding, tabular import, scheduling, pipelines and marketing pages. This is curated lexical retrieval, not a promise that every possible design exists.

Compare up to two relevant visual candidates against the brief and existing brand. Inspect their source/props before choosing a shell. Prefer a coherent shell, spacing scale and typography over mixing unrelated template styles. For functional sections choose generic workflows; local-demo starters illustrate behavior and are not production data layers. If no complete design fits, compose suitable sections or build a local adapter and state the gap. Do not label custom code as a library feature.

Use `--catalog /installed/citual-ui/registry/agent-catalog.json` with search/plan when the installed version differs from the bundled snapshot. Query only the relevant entries, then read their source/type contracts and selected recipes. Do not load the whole source catalog into context.

## Compose and implement

Decide the page shell, main sections, navigation, primary action and mobile arrangement. Briefly state the fit and implement without an extra design-approval round unless the user requests one.

- RecordManager covers list/detail/CRUD; DataTable is the smaller browsing surface. Use one record list, not both duplicated. Compose RecordForm + Drawer + RecordDetail for custom editing.
- ImportWizard parses/maps CSV or TSV. FileUploader transports attachments. MultiStepForm guides a form journey. XLSX needs another parser.
- EventScheduler owns event CRUD; CalendarMonth navigates dates; DatePicker chooses one date; DateRangePicker captures a range. Time zones, conflicts and recurrence remain application concerns.
- KanbanBoard groups records by stage. ActivityTimeline shows history. FilterBar/BulkActionToolbar add requested filtering or selected-record operations.
- ChartPanel hosts a caller-supplied chart and table alternative; StatCard shows a metric. SettingsPanel owns a settings draft with save/discard/defaults.
- Tabs, Accordion, MultiSelect, TreeView and CommandPalette provide their documented navigation/selection behaviors. Do not assume remote search, virtualization or asynchronous command handling.

Import foundation primitives from `citual-ui/foundations`, and compositions from `citual-ui/workflows`, `citual-ui/components` or `citual-ui/layouts`. Use real exported APIs, typed caller data and the project's existing data layer. Adapt content and brand via supported props/styles. Keep responsive structure and keyboard access. Implement loading, empty, pending, error and retry states where the requested flow needs them. Clearly identify demo data when real services were not requested or available.

Authentication, authorization, server validation, file transport and persistence remain caller-owned. Do not copy demo localStorage into production behavior. Purchased-template derivatives retain source license restrictions even in a private repository; inspect their metadata before redistribution.

## Verify and finish

Run appropriate consumer type/build checks. Use the agent's available browser tools to exercise the primary workflow, failure/retry where applicable and desktop/mobile rendering. Inspect console errors and visible focus, and capture screenshots when available. Fix observed defects within the requested scope; report environmental blockers accurately.

Finish with the implemented route/features, checks performed, screenshot or preview and material limitations. Do not end with another component roadmap. The library's 0.3 line is in maintenance: consumer pages may compose/adapt existing components, while library changes focus on bugs, compatibility, accessibility and security corrections unless the user explicitly changes that scope.

For one-time installation paths and example prompts, read [agent compatibility](agents.md). The skill works through the host coding agent; it is not a separate hosted builder or a guarantee of identical results across models.
