# docs

The Astro site published to GitHub Pages.

## Running

```sh
npm install
npm run dev      # localhost:4321
npm run build    # the only check this area has
```

There is no lint step and no test suite here, so `npm run build` is the whole
verification loop. `pre-push` runs it when anything under `docs/` moves.

## Deployment

`pages.yaml` builds and deploys on a push to master that touches `docs/`,
`spec/openapi.yaml` or `assets/`. It copies `spec/openapi.yaml` into
`docs/public/` before building, so the API reference reads a spec that is never
committed under `docs/`. Leave `docs/public/openapi.yaml` uncommitted.

`docs/dist/` and `docs/node_modules/` are gitignored. `astro.config.mjs` carries
the site URL and the base path.

## Layout

- `src/pages/` maps one to one onto routes, with `src/pages/docs/` as the
  documentation section
- `src/data/` holds the lists the pages render: `features.ts`, `controls.ts`,
  `nav.ts`, `install.ts`. A new feature goes there, not inline in a page
- `src/components/demos/` holds the animated terminal demos, one per feature
- `src/layouts/` has the two shells, `Layout.astro` for the marketing pages and
  `DocsLayout.astro` for the docs section

## Writing

Copy describes what fuku does now. Every claim on the site is an assertion about
the source, so check it against `internal/` before writing it, and cut it rather
than soften it when it cannot be checked.
