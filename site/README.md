# TRUSS site

The website and the documentation of TRUSS, built with [Astro](https://astro.build) and its
[Starlight](https://starlight.astro.build) docs theme. It is documentation tooling: the TRUSS
product does not depend on it, and building TRUSS never needs Node. Decision:
[ADR 0016](../docs/decisions/0016-documentation-site.md).

## Work on it

```bash
cd site
npm ci          # exact versions from package-lock.json
npm run dev     # http://localhost:4321/truss/ with live reload
npm run build   # static site in site/dist
npm run preview # serve what build made
npm test        # the sync script's tests (run by CI)
```

`npm run dev` and `npm run build` first run `scripts/sync.mjs`, which makes the docs pages from
`docs/*.md`, so **write documentation in `docs/`, never in `site/`**. It adds front matter, turns
links between docs into links between site pages (and links to other files into GitHub links), turns
```` ```mermaid ```` blocks into diagrams, and copies the logo, the images and Desktop's fonts. What it
writes is ignored by git. To add a page, add it to `pages` in `scripts/sync.mjs` and to the sidebar in
`astro.config.mjs`. `SYNC_STRICT=1` (CI sets it) fails the build on a link to a file that is not there.

## Where things are

| Path | What |
|---|---|
| `src/pages/index.astro`, `src/components/` | The home page: sections as Astro components |
| `src/styles/tokens.css` | The brand: colours and fonts, the same values as TRUSS Desktop |
| `src/styles/brand.css` | Maps the brand onto Starlight's variables; it replaces Starlight's default theme |
| `src/styles/home.css` | The home page's one system: scale, buttons, cards, motion |
| `src/lib/run.ts` | One pipeline run as data; the server draws it finished, `src/scripts/home.ts` replays it |
| `src/scripts/` | The home page's script (about 3.5 KB gzipped) and the docs' diagram renderer |
| `scripts/sync.mjs` | Builds the generated parts from the repository |

## Privacy and trust

- Astro sends anonymous usage data on a build unless you turn it off: `npx astro telemetry disable`,
  or set `ASTRO_TELEMETRY_DISABLED=1` (CI does). The published site makes no third-party request.
- `docs/` is trusted, reviewed content: raw HTML in a docs file reaches the site unchanged. `sync.mjs`
  refuses to copy a symbolic link, so a link planted in `docs/images/` is not published.
- CI installs with `npm ci --ignore-scripts`.

## The brand

Amber on graphite: the logo's mark and TRUSS Desktop's own tokens, set in IBM Plex (code blocks use
syntax themes made from the same palette, `src/styles/code-themes.mjs`). Graphite carries
the interface; amber is the one accent; green, blue and red appear only to show the state of a step,
as in Desktop. There is no violet anywhere (Starlight's purple is remapped). Change a colour in
`tokens.css` and both the docs and the home page follow. Text pairs are at least 4.5:1 in both schemes.

The home page moves in four places and nowhere else: the hero's entrance (once), the pipeline
section that reads a file as you scroll, two replayed runs and one diagram (each with a pause
button), and hover. Nothing reveals on scroll. It is complete without script and under reduced
motion, which show every picture in its finished state.

## Serving it

The site is built for a base path. By default it is `https://afri-bit.github.io/truss/` (GitHub
Pages); set `SITE_URL` and `SITE_BASE` for another host, for example `SITE_BASE=/` on a domain of
its own. Publishing is not automated yet: it needs the repository's Pages setting ("GitHub Actions")
and then a job that runs `actions/upload-pages-artifact` on `site/dist` and `actions/deploy-pages`.
