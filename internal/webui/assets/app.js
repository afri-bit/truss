// TRUSS Desktop. Plain JavaScript, no build step, no dependencies.
// It talks to the daemon's HTTP API (/api/v1) with the access token that
// `truss desktop` passes in the URL fragment.
//
// Resource figures (CPU, memory, step processes) come from the daemon's
// GET /metrics. In demo mode demo.js answers it with sample figures.
"use strict";

const API = "/api/v1";
const LOG_MS = 700;

// ---------- settings ----------

const DEFAULTS = { theme: "system", density: "comfortable", sidebar: "expanded", refresh: 1500, motion: "full", logWrap: false, logTs: true, logSize: 12 };
const settings = Object.assign({}, DEFAULTS, (() => {
  try { return JSON.parse(localStorage.getItem("truss-settings") || "{}"); } catch { return {}; }
})());
function saveSettings() {
  try { localStorage.setItem("truss-settings", JSON.stringify(settings)); } catch { /* private mode */ }
}
function applySettings() {
  const d = document.documentElement.dataset;
  if (settings.theme === "light" || settings.theme === "dark") d.theme = settings.theme; else delete d.theme;
  if (settings.density === "compact") d.density = "compact"; else delete d.density;
  if (settings.sidebar === "collapsed") d.sidebar = "collapsed"; else delete d.sidebar;
  if (settings.motion === "reduced") d.motion = "reduced"; else delete d.motion;
  d.logwrap = settings.logWrap ? "on" : "off";
  d.logts = settings.logTs ? "on" : "off";
  d.logsize = String(settings.logSize);
  const collapse = document.getElementById("collapse");
  const collapsed = settings.sidebar === "collapsed";
  collapse.setAttribute("aria-expanded", String(!collapsed));
  collapse.setAttribute("aria-label", collapsed ? "Expand sidebar" : "Collapse sidebar");
  collapse.title = (collapsed ? "Expand sidebar" : "Collapse sidebar") + "  [";
  document.getElementById("theme-toggle").replaceChildren(icon(isDark() ? "sun" : "moon"));
}
function setSetting(key, value) {
  settings[key] = value;
  saveSettings();
  applySettings();
  paintMenu();
  route(true);
}
const isDark = () => settings.theme === "dark" || (settings.theme === "system" && matchMedia("(prefers-color-scheme: dark)").matches);

// ---------- token ----------

const store = {
  get() { try { return localStorage.getItem("truss-token") || ""; } catch { return ""; } },
  set(v) { try { localStorage.setItem("truss-token", v); } catch { /* private mode */ } },
};
let token = store.get();
(function takeTokenFromAddress() {
  const m = /^#token=([0-9a-f]+)$/.exec(location.hash);
  if (!m) return;
  token = m[1];
  store.set(token);
  // A token link always means the real daemon, even in a tab that was in demo mode.
  try { sessionStorage.removeItem("truss-demo"); } catch { /* private mode */ }
  history.replaceState(null, "", location.pathname + "#/");
})();

// Demo mode (`truss desktop --demo`): sample data from demo.js, no daemon. Only the demo
// server has demo.js; on a daemon's page the script is missing and the page stays real.
// It lasts for this browser tab only.
let demo = null;
const demoWanted = (() => {
  let on = false;
  try { on = sessionStorage.getItem("truss-demo") === "1"; } catch { /* private mode */ }
  if (location.hash === "#demo") {
    on = true;
    try { sessionStorage.setItem("truss-demo", "1"); } catch { /* still on for this page */ }
    history.replaceState(null, "", location.pathname + "#/");
  }
  return on;
})();
function loadDemo() {
  return new Promise((resolve) => {
    const off = () => { try { sessionStorage.removeItem("truss-demo"); } catch { /* private mode */ } resolve(null); };
    const el = document.createElement("script");
    el.src = "demo.js";
    el.onload = () => (window.TrussDemo ? resolve(window.TrussDemo) : off());
    el.onerror = off;
    document.head.append(el);
  });
}
function leaveDemo() {
  try { sessionStorage.removeItem("truss-demo"); } catch { /* private mode */ }
  location.hash = "#/";
  location.reload();
}

// ---------- small helpers ----------

// h builds a DOM element. Text always goes in as text, never as markup.
function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === false || v == null) continue;
    if (k === "class") el.className = v;
    else if (k === "style") el.style.cssText = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat(Infinity)) {
    if (c == null || c === false) continue;
    el.append(c.nodeType ? c : document.createTextNode(String(c)));
  }
  return el;
}
const SVG = "http://www.w3.org/2000/svg";
function s(tag, attrs, ...children) {
  const el = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null) continue;
    if (k === "style") el.style.cssText = v; // the daemon's CSP blocks style attributes, not the CSSOM
    else el.setAttribute(k, v);
  }
  for (const c of children.flat()) if (c != null) el.append(c.nodeType ? c : document.createTextNode(String(c)));
  return el;
}

// Icons: one 16px grid, 1.5px stroke, drawn for this app.
const ICONS = {
  dashboard: '<rect x="2.25" y="2.25" width="5" height="6" rx="1"/><rect x="8.75" y="2.25" width="5" height="3.5" rx="1"/><rect x="8.75" y="7.25" width="5" height="6.5" rx="1"/><rect x="2.25" y="9.75" width="5" height="4" rx="1"/>',
  pipelines: '<circle cx="3.75" cy="4" r="1.75"/><circle cx="3.75" cy="12" r="1.75"/><circle cx="12.25" cy="8" r="1.75"/><path d="M5.5 4h1.25a2 2 0 0 1 2 2v0a2 2 0 0 0 2 2h0M5.5 12h1.25a2 2 0 0 0 2-2v0a2 2 0 0 1 2-2"/>',
  runs: '<path d="M2.5 4h11M2.5 8h5.5M2.5 12h5.5"/><path d="M10.5 9.25v4.5l3.75-2.25z"/>',
  monitor: '<path d="M1.5 8.5h2.75l1.75-4.5 3 8.5 1.75-4h3.75"/>',
  settings: '<path d="M2.5 4.5h6M11.5 4.5h2M2.5 11.5h2M7.5 11.5h6"/><circle cx="10" cy="4.5" r="1.5"/><circle cx="6" cy="11.5" r="1.5"/>',
  search: '<circle cx="7" cy="7" r="4.25"/><path d="m10.25 10.25 3 3"/>',
  sun: '<circle cx="8" cy="8" r="2.75"/><path d="M8 1.75v1.5M8 12.75v1.5M1.75 8h1.5M12.75 8h1.5M3.6 3.6l1.05 1.05M11.35 11.35l1.05 1.05M3.6 12.4l1.05-1.05M11.35 4.65l1.05-1.05"/>',
  moon: '<path d="M13.25 9.75A5.5 5.5 0 0 1 6.25 2.75a5.5 5.5 0 1 0 7 7z"/>',
  play: '<path d="M5 3.25v9.5l7.25-4.75z"/>',
  pause: '<path d="M5.75 3.5v9M10.25 3.5v9"/>',
  stop: '<rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1.25"/>',
  rotate: '<path d="M13.25 8a5.25 5.25 0 1 1-1.6-3.78"/><path d="M13.25 2.25v3.25H10"/>',
  trash: '<path d="M2.75 4.25h10.5M6.25 4.25V2.75h3.5v1.5M4.25 4.25l.6 9h6.3l.6-9"/>',
  folder: '<path d="M1.75 4.25a1 1 0 0 1 1-1h3.1l1.5 1.5h5.9a1 1 0 0 1 1 1v6.5a1 1 0 0 1-1 1H2.75a1 1 0 0 1-1-1z"/>',
  file: '<path d="M3.75 1.75h5.5l3 3v9.5h-8.5z"/><path d="M9 1.75v3.25h3.25M5.75 8.5h4.5M5.75 11h3"/>',
  up: '<path d="M8 13.25V2.75M3.75 7 8 2.75 12.25 7"/>',
  home: '<path d="M2.25 7.25 8 2.5l5.75 4.75"/><path d="M3.75 6.25v7.25h8.5V6.25"/>',
  alert: '<path d="M8 2.25 14.25 13.25H1.75z"/><path d="M8 6.5v3M8 11.4v.1"/>',
  cpu: '<rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1"/><rect x="6.25" y="6.25" width="3.5" height="3.5"/><path d="M6 1.5v2.25M10 1.5v2.25M6 12.25v2.25M10 12.25v2.25M1.5 6h2.25M1.5 10h2.25M12.25 6h2.25M12.25 10h2.25"/>',
  memory: '<rect x="1.75" y="4.25" width="12.5" height="6.5" rx="1"/><path d="M4.5 10.75v2M7 10.75v2M9.5 10.75v2M12 10.75v2M4.5 6.75v1.5M7 6.75v1.5M9.5 6.75v1.5M12 6.75v1.5"/>',
  heart: '<path d="M8 13.25S2.25 10 2.25 6.1A2.85 2.85 0 0 1 8 4.75 2.85 2.85 0 0 1 13.75 6.1C13.75 10 8 13.25 8 13.25z"/>',
  clock: '<circle cx="8" cy="8" r="6.25"/><path d="M8 4.5V8l2.25 1.5"/>',
  wrap: '<path d="M2.5 4h11M2.5 8h9a2 2 0 0 1 0 4H8.5"/><path d="M10 10.5 8.5 12l1.5 1.5M2.5 12H5"/>',
  follow: '<path d="M8 2.5v8M4.5 7 8 10.5 11.5 7M3 13.5h10"/>',
  terminal: '<rect x="1.75" y="2.75" width="12.5" height="10.5" rx="1.5"/><path d="m4.5 6 2 2-2 2M8.5 10.5h3"/>',
  x: '<path d="M4 4l8 8M12 4l-8 8"/>',
  key: '<circle cx="5.25" cy="10.75" r="2.75"/><path d="M7.25 8.75 13.25 2.75M11 5l1.75 1.75"/>',
  layers: '<path d="M8 2 14 5 8 8 2 5z"/><path d="m2 8 6 3 6-3M2 11l6 3 6-3"/>',
  sidebar: '<rect x="1.75" y="2.25" width="12.5" height="11.5" rx="2"/><path d="M6 2.5v11"/>',
  info: '<circle cx="8" cy="8" r="6.25"/><path d="M8 7.25v4M8 4.9v.1"/>',
  plus: '<path d="M8 3v10M3 8h10"/>',
  copy: '<rect x="5.25" y="5.25" width="8.5" height="8.5" rx="1"/><path d="M10.75 5.25V3.25a1 1 0 0 0-1-1h-6.5a1 1 0 0 0-1 1v6.5a1 1 0 0 0 1 1h2"/>',
  calendar: '<rect x="2" y="3" width="12" height="11" rx="1.5"/><path d="M2 6.5h12M5.25 1.75v2.5M10.75 1.75v2.5"/>',
  link: '<path d="M6.75 9.25a2.5 2.5 0 0 0 3.5 0l2.5-2.5a2.5 2.5 0 0 0-3.5-3.5l-.75.75M9.25 6.75a2.5 2.5 0 0 0-3.5 0l-2.5 2.5a2.5 2.5 0 0 0 3.5 3.5l.75-.75"/>',
};
function icon(name, cls) {
  const el = document.createElementNS(SVG, "svg");
  el.setAttribute("viewBox", "0 0 16 16");
  el.setAttribute("class", "i" + (cls ? " " + cls : ""));
  el.setAttribute("aria-hidden", "true");
  el.innerHTML = ICONS[name] || ""; // static, authored markup above
  return el;
}

// Status icons, shared by the step list, the graph and the finder.
function statusGlyph(status, size = 16) {
  const g = s("svg", { viewBox: "0 0 16 16", width: size, height: size, class: "stico", "aria-hidden": "true" });
  const add = (cls, ...shapes) => { g.setAttribute("class", "stico " + cls); shapes.forEach((x) => g.append(x)); };
  switch (status) {
    case "succeeded": add("ok", s("circle", { cx: 8, cy: 8, r: 6.25 }), s("path", { d: "m5.25 8.25 1.9 1.9 3.6-3.9" })); break;
    case "failed": add("bad", s("circle", { cx: 8, cy: 8, r: 6.25 }), s("path", { d: "M5.9 5.9l4.2 4.2M10.1 5.9l-4.2 4.2" })); break;
    case "running": add("run spin", s("circle", { cx: 8, cy: 8, r: 6.25, opacity: 0.25 }), s("path", { d: "M8 1.75A6.25 6.25 0 0 1 14.25 8" })); g.style.animationDelay = phase(900); break;
    case "tolerated": add("warn", s("path", { d: "M8 2.25 14.25 13.25H1.75z" }), s("path", { d: "M8 6.5v3M8 11.4v.1" })); break;
    case "queued": add("warn", s("circle", { cx: 8, cy: 8, r: 6.25 }), s("path", { d: "M8 4.75V8l2 1.25" })); break;
    case "skipped": case "cancelled": add("off", s("circle", { cx: 8, cy: 8, r: 6.25 }), s("path", { d: "M4.6 11.4l6.8-6.8" })); break;
    default: add("off", s("circle", { cx: 8, cy: 8, r: 6.25, "stroke-dasharray": "2.4 2.4" }));
  }
  return g;
}
// phase keeps CSS loops continuous across re-renders: every repaint starts the loop where the clock is.
const phase = (period) => `-${Date.now() % period}ms`;

class ApiError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}
async function api(method, path, body) {
  const res = demo ? await demo.request(method, path, body) : await fetch(API + path, {
    method,
    signal: AbortSignal.timeout(10000), // a daemon that accepts but never answers must not stall the page
    headers: Object.assign({ Authorization: "Bearer " + token }, body ? { "Content-Type": "application/json" } : {}),
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let message = res.statusText;
    try { message = (await res.json()).error || message; } catch { /* not JSON */ }
    throw new ApiError(res.status, message);
  }
  return res;
}
const getJSON = async (path) => (await api("GET", path)).json();

function toast(message, bad) {
  const el = h("div", { class: bad ? "bad" : "" }, message);
  document.getElementById("toast").append(el);
  setTimeout(() => el.remove(), bad ? 7000 : 3500);
}
async function act(label, fn) {
  try { await fn(); toast(label); refresh(true); } catch (e) { toast(e.message, true); }
}

const pad = (n) => String(n).padStart(2, "0");
function clock(iso) {
  if (!iso) return "–";
  const d = new Date(iso), now = new Date();
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  if (d.toDateString() === now.toDateString()) return time;
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${time}`;
}
function span(ms) {
  if (ms < 0) ms = 0;
  if (ms < 1000) return `${Math.round(ms)} ms`;
  const sec = ms / 1000;
  if (sec < 60) return `${sec.toFixed(1)} s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min} min ${Math.round(sec % 60)} s`;
  return `${Math.floor(min / 60)} h ${min % 60} min`;
}
function compact(ms) {
  if (ms < 0) ms = 0;
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${(ms / 1000).toFixed(1)}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ${pad(sec % 60)}s`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ${pad(min % 60)}m`;
  return `${Math.floor(hr / 24)}d ${hr % 24}h`;
}
function took(item) {
  if (!item.started_at) return "–";
  const end = item.finished_at ? new Date(item.finished_at) : new Date();
  return span(end - new Date(item.started_at));
}
function tookCompact(item) {
  if (!item.started_at) return "";
  const end = item.finished_at ? new Date(item.finished_at) : new Date();
  return compact(end - new Date(item.started_at));
}
function until(iso) {
  const ms = new Date(iso) - new Date();
  return ms <= 0 ? "now" : "in " + span(ms);
}
function ago(iso) {
  if (!iso) return "–";
  const ms = new Date() - new Date(iso);
  if (ms < 60000) return "just now";
  if (ms < 3600000) return `${Math.floor(ms / 60000)} min ago`;
  if (ms < 86400000) return `${Math.floor(ms / 3600000)} h ago`;
  return `${Math.floor(ms / 86400000)} d ago`;
}
const badge = (status) => h("span", { class: "badge s-" + status }, status);
// A step that failed but let the run go on (continue-on-error) is shown apart
// from a failure that failed the run.
const stepState = (st) => (st.status === "failed" && st.continued_on_error ? "tolerated" : st.status);
const isActive = (run) => run.status === "queued" || run.status === "running";
function pipelineState(p) {
  if (!p.valid) return "invalid";
  if (p.active_runs > 0) return "running";
  return p.paused ? "paused" : "ready";
}
function nextOf(p) {
  const times = (p.schedules || []).map((x) => x.next).filter(Boolean).sort();
  return times[0] || null;
}
const href = { pipeline: (name) => "#/pipelines/" + encodeURIComponent(name), run: (id) => "#/runs/" + encodeURIComponent(id) };
const go = (hash) => { location.hash = hash; };
const shortId = (id) => (id.length > 14 ? id.slice(0, 14) : id);
// In demo mode every figure is sample data; panels with resource figures say so.
const simTag = () => demo && h("span", { class: "sim", title: "demo mode: sample figures, no daemon" }, "sample");

// ---------- resource figures from the daemon ----------
// res keeps the latest GET /metrics answer, plus each active run's figures as
// seen by this page, so a run's chart can show more than the latest sample.

const GiB = 1024 ** 3, MiB = 1024 ** 2;
const res = {
  m: null, // the latest api.Metrics
  runs: new Map(), // run id -> { at: [], cpu: [], mem: [] }
  ok() { return !!(this.m && this.m.supported && this.m.host); },
  why() {
    if (!this.m) return "Waiting for the first sample from the daemon…";
    if (!this.m.supported) return `The daemon does not collect resource figures on ${this.m.os}. ${this.m.reason || ""}`.trim();
    return "Waiting for the first sample from the daemon…";
  },
  host() {
    const h = this.m.host;
    return { cpu: h.cpu_percent, cores: h.cores, memUsed: h.memory_used_bytes / GiB, memTotal: h.memory_total_bytes / GiB, load: h.load || null };
  },
  history() {
    const pts = (this.m && this.m.history) || [];
    return { cpu: pts.map((p) => p.cpu_percent), mem: pts.map((p) => p.memory_used_bytes / GiB), seconds: pts.length ? (Date.parse(pts[pts.length - 1].at) - Date.parse(pts[0].at)) / 1000 : 0 };
  },
  step(runId, stepId) { return (this.m && (this.m.steps || []).find((x) => x.run_id === runId && x.step_id === stepId)) || null; },
  run(runId) {
    const parts = ((this.m && this.m.steps) || []).filter((x) => x.run_id === runId);
    return { cpu: parts.reduce((a, b) => a + b.cpu_percent, 0), mem: parts.reduce((a, b) => a + b.rss_bytes, 0) / MiB, procs: parts.reduce((a, b) => a + b.processes, 0), steps: parts.length };
  },
  ingest(m) {
    const fresh = !this.m || m.sampled_at !== this.m.sampled_at;
    this.m = m;
    if (!fresh || !m.supported) return;
    const ids = new Set((m.steps || []).map((x) => x.run_id));
    for (const id of ids) {
      const r = this.runs.get(id) || { cpu: [], mem: [] };
      const u = this.run(id);
      r.cpu.push(u.cpu); r.mem.push(u.mem);
      if (r.cpu.length > 150) { r.cpu.shift(); r.mem.shift(); }
      this.runs.set(id, r);
    }
    for (const id of [...this.runs.keys()]) if (!ids.has(id) && !shell.runs.some((r) => r.id === id && isActive(r))) this.runs.delete(id);
  },
};
// ticksFor labels a chart that spans the given number of seconds.
function ticksFor(seconds) {
  if (seconds < 20) return ["", "", "", "", "now"];
  const fmt = (s) => (s >= 120 ? `-${Math.round(s / 60)}m` : `-${Math.round(s)}s`);
  return [fmt(seconds), fmt(seconds * 0.75), fmt(seconds / 2), fmt(seconds / 4), "now"];
}
const bytes = (n) => (n >= GiB ? (n / GiB).toFixed(1) + " GiB" : (n / MiB).toFixed(0) + " MiB");

// ---------- shared state from the shell poll ----------

const shell = { status: null, runs: [], up: null, latency: null, beats: [], pipelines: [] };

// ---------- shared pieces ----------

// noFigures stands where resource figures would be when the daemon has none.
const noFigures = () => h("div", { class: "empty" }, res.why());

function meter(kind, pct, label) {
  const p = Math.max(0, Math.min(100, pct));
  return h("div", { class: "meter " + kind + (p > 85 ? " hot" : "") },
    h("span", null, kind === "mem" ? "MEM" : "CPU"),
    h("div", { class: "track" }, h("i", { style: `width:${p.toFixed(1)}%` })),
    h("span", { class: "v" }, label));
}
function runMeters(r) {
  if (r.status === "queued") return h("span", { class: "muted mono" }, "waiting");
  if (!isActive(r)) return h("span", { class: "muted mono" }, "exited");
  if (r.trigger === "local") return h("span", { class: "muted", title: "the run executes in a shell outside the daemon" }, "not measured");
  if (!res.ok()) return h("span", { class: "muted", title: res.why() }, "not reported");
  const u = res.run(r.id);
  if (!u.steps) return h("span", { class: "muted mono" }, "starting");
  const memPct = res.m.host.memory_total_bytes ? (u.mem * MiB / res.m.host.memory_total_bytes) * 100 : 0;
  return h("div", { class: "meters" }, meter("cpu", u.cpu, u.cpu.toFixed(1) + "%"), meter("mem", memPct, u.mem.toFixed(0) + "M"));
}
function progress(r) {
  const steps = r.steps || [];
  if (!steps.length) return h("span", { class: "muted mono" }, "–");
  const done = steps.filter((st) => !["pending", "running", "queued"].includes(st.status)).length;
  return h("div", { style: "display:grid;grid-template-columns:minmax(80px,1fr) 38px;gap:8px;align-items:center" },
    h("div", { class: "progress", "aria-label": `${done} of ${steps.length} steps done` }, steps.map((st) => h("i", { class: "s-" + stepState(st), title: `${st.id}: ${stepState(st)}` }))),
    h("span", { class: "mono muted", style: "font-size:11.5px;text-align:right" }, `${done}/${steps.length}`));
}

// A local run executes in someone's shell (truss run --track); the daemon only records it.
function triggerLabel(trigger) {
  return trigger === "local" ? h("span", { class: "tag", title: "ran in a shell outside the daemon (truss run --track)" }, "local") : h("span", { class: "trig" }, trigger);
}

function runsTable(runs, opts = {}) {
  const { withPipeline = true, live = false, empty, hide, note = true } = opts;
  if (!runs.length) {
    return h("div", { class: "empty" }, empty || ["No runs yet. Start one with ", h("code", null, "truss trigger <name>"), " or the Run now button."]);
  }
  const open = (r) => () => go(href.run(r.id));
  return h("table", null,
    h("thead", null, h("tr", null,
      h("th", null, "Status"), h("th", null, "Run"), withPipeline && h("th", null, "Pipeline"), h("th", null, "Trigger"),
      h("th", null, "Steps"), live && h("th", null, h("span", null, "Resources")),
      h("th", null, "Created"), h("th", { class: "num" }, "Duration"), !live && note && h("th", null, "Note"))),
    h("tbody", null, runs.map((r) => h("tr", { class: "link", onclick: open(r), hidden: hide ? hide(r) : false, "data-filter": [r.pipeline, r.id, r.trigger].join(" ").toLowerCase() },
      h("td", null, h("a", { class: "rowlink", href: href.run(r.id), "aria-label": `Run ${r.id}, ${r.status}`, onclick: (e) => e.stopPropagation() }, badge(r.status))),
      h("td", null, h("span", { class: "id", title: r.id }, shortId(r.id))),
      withPipeline && h("td", null, h("a", { class: "pipe", href: href.pipeline(r.pipeline), onclick: (e) => e.stopPropagation() }, r.pipeline)),
      h("td", null, triggerLabel(r.trigger)),
      h("td", { style: "width:150px" }, progress(r)),
      live && h("td", { style: "width:190px" }, runMeters(r)),
      h("td", { class: "mono", title: r.created_at }, clock(r.created_at)),
      h("td", { class: "num" }, took(r)),
      !live && note && h("td", { class: "note", title: r.reason || "" }, r.reason || (r.scheduled_for ? "scheduled for " + clock(r.scheduled_for) : ""))))));
}

function pipelineActions(p, compactMode) {
  const name = encodeURIComponent(p.name);
  const hasSchedule = (p.schedules || []).length > 0;
  const pauseBtn = !hasSchedule
    ? h("button", { disabled: true, title: "This pipeline has no schedule" }, icon("pause", "sm"), "Pause schedule")
    : p.paused
      ? h("button", { onclick: () => act(`Schedules of ${p.name} resumed`, () => api("POST", `/pipelines/${name}/resume`)) }, icon("play", "sm"), "Resume schedule")
      : h("button", { onclick: () => act(`Schedules of ${p.name} paused`, () => api("POST", `/pipelines/${name}/pause`)) }, icon("pause", "sm"), "Pause schedule");
  return h("div", { class: "actions" },
    h("button", { class: "primary", disabled: !p.valid, onclick: () => act(`Run of ${p.name} requested`, async () => {
      const run = await (await api("POST", `/pipelines/${name}/runs`)).json();
      go(href.run(run.id));
    }) }, icon("play", "sm"), "Run now"),
    pauseBtn,
    compactMode && h("span", { class: "spacer" }),
    h("button", { class: "danger" + (compactMode ? " icon-only" : ""), "aria-label": "Unregister", title: `Unregister ${p.name}`, onclick: () => {
      if (!confirm(`Unregister ${p.name}? The file and its run history stay.`)) return;
      act(`${p.name} unregistered`, async () => { await api("DELETE", `/pipelines/${name}`); go("#/pipelines"); });
    } }, icon("trash", "sm"), !compactMode && "Unregister"));
}

function scheduleText(p) {
  if (!(p.schedules || []).length) return h("span", { class: "muted" }, "none – runs on demand");
  const sc = p.schedules[0];
  return [h("code", null, sc.cron), sc.timezone ? ` ${sc.timezone}` : "", p.schedules.length > 1 ? h("span", { class: "muted" }, ` +${p.schedules.length - 1}`) : ""];
}
function scheduleFacts(p) {
  if (!(p.schedules || []).length) return [h("dt", null, "Schedule"), h("dd", null, "none – runs on demand")];
  return p.schedules.flatMap((sc) => [
    h("dt", null, "Schedule"),
    h("dd", null, h("code", null, sc.cron), sc.timezone ? ` ${sc.timezone}` : "",
      p.paused ? " · paused" : sc.next ? ` · next ${clock(sc.next)} (${until(sc.next)})` : "",
      sc.missed === "run" ? " · catches up" : ""),
  ]);
}

function panel(title, opts, ...body) {
  const { hint, tools, cls, id, sim: simulated } = opts || {};
  return h("section", { class: "panel " + (cls || ""), id },
    h("div", { class: "panel-head" }, h("h2", null, title), hint && h("span", { class: "hint" }, hint), simulated && simTag(), tools && h("div", { class: "tools" }, tools)),
    ...body);
}

// ---------- step graph ----------
// Left to right, one column per dependency depth, like a CI workflow view.

function layoutSteps(steps) {
  const byId = new Map(steps.map((st) => [st.id, st]));
  const depth = new Map();
  const depthOf = (id) => {
    if (depth.has(id)) return depth.get(id);
    depth.set(id, 0); // guards against a cycle; the daemon rejects those anyway
    const d = Math.max(-1, ...(byId.get(id).needs || []).filter((n) => byId.has(n)).map(depthOf)) + 1;
    depth.set(id, d);
    return d;
  };
  const cols = [];
  for (const st of steps) (cols[depthOf(st.id)] ||= []).push(st);
  return cols;
}

function graph(steps, statusOf, selected, onSelect, timing) {
  const W = 188, H = 52, GX = 44, GY = 14, PAD = 4, HEAD = 22;
  const cols = layoutSteps(steps);
  const tallest = Math.max(...cols.map((c) => c.length));
  const pos = new Map();
  cols.forEach((col, x) => col.forEach((st, y) => pos.set(st.id, { x: PAD + x * (W + GX), y: HEAD + PAD + y * (H + GY) })));
  const width = PAD * 2 + cols.length * W + (cols.length - 1) * GX;
  const height = HEAD + PAD * 2 + tallest * (H + GY) - GY;
  const svg = s("svg", { width, height, viewBox: `0 0 ${width} ${height}`, role: "img", "aria-label": "Step graph" });
  cols.forEach((col, x) => svg.append(s("text", { x: PAD + x * (W + GX), y: 12, class: "colhead", fill: "var(--text-3)", "font-size": "10.5", "font-family": "var(--mono)" }, `stage ${x + 1} · ${col.length} step${col.length === 1 ? "" : "s"}`)));
  const R = 8;
  for (const st of steps) {
    for (const need of st.needs || []) {
      const a = pos.get(need), b = pos.get(st.id);
      if (!a) continue;
      const x1 = a.x + W, y1 = a.y + H / 2, x2 = b.x, y2 = b.y + H / 2, mid = x1 + GX / 2;
      let d;
      if (Math.abs(y2 - y1) < 1) d = `M${x1},${y1} H${x2}`;
      else {
        const dir = y2 > y1 ? 1 : -1, r = Math.min(R, Math.abs(y2 - y1) / 2);
        d = `M${x1},${y1} H${mid - r} Q${mid},${y1} ${mid},${y1 + dir * r} V${y2 - dir * r} Q${mid},${y2} ${mid + r},${y2} H${x2}`;
      }
      const from = statusOf({ id: need }), to = statusOf(st);
      svg.append(s("path", { class: "edge" + (from === "succeeded" || from === "tolerated" ? " done" : ""), d }));
      if (to === "running") svg.append(s("path", { class: "edge-flow", d, style: `animation-delay:${phase(900)}` }));
    }
  }
  for (const st of steps) {
    const p = pos.get(st.id);
    const slash = st.id.indexOf("/");
    const job = slash > 0 ? st.id.slice(0, slash) : "";
    const name = slash > 0 ? st.id.slice(slash + 1) : st.id;
    const status = statusOf(st);
    const g = s("g", { class: `node s-${status}` + (selected === st.id ? " selected" : ""), transform: `translate(${p.x},${p.y})`, role: onSelect ? "button" : "img", "aria-label": `${st.id}: ${status}` });
    g.append(s("title", null, `${st.id} – ${status}`));
    g.append(s("rect", { class: "box", width: W, height: H, rx: 6 }));
    // Nested <svg> elements cannot spin; move the glyph's shapes into a group that can.
    const glyph = statusGlyph(status, 16);
    const inner = s("g", { class: glyph.getAttribute("class"), style: glyph.style.cssText || null });
    inner.append(...glyph.childNodes);
    g.append(s("g", { transform: `translate(12,${H / 2 - 8})` }, inner));
    const clip = (t, n) => (t.length > n ? t.slice(0, n - 1) + "…" : t);
    const t = timing && timing(st);
    if (job) g.append(s("text", { class: "job", x: 38, y: 20 }, clip(job, 21)));
    g.append(s("text", { x: 38, y: job ? 37 : 30 }, clip(name, t ? 15 : 20)));
    if (t) g.append(s("text", { class: "dur", x: W - 12, y: job ? 37 : 30, "text-anchor": "end" }, t));
    if (onSelect) {
      g.addEventListener("click", () => onSelect(st.id));
      g.setAttribute("tabindex", "0");
      g.setAttribute("aria-pressed", selected === st.id ? "true" : "false");
      g.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onSelect(st.id); } });
    }
    svg.append(g);
  }
  return h("div", { class: "graph" }, svg);
}
const legend = () => h("div", { class: "graph-legend" },
  h("span", null, h("i", { class: "ok" }), "succeeded"), h("span", null, h("i", { class: "run" }), "running"),
  h("span", null, h("i", { class: "bad" }), "failed"), h("span", null, h("i"), "pending · skipped"));

// ---------- charts ----------

function lineChart(series, opts = {}) {
  const W = opts.width || 560, H = opts.height || 140, L = 34, R = 8, T = 8, B = 20;
  const max = opts.max || Math.max(1, ...series.flatMap((x) => x.values)) * 1.15;
  const n = Math.max(...series.map((x) => x.values.length));
  const X = (i) => L + (i / Math.max(1, n - 1)) * (W - L - R);
  // A value outside 0..max stays on the plot edge instead of drawing over the page.
  const Y = (v) => T + (1 - Math.min(max, Math.max(0, v)) / max) * (H - T - B);
  const svg = s("svg", { class: "chart", viewBox: `0 0 ${W} ${H}`, preserveAspectRatio: "none", role: "img", "aria-label": opts.label || "chart" });
  for (let k = 0; k <= 4; k++) {
    const v = (max / 4) * k, y = Y(v);
    svg.append(s("line", { class: "gridline", x1: L, x2: W - R, y1: y, y2: y }));
    svg.append(s("text", { class: "axis", x: L - 6, y: y + 3.5, "text-anchor": "end" }, opts.fmt ? opts.fmt(v) : Math.round(v)));
  }
  const ticks = opts.ticks || ["-60s", "-45s", "-30s", "-15s", "now"];
  ticks.forEach((t, i) => svg.append(s("text", { class: "axis", x: L + (i / (ticks.length - 1)) * (W - L - R), y: H - 4, "text-anchor": i === 0 ? "start" : i === ticks.length - 1 ? "end" : "middle" }, t)));
  series.forEach((ser, si) => {
    const pts = ser.values.map((v, i) => `${X(i + (n - ser.values.length)).toFixed(1)},${Y(v).toFixed(1)}`);
    if (!pts.length) return;
    const cls = ser.cls || "s" + (si + 1);
    if (ser.area) svg.append(s("path", { class: "area " + cls, d: `M${pts[0].split(",")[0]},${Y(0)} L${pts.join(" L")} L${X(n - 1)},${Y(0)} Z` }));
    svg.append(s("path", { class: "line " + cls, d: "M" + pts.join(" L"), "vector-effect": "non-scaling-stroke" }));
  });
  return svg;
}

// ---------- views ----------

const views = {};
const draft = { path: "", name: "" };
const ui = { runFilter: "all", runQuery: "", pipeQuery: "", pipeView: "grid" };

views.dashboard = {
  nav: "dashboard",
  crumbs: () => [["Dashboard"]],
  async load() {
    const [status, pipelines, runs] = await Promise.all([getJSON("/status"), getJSON("/pipelines"), getJSON("/runs?limit=200")]);
    return { status, pipelines, runs };
  },
  render({ status, pipelines, runs }) {
    const today = new Date().toDateString();
    const todays = runs.filter((r) => new Date(r.created_at).toDateString() === today);
    const count = (st) => todays.filter((r) => r.status === st).length;
    const active = runs.filter(isActive);
    const upcoming = pipelines.filter((p) => !p.paused && nextOf(p)).sort((a, b) => nextOf(a).localeCompare(nextOf(b)));
    const finished = todays.filter((r) => r.status === "succeeded" || r.status === "failed");
    const rate = finished.length ? Math.round((count("succeeded") / finished.length) * 100) + "% success" : "no finished runs";
    const durations = todays.filter((r) => r.status === "succeeded" && r.started_at && r.finished_at).map((r) => new Date(r.finished_at) - new Date(r.started_at));
    const avg = durations.length ? "avg " + compact(durations.reduce((a, b) => a + b, 0) / durations.length) : "–";
    const scheduled = pipelines.filter((p) => (p.schedules || []).length).length;
    const tile = (cls, num, label, foot) => h("div", { class: "tile " + cls }, h("div", { class: "label" }, h("i"), label), h("div", { class: "num" }, num), h("div", { class: "foot" }, foot));
    const up = shell.status ? Date.now() - new Date(shell.status.started_at) : 0;
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, "Dashboard"),
        h("p", { class: "sub" }, `Up to ${status.max_parallel_runs} runs at once · daemon ${status.version} · up ${compact(up)}`)),
      h("span", { class: "spacer" }),
      h("a", { class: "btn", href: "#/pipelines" }, icon("plus", "sm"), "Register pipeline")),
      h("div", { class: "tiles" },
        tile("", pipelines.length, "Pipelines registered", `${scheduled} on a schedule`),
        tile("run", status.running_runs, "Running now", `${status.running_runs} of ${status.max_parallel_runs} slots`),
        tile("wait", status.queued_runs, "Queued", status.queued_runs ? "waiting for a slot" : "queue empty"),
        tile("ok", count("succeeded"), "Succeeded today", avg),
        tile("bad", count("failed"), "Failed today", rate),
        tile("", count("skipped"), "Skipped today", "overlap or missed")),
      h("div", { class: "grid-main" },
        h("div", { class: "stack" },
          panel("Active runs", { hint: active.length ? `${active.length} in flight` : "", sim: active.length > 0 },
            active.length ? h("div", { class: "scroll" }, runsTable(active, { live: true })) : h("div", { class: "empty" }, h("div", { class: "empty-title" }, "Nothing is running."), "Runs appear here the moment they start.")),
          panel("Activity", { hint: "runs started per hour, last 24 hours" }, h("div", { class: "panel-body" }, activity(runs))),
          panel("Recent runs", { tools: h("a", { class: "btn", href: "#/runs" }, "All runs") }, h("div", { class: "scroll" }, runsTable(runs.slice(0, 10), { note: false })))),
        h("div", { class: "stack" },
          panel("System", { sim: true, hint: res.ok() ? "this machine" : "", tools: h("a", { class: "btn", href: "#/monitor" }, icon("monitor", "sm"), "Monitor") },
            !res.ok() ? noFigures() : systemBody()),
          panel("Health", { hint: healthSummary().text }, healthList(true)),
          panel("Coming up", { hint: upcoming.length ? `${upcoming.length} scheduled` : "" },
            upcoming.length
              ? h("table", null, h("tbody", null, upcoming.slice(0, 8).map((p) => h("tr", { class: "link", onclick: () => go(href.pipeline(p.name)) },
                h("td", null, h("a", { class: "pipe", href: href.pipeline(p.name), onclick: (e) => e.stopPropagation() }, p.name)),
                h("td", null, h("code", null, p.schedules[0].cron)),
                h("td", { class: "mono" }, clock(nextOf(p)).slice(-8)), h("td", { class: "num" }, new Date(nextOf(p)) <= new Date() ? "now" : "in " + compact(new Date(nextOf(p)) - new Date()))))))
              : h("div", { class: "empty" }, h("div", { class: "empty-title" }, "No schedules."), "Add ", h("code", null, "on.schedule"), " to a pipeline file.")))),
    ];
  },
};

function systemBody() {
  const host = res.host(), hist = res.history();
  const last = Math.min(hist.cpu.length, 60); // two minutes at the default interval
  const span = hist.seconds * (last / Math.max(1, hist.cpu.length));
  return h("div", { class: "panel-body", style: "display:grid;gap:14px" },
    h("div", { class: "metric-head" }, h("span", { class: "big" }, host.cpu.toFixed(1)), h("span", { class: "unit" }, "% CPU"),
      h("span", { class: "legend" }, h("span", null, h("i"), "cpu"), h("span", null, h("i", { class: "s2" }), "mem"))),
    lineChart([{ values: hist.cpu.slice(-last), area: true }, { values: hist.mem.slice(-last).map((m) => (m / host.memTotal) * 100), cls: "s2" }], { height: 120, max: 100, fmt: (v) => v + "%", ticks: ticksFor(span), label: "CPU and memory" }),
    h("div", { style: "display:grid;gap:6px" },
      meter("cpu", host.cpu, host.cpu.toFixed(0) + "%"),
      meter("mem", (host.memUsed / host.memTotal) * 100, host.memUsed.toFixed(1) + "G")));
}

function activity(runs) {
  const now = new Date();
  const hours = Array.from({ length: 24 }, () => ({ ok: 0, bad: 0, other: 0 }));
  for (const r of runs) {
    const idx = 23 - Math.floor((now - new Date(r.created_at)) / 3600000);
    if (idx < 0 || idx > 23) continue;
    hours[idx][r.status === "succeeded" ? "ok" : r.status === "failed" ? "bad" : "other"]++;
  }
  const max = Math.max(1, ...hours.map((x) => x.ok + x.bad + x.other));
  const at = (i) => pad(new Date(now - (23 - i) * 3600000).getHours()) + ":00";
  return h("div", null,
    h("div", { class: "activity", role: "img", "aria-label": "Runs per hour over the last 24 hours" }, hours.map((x, i) => {
      const total = x.ok + x.bad + x.other;
      return h("div", { class: "col" + (total ? "" : " empty-col"), title: `${at(i)} · ${x.ok} succeeded, ${x.bad} failed, ${x.other} other` },
        x.ok > 0 && h("i", { style: `height:${(x.ok / max) * 100}%` }),
        x.bad > 0 && h("i", { class: "bad", style: `height:${(x.bad / max) * 100}%` }),
        x.other > 0 && h("i", { class: "other", style: `height:${(x.other / max) * 100}%` }));
    })),
    h("div", { class: "axis-row" }, h("span", null, at(0)), h("span", null, at(6)), h("span", null, at(12)), h("span", null, at(18)), h("span", null, "now")));
}

views.pipelines = {
  nav: "pipelines",
  crumbs: () => [["Flows"]],
  load: () => getJSON("/pipelines"),
  render(pipelines) {
    // The draft survives the periodic refresh, so a half-typed form is never lost.
    const path = h("input", { id: "register-path", class: "grow mono", name: "path", placeholder: "/absolute/path/to/truss.yaml", required: true, autocomplete: "off", "aria-label": "Path of the pipeline file", value: draft.path,
      oninput: (e) => { draft.path = e.target.value; } });
    const name = h("input", { id: "register-name", name: "name", placeholder: "name (optional)", autocomplete: "off", "aria-label": "Registration name", value: draft.name,
      oninput: (e) => { draft.name = e.target.value; } });
    const form = h("form", { class: "register", onsubmit: (e) => {
      e.preventDefault();
      act("Pipeline registered", async () => {
        await api("POST", "/pipelines", { path: path.value.trim(), name: name.value.trim() });
        draft.path = draft.name = path.value = name.value = "";
      });
    } }, path, h("button", { type: "button", id: "browse", onclick: () => openPicker(path) }, icon("folder", "sm"), "Browse…"), name, h("button", { class: "primary", type: "submit" }, "Register"));
    const q = ui.pipeQuery.toLowerCase();
    const shown = pipelines.filter((p) => !q || p.name.toLowerCase().includes(q) || p.path.toLowerCase().includes(q));
    const states = ["ready", "running", "paused", "invalid"].map((st) => [st, pipelines.filter((p) => pipelineState(p) === st).length]).filter(([, n]) => n);
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, "Flows"),
        h("p", { class: "sub" }, "Pipeline files this daemon knows. The file stays where it is and is read again before every run."))),
      panel("Register a pipeline", { hint: "absolute path to a truss.yaml" }, form),
      h("div", { class: "toolbar" },
        h("input", { type: "search", id: "pipe-filter", class: "filter", placeholder: "Filter by name or path", "aria-label": "Filter pipelines", value: ui.pipeQuery, oninput: (e) => { ui.pipeQuery = e.target.value; filterInPlace(".cards > .card", ui.pipeQuery, "pipe-count", pipelines.length); } }),
        h("span", { class: "muted", id: "pipe-count", style: "font-size:12px" }, `${shown.length} of ${pipelines.length}`),
        h("span", { class: "spacer" }),
        states.map(([st, n]) => h("span", { class: "muted", style: "font-size:12px;display:inline-flex;gap:6px;align-items:center" }, badge(st), n))),
      pipelines.length
        ? h("div", { class: "cards" }, pipelines.map((p) => { const c = pipelineCard(p); c.hidden = !shown.includes(p); return c; }))
        : h("div", { class: "panel empty" }, h("div", { class: "empty-title" }, "No pipelines registered yet."), "Use the form above or ", h("code", null, "truss register <path>")),
    ];
  },
};

// filterInPlace hides rows or cards that do not match, without rebuilding the page under the cursor.
function filterInPlace(selector, query, countId, total) {
  const q = query.trim().toLowerCase();
  let n = 0;
  for (const el of document.querySelectorAll(selector)) { el.hidden = !!q && !el.dataset.filter.includes(q); if (!el.hidden) n++; }
  if (countId) document.getElementById(countId).textContent = `${n} of ${total}`;
}

const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

function pipelineCard(p) {
  const last = p.last_run;
  const statusOf = (id) => { const x = last && last.steps.find((y) => y.id === id); return x ? stepState(x) : "pending"; };
  const cols = p.valid ? layoutSteps(p.steps || []) : [];
  return h("article", { class: "card", "data-filter": (p.name + " " + p.path).toLowerCase() },
    h("header", null, h("span", { class: "glyph" }, icon("pipelines", "sm")), h("a", { href: href.pipeline(p.name) }, p.name), badge(pipelineState(p))),
    h("div", { class: "path mono", title: p.path }, h("bdi", null, p.path)),
    p.valid
      ? h("dl", { class: "facts" },
        h("dt", null, "Schedule"), h("dd", null, scheduleText(p)),
        h("dt", null, "Next run"), h("dd", { class: "mono" }, p.paused ? h("span", { class: "muted" }, "paused") : nextOf(p) ? [clock(nextOf(p)), h("span", { class: "muted" }, " · " + until(nextOf(p)))] : h("span", { class: "muted" }, "–")),
        h("dt", null, "Last run"), h("dd", null, last ? h("a", { href: href.run(last.id), style: "display:inline-flex;gap:8px;align-items:center;text-decoration:none" }, badge(last.status), h("span", { class: "mono muted" }, ago(last.created_at))) : h("span", { class: "muted" }, "never")),
        h("dt", null, "Duration"), h("dd", { class: "mono" }, last && last.started_at ? took(last) : h("span", { class: "muted" }, "–")))
      : h("div", { class: "card-error" }, p.error),
    h("div", { class: "shape" },
      h("span", { class: "dag", "aria-hidden": "true" }, cols.map((col) => h("span", null, col.slice(0, 4).map((st) => h("b", { class: "s-" + statusOf(st.id) }))))),
      p.valid ? h("span", null, `${plural((p.steps || []).length, "step")} · ${plural(cols.length, "stage")} · ${p.concurrency || 1}× concurrency`) : h("span", null, "not runnable")),
    pipelineActions(p, true));
}

views.pipeline = {
  nav: "pipelines",
  crumbs: (name) => [["Flows", "#/pipelines"], [name]],
  async load(name) {
    const enc = encodeURIComponent(name);
    const [pipeline, runs] = await Promise.all([getJSON("/pipelines/" + enc), getJSON(`/runs?pipeline=${enc}&limit=50`)]);
    return { pipeline, runs };
  },
  render({ pipeline: p, runs }) {
    const last = p.last_run;
    const statusOf = (st) => {
      const x = last && last.steps.find((y) => y.id === st.id);
      return x ? stepState(x) : "pending";
    };
    const done = runs.filter((r) => r.status === "succeeded" || r.status === "failed");
    const rate = done.length ? Math.round((done.filter((r) => r.status === "succeeded").length / done.length) * 100) + "%" : "–";
    const durs = runs.filter((r) => r.status === "succeeded" && r.finished_at && r.started_at).map((r) => new Date(r.finished_at) - new Date(r.started_at));
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, p.name), h("p", { class: "sub" }, h("span", { class: "mono" }, p.path))),
        badge(pipelineState(p)), h("span", { class: "spacer" }), pipelineActions(p)),
      !p.valid && h("div", { class: "callout bad", role: "alert" }, icon("alert"), h("div", { class: "mono" }, p.error)),
      p.valid && h("div", { class: "strip", style: "--n:6" },
        h("div", null, h("div", { class: "k" }, "Steps"), h("div", { class: "v" }, (p.steps || []).length)),
        h("div", null, h("div", { class: "k" }, "Concurrency"), h("div", { class: "v" }, `${p.concurrency}×`)),
        h("div", null, h("div", { class: "k" }, "Overlap"), h("div", { class: "v" }, p.overlap || "–")),
        h("div", null, h("div", { class: "k" }, "Success rate"), h("div", { class: "v" }, rate)),
        h("div", null, h("div", { class: "k" }, "Median duration"), h("div", { class: "v" }, durs.length ? compact(durs.sort((a, b) => a - b)[Math.floor(durs.length / 2)]) : "–")),
        h("div", null, h("div", { class: "k" }, "Next run"), h("div", { class: "v" }, p.paused ? "paused" : nextOf(p) ? clock(nextOf(p)) : "on demand"))),
      p.valid && panel(last ? "Workflow" : "Steps", { hint: last ? "colored by the last run" : "", tools: legend() },
        graph(p.steps, statusOf, null, null, (st) => { const x = last && last.steps.find((y) => y.id === st.id); return x && x.started_at ? tookCompact(x) : ""; })),
      h("div", { class: "grid-main" },
        panel("Runs", { hint: `${runs.length} most recent` }, h("div", { class: "table-wrap" }, runsTable(runs, { withPipeline: false }))),
        panel("Configuration", null, h("div", { class: "panel-body", style: "padding-top:4px;padding-bottom:4px" }, h("dl", { class: "facts" },
          h("dt", null, "File"), h("dd", { class: "mono", title: p.path }, p.path),
          p.definition_name && [h("dt", null, "Defined as"), h("dd", { class: "mono" }, p.definition_name)],
          p.registered_name && [h("dt", null, "Registered as"), h("dd", { class: "mono", title: "The name it was registered under keeps its history and data; either name works in commands" }, p.registered_name)],
          p.valid && scheduleFacts(p),
          p.valid && [h("dt", null, "Concurrency"), h("dd", null, `${p.concurrency} step${p.concurrency === 1 ? "" : "s"} at once`)],
          p.valid && [h("dt", null, "Overlap"), h("dd", null, { queue: "a second run waits for the first", skip: "a second run is skipped", parallel: "runs may overlap" }[p.overlap] || p.overlap)],
          h("dt", null, "Registered"), h("dd", { class: "mono" }, clock(p.registered_at)),
          h("dt", null, "State"), h("dd", null, p.paused ? "schedules paused" : "active"))))),
    ];
  },
};

views.runs = {
  nav: "runs",
  crumbs: () => [["Runs"]],
  load: () => getJSON("/runs?limit=200"),
  render(runs) {
    const groups = ["all", "running", "queued", "succeeded", "failed", "cancelled", "skipped"];
    const n = (g) => (g === "all" ? runs.length : runs.filter((r) => r.status === g).length);
    const q = ui.runQuery.toLowerCase();
    const byStatus = runs.filter((r) => ui.runFilter === "all" || r.status === ui.runFilter);
    const shown = byStatus.filter((r) => !q || [r.pipeline, r.id, r.trigger].join(" ").toLowerCase().includes(q));
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, "Runs"),
        h("p", { class: "sub" }, "Every run the daemon started or skipped, newest first."))),
      h("div", { class: "toolbar" },
        groups.filter((g) => g === "all" || n(g) > 0).map((g) => h("button", { class: "chip", "aria-pressed": String(ui.runFilter === g), onclick: () => { ui.runFilter = g; route(true); } }, g, h("span", { class: "n" }, n(g)))),
        h("span", { class: "spacer" }),
        h("input", { type: "search", id: "runs-filter", class: "filter", placeholder: "Filter by pipeline, id or trigger", "aria-label": "Filter runs", value: ui.runQuery, oninput: (e) => { ui.runQuery = e.target.value; filterInPlace("#runs-table tbody tr", ui.runQuery); } })),
      h("div", { class: "panel", id: "runs-table" }, h("div", { class: "table-wrap tall" }, runsTable(byStatus, { empty: runs.length ? "No run has this status." : null, hide: (r) => !shown.includes(r) }))),
    ];
  },
};

// The run view keeps its log between refreshes so the text is not re-fetched.
const logState = { id: "", offset: 0, text: "", step: "", follow: true, timer: 0, done: false, query: "", decoder: null };

function paintLog() {
  const pre = document.getElementById("log");
  if (!pre) return;
  const marker = logState.step ? ` step ${logState.step} ` : "";
  const q = logState.query.toLowerCase();
  const frag = document.createDocumentFragment();
  let count = 0, no = 0;
  for (const line of logState.text.split("\n")) {
    if (line) no++;
    if (!line || (marker && !(line.slice(19) + " ").startsWith(marker))) continue;
    if (q && !line.toLowerCase().includes(q)) continue;
    const rest = line.slice(20);
    let cls = "";
    if (/^(setup|summary succeeded)/.test(rest)) cls = "meta";
    if (/^summary succeeded|finished duration \S+ exit 0$/.test(rest)) cls = "good";
    if (/^(error|summary (failed|cancelled))|finished duration \S+ (exit [1-9-]|error)| stopped because /.test(rest)) cls = "fail";
    else if (!cls && !rest.includes(" | ")) cls = "meta";
    const body = [];
    const bar = rest.indexOf(" | ");
    if (!cls && bar > 0) body.push(h("span", { class: "stepname" }, rest.slice(0, bar)), " │ ", highlight(rest.slice(bar + 3), q));
    else body.push(cls ? h("span", { class: cls }, highlight(rest, q)) : highlight(rest, q));
    frag.append(h("span", { class: "ln" }, h("span", { class: "no", "aria-hidden": "true" }, no), h("span", { class: "ts" }, line.slice(11, 20)), body));
    count++;
  }
  if (!count) {
    const what = logState.step ? `step ${logState.step}` : "this run";
    frag.append(h("span", { class: "placeholder" }, logState.query ? `No line matches “${logState.query}”.` : logState.done ? `No log lines for ${what}.` : `Waiting for log lines from ${what}…`));
  } else if (!logState.done) {
    frag.append(h("span", { class: "cursor", style: `animation-delay:${phase(1000)}` }));
  }
  pre.replaceChildren(frag);
  const lines = document.getElementById("log-count");
  if (lines) lines.textContent = `${count} line${count === 1 ? "" : "s"}`;
  if (logState.follow) pre.scrollTop = pre.scrollHeight;
}
function highlight(text, q) {
  if (!q) return text;
  const out = [];
  let i = 0, low = text.toLowerCase();
  for (let j = low.indexOf(q); j >= 0; j = low.indexOf(q, i)) { out.push(text.slice(i, j), h("mark", null, text.slice(j, j + q.length))); i = j + q.length; }
  out.push(text.slice(i));
  return out;
}

async function pumpLog(id) {
  clearTimeout(logState.timer);
  if (logState.id !== id) return;
  try {
    for (;;) {
      const res = await api("GET", `/runs/${encodeURIComponent(id)}/log?offset=${logState.offset}`);
      const bytes = await res.arrayBuffer();
      if (logState.id !== id) return;
      // One streaming decoder, so a character split across two reads stays whole.
      const chunk = logState.decoder.decode(bytes, { stream: true });
      logState.offset = Number(res.headers.get("X-Truss-Log-Offset")) || logState.offset + bytes.byteLength;
      const status = res.headers.get("X-Truss-Run-Status");
      if (chunk) { logState.text += chunk; paintLog(); continue; }
      logState.done = status !== "queued" && status !== "running";
      if (logState.done) paintLog();
      break;
    }
  } catch { /* the next round tries again */ }
  if (!logState.done) logState.timer = setTimeout(() => pumpLog(id), LOG_MS);
}

views.run = {
  nav: "runs",
  crumbs: (id, data) => [["Runs", "#/runs"], data ? [data.run.pipeline, href.pipeline(data.run.pipeline)] : null, [shortId(id)]].filter(Boolean),
  async load(id) {
    const run = await getJSON("/runs/" + encodeURIComponent(id));
    const pipeline = await getJSON("/pipelines/" + encodeURIComponent(run.pipeline)).catch(() => null);
    return { run, pipeline };
  },
  render({ run, pipeline }) {
    if (logState.id !== run.id) {
      clearTimeout(logState.timer);
      Object.assign(logState, { id: run.id, offset: 0, text: "", step: "", follow: true, done: false, query: "", decoder: new TextDecoder("utf-8") });
      pumpLog(run.id);
    }
    const select = (id) => { logState.step = logState.step === id ? "" : id; route(true); };
    const pre = h("pre", { id: "log", tabindex: "0", "aria-label": "Run log", onscroll: (e) => {
      const el = e.currentTarget;
      const follow = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      if (follow !== logState.follow) { logState.follow = follow; document.getElementById("follow")?.setAttribute("aria-pressed", String(follow)); }
    } });
    queueMicrotask(paintLog);
    const steps = run.steps || [];
    const doneN = steps.filter((st) => !["pending", "running", "queued"].includes(st.status)).length;
    const current = steps.filter((st) => st.status === "running").map((st) => st.id);
    const usage = res.run(run.id);
    const active = isActive(run);
    const stepDetail = (st) => {
      if (st.reason) return st.reason;
      return [st.attempts > 1 ? `attempt ${st.attempts} · ` : "", st.exit_code != null && st.exit_code !== 0 ? `exit ${st.exit_code} · ` : "", took(st)].join("");
    };
    return [
      h("div", { class: "head" },
        h("div", { class: "title" }, h("h1", { class: "mono" }, run.id),
          h("p", { class: "sub" }, triggerLabel(run.trigger), ` run of `, h("a", { href: href.pipeline(run.pipeline) }, run.pipeline), ` · created ${clock(run.created_at)}`,
            run.scheduled_for ? ` · scheduled for ${clock(run.scheduled_for)}` : "", run.started_at ? ` · took ${took(run)}` : "")),
        badge(run.status), h("span", { class: "spacer" }),
        h("div", { class: "actions" },
          active && h("button", { class: "danger", title: run.trigger === "local" ? "Asks the local run to stop; it ends within a second" : null, onclick: () => act("Cancel requested", () => api("POST", `/runs/${run.id}/cancel`)) }, icon("stop", "sm"), "Cancel run"),
          !active && run.trigger !== "local" && h("button", { class: "primary", onclick: () => act("Run requested", async () => {
            const next = await (await api("POST", `/pipelines/${encodeURIComponent(run.pipeline)}/runs`)).json();
            go(href.run(next.id));
          }) }, icon("rotate", "sm"), "Re-run"))),
      h("div", { class: "strip", style: "--n:6" },
        h("div", null, h("div", { class: "k" }, "Progress"), h("div", { class: "v" }, `${doneN} / ${steps.length} steps`)),
        h("div", null, h("div", { class: "k" }, active ? "Running now" : "Ended"), h("div", { class: "v", title: current.join(", ") }, active ? (current.join(", ") || "waiting") : clock(run.finished_at))),
        h("div", null, h("div", { class: "k" }, "Started"), h("div", { class: "v" }, clock(run.started_at))),
        h("div", null, h("div", { class: "k" }, active ? "Elapsed" : "Duration"), h("div", { class: "v" }, took(run))),
        h("div", null, h("div", { class: "k" }, "Processes"), h("div", { class: "v", title: run.trigger === "local" ? "the run executes in a shell outside the daemon" : res.ok() ? "processes of this run's running steps" : res.why() }, !active ? "0 live" : run.trigger === "local" ? "local shell" : res.ok() ? `${usage.procs} live` : "not reported")),
        h("div", null, h("div", { class: "k" }, "Supervisor PID"), h("div", { class: "v", title: run.trigger === "local" ? "the run executes in a shell outside the daemon" : "the daemon process that runs this pipeline" }, run.trigger === "local" ? "local shell" : shell.status ? shell.status.pid : "–"))),
      run.reason && h("div", { class: "callout" + (run.status === "failed" ? " bad" : ""), role: run.status === "failed" ? "alert" : null }, icon(run.status === "failed" ? "alert" : "info"), h("div", null, run.reason)),
      steps.length > 0 && panel("Workflow", { hint: "click a step to filter the log", tools: legend() },
        graph(steps, stepState, logState.step, select, (st) => (st.started_at ? tookCompact(st) : ""))),
      steps.length > 0 && h("div", { class: "runlayout" },
        h("section", { class: "panel", style: "display:flex;flex-direction:column;min-height:0" },
          h("div", { class: "panel-head" }, h("h2", null, "Steps"), h("span", { class: "hint" }, `${doneN} of ${steps.length} done`), demo && h("span", { class: "tools" }, simTag())),
          h("ul", { class: "steps" },
            steps.map((st) => {
              const u = res.step(run.id, st.id);
              return h("li", { class: logState.step === st.id ? "selected" : "", title: st.error || st.reason || "", tabindex: "0", role: "button", "aria-pressed": logState.step === st.id ? "true" : "false",
                onclick: () => select(st.id), onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); select(st.id); } } },
                h("span", { class: "st" }, statusGlyph(stepState(st))),
                h("span", { class: "id" }, st.id),
                h("span", { class: "dur" }, stepDetail(st)),
                h("span", { class: "sub" },
                  h("span", { class: "badge s-" + stepState(st) }, stepState(st) === "tolerated" ? "failed · continued" : st.status),
                  st.status === "running" && u && h("span", { class: "mini", title: "CPU share of the machine" }, h("span", { class: "track" }, h("i", { style: `width:${Math.min(100, u.cpu_percent)}%` })), h("span", null, u.cpu_percent.toFixed(1) + "%")),
                  st.status === "running" && u && h("span", { class: "mono" }, bytes(u.rss_bytes)),
                  st.pid > 0 && h("span", { class: "mono", title: st.status === "running" ? "process id" : "process id of the last attempt" }, "pid " + st.pid)));
            }))),
        h("section", { class: "log", "aria-label": "Run log" },
          h("div", { class: "bar" },
            icon("terminal", "sm"),
            h("strong", null, "Log"),
            logState.step ? h("span", { class: "filterchip" }, `step ${logState.step}`, h("button", { onclick: () => select(logState.step), "aria-label": "Show all" }, "Show all")) : h("span", null, "whole run"),
            h("span", { id: "log-count", class: "mono" }),
            h("span", { class: "spacer" }),
            h("input", { type: "search", id: "log-find", placeholder: "Find in log", "aria-label": "Find in log", value: logState.query, oninput: (e) => { logState.query = e.target.value; paintLog(); } }),
            h("button", { type: "button", "aria-pressed": String(settings.logWrap), title: "Wrap long lines", onclick: () => setSetting("logWrap", !settings.logWrap) }, icon("wrap", "sm")),
            h("button", { type: "button", id: "follow", "aria-pressed": String(logState.follow), title: "Follow the end of the log", onclick: () => { logState.follow = true; paintLog(); route(true); } }, icon("follow", "sm")),
            active ? h("span", { class: "live" }, "live") : h("span", { class: "done" }, "complete")),
          pre)),
      h("div", { class: "grid-2" },
        panel("Resources", { sim: true, hint: active ? "this run's processes, since this page opened" : "this run has ended" },
          h("div", { class: "panel-body" }, runResources(run))),
        panel("Configuration", { hint: pipeline ? pipeline.path : "pipeline not registered" },
          h("div", { class: "panel-body", style: "padding-top:4px;padding-bottom:4px" }, h("dl", { class: "facts" },
            h("dt", null, "Pipeline"), h("dd", null, h("a", { href: href.pipeline(run.pipeline) }, run.pipeline)),
            h("dt", null, "Run id"), h("dd", { class: "mono" }, run.id),
            h("dt", null, "Trigger"), h("dd", null, triggerLabel(run.trigger), run.scheduled_for ? ` · for ${clock(run.scheduled_for)}` : ""),
            pipeline && [h("dt", null, "File"), h("dd", { class: "mono", title: pipeline.path }, pipeline.path)],
            pipeline && pipeline.valid && [h("dt", null, "Concurrency"), h("dd", null, `${pipeline.concurrency} step${pipeline.concurrency === 1 ? "" : "s"} at once`)],
            pipeline && pipeline.valid && [h("dt", null, "Overlap"), h("dd", null, pipeline.overlap)],
            pipeline && pipeline.valid && scheduleFacts(pipeline),
            h("dt", null, "Created"), h("dd", { class: "mono" }, run.created_at ? new Date(run.created_at).toISOString() : "–"),
            h("dt", null, "Finished"), h("dd", { class: "mono" }, run.finished_at ? new Date(run.finished_at).toISOString() : "–"))))),
    ];
  },
};

function runResources(run) {
  if (run.trigger === "local") return h("div", { class: "empty" }, "Not measured: this run executes in a shell outside the daemon (truss run --track).");
  if (!res.ok()) return noFigures();
  const active = isActive(run);
  if (!active) return h("div", { class: "empty" }, "Figures are kept only while a run is running.");
  const u = res.run(run.id), ser = res.runs.get(run.id) || { cpu: [], mem: [] };
  const live = (run.steps || []).filter((st) => st.status === "running");
  const memMax = Math.max(64, ...ser.mem) * 1.2;
  // Both lines share one axis, so memory is scaled into the axis range that CPU sets.
  const cpuMax = Math.max(10, ...ser.cpu) * 1.2;
  return h("div", { style: "display:grid;gap:14px" },
    h("div", { class: "metric-head" }, h("span", { class: "big" }, u.cpu.toFixed(1)), h("span", { class: "unit" }, "% CPU"),
      h("span", { class: "big", style: "margin-left:18px" }, u.mem.toFixed(0)), h("span", { class: "unit" }, "MiB RSS"),
      h("span", { class: "legend" }, h("span", null, h("i"), "cpu %"), h("span", { title: "Memory is scaled to fit the chart, so only its shape is comparable to CPU" }, h("i", { class: "s2" }), "mem, scaled"))),
    ser.cpu.length > 1
      ? lineChart([{ values: ser.cpu, area: true }, { values: ser.mem.map((m) => (m / memMax) * cpuMax), cls: "s2" }], { height: 130, max: cpuMax, fmt: (v) => v.toFixed(0) + "%", ticks: ticksFor((ser.cpu.length - 1) * (res.m.interval_seconds || 2)), label: "Run CPU and memory" })
      : h("div", { class: "empty", style: "padding:18px" }, "Collecting samples…"),
    h("table", null,
      h("thead", null, h("tr", null, h("th", null, "PID"), h("th", null, "Step"), h("th", { class: "num" }, "Procs"), h("th", { class: "num" }, "CPU"), h("th", { class: "num" }, "RSS"), h("th", { class: "num" }, "Thr"))),
      h("tbody", null, live.map((st) => {
        const x = res.step(run.id, st.id);
        return h("tr", null, h("td", { class: "mono" }, st.pid || "–"), h("td", { class: "mono" }, st.id),
          h("td", { class: "num", title: x && !x.tree ? "only the step's shell is measured on this OS" : "the step's process group" }, x ? x.processes + (x.tree ? "" : "*") : "–"),
          h("td", { class: "num" }, x ? x.cpu_percent.toFixed(1) + "%" : "–"), h("td", { class: "num" }, x ? bytes(x.rss_bytes) : "–"), h("td", { class: "num" }, x && x.threads ? x.threads : "–"));
      }))));
}

// ---------- monitor ----------

function healthChecks() {
  const st = shell.status;
  const checks = [];
  const beats = shell.beats.slice(-48);
  const okBeats = beats.filter(Boolean).length;
  checks.push({ name: "Daemon API", detail: st ? `${st.address} · ${st.version}` : "no answer", val: shell.latency != null ? `${shell.latency} ms` : "–", state: shell.up === false ? "bad" : shell.latency > 250 ? "warn" : "ok" });
  checks.push({ name: "Run slots", detail: st ? `${st.running_runs} running, ${st.queued_runs} queued` : "–", val: st ? `${st.running_runs}/${st.max_parallel_runs}` : "–", state: !st ? "off" : st.queued_runs > 0 ? "warn" : "ok" });
  const sched = shell.pipelines.filter((p) => (p.schedules || []).length);
  const paused = sched.filter((p) => p.paused).length;
  checks.push({ name: "Scheduler", detail: sched.length ? `${sched.length} scheduled pipeline${sched.length === 1 ? "" : "s"}${paused ? `, ${paused} paused` : ""}` : "nothing scheduled", val: sched.length ? `${sched.length - paused} active` : "idle", state: sched.length ? (paused ? "warn" : "ok") : "off" });
  const invalid = shell.pipelines.filter((p) => !p.valid).length;
  checks.push({ name: "Pipeline files", detail: invalid ? `${invalid} file${invalid === 1 ? "" : "s"} failed validation` : "all files parse", val: `${shell.pipelines.length - invalid}/${shell.pipelines.length}`, state: invalid ? "bad" : "ok" });
  const failedRecent = shell.runs.slice(0, 20).filter((r) => r.status === "failed").length;
  checks.push({ name: "Recent runs", detail: `${failedRecent} of the last ${Math.min(20, shell.runs.length)} failed`, val: shell.runs.length ? `${Math.round(100 - (failedRecent / Math.min(20, shell.runs.length)) * 100)}%` : "–", state: failedRecent > 3 ? "bad" : failedRecent ? "warn" : "ok" });
  const m = res.m;
  checks.push({ name: "Resource sampling", detail: !m ? "no sample yet" : m.supported ? `every ${m.interval_seconds} s on ${m.os}` : `not collected on ${m.os}`, val: m && m.supported && m.host ? `${(m.history || []).length} pts` : "–", state: !m ? "off" : m.supported ? "ok" : "off" });
  checks.push({ name: "Connection", detail: `${okBeats} of the last ${beats.length} polls answered`, val: beats.length ? `${Math.round((okBeats / beats.length) * 100)}%` : "–", state: okBeats === beats.length ? "ok" : "warn" });
  return checks;
}
function healthSummary() {
  const c = healthChecks();
  const bad = c.filter((x) => x.state === "bad").length, warn = c.filter((x) => x.state === "warn").length;
  return { text: bad ? `${bad} failing` : warn ? `${warn} need attention` : "all checks passing", state: bad ? "bad" : warn ? "warn" : "ok" };
}
function healthList(short) {
  const c = healthChecks();
  return h("ul", { class: "health" }, (short ? c.slice(0, 4) : c).map((x) => h("li", null,
    h("span", { class: "hstat " + x.state, title: x.state }),
    h("div", { style: "min-width:0" }, h("div", { class: "name" }, x.name), h("div", { class: "detail" }, x.detail)),
    h("span", { class: "val" }, x.val))));
}

views.monitor = {
  nav: "monitor",
  crumbs: () => [["Monitor"]],
  async load() {
    const [status, runs] = await Promise.all([getJSON("/status"), getJSON("/runs?limit=100")]);
    return { status, runs };
  },
  render({ status, runs }) {
    const ok = res.ok(), host = ok ? res.host() : null, hist = res.history();
    const active = runs.filter(isActive);
    const m = res.m;
    const procs = [];
    const d = m && m.supported ? m.daemon : null;
    procs.push({ pid: status.pid, name: "truss daemon", run: null, state: "S", cpu: d ? d.cpu_percent : null, rss: d ? d.rss_bytes : null, threads: d ? d.threads : 0, processes: 1, tree: false, daemon: true });
    for (const r of active) for (const st of (r.steps || []).filter((x) => x.status === "running")) {
      const x = res.step(r.id, st.id);
      procs.push({ pid: st.pid || null, name: st.id, run: r, state: "R", cpu: x ? x.cpu_percent : null, rss: x ? x.rss_bytes : null, threads: x ? x.threads : 0, processes: x ? x.processes : null, tree: x ? x.tree : false });
    }
    const summary = healthSummary();
    const up = Date.now() - new Date(status.started_at);
    const beats = shell.beats.slice(-48);
    const span = ticksFor(hist.seconds);
    const v = (x) => (ok ? x : "–");
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, "Monitor"),
        h("p", { class: "sub" }, `Daemon pid ${status.pid} · ${status.address} · up ${compact(up)}` + (ok ? ` · ${host.cores.length} cores · sampled every ${m.interval_seconds} s` : ""))),
        h("span", { class: "spacer" }), simTag()),
      h("div", { class: "strip", style: "--n:6" },
        h("div", null, h("div", { class: "k" }, "Health"), h("div", { class: "v sans", style: `color:var(--${summary.state === "ok" ? "ok" : summary.state === "warn" ? "warn" : "bad"})` }, summary.text)),
        h("div", null, h("div", { class: "k" }, "CPU"), h("div", { class: "v" }, v(ok && host.cpu.toFixed(1) + "%"))),
        h("div", null, h("div", { class: "k" }, "Memory"), h("div", { class: "v" }, v(ok && `${host.memUsed.toFixed(1)} / ${host.memTotal.toFixed(1)} GiB`))),
        h("div", null, h("div", { class: "k" }, "Load average"), h("div", { class: "v", title: ok && !host.load ? `${m.os} has no load average` : "" }, ok && host.load ? host.load.map((x) => x.toFixed(2)).join(" ") : "–")),
        h("div", null, h("div", { class: "k" }, "Run slots"), h("div", { class: "v" }, `${status.running_runs} / ${status.max_parallel_runs}`)),
        h("div", null, h("div", { class: "k" }, "Step processes"), h("div", { class: "v" }, ok ? (m.steps || []).reduce((a, x) => a + x.processes, 0) : "–"))),
      h("div", { class: "grid-2" },
        panel("CPU", { sim: true, hint: ok ? "this machine, last " + (span[0] || "moments").replace("-", "") : "" }, !ok ? noFigures() : h("div", { class: "panel-body" },
          h("div", { class: "metric-head" }, h("span", { class: "big" }, host.cpu.toFixed(1)), h("span", { class: "unit" }, "%"),
            h("span", { class: "legend" }, h("span", null, h("i"), "utilisation"))),
          lineChart([{ values: hist.cpu, area: true }], { height: 150, max: 100, fmt: (x) => x + "%", ticks: span, label: "Host CPU" }))),
        panel("Memory", { sim: true, hint: ok ? `${host.memTotal.toFixed(1)} GiB installed` : "" }, !ok ? noFigures() : h("div", { class: "panel-body" },
          h("div", { class: "metric-head" }, h("span", { class: "big" }, host.memUsed.toFixed(1)), h("span", { class: "unit" }, "GiB used"),
            h("span", { class: "legend" }, h("span", null, h("i", { class: "s2" }), "used"))),
          lineChart([{ values: hist.mem, area: true, cls: "s2" }], { height: 150, max: host.memTotal, fmt: (x) => x.toFixed(0) + "G", ticks: span, label: "Host memory" })))),
      h("div", { class: "grid-main" },
        h("div", { class: "stack" },
          panel("Cores", { sim: true, hint: ok ? `${host.cores.length} logical cores` : "" }, !ok ? noFigures() : h("div", { class: "panel-body" },
            h("div", { class: "cores" }, host.cores.map((x, i) => h("div", { class: "core" + (x > 80 ? " hot" : "") }, h("span", { class: "k" }, "cpu" + i), h("span", { class: "v" }, x.toFixed(0) + "%"), h("span", { class: "track" }, h("i", { style: `width:${x}%` }))))))),
          panel("Processes", { sim: true, hint: ok ? "the daemon and every running step; CPU is a share of the whole machine" : "process ids only" }, h("div", { class: "scroll" }, h("table", null,
            h("thead", null, h("tr", null, h("th", null, "PID"), h("th", null, "Process"), h("th", null, "Run"), h("th", { class: "num" }, "Procs"), h("th", { style: "width:170px" }, "CPU"), h("th", { class: "num" }, "RSS"), h("th", { class: "num" }, "Thr"))),
            h("tbody", null, procs.map((p) => h("tr", { class: p.run ? "link" : "", onclick: p.run ? () => go(href.run(p.run.id)) : null },
              h("td", { class: "mono" }, p.pid || "–", p.daemon && [" ", h("span", { class: "tag" }, "daemon")]),
              h("td", { class: "mono" }, p.name),
              h("td", null, p.run ? h("span", { class: "id" }, `${p.run.pipeline} · ${shortId(p.run.id)}`) : h("span", { class: "muted" }, "–")),
              h("td", { class: "num", title: p.processes != null && !p.tree && !p.daemon ? "only the step's shell is measured on this OS" : "" }, p.processes == null ? "–" : p.processes + (p.tree || p.daemon ? "" : "*")),
              h("td", null, p.cpu == null ? (p.run && p.run.trigger === "local" ? h("span", { class: "muted", title: "the run executes in a shell outside the daemon" }, "local shell") : h("span", { class: "muted", title: res.why() }, "not reported")) : meter("cpu", p.cpu, p.cpu.toFixed(1) + "%")),
              h("td", { class: "num" }, p.rss == null ? h("span", { class: "muted" }, "–") : bytes(p.rss)),
              h("td", { class: "num" }, p.threads || "–")))))))),
        h("div", { class: "stack" },
          panel("Health", { hint: summary.text }, healthList(false)),
          panel("Availability", { hint: "each cell is one poll of the daemon" }, h("div", { class: "panel-body" },
            h("div", { class: "uptime" }, Array.from({ length: 48 }, (_, i) => { const b = beats[i - (48 - beats.length)]; return h("i", { class: b === undefined ? "off" : b ? "" : "bad" }); })),
            h("div", { class: "axis-row" }, h("span", null, `-${Math.round((48 * settings.refresh) / 1000)}s`), h("span", null, "now")))),
          panel("Daemon", null, h("div", { class: "panel-body", style: "padding-top:4px;padding-bottom:4px" }, h("dl", { class: "facts" },
            h("dt", null, "Version"), h("dd", { class: "mono" }, status.version),
            h("dt", null, "PID"), h("dd", { class: "mono" }, status.pid),
            h("dt", null, "Address"), h("dd", { class: "mono" }, status.address),
            h("dt", null, "Home"), h("dd", { class: "mono", title: status.home }, status.home),
            h("dt", null, "Started"), h("dd", { class: "mono" }, clock(status.started_at)),
            h("dt", null, "Pipelines"), h("dd", { class: "mono" }, status.pipelines)))))),
    ];
  },
};

// ---------- settings ----------

function seg(key, options) {
  return h("div", { class: "seg", role: "group" }, options.map(([value, label]) => h("button", { type: "button", "aria-pressed": String(settings[key] === value), onclick: () => setSetting(key, value) }, label)));
}
function toggle(key, label) {
  return h("label", { class: "check" }, h("input", { type: "checkbox", checked: !!settings[key], onchange: (e) => setSetting(key, e.target.checked) }), label);
}

views.settings = {
  nav: "settings",
  crumbs: () => [["Settings"]],
  load: () => getJSON("/status"),
  render(status) {
    const row = (title, text, control) => h("div", { class: "setting" }, h("div", { class: "what" }, h("strong", null, title), h("span", null, text)), control);
    return [
      h("div", { class: "head" }, h("div", { class: "title" }, h("h1", null, "Settings"), h("p", { class: "sub" }, "Stored in this browser. The daemon is not changed."))),
      h("div", { class: "settings" },
        h("nav", { class: "settings-nav", "aria-label": "Settings sections" },
          h("a", { href: "#/settings", onclick: (e) => { e.preventDefault(); document.getElementById("set-appearance").scrollIntoView(); } }, "Appearance"),
          h("a", { href: "#/settings", onclick: (e) => { e.preventDefault(); document.getElementById("set-data").scrollIntoView(); } }, "Data"),
          h("a", { href: "#/settings", onclick: (e) => { e.preventDefault(); document.getElementById("set-log").scrollIntoView(); } }, "Log viewer"),
          h("a", { href: "#/settings", onclick: (e) => { e.preventDefault(); document.getElementById("set-daemon").scrollIntoView(); } }, "Daemon")),
        h("div", { class: "stack" },
          panel("Appearance", { id: "set-appearance" },
            row("Theme", "Follow the system, or pin light or dark.", seg("theme", [["system", "System"], ["light", "Light"], ["dark", "Dark"]])),
            row("Density", "Compact fits more rows on a screen.", seg("density", [["comfortable", "Comfortable"], ["compact", "Compact"]])),
            row("Sidebar", "Collapse to icons. Shortcut: [", seg("sidebar", [["expanded", "Expanded"], ["collapsed", "Collapsed"]])),
            row("Motion", "Turn off spinners, pulses and transitions.", seg("motion", [["full", "Full"], ["reduced", "Reduced"]]))),
          panel("Data", { id: "set-data" },
            row("Refresh interval", "How often pages ask the daemon for news. The daemon samples CPU and memory every 2 s.", seg("refresh", [[1000, "1 s"], [1500, "1.5 s"], [3000, "3 s"], [5000, "5 s"]]))),
          panel("Log viewer", { id: "set-log" },
            row("Wrap long lines", "Otherwise lines scroll sideways.", toggle("logWrap", "Wrap")),
            row("Timestamps", "Show the time at the start of each line.", toggle("logTs", "Show")),
            row("Font size", "Size of log text.", seg("logSize", [[11, "11"], [12, "12"], [13, "13"]]))),
          panel("Daemon", { id: "set-daemon" },
            h("div", { class: "panel-body", style: "padding-top:4px;padding-bottom:4px" }, h("dl", { class: "facts" },
              h("dt", null, "Version"), h("dd", { class: "mono" }, status.version),
              h("dt", null, "Address"), h("dd", { class: "mono" }, status.address),
              h("dt", null, "PID"), h("dd", { class: "mono" }, status.pid),
              h("dt", null, "Home"), h("dd", { class: "mono" }, status.home),
              h("dt", null, "Parallel runs"), h("dd", { class: "mono" }, status.max_parallel_runs))),
            row("Access token", "Forget the token this browser keeps. Run truss desktop to open the page again.", h("button", { class: "danger", onclick: forgetToken }, icon("key", "sm"), "Forget token"))))),
    ];
  },
};
function forgetToken() {
  if (!confirm("Forget the access token? You will need to run truss desktop again.")) return;
  try { localStorage.removeItem("truss-token"); } catch { /* private mode */ }
  token = "";
  shell.up = null; shell.status = null;
  paintConn();
  route(true);
}

// ---------- file picker for the register form ----------

// The browser cannot reveal absolute paths, so the daemon lists directories
// (names only, GET /fs) and the dialog builds the path from them.
const picker = { open: false, hidden: false, listing: null, error: "" };

// safeName shows bidirectional control characters as visible escapes, so a
// file name cannot pretend to end in .yaml.
function safeName(name) {
  return name.replace(/[‪-‮⁦-⁩‎‏]/g, (c) => "\\u" + c.charCodeAt(0).toString(16));
}

function joinPath(dir, name) {
  const sep = dir.includes("\\") && !dir.includes("/") ? "\\" : "/";
  return dir.endsWith(sep) ? dir + name : dir + sep + name;
}

let pickerSeq = 0;
async function pickerLoad(path) {
  const mine = ++pickerSeq; // only the newest request may paint
  const query = `?path=${encodeURIComponent(path)}&hidden=${picker.hidden}`;
  const hadFocus = document.getElementById("picker").contains(document.activeElement);
  try {
    const listing = await getJSON("/fs" + query);
    if (mine !== pickerSeq) return;
    picker.listing = listing;
    picker.error = "";
  } catch (e) {
    if (mine !== pickerSeq) return;
    picker.error = e.message;
  }
  pickerPaint();
  // Repainting replaces the buttons; keep keyboard focus inside the dialog.
  if (hadFocus) (document.getElementById("picker-list")?.querySelector("button") || document.getElementById("picker-cancel")).focus();
}

function pickerPaint() {
  const dlg = document.getElementById("picker");
  const l = picker.listing;
  const choose = (name) => {
    const input = document.getElementById("register-path");
    input.value = draft.path = joinPath(l.path, name);
    closePicker();
  };
  dlg.replaceChildren(...[
    h("h2", { id: "picker-title" }, "Choose a pipeline file"),
    picker.error && h("p", { class: "error", role: "alert", id: "picker-error" }, picker.error),
    l && h("div", { class: "picker-bar" },
      h("button", { type: "button", id: "picker-up", disabled: !l.parent, onclick: () => pickerLoad(l.parent) }, icon("up", "sm"), "Up"),
      h("button", { type: "button", id: "picker-home", onclick: () => pickerLoad("") }, icon("home", "sm"), "Home"),
      l.roots.length > 1 && l.roots.map((r) => h("button", { type: "button", class: "root", onclick: () => pickerLoad(r) }, r)),
      h("label", { class: "check" }, h("input", { type: "checkbox", id: "picker-hidden", checked: picker.hidden, onchange: (e) => { picker.hidden = e.target.checked; pickerLoad(l.path); } }), "Show hidden")),
    l && h("div", { class: "picker-path mono", id: "picker-path" }, l.path),
    l && (l.entries.length
      ? h("ul", { class: "picker-list", id: "picker-list" }, l.entries.map((en) => h("li", null,
        h("button", { type: "button", class: "entry " + en.kind, onclick: () => (en.kind === "dir" ? pickerLoad(joinPath(l.path, en.name)) : choose(en.name)) },
          h("span", { class: "kind", "aria-hidden": "true" }, icon(en.kind === "dir" ? "folder" : "file", "sm")), safeName(en.name) + (en.link ? " →" : ""), en.kind === "dir" && h("span", { class: "sr" }, " (folder)")))))
      : h("div", { class: "empty" }, "No folders or .yaml files here.")),
    l && l.truncated && h("p", { class: "sub" }, "Only the first entries are shown. Type the path instead."),
    h("div", { class: "picker-foot" }, h("button", { type: "button", id: "picker-cancel", onclick: closePicker }, "Cancel"))].filter((x) => x && x.nodeType));
}

async function openPicker(input) {
  const dlg = document.getElementById("picker");
  picker.open = true;
  picker.listing = null;
  picker.error = "";
  const typed = input.value.trim();
  const cut = Math.max(typed.lastIndexOf("/"), typed.lastIndexOf("\\"));
  pickerPaint();
  dlg.showModal();
  await pickerLoad(cut >= 0 ? typed.slice(0, cut + 1) : "");
  if (picker.error && cut >= 0) await pickerLoad(""); // the typed folder is gone: start at home
  document.getElementById("picker-up")?.focus();
}

// closePicker closes the dialog and returns focus in the same task, so nothing can run in between.
function closePicker() {
  const dlg = document.getElementById("picker");
  if (dlg.open) dlg.close();
  picker.open = false;
  document.getElementById("register-path")?.focus();
}
document.getElementById("picker").addEventListener("cancel", (e) => { e.preventDefault(); closePicker(); });
document.getElementById("picker").addEventListener("close", () => {
  picker.open = false;
  // Give focus back only if the user has not already moved on: this event arrives late.
  const active = document.activeElement;
  const input = document.getElementById("register-path");
  if (input && (!active || active === document.body || active.id === "browse")) input.focus();
});

// ---------- finder (jump to a pipeline or run) ----------

const finder = { q: "", sel: 0, items: [] };
function finderItems() {
  const q = finder.q.toLowerCase();
  const pages = [["Dashboard", "#/", "dashboard"], ["Flows", "#/pipelines", "pipelines"], ["Runs", "#/runs", "runs"], ["Monitor", "#/monitor", "monitor"], ["Settings", "#/settings", "settings"]]
    .filter(([n]) => !q || n.toLowerCase().includes(q)).map(([n, to, ic]) => ({ group: "Pages", label: n, to, icon: icon(ic, "sm"), k: "" }));
  const pipes = shell.pipelines.filter((p) => !q || p.name.toLowerCase().includes(q)).slice(0, 6)
    .map((p) => ({ group: "Flows", label: p.name, to: href.pipeline(p.name), icon: icon("pipelines", "sm"), k: pipelineState(p) }));
  const runs = shell.runs.filter((r) => !q || r.id.toLowerCase().includes(q) || r.pipeline.toLowerCase().includes(q)).slice(0, 6)
    .map((r) => ({ group: "Runs", label: `${r.pipeline} · ${shortId(r.id)}`, to: href.run(r.id), icon: statusGlyph(r.status, 14), k: r.status }));
  return [...pages, ...pipes, ...runs];
}
function finderPaint() {
  const dlg = document.getElementById("finder");
  finder.items = finderItems();
  finder.sel = Math.min(finder.sel, Math.max(0, finder.items.length - 1));
  let group = "";
  const list = h("ul", { class: "finder-list", role: "listbox", id: "finder-list" });
  finder.items.forEach((it, i) => {
    if (it.group !== group) { group = it.group; list.append(h("li", { class: "finder-group", role: "presentation" }, group)); }
    list.append(h("li", { role: "presentation" }, h("a", { href: it.to, id: "finder-opt-" + i, role: "option", "aria-selected": String(i === finder.sel), onclick: () => dlg.close(), onmousemove: () => { if (finder.sel !== i) { finder.sel = i; finderPaint(); } } }, it.icon, h("span", { class: "ellipsis" }, it.label), h("span", { class: "k" }, it.k))));
  });
  if (!finder.items.length) list.append(h("li", { class: "empty" }, "Nothing matches."));
  const old = dlg.querySelector("#finder-list");
  if (old) { old.replaceWith(list); dlg.querySelector("input").setAttribute("aria-activedescendant", finder.items.length ? "finder-opt-" + finder.sel : ""); return; }
  const input = h("input", { type: "text", placeholder: "Jump to a page, pipeline or run…", "aria-label": "Jump to", "aria-controls": "finder-list", autocomplete: "off",
    oninput: (e) => { finder.q = e.target.value; finder.sel = 0; finderPaint(); },
    onkeydown: (e) => {
      if (e.key === "ArrowDown") { e.preventDefault(); finder.sel = Math.min(finder.items.length - 1, finder.sel + 1); finderPaint(); }
      else if (e.key === "ArrowUp") { e.preventDefault(); finder.sel = Math.max(0, finder.sel - 1); finderPaint(); }
      else if (e.key === "Enter") { e.preventDefault(); const it = finder.items[finder.sel]; if (it) { dlg.close(); go(it.to); } }
    } });
  if (finder.items.length) input.setAttribute("aria-activedescendant", "finder-opt-" + finder.sel);
  dlg.replaceChildren(input, list, h("div", { class: "finder-foot" }, h("span", null, h("kbd", null, "↑"), " ", h("kbd", null, "↓"), " move"), h("span", null, h("kbd", null, "↵"), " open"), h("span", null, h("kbd", null, "esc"), " close")));
}
function openFinder() {
  const dlg = document.getElementById("finder");
  if (dlg.open) return;
  finder.q = ""; finder.sel = 0;
  dlg.replaceChildren();
  finderPaint();
  dlg.showModal();
  dlg.querySelector("input").focus();
}
document.getElementById("finder-open").addEventListener("click", openFinder);
document.getElementById("finder").addEventListener("click", (e) => { if (e.target.id === "finder") e.target.close(); });

// ---------- settings menu ----------

function paintMenu() {
  const menu = document.getElementById("settings-menu");
  const at = [...menu.querySelectorAll("button")].indexOf(document.activeElement);
  const st = shell.status;
  const item = (ic, label, onclick, kbd) => h("button", { type: "button", class: "item", role: "menuitem", onclick }, icon(ic, "sm"), label, kbd && h("kbd", null, kbd));
  const menuSeg = (key, options) => h("div", { class: "seg full", role: "group", "aria-label": key }, options.map(([v, l]) => h("button", { type: "button", "aria-pressed": String(settings[key] === v), onclick: () => setSetting(key, v) }, l)));
  menu.replaceChildren(
    h("div", { class: "menu-head" }, h("strong", null, "TRUSS Desktop"), h("span", null, demo ? "demo mode · sample data, no daemon" : st ? `daemon ${st.version} · pid ${st.pid}` : "daemon not connected")),
    h("div", { class: "menu-label" }, "Theme"), h("div", { class: "menu-row" }, menuSeg("theme", [["system", "System"], ["light", "Light"], ["dark", "Dark"]])),
    h("div", { class: "menu-label" }, "Density"), h("div", { class: "menu-row" }, menuSeg("density", [["comfortable", "Comfortable"], ["compact", "Compact"]])),
    h("hr"),
    item("sidebar", settings.sidebar === "collapsed" ? "Expand sidebar" : "Collapse sidebar", () => { closeMenu(); toggleSidebar(); }, "["),
    item("search", "Jump to…", () => { closeMenu(); openFinder(); }, "/"),
    item("monitor", "System monitor", () => { closeMenu(); go("#/monitor"); }),
    item("settings", "All settings", () => { closeMenu(); go("#/settings"); }),
    h("hr"),
    demo ? item("x", "Leave demo", () => { closeMenu(); leaveDemo(); }) : item("key", "Forget access token", () => { closeMenu(); forgetToken(); }));
  if (at >= 0) menu.querySelectorAll("button")[at]?.focus(); // repainting must not drop keyboard focus
}
function closeMenu() {
  document.getElementById("settings-menu").hidden = true;
  document.getElementById("settings-btn").setAttribute("aria-expanded", "false");
}
document.getElementById("settings-btn").addEventListener("click", (e) => {
  e.stopPropagation();
  const menu = document.getElementById("settings-menu");
  const open = menu.hidden;
  if (open) { paintMenu(); menu.hidden = false; e.currentTarget.setAttribute("aria-expanded", "true"); menu.querySelector("button")?.focus(); } else closeMenu();
});
// composedPath still holds the clicked button when a repaint has detached it.
document.addEventListener("click", (e) => { if (!e.composedPath().some((el) => el.classList && el.classList.contains("menu-wrap"))) closeMenu(); });
document.getElementById("theme-toggle").addEventListener("click", () => setSetting("theme", isDark() ? "light" : "dark"));
function toggleSidebar() { setSetting("sidebar", settings.sidebar === "collapsed" ? "expanded" : "collapsed"); }
document.getElementById("collapse").addEventListener("click", toggleSidebar);
document.addEventListener("keydown", (e) => {
  const typing = /^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement?.tagName) || document.querySelector("dialog[open]");
  if (e.key === "Escape" && !document.getElementById("settings-menu").hidden) { closeMenu(); document.getElementById("settings-btn").focus(); }
  if (typing || e.metaKey || e.ctrlKey || e.altKey) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") { e.preventDefault(); openFinder(); }
    return;
  }
  if (e.key === "/") { e.preventDefault(); openFinder(); }
  else if (e.key === "[") toggleSidebar();
});
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => applySettings());

// ---------- shell: nav, sidebar live runs, connection ----------

const NAV = [["dashboard", "Dashboard", "#/"], ["pipelines", "Flows", "#/pipelines"], ["runs", "Runs", "#/runs"], ["monitor", "Monitor", "#/monitor"], ["settings", "Settings", "#/settings"]];
function paintNav(current) {
  const nav = document.getElementById("nav");
  if (!nav.children.length) nav.append(...NAV.map(([key, label, to]) => h("a", { href: to, "data-route": key, title: label }, icon(key), h("span", null, label))));
  const live = shell.runs.filter(isActive).length;
  for (const a of nav.children) {
    const key = a.dataset.route;
    a.classList.toggle("active", key === current);
    if (key === current) a.setAttribute("aria-current", "page"); else a.removeAttribute("aria-current");
    if (key !== "runs") continue;
    a.classList.toggle("has-live", live > 0);
    let c = a.querySelector(".count");
    if (live && !c) a.append(c = h("span", { class: "count" }));
    if (c) { if (live) { c.textContent = live; c.title = `${live} active`; } else c.remove(); }
  }
}
function paintSideLive() {
  const live = shell.runs.filter(isActive);
  const box = document.getElementById("side-live");
  const focused = box.contains(document.activeElement) ? document.activeElement.getAttribute("href") : null;
  box.replaceChildren(...[
    h("div", { class: "side-label" }, live.length ? h("span", { class: "dot run", style: `animation-delay:${phase(1400)}` }) : null, `Active runs`, h("span", { class: "spacer" }), h("span", { class: "mono" }, live.length)),
    live.length
      ? live.slice(0, 8).map((r) => {
        const steps = r.steps || [];
        const done = steps.filter((st) => !["pending", "running", "queued"].includes(st.status)).length;
        return h("a", { class: "side-run", href: href.run(r.id), title: `${r.pipeline} · ${r.id}` },
          statusGlyph(r.status, 12), h("span", { class: "name" }, r.pipeline), h("span", { class: "t" }, tookCompact(r) || "queued"),
          h("span", { class: "bar" }, h("i", { style: `width:${steps.length ? (done / steps.length) * 100 : 4}%` })));
      })
      : h("div", { class: "side-empty" }, "Idle")].flat());
  if (focused) {
    const again = [...box.querySelectorAll("a")].find((a) => a.getAttribute("href") === focused);
    if (again) again.focus({ preventScroll: true }); else document.getElementById("nav").querySelector("a[data-route=runs]").focus({ preventScroll: true });
  }
}
function paintCrumbs(view, arg, data) {
  const parts = view.crumbs(arg, data);
  const el = document.getElementById("crumbs");
  const out = [h("a", { href: "#/" }, icon("home", "sm"))];
  parts.forEach((p, i) => {
    out.push(h("span", { class: "sep" }, "/"));
    out.push(p[1] && i < parts.length - 1 ? h("a", { href: p[1] }, p[0]) : h("span", { class: "here" }, p[0]));
  });
  const key = JSON.stringify(parts);
  if (el.dataset.key === key) return; // rebuilding would drop keyboard focus on a crumb
  el.dataset.key = key;
  el.replaceChildren(...out);
}

let shellBusy = false;
async function pollShell() {
  if (shellBusy) return;
  shellBusy = true;
  const t0 = performance.now();
  try {
    const status = await getJSON("/status");
    shell.latency = Math.max(1, Math.round(performance.now() - t0));
    shell.status = status;
    shell.up = true;
    const [runs, pipelines, metrics] = await Promise.all([getJSON("/runs?limit=100"), getJSON("/pipelines"), getJSON("/metrics").catch(() => null)]);
    shell.runs = runs; shell.pipelines = pipelines;
    if (metrics) res.ingest(metrics);
    shell.beats.push(true);
  } catch (e) {
    if (!(e instanceof ApiError)) { shell.up = false; shell.status = null; shell.beats.push(false); }
  } finally {
    shellBusy = false;
  }
  if (shell.beats.length > 96) shell.beats.shift();
  paintConn();
  paintSideLive();
  paintNav(parseRoute().view.nav);
}
function paintConn() {
  const el = document.getElementById("conn");
  el.classList.toggle("demo", !!demo);
  if (demo) el.replaceChildren(h("span", { class: "dot" }), "Demo data", h("span", { class: "lat", title: "sample data generated in this page; no daemon is contacted" }, "no daemon"));
  else if (shell.up === false) el.replaceChildren(h("span", { class: "dot down" }), "offline");
  else if (shell.up) el.replaceChildren(h("span", { class: "dot" }), "connected", h("span", { class: "lat" }, `${shell.latency} ms`));
  else el.replaceChildren(h("span", { class: "dot", style: "background:var(--off);box-shadow:none" }), "connecting");
}

// ---------- routing and refresh ----------

function parseRoute() {
  const parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean).map((p) => { try { return decodeURIComponent(p); } catch { return p; } });
  if (parts[0] === "pipelines") return parts[1] ? { view: views.pipeline, arg: parts[1] } : { view: views.pipelines };
  if (parts[0] === "runs") return parts[1] ? { view: views.run, arg: parts[1] } : { view: views.runs };
  if (parts[0] === "monitor") return { view: views.monitor };
  if (parts[0] === "settings") return { view: views.settings };
  return { view: views.dashboard };
}

const FOCUSABLE = "a[href], button:not(:disabled), input, select, textarea, summary, [tabindex]:not([tabindex='-1'])";
// focusIndex says which focusable element of the view has focus, so a refresh can give it back.
function focusIndex(main) {
  const el = document.activeElement;
  if (!el || el === main || !main.contains(el)) return -1;
  return [...main.querySelectorAll(FOCUSABLE)].indexOf(el);
}

let generation = 0;
let lastHash = null;

function gate(cls, ic, title, text, extra) {
  return h("div", { class: "gate " + cls, role: cls ? "alert" : null }, h("div", { class: "gate-icon" }, icon(ic)), h("h1", null, title), h("p", { class: "sub" }, text), extra);
}

// route draws the current view. force redraws even while a field has focus: the user just acted.
async function route(force) {
  const mine = ++generation;
  const { view, arg } = parseRoute();
  const main = document.getElementById("view");
  paintNav(view.nav);
  if (lastHash !== location.hash) paintCrumbs(view, arg, null);
  if (view !== views.run) { clearTimeout(logState.timer); logState.id = ""; }
  let loaded = false;
  try {
    const data = await view.load(arg);
    loaded = true;
    if (mine !== generation) return;
    // Do not replace a form the user is typing in, or a dialog that is open.
    // Decided now, not before the request: the user may have moved on meanwhile.
    // Search fields keep their text in ui/logState, so only form fields hold the page still.
    const active = document.activeElement;
    const typing = main.contains(active) && active.tagName === "INPUT" && !!active.closest("form");
    const caret = main.contains(active) && active.tagName === "INPUT" && active.type !== "checkbox" ? [active.selectionStart, active.selectionEnd] : null;
    if (picker.open || (typing && !force && lastHash === location.hash)) return;
    const kept = lastHash === location.hash ? focusIndex(main) : -1;
    // Controls above a field can come and go between repaints; an id outlives that, a position does not.
    const keptId = lastHash === location.hash && main.contains(active) && active.id ? active.id : "";
    const scroll = document.getElementById("log") ? document.getElementById("log").scrollTop : null;
    const pageScroll = lastHash === location.hash ? main.scrollTop : 0;
    paintCrumbs(view, arg, data);
    main.replaceChildren(...view.render(data).flat(Infinity).filter(Boolean));
    main.scrollTop = pageScroll;
    if (scroll != null && document.getElementById("log") && !logState.follow) {
      queueMicrotask(() => { document.getElementById("log").scrollTop = scroll; });
    }
    if (kept >= 0) {
      const target = (keptId && document.getElementById(keptId)) || main.querySelectorAll(FOCUSABLE)[kept];
      if (target) {
        target.focus({ preventScroll: true });
        if (caret && target.tagName === "INPUT") try { target.setSelectionRange(caret[0], caret[1]); } catch { /* not a text field */ }
      }
    } else if (lastHash !== location.hash) {
      main.focus({ preventScroll: true });
    }
    lastHash = location.hash;
    paintDaemon(true);
  } catch (e) {
    if (mine !== generation) return;
    if (e instanceof ApiError && e.status === 401) {
      main.replaceChildren(gate("", "key", "Open TRUSS Desktop from your terminal", "This page needs the daemon's access token. Run this command and it opens with the token:", h("code", null, "truss desktop")));
    } else if (e instanceof ApiError && e.status === 404) {
      main.replaceChildren(gate("", "search", "Not found", e.message, h("a", { href: "#/" }, "Back to the dashboard")));
    } else if (e instanceof ApiError || loaded) {
      // The daemon answered with an error, or the page itself failed.
      main.replaceChildren(gate("", "alert", "Something went wrong", e.message, h("button", { onclick: () => route(true) }, "Try again")));
      if (!(e instanceof ApiError)) console.warn(e); // eslint-disable-line no-console
    } else {
      paintDaemon(false);
      lastHash = null;
      if (!main.querySelector(".gate.unreachable")) {
        main.replaceChildren(gate("unreachable", "alert", "Daemon unreachable", "TRUSS Desktop cannot reach the daemon. Start it and this page reconnects by itself:", h("code", null, "truss daemon start")));
      }
    }
  }
}
const refresh = route;

function paintDaemon(up) {
  const el = document.getElementById("daemon");
  el.classList.toggle("down", !up);
  if (!up) {
    el.replaceChildren(h("strong", null, h("span", { class: "dot" }), "Daemon unreachable"), h("span", { class: "mono" }, "truss daemon start"));
    return;
  }
  const st = shell.status;
  if (!st) return;
  if (demo) {
    el.replaceChildren(h("strong", null, h("span", { class: "dot demo" }), "Demo mode"), h("span", null, "sample data, no daemon"), h("br"), h("span", { class: "mono" }, "truss desktop --demo"));
    return;
  }
  el.replaceChildren(h("strong", null, h("span", { class: "dot" }), "Daemon running"),
    h("span", { class: "mono" }, `${st.address} · pid ${st.pid}`), h("br"), `up ${compact(Date.now() - new Date(st.started_at))} · ${st.version}`);
}

applySettings();
window.addEventListener("hashchange", () => {
  // A token or demo link pasted into an open tab only changes the hash; start over so it takes effect.
  if (/^#token=/.test(location.hash) || location.hash === "#demo") { location.reload(); return; }
  route();
});
let ticker = 0;
function schedule() {
  clearTimeout(ticker);
  ticker = setTimeout(async () => {
    if (!document.hidden) { await pollShell(); route(); }
    schedule();
  }, settings.refresh);
}
(async function start() {
  if (demoWanted) demo = await loadDemo();
  paintMenu();
  pollShell().then(() => paintDaemon(shell.up !== false));
  route();
  schedule();
})();
