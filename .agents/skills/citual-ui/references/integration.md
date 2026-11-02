# Integrate Citual UI

Requires React 18+ and a supported React bundler. The consumer matrix tests Next.js 15 and Vite 6 with React 18, Tailwind 3.4 and Tailwind 4. This does not verify all versions permitted by peer ranges.

For supported existing projects, use `npx --no-install citual-ui init --dry-run` then `init` after installing the package. See `recipes/project-setup.md` in the installed library for adapters and manual fallbacks.

The package is source TypeScript. It is not yet published to npm: do not assume `npm install citual-ui` resolves this repository. For a local checkout:

```sh
npm install /absolute/path/to/citual-ui
```

For a private GitHub repository, use an authenticated clone and local install, or an authenticated Git dependency pinned to a reviewed commit. Use the actual owner/URL from the repository remote. Never embed access tokens in package URLs. The skill alone provides no GitHub access or package installation.

Next.js configuration:

```js
export default { transpilePackages: ['citual-ui'] };
```

Import `citual-ui/styles.css` once from the app root. Tailwind 3 content must include:

```js
content: [
  './app/**/*.{ts,tsx}',
  './components/**/*.{ts,tsx}',
  './node_modules/citual-ui/{components,layouts}/**/*.{ts,tsx}',
]
```

Use `"use client"` in consumers owning interactive state. Import workflows from `citual-ui/workflows`, template components from `citual-ui/components`, layouts from `citual-ui/layouts`, and API helpers from `citual-ui/lib/api`. Legacy `spur-*` CSS classes/custom properties are intentionally retained for compatibility; package imports and branding use Citual UI.

```tsx
'use client';
import { DataTable, type TableColumn } from 'citual-ui/workflows';
type Product = { id: string; name: string };
const columns: TableColumn<Product>[] = [
  { key: 'name', label: 'Name', value: row => row.name },
];
export function Products({ rows }: { rows: Product[] }) {
  return <DataTable rows={rows} columns={columns} rowId={row => row.id} />;
}
```

Find recipe paths and source paths in the selected catalog entry, relative to the installed package/checkout, not the consuming project's source directory. Source props and recipes document whether callbacks must return confirmed records and whether AbortSignal/progress are supported. Use the consumer's existing fetching/caching strategy; components do not supply a generic server-pagination engine.

To install this skill in another project, copy the entire `skills/citual-ui` directory into that agent's skill location (for Codex, `.agents/skills/citual-ui` in the project or `~/.codex/skills/citual-ui` globally). Keep `scripts/` and `references/` together. For agents without skill discovery, add a project instruction to read its SKILL.md for Citual UI work.


For manual Tailwind 4 integration, use the project's existing Tailwind 4 compiler/plugin. In its root CSS, import Tailwind and `citual-ui/styles.css`, then add `@source` for the package's component/layout TSX paths relative to that CSS file. Example for `app/globals.css`: `@source "../node_modules/citual-ui/{components,layouts}/**/*.{ts,tsx}";`. An explicit source is required because dependencies are normally excluded from automatic detection. Vite should use automatic JSX transformation or the React plugin when consuming the source package.
