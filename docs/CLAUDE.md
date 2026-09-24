# docs

The Astro site published to GitHub Pages.

## Running

```sh
npm install
npm run dev      # localhost:4321
npm run build    # the only check this area has
```

There is no lint and no test suite here. `npm run build` is the whole check. `pre-push` runs it when anything under `docs/` moves.

## Deployment

`pages.yaml` builds and deploys on a push to master that touches `docs/`, `spec/openapi.yaml` or `assets/`.
It copies `spec/openapi.yaml` into `docs/public/` before the build. Leave `docs/public/openapi.yaml` uncommitted.

`docs/dist/` and `docs/node_modules/` are gitignored. `astro.config.mjs` carries the site URL and the base path.

## Layout

- `src/pages/` maps one to one onto routes. `src/pages/docs/` is the documentation section
- `src/data/` holds the lists the pages render: `features.ts`, `controls.ts`, `nav.ts`, `install.ts`. A new feature goes there, not inline in a page
- `src/components/demos/` holds the animated terminal demos, one per feature
- `src/layouts/` has the two shells: `Layout.astro` for the marketing pages, `DocsLayout.astro` for the docs

## Writing

Copy describes what fuku does now. Check every claim against `internal/` before writing it. If it cannot be checked, cut it.
Do not soften it.
