// The home page's behaviour. The page is complete without it: the server renders every picture in
// its finished state. With script, two product windows replay a run, one section reads a pipeline
// file as it is scrolled, and the menu and copy button work. Everything is a function of a time or
// a scroll position, so a pause, a resume and the finished frame all come from the same code.
import { CYCLE, LOG, STEPS, SUMMARY_AT, duration, stamp } from '../lib/run';

const root = document.documentElement;
root.classList.add('js');
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

/* ---------- menu ---------- */
(() => {
  const toggle = document.querySelector<HTMLButtonElement>('.nav__toggle');
  if (!toggle) return;
  const set = (open: boolean) => {
    root.classList.toggle('nav-open', open);
    toggle.setAttribute('aria-expanded', String(open));
  };
  toggle.addEventListener('click', () => {
    const open = !root.classList.contains('nav-open');
    set(open);
    if (open) document.querySelector<HTMLElement>('#site-nav a')?.focus(); // the sheet is first in the focus order
  });
  document.querySelectorAll('#site-nav a').forEach((a) => a.addEventListener('click', () => set(false)));
  addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !root.classList.contains('nav-open')) return;
    set(false);
    toggle.focus();
  });
  // Tabbing out of the open sheet closes it, so focus never lands on the page behind it.
  document.querySelector('.nav')?.addEventListener('focusout', (e) => {
    const next = (e as FocusEvent).relatedTarget as Node | null;
    if (next && !document.querySelector('.nav')!.contains(next)) set(false);
  });
  matchMedia('(min-width: 1024px)').addEventListener('change', (e) => e.matches && set(false));
})();

/* ---------- a run, replayed ---------- */
function driveRun(host: HTMLElement) {
  const speed = Number(host.dataset.speed || 1);
  const q = <T extends Element>(sel: string) => Array.from(host.querySelectorAll<T>(sel));
  const els = new Map<string, Element[]>();
  q<HTMLElement | SVGElement>('[data-id]').forEach((el) => {
    const id = (el as HTMLElement).dataset.id!;
    els.set(id, [...(els.get(id) ?? []), el]);
  });
  const edgeDone = q<SVGPathElement>('.edge[data-from]');
  const edgeFlow = q<SVGPathElement>('.edge-flow[data-from]');
  const status = q<HTMLElement>('[data-status]');
  const elapsed = q<HTMLElement>('[data-elapsed], [data-elapsed-side]');
  const progress = q<HTMLElement>('[data-progress]');
  const now = q<HTMLElement>('[data-now]');
  const nowLabel = q<HTMLElement>('[data-now-label]');
  const bar = q<HTMLElement>('[data-bar]');
  const live = q<HTMLElement>('[data-live]');
  const lines = host.querySelector<HTMLElement>('[data-lines]');
  const first = STEPS[0].start;
  const stateOf = (s: (typeof STEPS)[number], t: number) => (t < s.start ? 'queued' : t < s.end ? 'running' : 'ok');

  const setText = (el: Element, text: string) => {
    if (el.textContent !== text) el.textContent = text;
  };
  let shown = -1; // log lines written so far

  const apply = (t: number) => {
    const state = new Map(STEPS.map((s) => [s.id, stateOf(s, t)]));
    for (const s of STEPS) {
      const st = state.get(s.id)!;
      const time = st === 'queued' ? '' : st === 'running' ? `${((t - s.start) / 1000).toFixed(1)}s` : duration(s);
      for (const el of els.get(s.id) ?? []) {
        el.classList.remove('s-queued', 's-running', 's-ok');
        el.classList.add(`s-${st}`);
        const timeEl = el.querySelector('[data-time]');
        if (timeEl) setText(timeEl, time);
      }
    }
    for (const e of edgeDone) e.classList.toggle('done', state.get(e.dataset.from!) === 'ok');
    for (const e of edgeFlow) e.classList.toggle('on', state.get(e.dataset.to!) === 'running');

    const done = t >= SUMMARY_AT;
    const label = done ? 'succeeded' : t >= first ? 'running' : 'queued';
    for (const el of status) {
      setText(el, label);
      el.classList.toggle('chip--ok', done);
      el.classList.toggle('chip--run', !done && t >= first);
    }
    const secs = Math.max(0, Math.min(t, SUMMARY_AT) - first) / 1000;
    for (const el of elapsed) setText(el, `${secs.toFixed(1)}s`);
    const okCount = STEPS.filter((s) => state.get(s.id) === 'ok').length;
    for (const el of progress) setText(el, `${okCount} / ${STEPS.length} steps`);
    const running = STEPS.filter((s) => state.get(s.id) === 'running').map((s) => s.id);
    // Desktop shows "Running now" while a run goes on and "Ended <time>" once it is done.
    for (const el of nowLabel) setText(el, done ? 'Ended' : 'Running now');
    for (const el of now) setText(el, done ? stamp(SUMMARY_AT) : running.length ? running.join(', ') : '–');
    for (const el of bar) el.style.transform = `scaleX(${Math.max(0, Math.min(1, (t - first) / (SUMMARY_AT - first)))})`;
    for (const el of live) {
      setText(el, done ? 'done' : 'live');
      el.classList.toggle('is-live', !done && t >= first);
    }

    if (lines) {
      const visible = LOG.filter((l) => l.at <= t);
      if (visible.length !== shown) {
        if (visible.length < shown || shown < 0) lines.textContent = '';
        for (let i = Math.max(shown, 0); i < visible.length; i++) lines.append(logLine(visible[i], i));
        shown = visible.length;
      }
    }
  };

  if (reduce) return; // the server's finished frame stands

  const pause = host.querySelector<HTMLButtonElement>('[data-pause]');
  let paused = false;
  let t = -900; // the graph assembles first (the hero's entrance), then the run starts
  let last = performance.now();
  let resetTimer = 0;
  let raf = 0;
  const frame = (clock: number) => {
    const dt = Math.min(clock - last, 100);
    last = clock;
    t += dt * speed;
    if (t > CYCLE) {
      t = 0;
      host.classList.add('is-reset'); // dims, then rewrites: the window is never empty
      clearTimeout(resetTimer);
      resetTimer = window.setTimeout(() => host.classList.remove('is-reset'), 260);
    }
    apply(t);
    raf = requestAnimationFrame(frame);
  };
  pause?.addEventListener('click', () => {
    paused = !paused;
    // `is-paused` also stops the spinners and the moving lines, which are CSS animations.
    host.classList.toggle('is-paused', paused);
    pause.setAttribute('aria-pressed', String(paused));
    pause.setAttribute('aria-label', paused ? 'Play the animation' : 'Pause the animation');
    cancelAnimationFrame(raf);
    if (!paused) {
      last = performance.now();
      raf = requestAnimationFrame(frame);
    }
  });
  apply(t);
  raf = requestAnimationFrame(frame);
}

function logLine(l: (typeof LOG)[number], i: number): HTMLLIElement {
  const li = document.createElement('li');
  li.className = `ln ln--${l.kind}`;
  const span = (cls: string, text: string) => {
    const s = document.createElement('span');
    s.className = cls;
    s.textContent = text;
    return s;
  };
  li.append(span('ln__n', String(i + 1)), span('ln__t', stamp(l.at)));
  if (l.id) li.append(span('ln__s', `step ${l.id}`), span('ln__x', l.text));
  else li.append(span('ln__x', l.text));
  return li;
}

document.querySelectorAll<HTMLElement>('[data-run]').forEach(driveRun);

/* ---------- the schedule diagram's pause button ---------- */
document.querySelectorAll<HTMLButtonElement>('[data-pause]').forEach((btn) => {
  const visual = btn.closest<HTMLElement>('[data-visual]');
  if (!visual || visual.hasAttribute('data-run')) return;
  btn.addEventListener('click', () => {
    const paused = visual.classList.toggle('is-paused');
    btn.setAttribute('aria-pressed', String(paused));
    btn.setAttribute('aria-label', paused ? 'Play the animation' : 'Pause the animation');
  });
});

/* ---------- the pipeline moment: a pipeline file read as the page scrolls ---------- */
(() => {
  const section = document.querySelector<HTMLElement>('[data-moment="yaml"]');
  const stick = section?.querySelector<HTMLElement>('.moment__stick');
  if (!section || !stick) return;
  const steps = 6; // 0 to 4 are the file's parts, 5 is `truss validate`
  const parts = Array.from(section.querySelectorAll<HTMLElement | SVGElement>('[data-at]'));
  const codeLines = Array.from(section.querySelectorAll<HTMLElement>('.cl'));
  const still = () =>
    reduce || matchMedia('(max-width: 899px)').matches || matchMedia('(max-height: 640px)').matches;
  let frame = 0;
  let shown = -2;

  const update = () => {
    frame = 0;
    if (still()) {
      section.classList.remove('is-live');
      codeLines.forEach((l) => l.classList.remove('is-hot'));
      shown = -2;
      return;
    }
    section.classList.add('is-live');
    const nav = parseFloat(getComputedStyle(root).getPropertyValue('--nav')) || 72;
    const travel = section.offsetHeight - stick.offsetHeight;
    const p = travel > 0 ? Math.min(1, Math.max(0, (nav - section.getBoundingClientRect().top) / travel)) : 1;
    const step = Math.min(steps - 1, Math.floor(p * steps));
    if (step === shown) return;
    shown = step;
    for (const el of parts) el.classList.toggle('on', Number(el.getAttribute('data-at')) <= step);
    for (const l of codeLines) l.classList.toggle('is-hot', Number(l.dataset.s) === step);
  };
  const request = () => {
    if (!frame) frame = requestAnimationFrame(update);
  };
  addEventListener('scroll', request, { passive: true });
  addEventListener('resize', request);
  update();
})();

/* ---------- copy ---------- */
document.querySelectorAll<HTMLButtonElement>('[data-copy]').forEach((btn) => {
  const label = btn.getAttribute('aria-label') ?? '';
  btn.addEventListener('click', async () => {
    const text = btn.closest('[data-copy-root]')?.querySelector('[data-copy-text]')?.textContent?.trim() ?? '';
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      return;
    }
    btn.classList.add('is-done');
    btn.setAttribute('aria-label', 'Copied');
    setTimeout(() => {
      btn.classList.remove('is-done');
      btn.setAttribute('aria-label', label);
    }, 1800);
  });
});
