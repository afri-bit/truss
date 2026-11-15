// Builds the parts of the site that come from the repository itself, so that documentation is
// written once, in docs/, and read in two places (GitHub and this site):
//   - docs/*.md          -> src/content/docs/docs/*.md  (frontmatter added, links made relative)
//   - docs/images/*      -> src/content/docs/images/*  (Astro optimizes and hashes them)
//   - docs/images/logo.svg -> src/assets/logo.svg and public/favicon.svg
//   - Desktop's fonts    -> public/fonts/*  (IBM Plex, SIL OFL 1.1; the site and Desktop share them)
// Everything it writes is ignored by git. Run by `npm run dev` and `npm run build`.
import { cp, mkdir, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises';
import { dirname, join, posix, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const site = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repo = resolve(site, '..');
const docsDir = join(repo, 'docs');
const outDocs = join(site, 'src', 'content', 'docs', 'docs');
const REPO_URL = 'https://github.com/afri-bit/truss';
const BRANCH = process.env.TRUSS_DOCS_BRANCH || 'main';

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

// A link in a docs file becomes a link between site pages, or a link into the repository on GitHub.
// Site links are relative (../<slug>/), so they work under any base path.
async function rewriteTarget(target, fromFile) {
  if (/^([a-z][a-z0-9+.-]*:|#|\/\/)/i.test(target)) return target; // external, mailto, anchor
  const hashAt = target.indexOf('#');
  const path = hashAt === -1 ? target : target.slice(0, hashAt);
  const hash = hashAt === -1 ? '' : target.slice(hashAt);
  if (path === '') return target;

  // Resolve against the file's folder, as a path inside the repository.
  const repoPath = posix.normalize(posix.join('docs', posix.dirname(fromFile), path));
  if (repoPath.startsWith('docs/images/')) return `../images/${repoPath.slice('docs/images/'.length)}${hash}`;
  if (repoPath.startsWith('docs/') && slugOf.has(repoPath.slice('docs/'.length))) {
    return `../${slugOf.get(repoPath.slice('docs/'.length))}/${hash}`;
  }
  if (repoPath.startsWith('..')) return target; // outside the repository: leave it
  const onDisk = join(repo, repoPath);
  if (!(await exists(onDisk))) {
    warn(`${fromFile}: link target not found in the repository: ${target}`);
    return target;
  }
  const kind = (await isDir(onDisk)) ? 'tree' : 'blob';
  return `${REPO_URL}/${kind}/${BRANCH}/${repoPath.replace(/\/$/, '')}${hash}`;
}

const stripMarkdown = (s) =>
  s
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[`*_]/g, '')
    .replace(/\s+/g, ' ')
    .trim();

function describe(lines) {
  // The first plain paragraph after the title.
  let para = [];
  let inFence = false;
  for (const line of lines) {
    if (line.startsWith('```')) inFence = !inFence;
    if (inFence || line.startsWith('```')) continue;
    if (/^(#|>|\||\s*[-*] |\d+\. |<)/.test(line)) {
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
  return `${cut.slice(0, cut.lastIndexOf(' '))}...`;
}

const escapeHtml = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

async function transform(page) {
  const source = await readFile(join(docsDir, page.file), 'utf8');
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  if (lines[0].startsWith('# ')) lines.shift(); // Starlight renders the title itself
  while (lines.length && lines[0].trim() === '') lines.shift();

  const out = [];
  let fence = null; // the fence line that opened a code block
  let mermaid = null; // collected lines of a mermaid block
  for (const line of lines) {
    if (mermaid) {
      if (line.startsWith('```')) {
        out.push('', `<pre class="mermaid">${escapeHtml(mermaid.join('\n'))}</pre>`, '');
        mermaid = null;
      } else {
        mermaid.push(line);
      }
      continue;
    }
    if (fence) {
      if (line.startsWith('```')) fence = null;
      out.push(line);
      continue;
    }
    if (/^```\s*mermaid\s*$/.test(line)) {
      mermaid = [];
      continue;
    }
    if (line.startsWith('```')) {
      fence = line;
      out.push(line);
      continue;
    }
    // Links outside code: [text](target) and ![alt](target); inline code spans are left alone.
    const parts = line.split(/(`[^`]*`)/);
    for (let i = 0; i < parts.length; i += 2) {
      const pending = [];
      parts[i].replace(/\]\(([^)\s]+)((?:\s+"[^"]*")?)\)/g, (m, target, title, offset) => {
        pending.push({ m, target, title, offset });
        return m;
      });
      let rebuilt = '';
      let last = 0;
      for (const p of pending) {
        rebuilt += parts[i].slice(last, p.offset) + `](${await rewriteTarget(p.target, page.file)}${p.title})`;
        last = p.offset + p.m.length;
      }
      parts[i] = rebuilt + parts[i].slice(last);
    }
    out.push(parts.join(''));
  }

  const body = out.join('\n');
  const description = describe(lines);
  const front = [
    '---',
    `title: ${JSON.stringify(page.title)}`,
    description ? `description: ${JSON.stringify(description)}` : null,
    `editUrl: ${REPO_URL}/edit/${BRANCH}/docs/${page.file}`,
    '---',
    '',
  ]
    .filter((l) => l !== null)
    .join('\n');
  await writeFile(join(outDocs, `${page.slug}.md`), `${front}${body}\n`);
}

await rm(outDocs, { recursive: true, force: true });
await mkdir(outDocs, { recursive: true });
for (const page of pages) await transform(page);

// Images the docs show.
const imagesOut = join(site, 'src', 'content', 'docs', 'images');
await rm(imagesOut, { recursive: true, force: true });
await mkdir(imagesOut, { recursive: true });
for (const name of await readdir(join(docsDir, 'images'))) {
  if (name !== 'logo.svg') await cp(join(docsDir, 'images', name), join(imagesOut, name));
}

// The logo, as the site's logo and its favicon.
await mkdir(join(site, 'src', 'assets'), { recursive: true });
await cp(join(docsDir, 'images', 'logo.svg'), join(site, 'src', 'assets', 'logo.svg'));
await cp(join(docsDir, 'images', 'logo.svg'), join(site, 'public', 'favicon.svg'));

// Desktop's fonts (IBM Plex Sans and Mono, SIL OFL 1.1, licence included).
const fontsOut = join(site, 'public', 'fonts');
await rm(fontsOut, { recursive: true, force: true });
await cp(join(repo, 'internal', 'webui', 'assets', 'fonts'), fontsOut, { recursive: true });

console.log(`sync: ${pages.length} pages, images, logo and fonts`);
if (warnings.length && process.env.SYNC_STRICT === '1') {
  console.error(`sync: ${warnings.length} problem(s) and SYNC_STRICT=1`);
  process.exit(1);
}
