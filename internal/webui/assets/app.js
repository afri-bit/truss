// TRUSS Desktop. Plain JavaScript, no build step, no dependencies.
// It talks to the daemon's HTTP API (/api/v1) with the access token that
// `truss desktop` passes in the URL fragment.
"use strict";

const API = "/api/v1";
const REFRESH_MS = 1500;
const LOG_MS = 700;

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
  history.replaceState(null, "", location.pathname + "#/");
})();

// ---------- small helpers ----------

// h builds a DOM element. Text always goes in as text, never as markup.
function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === false || v == null) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    el.append(c.nodeType ? c : document.createTextNode(String(c)));
  }
  return el;
}
const SVG = "http://www.w3.org/2000/svg";
function s(tag, attrs, ...children) {
  const el = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs || {})) el.setAttribute(k, v);
  for (const c of children) el.append(c.nodeType ? c : document.createTextNode(String(c)));
  return el;
}

class ApiError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}
async function api(method, path, body) {
  const res = await fetch(API + path, {
    method,
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
function took(item) {
  if (!item.started_at) return "–";
  const end = item.finished_at ? new Date(item.finished_at) : new Date();
  return span(end - new Date(item.started_at));
}
function until(iso) {
  const ms = new Date(iso) - new Date();
  return ms <= 0 ? "now" : "in " + span(ms);
}
const badge = (status) => h("span", { class: "badge s-" + status }, status);
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

// ---------- shared pieces ----------

function runsTable(runs, withPipeline) {
  if (!runs.length) {
    return h("div", { class: "empty" }, "No runs yet. Start one with ", h("code", null, "truss trigger <name>"), " or the Run now button.");
  }
  return h("div", { class: "scroll" }, h("table", null,
    h("thead", null, h("tr", null,
      h("th", null, "Status"), withPipeline && h("th", null, "Pipeline"), h("th", null, "Trigger"),
      h("th", null, "Created"), h("th", { class: "num" }, "Duration"), h("th", null, "Note"))),
    h("tbody", null, runs.map((r) => h("tr", { class: "link", onclick: () => { location.hash = "#/runs/" + r.id; } },
      h("td", null, h("a", { class: "rowlink", href: "#/runs/" + encodeURIComponent(r.id), "aria-label": `Run ${r.id}, ${r.status}`, onclick: (e) => e.stopPropagation() }, badge(r.status))),
      withPipeline && h("td", null, h("a", { href: "#/pipelines/" + encodeURIComponent(r.pipeline), onclick: (e) => e.stopPropagation() }, r.pipeline)),
      h("td", null, r.trigger),
      h("td", null, clock(r.created_at)),
      h("td", { class: "num" }, took(r)),
      h("td", { class: "note", title: r.reason || "" }, r.reason || (r.scheduled_for ? "scheduled for " + clock(r.scheduled_for) : "")))))));
}

function pipelineActions(p) {
  const name = encodeURIComponent(p.name);
  return h("div", { class: "actions" },
    h("button", { class: "primary", disabled: !p.valid, onclick: () => act(`Run of ${p.name} requested`, async () => {
      const run = await (await api("POST", `/pipelines/${name}/runs`)).json();
      location.hash = "#/runs/" + run.id;
    }) }, "Run now"),
    (p.schedules || []).length > 0 && (p.paused
      ? h("button", { onclick: () => act(`Schedules of ${p.name} resumed`, () => api("POST", `/pipelines/${name}/resume`)) }, "Resume schedule")
      : h("button", { onclick: () => act(`Schedules of ${p.name} paused`, () => api("POST", `/pipelines/${name}/pause`)) }, "Pause schedule")),
    h("button", { class: "danger", onclick: () => {
      if (!confirm(`Unregister ${p.name}? The file and its run history stay.`)) return;
      act(`${p.name} unregistered`, async () => { await api("DELETE", `/pipelines/${name}`); location.hash = "#/pipelines"; });
    } }, "Unregister"));
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

// graph draws the step graph, left to right, one column per dependency depth.
function graph(steps, statusOf, selected, onSelect) {
  const W = 168, H = 44, GX = 54, GY = 14, PAD = 6;
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
  const pos = new Map();
  cols.forEach((col, x) => col.forEach((st, y) => pos.set(st.id, { x: PAD + x * (W + GX), y: PAD + y * (H + GY) })));
  const width = PAD * 2 + cols.length * W + (cols.length - 1) * GX;
  const height = PAD * 2 + Math.max(...cols.map((c) => c.length)) * (H + GY) - GY;
  const svg = s("svg", { width, height, viewBox: `0 0 ${width} ${height}`, role: "img", "aria-label": "Step graph" });
  for (const st of steps) {
    for (const need of st.needs || []) {
      const a = pos.get(need), b = pos.get(st.id);
      if (!a) continue;
      const x1 = a.x + W, y1 = a.y + H / 2, x2 = b.x, y2 = b.y + H / 2, mid = (x1 + x2) / 2;
      svg.append(s("path", { class: "edge", d: `M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}` }));
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
    g.append(s("rect", { width: W, height: H }));
    g.append(s("circle", { cx: 16, cy: H / 2, r: 4.5 }));
    const clip = (t) => (t.length > 19 ? t.slice(0, 18) + "…" : t);
    if (job) g.append(s("text", { class: "job", x: 30, y: 17 }, clip(job)));
    g.append(s("text", { x: 30, y: job ? 33 : 27 }, clip(name)));
    if (onSelect) {
      g.addEventListener("click", () => onSelect(st.id));
      g.setAttribute("tabindex", "0");
      g.setAttribute("aria-pressed", selected === st.id ? "true" : "false");
      g.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onSelect(st.id); } });
    }
    svg.append(g);
  }
  return h("div", { class: "panel graph" }, svg);
}

// ---------- views ----------

const views = {};
const draft = { path: "", name: "" };

views.dashboard = {
  nav: "dashboard",
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
    const tile = (cls, num, label) => h("div", { class: "tile " + cls }, h("div", { class: "num" }, num), h("div", { class: "label" }, label));
    return [
      h("div", { class: "head" }, h("div", null, h("h1", null, "Dashboard"),
        h("p", { class: "sub" }, `Up to ${status.max_parallel_runs} runs at once · daemon ${status.version}`))),
      h("div", { class: "tiles" },
        tile("", pipelines.length, "pipelines registered"),
        tile("run", status.running_runs, "running now"),
        tile("", status.queued_runs, "queued"),
        tile("ok", count("succeeded"), "succeeded today"),
        tile("bad", count("failed"), "failed today"),
        tile("", count("skipped"), "skipped today")),
      h("div", { class: "cols" },
        h("section", null, h("h2", null, "Active runs"), h("div", { class: "panel" },
          active.length ? runsTable(active, true) : h("div", { class: "empty" }, "Nothing is running."))),
        h("section", null, h("h2", null, "Coming up"), h("div", { class: "panel" },
          upcoming.length
            ? h("table", null, h("tbody", null, upcoming.slice(0, 8).map((p) => h("tr", { class: "link", onclick: () => { location.hash = "#/pipelines/" + encodeURIComponent(p.name); } },
              h("td", null, h("a", { href: "#/pipelines/" + encodeURIComponent(p.name), onclick: (e) => e.stopPropagation() }, p.name)), h("td", null, clock(nextOf(p))), h("td", { class: "num" }, until(nextOf(p)))))))
            : h("div", { class: "empty" }, "No schedules. Add ", h("code", null, "on.schedule"), " to a pipeline file.")))),
      h("section", null, h("h2", null, "Recent runs"), h("div", { class: "panel" }, runsTable(runs.slice(0, 12), true))),
    ];
  },
};

views.pipelines = {
  nav: "pipelines",
  load: () => getJSON("/pipelines"),
  render(pipelines) {
    // The draft survives the periodic refresh, so a half-typed form is never lost.
    const path = h("input", { id: "register-path", class: "grow mono", name: "path", placeholder: "/absolute/path/to/truss.yaml", required: true, autocomplete: "off", "aria-label": "Path of the pipeline file", value: draft.path,
      oninput: (e) => { draft.path = e.target.value; } });
    const name = h("input", { id: "register-name", name: "name", placeholder: "name (optional)", autocomplete: "off", "aria-label": "Registration name", value: draft.name,
      oninput: (e) => { draft.name = e.target.value; } });
    const form = h("form", { class: "register panel", onsubmit: (e) => {
      e.preventDefault();
      act("Pipeline registered", async () => {
        await api("POST", "/pipelines", { path: path.value.trim(), name: name.value.trim() });
        draft.path = draft.name = path.value = name.value = "";
      });
    } }, path, h("button", { type: "button", id: "browse", onclick: () => openPicker(path) }, "Browse…"), name, h("button", { class: "primary", type: "submit" }, "Register"));
    return [
      h("div", { class: "head" }, h("div", null, h("h1", null, "Pipelines"),
        h("p", { class: "sub" }, "Pipeline files this daemon knows. The file stays where it is and is read again before every run."))),
      h("section", null, h("h2", null, "Register a pipeline"), form),
      h("section", null, pipelines.length
        ? h("div", { class: "cards" }, pipelines.map((p) => h("article", { class: "card" },
          h("header", null, h("a", { href: "#/pipelines/" + encodeURIComponent(p.name) }, p.name), h("span", { class: "spacer" }), badge(pipelineState(p))),
          h("div", { class: "path mono" }, p.path),
          p.valid ? h("dl", { class: "facts" }, scheduleFacts(p),
            h("dt", null, "Last run"), h("dd", null, p.last_run ? [badge(p.last_run.status), " ", clock(p.last_run.created_at)] : "never"))
            : h("div", { class: "error" }, p.error),
          pipelineActions(p))))
        : h("div", { class: "panel empty" }, "No pipelines registered yet. Use the form above or ", h("code", null, "truss register <path>"))),
    ];
  },
};

views.pipeline = {
  nav: "pipelines",
  async load(name) {
    const enc = encodeURIComponent(name);
    const [pipeline, runs] = await Promise.all([getJSON("/pipelines/" + enc), getJSON(`/runs?pipeline=${enc}&limit=50`)]);
    return { pipeline, runs };
  },
  render({ pipeline: p, runs }) {
    const last = p.last_run;
    const statusOf = (st) => (last && (last.steps.find((x) => x.id === st.id) || {}).status) || "pending";
    return [
      h("a", { class: "crumb", href: "#/pipelines" }, "← Pipelines"),
      h("div", { class: "head" }, h("div", null, h("h1", null, p.name), h("p", { class: "sub mono" }, p.path)),
        badge(pipelineState(p)), h("span", { class: "spacer" }), pipelineActions(p)),
      !p.valid && h("section", null, h("div", { class: "panel empty error" }, p.error)),
      p.valid && h("section", null, h("dl", { class: "facts" }, scheduleFacts(p),
        h("dt", null, "Concurrency"), h("dd", null, `${p.concurrency} step${p.concurrency === 1 ? "" : "s"} at once`),
        h("dt", null, "Overlap"), h("dd", null, { queue: "a second run waits for the first", skip: "a second run is skipped", parallel: "runs may overlap" }[p.overlap] || p.overlap))),
      p.valid && h("section", null, h("h2", null, last ? "Steps · colored by the last run" : "Steps"), graph(p.steps, statusOf)),
      h("section", null, h("h2", null, "Runs"), h("div", { class: "panel" }, runsTable(runs, false))),
    ];
  },
};

views.runs = {
  nav: "runs",
  load: () => getJSON("/runs?limit=200"),
  render(runs) {
    return [
      h("div", { class: "head" }, h("div", null, h("h1", null, "Runs"),
        h("p", { class: "sub" }, "Every run the daemon started or skipped, newest first."))),
      h("div", { class: "panel" }, runsTable(runs, true)),
    ];
  },
};

// The run view keeps its log between refreshes so the text is not re-fetched.
const logState = { id: "", offset: 0, text: "", step: "", follow: true, timer: 0, done: false };

function paintLog() {
  const pre = document.getElementById("log");
  if (!pre) return;
  const marker = logState.step ? ` step ${logState.step} ` : "";
  const frag = document.createDocumentFragment();
  for (const line of logState.text.split("\n")) {
    if (!line || (marker && !(line.slice(19) + " ").startsWith(marker))) continue;
    const rest = line.slice(20);
    let cls = "";
    if (/^(setup|summary succeeded)/.test(rest)) cls = "meta";
    if (/^summary succeeded|finished duration \S+ exit 0$/.test(rest)) cls = "good";
    if (/^(error|summary (failed|cancelled))|finished duration \S+ (exit [1-9-]|error)| stopped because /.test(rest)) cls = "fail";
    else if (!cls && !rest.includes(" | ")) cls = "meta";
    frag.append(h("span", { class: "ts" }, line.slice(0, 20)), cls ? h("span", { class: cls }, rest) : rest, "\n");
  }
  if (!frag.childNodes.length) {
    const what = logState.step ? `step ${logState.step}` : "this run";
    frag.append(h("span", { class: "ts" }, logState.done ? `No log lines for ${what}.` : `Waiting for log lines from ${what}…`));
  }
  pre.replaceChildren(frag);
  if (logState.follow) pre.scrollTop = pre.scrollHeight;
}

async function pumpLog(id) {
  clearTimeout(logState.timer);
  if (logState.id !== id) return;
  try {
    for (;;) {
      const res = await api("GET", `/runs/${encodeURIComponent(id)}/log?offset=${logState.offset}`);
      const chunk = await res.text();
      if (logState.id !== id) return;
      logState.offset = Number(res.headers.get("X-Truss-Log-Offset")) || logState.offset + chunk.length;
      const status = res.headers.get("X-Truss-Run-Status");
      if (chunk) { logState.text += chunk; paintLog(); continue; }
      logState.done = status !== "queued" && status !== "running";
      break;
    }
  } catch { /* the next round tries again */ }
  if (!logState.done) logState.timer = setTimeout(() => pumpLog(id), LOG_MS);
}

views.run = {
  nav: "runs",
  load: (id) => getJSON("/runs/" + encodeURIComponent(id)),
  render(run) {
    if (logState.id !== run.id) {
      clearTimeout(logState.timer);
      Object.assign(logState, { id: run.id, offset: 0, text: "", step: "", follow: true, done: false });
      pumpLog(run.id);
    }
    const select = (id) => { logState.step = logState.step === id ? "" : id; route(true); };
    const pre = h("pre", { id: "log", tabindex: "0", onscroll: (e) => {
      const el = e.currentTarget;
      logState.follow = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    } });
    queueMicrotask(paintLog);
    return [
      h("a", { class: "crumb", href: "#/pipelines/" + encodeURIComponent(run.pipeline) }, "← " + run.pipeline),
      h("div", { class: "head" },
        h("div", null, h("h1", { class: "mono" }, run.id),
          h("p", { class: "sub" }, `${run.trigger} run of ${run.pipeline} · created ${clock(run.created_at)}`,
            run.scheduled_for ? ` · scheduled for ${clock(run.scheduled_for)}` : "", run.started_at ? ` · took ${took(run)}` : "")),
        badge(run.status), h("span", { class: "spacer" }),
        h("div", { class: "actions" },
          isActive(run) && h("button", { class: "danger", onclick: () => act("Cancel requested", () => api("POST", `/runs/${run.id}/cancel`)) }, "Cancel run"),
          !isActive(run) && h("button", { onclick: () => act("Run requested", async () => {
            const next = await (await api("POST", `/pipelines/${encodeURIComponent(run.pipeline)}/runs`)).json();
            location.hash = "#/runs/" + next.id;
          }) }, "Run again"))),
      run.reason && h("section", null, h("div", { class: "panel empty" + (run.status === "failed" ? " error" : "") }, run.reason)),
      run.steps.length > 0 && h("section", null, h("h2", null, "Steps"), graph(run.steps, (st) => st.status, logState.step, select)),
      run.steps.length > 0 && h("div", { class: "runlayout" },
        h("section", null, h("h2", null, "Step results"), h("div", { class: "panel" }, h("ul", { class: "steps" },
          run.steps.map((st) => h("li", { class: logState.step === st.id ? "selected" : "", title: st.error || "", tabindex: "0", role: "button", "aria-pressed": logState.step === st.id ? "true" : "false",
            onclick: () => select(st.id), onkeydown: (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); select(st.id); } } },
            badge(st.status), h("span", { class: "id" }, st.id),
            h("span", { class: "dur" }, st.exit_code != null && st.exit_code !== 0 ? `exit ${st.exit_code} · ` : "", took(st))))))),
        h("section", null, h("h2", null, "Run log"), h("div", { class: "log" },
          h("div", { class: "bar" },
            logState.step ? [`only step ${logState.step}`, h("button", { onclick: () => select(logState.step) }, "Show all")] : "whole run · click a step to filter",
            h("span", { class: "spacer" }), isActive(run) ? "live" : "complete"),
          pre))),
    ];
  },
};

// ---------- file picker for the register form ----------

// The browser cannot reveal absolute paths, so the daemon lists directories
// (names only, GET /fs) and the dialog builds the path from them.
const picker = { open: false, hidden: false, listing: null, error: "" };

// safeName shows bidirectional control characters as visible escapes, so a
// file name cannot pretend to end in .yaml.
function safeName(name) {
  return name.replace(/[\u202a-\u202e\u2066-\u2069\u200e\u200f]/g, (c) => "\\u" + c.charCodeAt(0).toString(16));
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
    dlg.close();
  };
  dlg.replaceChildren(
    h("h2", { id: "picker-title" }, "Choose a pipeline file"),
    picker.error && h("p", { class: "error", role: "alert", id: "picker-error" }, picker.error),
    l && h("div", { class: "picker-bar" },
      h("button", { type: "button", id: "picker-up", disabled: !l.parent, onclick: () => pickerLoad(l.parent) }, "Up"),
      h("button", { type: "button", id: "picker-home", onclick: () => pickerLoad("") }, "Home"),
      l.roots.length > 1 && l.roots.map((r) => h("button", { type: "button", class: "root", onclick: () => pickerLoad(r) }, r)),
      h("label", { class: "check" }, h("input", { type: "checkbox", id: "picker-hidden", checked: picker.hidden, onchange: (e) => { picker.hidden = e.target.checked; pickerLoad(l.path); } }), "Show hidden")),
    l && h("div", { class: "picker-path mono", id: "picker-path" }, l.path),
    l && (l.entries.length
      ? h("ul", { class: "picker-list", id: "picker-list" }, l.entries.map((en) => h("li", null,
        h("button", { type: "button", class: "entry " + en.kind, onclick: () => (en.kind === "dir" ? pickerLoad(joinPath(l.path, en.name)) : choose(en.name)) },
          h("span", { class: "kind", "aria-hidden": "true" }, en.kind === "dir" ? "📁" : "📄"), safeName(en.name) + (en.link ? " →" : ""), en.kind === "dir" && h("span", { class: "sr" }, " (folder)")))))
      : h("div", { class: "empty" }, "No folders or .yaml files here.")),
    l && l.truncated && h("p", { class: "sub" }, "Only the first entries are shown. Type the path instead."),
    h("div", { class: "picker-foot" }, h("button", { type: "button", id: "picker-cancel", onclick: () => dlg.close() }, "Cancel")));
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

document.getElementById("picker").addEventListener("close", () => {
  picker.open = false;
  // Give focus back only if the user has not already moved on: this event arrives late.
  const active = document.activeElement;
  const input = document.getElementById("register-path");
  if (input && (!active || active === document.body || active.id === "browse")) input.focus();
});

// ---------- routing and refresh ----------

function parseRoute() {
  const parts = location.hash.replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  if (parts[0] === "pipelines") return parts[1] ? { view: views.pipeline, arg: parts[1] } : { view: views.pipelines };
  if (parts[0] === "runs") return parts[1] ? { view: views.run, arg: parts[1] } : { view: views.runs };
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

// route draws the current view. force redraws even while a field has focus: the user just acted.
async function route(force) {
  const mine = ++generation;
  const { view, arg } = parseRoute();
  const main = document.getElementById("view");
  for (const a of document.querySelectorAll("#nav a")) a.classList.toggle("active", a.dataset.route === view.nav);
  if (view !== views.run) { clearTimeout(logState.timer); logState.id = ""; }
  let loaded = false;
  try {
    const data = await view.load(arg);
    loaded = true;
    if (mine !== generation) return;
    // Do not replace a form the user is typing in, or a dialog that is open.
    // Decided now, not before the request: the user may have moved on meanwhile.
    const typing = main.contains(document.activeElement) && document.activeElement.tagName === "INPUT";
    if (picker.open || (typing && !force && lastHash === location.hash)) return;
    const kept = lastHash === location.hash ? focusIndex(main) : -1;
    const scroll = document.getElementById("log") ? document.getElementById("log").scrollTop : null;
    main.replaceChildren(...view.render(data).flat().filter(Boolean));
    if (scroll != null && document.getElementById("log") && !logState.follow) {
      queueMicrotask(() => { document.getElementById("log").scrollTop = scroll; });
    }
    if (kept >= 0) {
      const target = main.querySelectorAll(FOCUSABLE)[kept];
      if (target) target.focus({ preventScroll: true });
    } else if (lastHash !== location.hash) {
      main.focus({ preventScroll: true });
    }
    lastHash = location.hash;
    paintDaemon(true);
  } catch (e) {
    if (mine !== generation) return;
    if (e instanceof ApiError && e.status === 401) {
      main.replaceChildren(h("div", { class: "gate" }, h("h1", null, "Open TRUSS Desktop from your terminal"),
        h("p", { class: "sub" }, "This page needs the daemon's access token. Run this command and it opens with the token:"),
        h("code", null, "truss desktop")));
    } else if (e instanceof ApiError && e.status === 404) {
      main.replaceChildren(h("div", { class: "gate" }, h("h1", null, "Not found"), h("p", { class: "sub" }, e.message), h("a", { href: "#/" }, "Back to the dashboard")));
    } else if (e instanceof ApiError) {
      main.replaceChildren(h("div", { class: "gate", role: "alert" }, h("h1", null, "Something went wrong"), h("p", { class: "sub" }, e.message),
        h("button", { onclick: () => route(true) }, "Try again")));
    } else if (loaded) {
      // The daemon answered; the page itself failed.
      main.replaceChildren(h("div", { class: "gate", role: "alert" }, h("h1", null, "Something went wrong"), h("p", { class: "sub" }, e.message),
        h("button", { onclick: () => route(true) }, "Try again")));
    } else {
      paintDaemon(false);
      lastHash = null;
      if (!main.querySelector(".gate.unreachable")) {
        main.replaceChildren(h("div", { class: "gate unreachable", role: "alert" }, h("h1", null, "Daemon unreachable"),
          h("p", { class: "sub" }, "TRUSS Desktop cannot reach the daemon. Start it and this page reconnects by itself:"),
          h("code", null, "truss daemon start")));
      }
    }
  }
}
const refresh = route;

async function paintDaemon(up) {
  const el = document.getElementById("daemon");
  el.classList.toggle("down", !up);
  if (!up) {
    el.replaceChildren(h("strong", null, h("span", { class: "dot" }), "Daemon unreachable"), "Start it with: truss daemon start");
    return;
  }
  try {
    const st = await getJSON("/status");
    el.replaceChildren(h("strong", null, h("span", { class: "dot" }), "Daemon running"),
      `${st.address} · pid ${st.pid}`, h("br"), `since ${clock(st.started_at)}`);
  } catch { /* shown on the next refresh */ }
}

window.addEventListener("hashchange", () => route());
setInterval(() => { if (!document.hidden) route(); }, REFRESH_MS);
route();
