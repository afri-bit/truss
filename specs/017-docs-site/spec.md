# Feature Specification: Documentation site

**Spec**: `017-docs-site` | **Created**: 2026-11-16 | **Status**: Implemented (the first deploy needs the repository's Pages setting)

**Input**: The docs are Markdown in `docs/`. The maintainer wants, before v0.1.0, a site with search,
a sidebar and a brand, and a home page that shows the product (decision 2026-11-16). Release scope:
`docs/MILESTONE.md`. Decision: [ADR 0016](../../docs/decisions/0016-documentation-site.md). The site
lives in `site/`; see `site/README.md`.

Builds on [spec 016](../016-project-layout/spec.md): the docs and the getting-started page describe
the project layout. The same branch carries the repository's own pipelines (`.truss/flows/`, a chore
with no spec of its own).

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
  Plex, replacing Starlight's default theme in both schemes, code blocks included (they use syntax
  themes made from the brand palette). No violet appears anywhere. Text pairs are at least 4.5:1.
  The docs follow the system scheme; the home page is dark.
- **FR-004** The home page is complete without script: every picture is drawn in its finished
  state. Under reduced motion nothing moves. It moves in four places only: the hero's entrance (once),
  the pipeline section read as it is scrolled, two replayed runs and one diagram, and hover. Anything
  that keeps moving has a pause button, and pausing stops everything that moves in it. Nothing reveals
  on scroll, with one exception: the pipeline section shows the parts of a file as the scroll reaches
  them (and shows all of it on a phone, under reduced motion and without script).
- **FR-005** The home page states only facts the product and its docs state. It shows no customer,
  quote, rating or figure the project cannot back.
- **FR-006** The site works under a base path (`SITE_URL`, `SITE_BASE`; default
  `https://afri-bit.github.io/truss/`). Fonts, links and assets all honour it.
- **FR-007** CI builds the site when `site/`, `docs/` or Desktop's fonts change
  (`.github/workflows/docs.yml`).
- **FR-009** On a push to `main` CI publishes the built site to GitHub Pages by uploading it as a Pages
  package and deploying that; there is no `gh-pages` branch. The package contains `.nojekyll`. The
  repository's Pages source must be "GitHub Actions" (a setting, not code).
- **FR-008** The docs have a dark and light toggle: one button in the header (in the menu on a phone) that
  switches between the two, with a label that names the action. The docs follow the system scheme
  until it is used; after that the choice is remembered and applies to every page before it paints.

## Not in this spec

A share image for social cards, a dependency update routine for `site/`, and an accessibility check
in CI.

## Verification

- `npm test` (`site/scripts/sync.test.mjs`, 11 tests, run by CI): link rewriting (between docs, to
  GitHub, images, reference-style, angle brackets, titles), code left alone (inline, backtick and
  tilde fences), Mermaid blocks, front matter, strict mode on a missing link or image, and a
  symbolic link refused.
- Built and viewed in Chrome at 1440, 1024 and 390 pixels: no console error, no failed request, no
  horizontal overflow. With reduced motion every run shows its finished frame and no pause button
  shows; with script off every node, log line and the validate output are present.
- Pausing a run stops its spinners and flowing lines as well as its clock; the schedule diagram loops
  only when script runs, so its pause button always exists.
- No violet: every computed colour (text, background, border, fill, stroke) of 9 pages in both schemes,
  about 131,000 values, was scanned for hues 255 to 335, with none found. The same scan finds a
  planted violet on all 18 page and scheme combinations.
- The theme toggle (`ThemeToggle.astro`): starting from a dark and from a light system scheme, it
  switches, stores the choice, keeps it on another page set before paint, works from the keyboard,
  updates its label, has a 44 px target and causes no console error; on a phone it is in the menu.
- All Mermaid diagrams draw; docs pages have no console error in either scheme.
- Text contrast: every measured pair is at least 4.5:1 in both schemes (the lowest rendered pair is 4.56:1;
  the light scheme's muted text was darkened to 5.25:1 on its tightest surface).
- The home page loads about 6 KB HTML, 6 KB CSS and 3.5 KB script, gzipped.
- The product windows show what TRUSS prints: three-decimal durations and the setup lines in the log,
  the real `truss validate` output for the file shown, Desktop's 14-character run id in the
  breadcrumb, and "Ended" for a finished run.
- Reviews: independent reviews (general, spec, security) ran on `a3e9d2a` (rebased as `442b53a`, same
  site content), and delta reviews on the commits after it; all four verdicts are approve.
- Not proved: Safari and Firefox rendering (`offset-path` is used by the schedule diagram, with a
  `@supports` guard), a screen-reader pass, and the first runs of the CI job and of the deploy (they need
  the pull request and the Pages setting). Checked locally: the workflow parses, every action is pinned
  to a commit, and `dist/.nojekyll` is in the build output.
