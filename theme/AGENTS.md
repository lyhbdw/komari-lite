# Theme Development Guide

This is the Vue visitor UI inside the Komari Lite monorepo. The React admin UI
lives in `../frontend`; framework-neutral helpers under `src/utils/*Core.ts`
are consumed by both. Inspect both consumers before changing these helpers.

## Build And Test

Run commands from `theme/`:

- `npm ci` installs the lockfile in `package-lock.json`.
- `npm test` runs the built-in Node runner through `tests/*.test.mjs`.
- `npm run type-check` checks Vue/TypeScript, including unused locals/parameters.
- `npm run build-only` builds Vite and creates the importable theme ZIP.
- `npm run build` performs type checking and the production build.
- `npm run lint` fixes ESLint findings; use `npx eslint .` for read-only linting.

If host Node is missing, use an existing Node 22 container with the monorepo
mounted so sibling shared-source imports remain available. Bound memory and
worker concurrency on low-memory hosts.

## Packaging Contract

Keep the actual `vite.config.ts` packaging contract intact:

- `dist/` contains runtime assets.
- `komari-theme-lite-build-<sha>.zip` contains `dist/`, `komari-theme.json`, and
  `preview.png` (from `docs/preview.png`).
- `komari-theme.json` is the canonical managed configuration manifest. The admin
  development middleware reads it; `../scripts/build-theme.sh` copies it into
  the Go embed directory. Preserve that embedded copy.
- `__BUILD_GIT_HASH__` is used for map asset cache versioning.
- Root `.github/actions/build-frontend/action.yml` builds both frontends with
  `npm ci`; GitHub does not execute workflows nested under this subtree.

## Runtime Conventions

- Preserve the `@` alias to `src`, the manifest schema, preview filename, and
  public flag/logo URL contracts. Assets can be selected dynamically.
- Use reka-ui plus Tailwind CSS v4; do not restore Naive UI, UnoCSS, or SCSS.
- Keep primary viewport cards statically imported. Load maps/globes/chart
  alternatives asynchronously, and retain active-view/KeepAlive cleanup.
- Do not persist privileged node snapshots in identity-agnostic browser storage.
- `../scripts/build-theme.sh --sync` changes production assets: never run it
  during source-only cleanup or verification without deployment authorization.
- Child guides, when present, override this guide in their own subtrees.
