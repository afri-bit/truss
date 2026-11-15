# Feature Specification: Documentation site

**Spec**: `017-docs-site` | **Created**: 2026-11-16 | **Status**: Implemented (publishing is a maintainer step)

**Input**: The docs are Markdown in `docs/`. The maintainer wants, before v0.1.0, a site with search,
a sidebar and a brand, and a home page that shows the product (decision 2026-11-16). Release scope:
`docs/MILESTONE.md`. Decision: [ADR 0016](../../docs/decisions/0016-documentation-site.md). The site
lives in `site/`; see `site/README.md`.

**Compatibility note.** None. The site is documentation tooling. The TRUSS binary, the pipeline
file, the CLI and the daemon API do not change, and building TRUSS never needs Node.

## User stories

1. **Read the docs with search and a sidebar** (P1). Every page of `docs/` that a user needs is on the
   site, in the sidebar and in search, in a dark and a light scheme. *Independent test*: build the
   site; each page exists, the sidebar lists it, and search finds a word from it.
2. **Start from a home page that shows the product** (P1). The home page says what TRUSS is and shows
   a pipeline being built and run, the three ways to use it, schedules with a catch-up run, and
   TRUSS Desktop. *Independent test*: open it at 1440, 1024 and 390 pixels wide; every section is
   there and nothing scrolls sideways.
3. **Docs are written once** (P1). A change to `docs/*.md` changes the site, and a link to a file
   that does not exist fails the build. *Independent test*: add a bad link to a docs file and build
   with `SYNC_STRICT=1`; the build fails.
4. **A getting-started path** (P1). `docs/getting-started.md` takes a new user from install to a
   scheduled pipeline, and every command and file in it works as written.

## Requirements

- **FR-001** `site/` builds with `npm ci && npm run build` to a static site. The pages are the home
  page and the docs pages made from `docs/getting-started.md`, `pipeline-reference.md`, `cli.md`,
  `daemon.md`, `desktop.md`, `architecture.md` and `GLOSSARY.md`, plus search and a 404 page.
- **FR-002** `docs/` is the only place documentation is written. `site/scripts/sync.mjs` generates
  the docs pages, makes links between docs relative links between site pages, links other repository
  files to GitHub, draws ```` ```mermaid ```` blocks as diagrams and copies the logo, images and
  Desktop's fonts. Generated files are ignored by git. With `SYNC_STRICT=1` a link to a file that does
  not exist fails the build.
- **FR-003** The brand is amber on graphite: the colours of TRUSS Desktop and the logo, set in IBM
  Plex, replacing Starlight's default theme in both schemes. No violet appears. Text pairs are at
  least 4.5:1.
- **FR-004** The home page is complete without script: every picture is drawn in its finished
  state. Under reduced motion nothing moves. It moves in four places only: the hero's entrance (once),
  the pipeline section read as it is scrolled, two replayed runs and one diagram, and hover. Anything
  that keeps moving has a pause button. Nothing reveals on scroll.
- **FR-005** The home page states only facts the product and its docs state. It shows no customer,
  quote, rating or figure the project cannot back.
- **FR-006** The site works under a base path (`SITE_URL`, `SITE_BASE`; default
  `https://afri-bit.github.io/truss/`). Fonts, links and assets all honour it.
- **FR-007** CI builds the site when `site/`, `docs/` or Desktop's fonts change
  (`.github/workflows/docs.yml`).

## Not in this spec

Publishing (GitHub Pages needs the repository's settings), a share image for social cards, a
dependency update routine for `site/`, and a link and accessibility check in CI.

## Verification

- Built and viewed in Chrome at 1440, 1024 and 390 pixels: no console error, no failed request, no
  horizontal overflow; with reduced motion every run shows its finished frame and no pause button;
  with script off every node, log line and the validate output are present.
- All Mermaid diagrams of the architecture page draw; docs pages have no console error in either scheme.
- `SYNC_STRICT=1` fails on a bad link (tested against a copy of the tree).
- Contrast of the colour pairs measured (4.8 to 10.5:1). The home page loads about 6 KB HTML, 6 KB
  CSS and 3.5 KB script, gzipped.
- Not proved: Safari and Firefox rendering (`offset-path` is used by the schedule diagram), the CI
  job (first run is the pull request), publishing.
