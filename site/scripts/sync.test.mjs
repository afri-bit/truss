// Tests for scripts/sync.mjs: it runs the real script against a small fixture repository in a
// temporary folder (SYNC_REPO and SYNC_SITE), so nothing real is read or written. `npm test`.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { after, test } from 'node:test';
import { fileURLToPath } from 'node:url';

const script = join(dirname(fileURLToPath(import.meta.url)), 'sync.mjs');
const PAGES = ['overview', 'getting-started', 'guide-schedules', 'guide-hooks', 'guide-notifications', 'guide-variables', 'cli-guide', 'guide-platforms', 'how-a-run-works', 'GLOSSARY', 'pipeline-reference', 'cli', 'daemon', 'desktop', 'troubleshooting', 'architecture'];

// A repository with the files sync reads. `page` is the body of docs/getting-started.md.
function fixture(page) {
  const root = mkdtempSync(join(tmpdir(), 'truss-sync-'));
  const repo = join(root, 'repo');
  const site = join(root, 'site');
  const write = (path, text) => {
    mkdirSync(dirname(join(repo, path)), { recursive: true });
    writeFileSync(join(repo, path), text);
  };
  for (const name of PAGES) write(`docs/${name}.md`, `# ${name}\n\nIntro of ${name}.\n`);
  write('docs/getting-started.md', page);
  write('docs/decisions/0001-x.md', '# x\n');
  write('docs/images/logo.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>');
  write('docs/images/shot.png', 'png');
  write('README.md', '# r\n');
  write('internal/webui/assets/fonts/a.woff2', 'font');
  write('internal/webui/assets/fonts/OFL.txt', 'licence');
  mkdirSync(site, { recursive: true });
  cleanup.push(root);
  return { repo, site };
}

// Every fixture lives in its own temporary folder; remove them when the tests are done.
const cleanup = [];
after(() => cleanup.forEach((dir) => rmSync(dir, { recursive: true, force: true })));

function run({ repo, site }, env = {}) {
  const r = spawnSync(process.execPath, [script], { env: { ...process.env, SYNC_REPO: repo, SYNC_SITE: site, ...env }, encoding: 'utf8' });
  const out = join(site, 'src', 'content', 'docs', 'docs', 'getting-started.md');
  return { status: r.status, stderr: r.stderr, stdout: r.stdout, page: existsSync(out) ? readFileSync(out, 'utf8') : '' };
}

test('links between docs become relative site links, anchors and titles survive', () => {
  const r = run(fixture('# T\n\nSee [the CLI](cli.md#exit-codes "Commands") and [daemon](daemon.md).\n'));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /\[the CLI\]\(\.\.\/cli\/#exit-codes "Commands"\)/);
  assert.match(r.page, /\[daemon\]\(\.\.\/daemon\/\)/);
});

test('links to other repository files become GitHub links, files as blob and folders as tree', () => {
  const r = run(fixture('# T\n\n[readme](../README.md) and [decisions](decisions/) and [one](decisions/0001-x.md#a).\n'));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /\(https:\/\/github\.com\/afri-bit\/truss\/blob\/main\/README\.md\)/);
  assert.match(r.page, /\(https:\/\/github\.com\/afri-bit\/truss\/tree\/main\/docs\/decisions\)/);
  assert.match(r.page, /\(https:\/\/github\.com\/afri-bit\/truss\/blob\/main\/docs\/decisions\/0001-x\.md#a\)/);
});

test('images are linked from the generated images folder and copied', () => {
  const f = fixture('# T\n\n![shot](images/shot.png)\n');
  const r = run(f);
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /!\[shot\]\(\.\.\/images\/shot\.png\)/);
  assert.ok(existsSync(join(f.site, 'src', 'content', 'docs', 'images', 'shot.png')));
  assert.ok(existsSync(join(f.site, 'public', 'favicon.svg')), 'the logo becomes the favicon');
});

test('reference definitions, angle brackets and single-quoted titles are rewritten', () => {
  const r = run(fixture("# T\n\n[a][ref] [b](<cli.md>) [c](cli.md 'T')\n\n[ref]: cli.md#x\n"));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /\[ref\]: \.\.\/cli\/#x/);
  assert.match(r.page, /\[b\]\(<\.\.\/cli\/>\)/);
  assert.match(r.page, /\[c\]\(\.\.\/cli\/ 'T'\)/);
});

test('code is left alone: inline code, backtick fences and tilde fences', () => {
  const r = run(fixture('# T\n\n`[x](cli.md)`\n\n```md\n[x](cli.md)\n```\n\n~~~md\n[y](daemon.md)\n~~~\n\nAfter [z](cli.md).\n'));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /`\[x\]\(cli\.md\)`/);
  assert.match(r.page, /```md\n\[x\]\(cli\.md\)\n```/);
  assert.match(r.page, /~~~md\n\[y\]\(daemon\.md\)\n~~~/);
  assert.match(r.page, /After \[z\]\(\.\.\/cli\/\)\./);
});

test('a longer fence is closed only by an equal or longer one', () => {
  const r = run(fixture('# T\n\n````md\n```\n[x](cli.md)\n```\n````\n\n[y](cli.md)\n'));
  assert.match(r.page, /\[x\]\(cli\.md\)/, 'inside the four-backtick fence');
  assert.match(r.page, /\[y\]\(\.\.\/cli\/\)/, 'after it');
});

test('mermaid blocks become escaped <pre class="mermaid"> blocks', () => {
  const r = run(fixture('# T\n\n```mermaid\nflowchart LR\n  a --> b\n\n  b --> c\n```\n\nAfter.\n'));
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.page, /<pre class="mermaid">flowchart LR\n {2}a --&gt; b\n\n {2}b --&gt; c<\/pre>/);
});

test('front matter has the title and description, the H1 is dropped', () => {
  const r = run(fixture('# Original title\n\nThe **first** paragraph has `code` and a [link](cli.md).\n\nSecond.\n'));
  assert.match(r.page, /^---\ntitle: "Getting started"\ndescription: "The first paragraph has code and a link\."\neditUrl: "https:\/\/github\.com\/afri-bit\/truss\/edit\/main\/docs\/getting-started\.md"\n---\n/);
  assert.doesNotMatch(r.page, /Original title/);
});

test('a link to a file that does not exist warns, and fails the build in strict mode', () => {
  const f = fixture('# T\n\n[gone](nope/gone.md)\n');
  const lax = run(f);
  assert.equal(lax.status, 0);
  assert.match(lax.stderr, /link target not found/);
  const strict = run(f, { SYNC_STRICT: '1' });
  assert.equal(strict.status, 1);
  assert.match(strict.stderr, /SYNC_STRICT=1/);
});

test('an image that does not exist fails strict mode too', () => {
  const strict = run(fixture('# T\n\n![x](images/missing.png)\n'), { SYNC_STRICT: '1' });
  assert.equal(strict.status, 1);
  assert.match(strict.stderr, /image not found/);
});

test('a symbolic link in docs/images is not copied', () => {
  const f = fixture('# T\n\nBody.\n');
  const logo = join(f.repo, 'docs', 'images', 'logo.svg');
  writeFileSync(join(f.repo, 'secret.txt'), 'secret');
  // Replace the logo with a link to a file outside docs/.
  unlinkSync(logo);
  symlinkSync(join(f.repo, 'secret.txt'), logo);
  const r = run(f);
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /not a regular file/);
  assert.ok(!existsSync(join(f.site, 'public', 'favicon.svg')), 'nothing was published');
});

test('a docs page that is a symbolic link is refused', () => {
  const f = fixture('# T\n\nBody.\n');
  const page = join(f.repo, 'docs', 'getting-started.md');
  writeFileSync(join(f.repo, 'secret.txt'), 'secret');
  unlinkSync(page);
  symlinkSync(join(f.repo, 'secret.txt'), page);
  const r = run(f);
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /not a regular file/);
});

test('a branch name with link characters is refused', () => {
  const r = run(fixture('# T\n\nBody.\n'), { TRUSS_DOCS_BRANCH: 'main) [x](evil' });
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /not a branch name/);
});

test('fonts are bundled assets (so the base path applies in dev too); the licence is a public file', () => {
  const f = fixture('# T\n\nBody.\n');
  const r = run(f);
  assert.equal(r.status, 0, r.stderr);
  assert.ok(existsSync(join(f.site, 'src', 'assets', 'fonts', 'a.woff2')));
  assert.ok(!existsSync(join(f.site, 'public', 'fonts', 'a.woff2')), 'not as a public path');
  assert.ok(existsSync(join(f.site, 'public', 'fonts', 'OFL.txt')));
});
