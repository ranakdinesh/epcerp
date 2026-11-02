# Apply the consuming project's theme

Read this when changing visual styling or integrating the foundation for the first time. Existing project design rules and primitives remain authoritative.

Inspect the relevant global CSS, theme/style config, Tailwind config and existing components. Reuse the project's semantic roles, spacing, fonts, density, radii and light/dark states. Map them to `--citual-*` in the existing theme stylesheet; never import the showcase's green palette as a new default. If the mapping is already correct, use it rather than generating another theme.

After locating the library, read `recipes/design-foundation.md` for the complete token contract, and `components/foundations/index.ts` for available primitives. These paths are relative to the selected library, not the skill folder. Read component source/types only for selected controls.

- Complete CSS colors use `--citual-primary: var(--primary)`; legacy HSL channels need `--citual-primary: hsl(var(--primary))`. Inspect the actual value before choosing.
- Tailwind 3 JS/TS palette keys are not runtime CSS variables. Expose the chosen project values through its existing theme pipeline. Tailwind 4 semantic `--color-*` fallbacks work only when the actual CSS variables exist and contain complete values.
- Map background/foreground pairs together: page, surface, primary, selected, muted text, border, focus ring and status surfaces/text. Include the project's dark selector. Use `data-citual-theme` and local mappings at nested theme boundaries so inherited aliases resolve locally.
- Shared foundation controls/layout/typography come from `citual-ui/foundations`; generic business behavior comes from `citual-ui/workflows`. Reuse equivalent host primitives first. Template references are not fully themeable foundations; preserve attribution and original licensing if adapting them, and name new abstractions by purpose.

Validate the changed visual states in the actual application, including keyboard focus and relevant mobile/dark behavior. A valid CSS variable or a build alone does not demonstrate correct contrast or layout.
