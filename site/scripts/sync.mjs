// Builds the parts of the site that come from the repository itself, so that documentation is
// written once, in docs/, and read in two places (GitHub and this site):
//   - docs/*.md          -> src/content/docs/docs/*.md  (frontmatter added, links made relative)
//   - docs/images/*      -> src/content/docs/images/*  (Astro optimizes and hashes them)
//   - docs/images/logo.svg -> src/assets/logo.svg and public/favicon.svg
//   - Desktop's fonts    -> src/assets/fonts/*.woff2 (IBM Plex, SIL OFL 1.1; the site and Desktop share them),
//                           and the licence to public/fonts/OFL.txt
// Everything it writes is ignored by git. Run by `npm run dev` and `npm run build`.
//
// Environment: SYNC_STRICT=1 fails on a problem the docs have (CI sets it); SYNC_REPO and SYNC_SITE
// replace the repository and site folders (the tests use them); TRUSS_DOCS_BRANCH is the branch the
// GitHub links point at (default main).
//
// Limits: only fenced code (``` or ~~~) is recognised as code, so use fences, not indentation.
import { cp, lstat, mkdir, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises';
import { dirname, join, posix, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const site = process.env.SYNC_SITE ? resolve(process.env.SYNC_SITE) : resolve(here, '..');
const repo = process.env.SYNC_REPO ? resolve(process.env.SYNC_REPO) : resolve(site, '..');
const docsDir = join(repo, 'docs');
const outDocs = join(site, 'src', 'content', 'docs', 'docs');
const REPO_URL = 'https://github.com/afri-bit/truss';
const BRANCH = process.env.TRUSS_DOCS_BRANCH || 'main';
if (!/^[A-Za-z0-9._/-]+$/.test(BRANCH)) throw new Error(`sync: TRUSS_DOCS_BRANCH is not a branch name: ${BRANCH}`);

// The pages of the site, in sidebar order (astro.config.mjs lists the same slugs).
const pages = [
  { file: 'getting-started.md', slug: 'getting-started', title: 'Getting started' },
  { file: 'pipeline-reference.md', slug: 'pipeline-reference', title: 'Pipeline file reference' },
  { file: 'cli.md', slug: 'cli', title: 'Command line reference' },
  { file: 'daemon.md', slug: 'daemon', title: 'The daemon' },
  { file: 'desktop.md', slug: 'desktop', title: 'TRUSS Desktop' },
  { file: 'architecture.md', slug: 'architecture', title: 'Architecture' },
  { file: 'GLOSSARY.md', slug: 'glossary', title: 'Glossary' },
];
const slugOf = new Map(pages.map((p) => [p.file, p.slug]));

// A problem the docs have (a link to a file that is not there). CI sets SYNC_STRICT=1 so it fails the build.
const warnings = [];
const warn = (msg) => {
  warnings.push(msg);
  console.warn(`sync: ${msg}`);
};

const exists = async (p) => stat(p).then(() => true, () => false);
const isDir = async (p) => stat(p).then((s) => s.isDirectory(), () => false);

// What the site publishes must be a plain file of the repository: a link planted in docs/ would
// otherwise publish whatever it points at (the logo becomes public/favicon.svg).
async function copyRegular(src, dst) {
  const info = await lstat(src);
  if (!info.isFile()) throw new Error(`sync: ${src} is not a regular file (links are not followed)`);
  await mkdir(dirname(dst), { recursive: true });
  await cp(src, dst);
}

// A link in a docs file becomes a link between site pages, or a link into the repository on GitHub.
// Site links are relative (../<slug>/), so they work under any base path.
async function rewriteTarget(rawTarget, fromFile) {
  const angle = rawTarget.startsWith('<') && rawTarget.endsWith('>');
  const target = angle ? rawTarget.slice(1, -1) : rawTarget;
  const keep = (value) => (angle ? `<${value}>` : value); // a target written in <...> stays in <...>
  if (/^([a-z][a-z0-9+.-]*:|#|\/\/)/i.test(target)) return rawTarget; // external, mailto, anchor
  const hashAt = target.indexOf('#');
  const path = hashAt === -1 ? target : target.slice(0, hashAt);
  const hash = hashAt === -1 ? '' : target.slice(hashAt);
  if (path === '') return rawTarget;

  // Resolve against the file's folder, as a path inside the repository.
  const repoPath = posix.normalize(posix.join('docs', posix.dirname(fromFile), path));
  if (repoPath.startsWith('..')) return rawTarget; // outside the repository: leave it
  if (repoPath.startsWith('docs/images/') && repoPath !== 'docs/images/logo.svg') {
    if (!(await exists(join(repo, repoPath)))) warn(`${fromFile}: image not found: ${target}`);
    return keep(`../images/${repoPath.slice('docs/images/'.length)}${hash}`);
  }
  if (repoPath.startsWith('docs/') && slugOf.has(repoPath.slice('docs/'.length))) {
    return keep(`../${slugOf.get(repoPath.slice('docs/'.length))}/${hash}`);
  }
  const onDisk = join(repo, repoPath);
  if (!(await exists(onDisk))) {
    warn(`${fromFile}: link target not found in the repository: ${target}`);
    return rawTarget;
  }
  const kind = (await isDir(onDisk)) ? 'tree' : 'blob';
  return keep(`${REPO_URL}/${kind}/${BRANCH}/${repoPath.replace(/\/$/, '')}${hash}`);
}

const stripMarkdown = (s) =>
  s
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[`*_]/g, '')
    .replace(/\s+/g, ' ')
    .trim();

const FENCE = /^ {0,3}(`{3,}|~{3,})(.*)$/;
const closesFence = (m, marker) => m && m[1][0] === marker[0] && m[1].length >= marker.length && m[2].trim() === '';

function describe(lines) {
  // The first plain paragraph after the title.
  const para = [];
  let fence = null;
  for (const line of lines) {
    const f = line.match(FENCE);
    if (fence) {
      if (closesFence(f, fence)) fence = null;
      continue;
    }
    if (f) {
      fence = f[1];
      continue;
    }
    if (/^(#|>|\||\s*[-*] |\s*\d+\. |<)/.test(line)) {
      if (para.length) break;
      continue;
    }
    if (line.trim() === '') {
      if (para.length) break;
      continue;
    }
    para.push(line.trim());
  }
  const text = stripMarkdown(para.join(' '));
  if (text.length <= 160) return text;
  const cut = text.slice(0, 157);
  const at = cut.lastIndexOf(' ');
  return `${at > 0 ? cut.slice(0, at) : cut}...`;
}

const escapeHtml = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// [text](target "title"), ![alt](<target with spaces>), titles in "", '' or ().
const INLINE = /\]\(\s*(<[^>]*>|[^)\s]+)((?:\s+(?:"[^"]*"|'[^']*'|\([^)]*\)))?)\s*\)/g;
// [ref]: target "title"
const DEFINITION = /^( {0,3}\[[^\]]+\]:\s*)(<[^>]*>|\S+)(.*)$/;

async function rewriteLine(line, fromFile) {
  const def = line.match(DEFINITION);
  if (def) return `${def[1]}${await rewriteTarget(def[2], fromFile)}${def[3]}`;
  // Inline code spans are left alone.
  const parts = line.split(/(`+[^`]*`+)/);
  for (let i = 0; i < parts.length; i += 2) {
    const found = [...parts[i].matchAll(INLINE)];
    let rebuilt = '';
    let last = 0;
    for (const m of found) {
      rebuilt += `${parts[i].slice(last, m.index)}](${await rewriteTarget(m[1], fromFile)}${m[2]})`;
      last = m.index + m[0].length;
    }
    parts[i] = rebuilt + parts[i].slice(last);
  }
  return parts.join('');
}

async function transform(page) {
  if (!(await lstat(join(docsDir, page.file))).isFile()) throw new Error(`sync: docs/${page.file} is not a regular file (links are not followed)`);
  const source = await readFile(join(docsDir, page.file), 'utf8');
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  if (lines[0].startsWith('# ')) lines.shift(); // Starlight renders the title itself
  while (lines.length && lines[0].trim() === '') lines.shift();

  const out = [];
  let fence = null; // the opening fence marker of a code block
  let mermaid = null; // the marker and collected lines of a mermaid block
  for (const line of lines) {
    const f = line.match(FENCE);
    if (mermaid) {
      if (closesFence(f, mermaid.marker)) {
        out.push('', `<pre class="mermaid">${escapeHtml(mermaid.lines.join('\n'))}</pre>`, '');
        mermaid = null;
      } else {
        mermaid.lines.push(line);
      }
      continue;
    }
    if (fence) {
      if (closesFence(f, fence)) fence = null;
      out.push(line);
      continue;
    }
    if (f) {
      if (f[2].trim() === 'mermaid') {
        mermaid = { marker: f[1], lines: [] };
      } else {
        fence = f[1];
        out.push(line);
      }
      continue;
    }
    out.push(await rewriteLine(line, page.file));
  }

  const description = describe(lines);
  const front = [
    '---',
    `title: ${JSON.stringify(page.title)}`,
    description ? `description: ${JSON.stringify(description)}` : null,
    `editUrl: ${JSON.stringify(`${REPO_URL}/edit/${BRANCH}/docs/${page.file}`)}`,
    '---',
    '',
  ]
    .filter((l) => l !== null)
    .join('\n');
  await writeFile(join(outDocs, `${page.slug}.md`), `${front}${out.join('\n')}\n`);
}

await rm(outDocs, { recursive: true, force: true });
await mkdir(outDocs, { recursive: true });
for (const page of pages) await transform(page);

// Images the docs show.
const imagesOut = join(site, 'src', 'content', 'docs', 'images');
await rm(imagesOut, { recursive: true, force: true });
await mkdir(imagesOut, { recursive: true });
for (const name of await readdir(join(docsDir, 'images'))) {
  if (name !== 'logo.svg') await copyRegular(join(docsDir, 'images', name), join(imagesOut, name));
}

// The logo, as the site's logo and its favicon.
await copyRegular(join(docsDir, 'images', 'logo.svg'), join(site, 'src', 'assets', 'logo.svg'));
await copyRegular(join(docsDir, 'images', 'logo.svg'), join(site, 'public', 'favicon.svg'));

// Desktop's fonts (IBM Plex Sans and Mono, SIL OFL 1.1, licence included).
const fontsSrc = join(repo, 'internal', 'webui', 'assets', 'fonts');
const fontsOut = join(site, 'src', 'assets', 'fonts'); // imported by fonts.css, so Vite adds the base path
const licenceOut = join(site, 'public', 'fonts');
await rm(fontsOut, { recursive: true, force: true });
await rm(licenceOut, { recursive: true, force: true });
await mkdir(fontsOut, { recursive: true });
for (const name of await readdir(fontsSrc)) {
  if (name.endsWith('.woff2')) await copyRegular(join(fontsSrc, name), join(fontsOut, name));
  else if (name === 'OFL.txt') await copyRegular(join(fontsSrc, name), join(licenceOut, name));
}

console.log(`sync: ${pages.length} pages, images, logo and fonts`);
if (warnings.length && process.env.SYNC_STRICT === '1') {
  console.error(`sync: ${warnings.length} problem(s) and SYNC_STRICT=1`);
  process.exit(1);
}
