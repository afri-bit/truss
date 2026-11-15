# 0016. Documentation site

- Status: Proposed
- Date: 2026-11-16

## Context

The product docs are Markdown in `docs/` and render on GitHub. Before v0.1.0 a user-facing site is
wanted, with search, a sidebar, a brand and a home page. This is a maintainer decision of
2026-11-16; `docs/MILESTONE.md` gets a row for it. The repository is Go-only. The browser tests of
TRUSS Desktop already use Node (`tests/ui`, Playwright), so Node is not new to CI.

## Decision

The site lives in `site/` and is built with Astro and its Starlight docs theme. It is a build tool
and documentation only. The TRUSS product (CLI, daemon, TRUSS Desktop) does not depend on it, and
building TRUSS never needs Node.

New third-party packages are pinned to exact versions in `site/package.json` and
`site/package-lock.json`: `astro` 7.4.12, `@astrojs/starlight` 0.43.1 and `mermaid` 12.3.0 (diagrams,
loaded only on pages that contain one). They are development tooling outside the Go module, so
`go.mod` and `go.sum` do not change. CI installs with `npm ci`.

`docs/` stays the one place where documentation is written. `site/scripts/sync.mjs` generates the
site's pages from `docs/*.md`: it adds front matter, makes links relative and turns Mermaid fences
into blocks the page draws. It also copies the logo, the images and TRUSS Desktop's IBM Plex fonts
(SIL OFL 1.1) from the repository. Everything it writes is ignored by git. In CI, a link to a file
that does not exist fails the build (`SYNC_STRICT=1`).

The brand is amber on graphite, the colours of TRUSS Desktop and the logo (`src/styles/tokens.css`,
`brand.css`), set in IBM Plex. It replaces Starlight's default theme. Starlight's purple, used for
tips, is remapped so that no violet appears. Dark is the default. The docs also follow the light
scheme.

The home page is a custom Astro page (`site/src/pages/index.astro`): plain HTML, CSS and a small
script (about 3.5 KB gzipped), with no animation library. Its pictures are live HTML built from
TRUSS Desktop's own parts. The page is complete without script and under reduced motion.

The site is served from a base path set by `SITE_URL` and `SITE_BASE` (default: GitHub Pages at
`/truss`). Publishing is not part of this decision. Deploying needs the maintainer's repository
settings.

## Consequences

- Good: search, a sidebar, light and dark docs, and a home page that shows the product.
- Good: one source for the docs. The product is unaffected.
- Bad: a Node toolchain and a large tree for the site: the lockfile lists 479 entries (about 390 install on
  one platform; the rest are optional per-platform binaries). That is a supply-chain surface. It is
  kept out of the product and pinned by lockfile, every entry has an integrity hash and comes from
  the npm registry, `npm audit` reports nothing, and CI installs with `--ignore-scripts`.
- Bad: Astro sends anonymous usage data on a build unless told not to; CI sets
  `ASTRO_TELEMETRY_DISABLED=1` and `site/README.md` says how to turn it off locally.
- Licences: some build tools carry other licences than Apache 2.0. `elkjs` (inside Mermaid's browser
  bundle) is EPL-2.0 or GPL-3.0-or-later, the libvips binaries are LGPL-3.0, and `khroma` states no
  licence. None is part of the TRUSS binary or its module.
- Bad: another CI job. Astro 7 and Starlight are young and may need updates.
- Follow-ups: deploy to GitHub Pages (needs repository settings); a share image for social cards,
  if wanted; a dependency update routine (for example Dependabot for `site/`); a link and
  accessibility check in CI.
