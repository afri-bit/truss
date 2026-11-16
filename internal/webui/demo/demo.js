// TRUSS Desktop demo data. Used only when the page is opened with #demo
// (`truss desktop --demo`); otherwise nothing here runs.
//
// It answers the same /api/v1 paths as the daemon, from an in-memory world whose
// runs move forward in real time: a live multi-stage run, a long backup, a fan-out,
// a failing pipeline, a tolerated failure, a queued run, a paused schedule, an
// invalid file, a long name, and 24 hours of history. Nothing is sent anywhere.
"use strict";

window.TrussDemo = (function () {
  const T0 = Date.now();
  const S = 1000, M = 60 * S, H = 60 * M;
  const pad = (n) => String(n).padStart(2, "0");
  const stamp = (ms) => { const d = new Date(ms); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`; };
  const iso = (ms) => (ms == null ? undefined : new Date(ms).toISOString());
  const hash = (str) => { let x = 2166136261; for (let i = 0; i < str.length; i++) { x ^= str.charCodeAt(i); x = Math.imul(x, 16777619); } return x >>> 0; };
  const runId = (ms, salt) => { const d = new Date(ms); return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}-${(hash(salt + ms) % 0xffffff).toString(16).padStart(6, "0")}`; };
  const HOME = "/home/demo";

  // step(id, needs, seconds, lines, extra): extra may set fail (exit code), tolerate (continue-on-error).
  const st = (id, needs, secs, lines, extra) => Object.assign({ id, needs: needs || [], secs, lines: lines || [] }, extra || {});
  const PIPELINES = [
    { name: "web-app", dir: "web-app", concurrency: 3, overlap: "queue", cron: "*/30 * * * *", every: 30 * M, steps: [
      st("setup/checkout", [], 2, ["fetching origin/main", "HEAD is now at 4f2c9e1 fix: header alignment"]),
      st("setup/install", ["setup/checkout"], 6, ["resolving 412 packages", "added 412 packages in 5.8s"]),
      st("lint/eslint", ["setup/install"], 8, ["linting src/ chunk 1/4", "linting src/ chunk 2/4", "linting src/ chunk 3/4", "linting src/ chunk 4/4", "0 problems"]),
      st("test/unit", ["setup/install"], 40, Array.from({ length: 20 }, (_, i) => `PASS src/module_${pad(i + 1)}.test.ts (${(0.4 + (i % 5) * 0.3).toFixed(1)} s)`)),
      st("types/tsc", ["setup/install"], 7, ["checking 318 files", "no type errors"]),
      st("build/bundle", ["lint/eslint", "test/unit", "types/tsc"], 12, ["bundling 1,204 modules", "dist/app.js  412.6 kB  gzip 121.3 kB"]),
      st("e2e/playwright", ["build/bundle"], 20, ["running 38 tests using 4 workers", "38 passed (19.4s)"]),
      st("package/image", ["build/bundle"], 10, ["building image web-app:4f2c9e1", "layer 7/7 done"]),
      st("publish/upload", ["e2e/playwright", "package/image"], 4, ["uploaded web-app:4f2c9e1"]),
    ] },
    { name: "api-service", dir: "api", concurrency: 2, overlap: "queue", cron: "0 * * * *", every: 60 * M, steps: [
      st("build", [], 8, ["go build ./...", "ok"]),
      st("vet", ["build"], 4, ["go vet ./...", "ok"]),
      st("test", ["build"], 18, ["ok  api/internal/store   2.1s", "ok  api/internal/http    3.4s", "ok  api/internal/auth    1.2s"]),
      st("migrate-check", ["test", "vet"], 3, ["14 migrations, none pending"]),
      st("deploy-staging", ["migrate-check"], 6, ["rolled out 3/3 replicas"]),
    ] },
    { name: "data-sync", dir: "data-sync", concurrency: 2, overlap: "skip", cron: "15 * * * *", every: 60 * M, steps: [
      st("extract", [], 6, ["pulled batch 1/3", "pulled batch 2/3", "pulled batch 3/3"]),
      st("transform", ["extract"], 3, ["normalising 18,204 rows", "schema mismatch in column created_at"], { fail: 3 }),
      st("load", ["transform"], 4, ["loaded"]),
    ] },
    { name: "docs-site", dir: "docs", concurrency: 2, overlap: "queue", steps: [
      st("build", [], 4, ["building 128 pages", "done"]),
      st("links", ["build"], 3, ["checked 1,904 links"]),
      st("spell", ["build"], 2, ["3 unknown words: daemonless, runlog, truss.yaml"], { fail: 1, tolerate: true }),
      st("deploy", ["links"], 3, ["deployed preview"]),
    ] },
    { name: "nightly-backup", dir: "backup", concurrency: 1, overlap: "skip", cron: "30 2 * * *", every: 24 * H, steps: [
      st("snapshot", [], 5, ["snapshot of /srv/data created"]),
      st("compress", ["snapshot"], 600, Array.from({ length: 60 }, (_, i) => `compressing volume ${i + 1}/60`)),
      st("upload", ["compress"], 30, ["uploading 18.2 GiB", "upload complete"]),
    ] },
    { name: "ml-training", dir: "ml", concurrency: 6, overlap: "queue", steps: [
      st("prepare", [], 6, ["sharding dataset into 6 parts"]),
      ...Array.from({ length: 6 }, (_, i) => st(`train/shard-${i + 1}`, ["prepare"], 70 + i * 9, Array.from({ length: 10 }, (_, k) => `epoch ${k + 1}/10 loss ${(1.2 - k * 0.09 - i * 0.01).toFixed(3)}`))),
      st("evaluate", Array.from({ length: 6 }, (_, i) => `train/shard-${i + 1}`), 15, ["accuracy 0.913"]),
    ] },
    { name: "infra-plan", dir: "infra", concurrency: 1, overlap: "skip", cron: "0 9 * * 1-5", every: 24 * H, paused: true, steps: [
      st("init", [], 3, ["initialising providers"]),
      st("plan", ["init"], 5, ["Plan: 3 to add, 1 to change, 0 to destroy."]),
    ] },
    { name: "mobile-release-candidate-build-and-sign", dir: "mobile/release-candidate-with-a-rather-long-folder-name", concurrency: 2, overlap: "queue", steps: [
      st("android/assemble-release-bundle", [], 25, ["bundleRelease", "BUILD SUCCESSFUL in 24s"]),
      st("ios/archive-and-export-ipa", [], 30, ["xcodebuild archive", "EXPORT SUCCEEDED"]),
      st("sign/notarize-and-staple", ["android/assemble-release-bundle", "ios/archive-and-export-ipa"], 8, ["signed 2 artifacts"]),
    ] },
    { name: "cleanup", dir: "cleanup", concurrency: 1, overlap: "parallel", steps: [st("prune", [], 2, ["removed 14 old artifacts"])] },
  ];
  const INVALID = { name: "release", dir: "release", error: 'step "tag" timeout must be a duration such as 90s, 5m or 1h30m, got "soon"' };

  const world = { pipelines: new Map(), runs: [], cancelled: new Map(), seq: 0 };
  for (const p of PIPELINES) world.pipelines.set(p.name, { def: p, paused: !!p.paused, registered: T0 - 9 * 24 * H + hash(p.name) % H, valid: true });
  world.pipelines.set(INVALID.name, { def: { name: INVALID.name, dir: INVALID.dir, steps: [] }, paused: false, registered: T0 - 2 * H, valid: false, error: INVALID.error });

  // plan places each step in time: a step starts when its needs are done and a slot is free.
  function plan(def, salt) {
    const out = new Map(), slots = Math.max(1, def.concurrency || 1);
    let pending = def.steps.slice(), running = [], t = 0;
    while (pending.length || running.length) {
      for (const s of pending.slice()) {
        if (running.length >= slots) break;
        const needs = s.needs.map((n) => out.get(n));
        if (needs.some((n) => !n || n.end > t)) continue;
        pending = pending.filter((x) => x !== s);
        if (needs.some((n) => n.state === "failed" || n.state === "skipped")) {
          const why = s.needs.find((n) => ["failed", "skipped"].includes(out.get(n).state));
          out.set(s.id, { start: t, end: t, state: "skipped", reason: `skipped because ${why} ${out.get(why).state === "failed" ? "failed" : "was skipped"}` });
          continue;
        }
        const jitter = 0.85 + (hash(salt + s.id) % 30) / 100;
        const dur = Math.max(1, Math.round(s.secs * jitter)) * S;
        const state = s.fail ? (s.tolerate ? "tolerated" : "failed") : "succeeded";
        const e = { start: t, end: t + dur, state, exit: s.fail || 0 };
        out.set(s.id, e);
        running.push(e);
      }
      if (!running.length) { if (pending.length) { t += S; continue; } break; }
      t = Math.min(...running.map((r) => r.end));
      running = running.filter((r) => r.end > t);
    }
    return out;
  }

  function addRun(name, created, trigger, extra) {
    const p = world.pipelines.get(name);
    const id = runId(created, name + world.seq++);
    const run = Object.assign({ id, pipeline: name, trigger, created, start: created + 600, plan: plan(p.def, id) }, extra || {});
    world.runs.push(run);
    return run;
  }

  // History: the last 24 hours, plus live runs that keep cycling while the page is open.
  for (const p of PIPELINES) {
    const every = p.every || 3 * H;
    for (let t = T0 - 24 * H + (hash(p.name) % every); t < T0 - 15 * M; t += every) {
      if (p.paused && t > T0 - 6 * H) continue;
      addRun(p.name, t, p.cron ? "schedule" : "manual", { scheduled: p.cron ? t : null });
    }
  }
  addRun("docs-site", T0 - 50 * M, "local");
  addRun("data-sync", T0 - 70 * M, "schedule", { skipped: "a run of data-sync was still running (overlap: skip)" });
  addRun("web-app", T0 - 25 * S, "manual");
  addRun("nightly-backup", T0 - 3 * M, "schedule", { scheduled: T0 - 3 * M });
  addRun("ml-training", T0 - 40 * S, "manual");
  addRun("api-service", T0 - 4 * S, "schedule", { start: T0 + 45 * S, scheduled: T0 - 4 * S }); // queued: waits for a slot
  addRun("data-sync", T0 - 8 * S, "manual");
  const LIVE = [["web-app", 150 * S], ["data-sync", 60 * S], ["docs-site", 90 * S]];
  function cycle(now) {
    for (const [name, every] of LIVE) {
      const last = world.runs.filter((r) => r.pipeline === name).reduce((a, b) => (b.created > a.created ? b : a));
      if (now - last.created >= every) addRun(name, now, name === "data-sync" ? "schedule" : "manual");
    }
  }

  function stepView(run, s, now) {
    const e = run.plan.get(s.id);
    const begin = run.start + e.start, end = run.start + e.end;
    const cancelAt = world.cancelled.get(run.id);
    const view = { id: s.id, needs: s.needs.length ? s.needs : undefined };
    if (cancelAt != null && cancelAt < end) {
      if (cancelAt <= begin) return Object.assign(view, { status: "skipped", reason: "skipped because the run was cancelled" });
      return Object.assign(view, { status: "cancelled", started_at: iso(begin), finished_at: iso(cancelAt) });
    }
    if (e.state === "skipped") return now < begin ? Object.assign(view, { status: "pending" }) : Object.assign(view, { status: "skipped", reason: e.reason });
    if (now < begin) return Object.assign(view, { status: "pending" });
    if (now < end) return Object.assign(view, { status: "running", started_at: iso(begin), pid: 2000 + (hash(run.id + s.id) % 61000) });
    return Object.assign(view, {
      pid: 2000 + (hash(run.id + s.id) % 61000),
      status: e.state === "tolerated" ? "failed" : e.state, continued_on_error: e.state === "tolerated" || undefined,
      exit_code: e.exit || undefined, started_at: iso(begin), finished_at: iso(end),
    });
  }

  function runView(run, now) {
    const def = world.pipelines.get(run.pipeline).def;
    const base = { id: run.id, pipeline: run.pipeline, trigger: run.trigger, created_at: iso(run.created), scheduled_for: iso(run.scheduled || undefined) };
    if (run.skipped) return Object.assign(base, { status: "skipped", reason: run.skipped, steps: [] });
    if (now < run.start) return Object.assign(base, { status: "queued", steps: def.steps.map((s) => ({ id: s.id, needs: s.needs.length ? s.needs : undefined, status: "pending" })) });
    const steps = def.steps.map((s) => stepView(run, s, now));
    const ends = [...run.plan.values()].map((e) => run.start + e.end);
    const cancelAt = world.cancelled.get(run.id);
    const doneAt = cancelAt != null ? Math.min(cancelAt, Math.max(...ends)) : Math.max(...ends);
    const out = Object.assign(base, { started_at: iso(run.start), steps });
    if (now < doneAt) return Object.assign(out, { status: "running" });
    out.finished_at = iso(doneAt);
    if (cancelAt != null && cancelAt < Math.max(...ends)) return Object.assign(out, { status: "cancelled", reason: "cancelled from TRUSS Desktop" });
    const bad = steps.find((s) => s.status === "failed" && !s.continued_on_error);
    if (bad) return Object.assign(out, { status: "failed", reason: `${run.pipeline} failed at ${bad.id} (exit ${bad.exit_code})` });
    return Object.assign(out, { status: "succeeded" });
  }

  function logText(run, now) {
    const def = world.pipelines.get(run.pipeline).def;
    if (run.skipped || now < run.start) return "";
    const lines = [];
    const at = (ms, text) => { if (ms <= now) lines.push([ms, text]); };
    at(run.start, "==== Setup: the environment the steps run in ====");
    at(run.start, "setup environment ready");
    at(run.start, "setup os linux");
    at(run.start, "setup arch amd64");
    at(run.start, "setup shell /bin/sh");
    at(run.start, `setup directory ${HOME}/projects/${def.dir}`);
    const cancelAt = world.cancelled.get(run.id);
    for (const s of def.steps) {
      const e = run.plan.get(s.id);
      const begin = run.start + e.start, end = run.start + e.end;
      if (cancelAt != null && cancelAt <= begin) continue;
      if (e.state === "skipped") { at(begin, `step ${s.id} ${e.reason}`); continue; }
      at(begin, `==== Step ${s.id} ====`);
      at(begin, `step ${s.id} started`);
      const stop = cancelAt != null ? Math.min(cancelAt, end) : end;
      s.lines.forEach((text, i) => { const ms = begin + ((i + 1) / (s.lines.length + 1)) * (end - begin); if (ms < stop) at(ms, `step ${s.id} | ${text}`); });
      const secs = ((end - begin) / 1000).toFixed(3);
      if (cancelAt != null && cancelAt < end) { at(cancelAt, `step ${s.id} stopped because the run was cancelled`); continue; }
      at(end, `step ${s.id} finished duration ${secs}s exit ${e.exit}`);
      if (e.state === "tolerated") at(end, `step ${s.id} failed but the run continues (continue-on-error)`);
    }
    const v = runView(run, now);
    if (v.finished_at) {
      const fin = Date.parse(v.finished_at), total = ((fin - run.start) / 1000).toFixed(3);
      at(fin, "==== Summary ====");
      at(fin, `summary ${v.status} steps ${def.steps.length} duration ${total}s`);
      if (v.status === "failed") at(fin, `error ${v.reason}`);
    }
    lines.sort((a, b) => a[0] - b[0]);
    return lines.map(([ms, text]) => `${stamp(ms)} ${text}\n`).join("");
  }

  function nextFire(p, now) {
    const def = p.def;
    if (!def.cron) return null;
    const every = def.every, phase = hash(def.name) % every;
    return now + ((phase - ((now - T0) % every) + every) % every || every);
  }

  function pipelineView(name, now) {
    const p = world.pipelines.get(name), def = p.def;
    const runs = world.runs.filter((r) => r.pipeline === name).sort((a, b) => b.created - a.created).map((r) => runView(r, now));
    const base = { name, path: `${HOME}/projects/${def.dir}/truss.yaml`, paused: p.paused, registered_at: iso(p.registered), valid: p.valid, schedules: [], steps: [], active_runs: 0 };
    if (!p.valid) return Object.assign(base, { error: p.error });
    const next = nextFire(p, now);
    return Object.assign(base, {
      definition_name: name, concurrency: def.concurrency, overlap: def.overlap,
      schedules: def.cron ? [{ cron: def.cron, missed: "skip", next: p.paused ? undefined : iso(next) }] : [],
      steps: def.steps.map((s) => ({ id: s.id, needs: s.needs.length ? s.needs : undefined })),
      active_runs: runs.filter((r) => r.status === "running" || r.status === "queued").length,
      last_run: runs[0],
    });
  }

  const FS = {
    [HOME]: { dirs: ["projects", "notes"], files: ["truss.yaml"] },
    [HOME + "/projects"]: { dirs: PIPELINES.map((p) => p.dir.split("/")[0]).concat("release", "sandbox").filter((x, i, a) => a.indexOf(x) === i).sort(), files: [] },
    [HOME + "/notes"]: { dirs: [], files: [] },
  };
  function listing(path, hidden) {
    const dir = (path || HOME).replace(/\/+$/, "") || "/";
    const node = FS[dir] || (dir.startsWith(HOME + "/projects/") ? { dirs: [], files: ["truss.yaml", "truss.ci.yml"] } : null);
    if (!node) return null;
    const entries = node.dirs.map((name) => ({ name, kind: "dir" })).concat(node.files.map((name) => ({ name, kind: "file" })));
    if (hidden) entries.unshift({ name: ".config", kind: "dir" });
    return { path: dir, parent: dir === "/" ? "" : dir.slice(0, dir.lastIndexOf("/")) || "/", roots: ["/"], entries };
  }

  // Sample resource figures in the shape of GET /metrics: smooth, deterministic noise.
  const noise = (seed, t) => {
    const a = (seed % 997) / 997 * 6.283, b = (seed % 431) / 431 * 6.283, c = (seed % 211) / 211 * 6.283;
    return (Math.sin(t * 0.21 + a) * 0.5 + Math.sin(t * 0.57 + b) * 0.3 + Math.sin(t * 1.33 + c) * 0.2 + 1) / 2;
  };
  const clamp = (v) => Math.max(0, Math.min(100, v));
  const CORES = 8, MEM = 32 * 1024 ** 3, INTERVAL = 2;
  function hostAt(ms, running) {
    const t = ms / 1000;
    const cpu = clamp(6 + running * 9 + noise(11, t) * 14 + (running ? noise(97, t * 2) * 8 : 0));
    const used = (9.4 + running * 0.7 + noise(23, t * 0.3) * 0.8) * 1024 ** 3;
    return { cpu, used, cores: Array.from({ length: CORES }, (_, i) => clamp(cpu + (noise(300 + i, t) - 0.5) * 50 * (i % 3 === 0 ? 1.3 : 0.8))) };
  }
  function metrics(now) {
    const sampled = now - (now % (INTERVAL * 1000));
    const live = world.runs.map((r) => [r, runView(r, sampled)]).filter(([, v]) => v.status === "running");
    const steps = [];
    for (const [r, v] of live) for (const s of v.steps.filter((x) => x.status === "running")) {
      const seed = hash(r.id + "/" + s.id), t = sampled / 1000;
      const procs = 1 + (seed % 4);
      steps.push({ run_id: r.id, step_id: s.id, pid: s.pid, cpu_percent: clamp(1 + noise(seed, t) * 11), rss_bytes: Math.round((18 + (seed % 140) + noise(seed + 7, t * 0.4) * 40) * 1024 ** 2), threads: procs + (seed % 6), processes: procs, tree: true });
    }
    const h = hostAt(sampled, live.length);
    const history = [];
    for (let at = sampled - 10 * 60 * 1000 + INTERVAL * 1000; at <= sampled; at += INTERVAL * 1000) {
      const x = hostAt(at, live.length);
      history.push({ at: iso(at), cpu_percent: x.cpu, memory_used_bytes: Math.round(x.used) });
    }
    return { supported: true, os: "demo", interval_seconds: INTERVAL, sampled_at: iso(sampled),
      host: { cpu_percent: h.cpu, cores: h.cores, memory_total_bytes: MEM, memory_used_bytes: Math.round(h.used), load: [h.cpu / 100 * CORES * 0.9, 1.4, 1.1] },
      daemon: { pid: 4242, cpu_percent: 0.3 + h.cpu / 80, rss_bytes: 41 * 1024 ** 2, threads: 14, processes: 1, tree: false },
      steps, history };
  }

  const json = (status, body, headers) => new Response(body === undefined ? null : JSON.stringify(body), { status, headers: Object.assign({ "Content-Type": "application/json" }, headers || {}) });
  const fail = (status, message) => json(status, { error: message });

  async function request(method, path, body) {
    const now = Date.now();
    cycle(now);
    await new Promise((r) => setTimeout(r, 25)); // a little latency, like a real daemon
    const url = new URL(path, "http://demo");
    const parts = url.pathname.split("/").filter(Boolean).map(decodeURIComponent);
    const all = () => world.runs.slice().sort((a, b) => b.created - a.created);
    const route = method + " " + parts.map((x, i) => (i === 1 ? ":x" : i === 3 ? ":y" : x)).join("/");
    switch (route) {
      case "GET status": {
        const views = all().map((r) => runView(r, now));
        return json(200, { version: "0.2.0-beta-demo", pid: 4242, started_at: iso(T0 - 3 * H - 12 * M), home: HOME + "/.truss", address: location.host, pipelines: world.pipelines.size,
          running_runs: views.filter((r) => r.status === "running").length, queued_runs: views.filter((r) => r.status === "queued").length, max_parallel_runs: 4 });
      }
      case "GET metrics": return json(200, metrics(now));
      case "GET pipelines": return json(200, [...world.pipelines.keys()].sort().map((n) => pipelineView(n, now)));
      case "GET pipelines/:x": return world.pipelines.has(parts[1]) ? json(200, pipelineView(parts[1], now)) : fail(404, `pipeline is not registered: "${parts[1]}"`);
      case "POST pipelines": {
        const file = String((body && body.path) || "").trim();
        if (!file.startsWith("/")) return fail(400, "the path must be absolute");
        const name = (body.name || "").trim() || file.split("/").slice(-2, -1)[0] || "pipeline";
        if (world.pipelines.has(name)) return fail(409, `a pipeline named "${name}" is already registered`);
        world.pipelines.set(name, { def: { name, dir: file.replace(HOME + "/projects/", "").replace(/\/[^/]*$/, ""), concurrency: 1, overlap: "queue", steps: [st("hello", [], 2, ["hello from the demo"])] }, paused: false, registered: now, valid: true });
        return json(201, pipelineView(name, now));
      }
      case "DELETE pipelines/:x": return world.pipelines.delete(parts[1]) ? new Response(null, { status: 204 }) : fail(404, "not registered");
      case "POST pipelines/:x/pause": case "POST pipelines/:x/resume": {
        const p = world.pipelines.get(parts[1]);
        if (!p) return fail(404, "not registered");
        p.paused = parts[2] === "pause";
        return json(200, pipelineView(parts[1], now));
      }
      case "POST pipelines/:x/runs": {
        const p = world.pipelines.get(parts[1]);
        if (!p) return fail(404, "not registered");
        if (!p.valid) return fail(400, p.error);
        return json(201, runView(addRun(parts[1], now, "manual"), now));
      }
      case "GET runs": {
        const limit = Number(url.searchParams.get("limit")) || 50, only = url.searchParams.get("pipeline");
        return json(200, all().filter((r) => !only || r.pipeline === only).slice(0, limit).map((r) => runView(r, now)));
      }
      case "GET runs/:x": { const r = world.runs.find((x) => x.id === parts[1]); return r ? json(200, runView(r, now)) : fail(404, `no run with id ${parts[1]}`); }
      case "POST runs/:x/cancel": {
        const r = world.runs.find((x) => x.id === parts[1]);
        if (!r) return fail(404, "no such run");
        const v = runView(r, now);
        if (v.status !== "running" && v.status !== "queued") return fail(409, "the run has already ended");
        world.cancelled.set(r.id, now);
        if (now < r.start) r.start = now;
        return json(200, runView(r, now));
      }
      case "GET runs/:x/log": {
        const r = world.runs.find((x) => x.id === parts[1]);
        if (!r) return fail(404, "no such run");
        const bytes = new TextEncoder().encode(logText(r, now));
        const from = Math.min(bytes.length, Number(url.searchParams.get("offset")) || 0);
        return new Response(bytes.slice(from), { status: 200, headers: { "Content-Type": "text/plain; charset=utf-8", "X-Truss-Log-Offset": String(bytes.length), "X-Truss-Run-Status": runView(r, now).status } });
      }
      case "GET fs": {
        const l = listing(url.searchParams.get("path"), url.searchParams.get("hidden") === "true");
        return l ? json(200, l) : fail(404, "no such folder");
      }
      case "POST shutdown": return fail(400, "the demo has no daemon to stop");
      default: return fail(404, "no such API endpoint");
    }
  }

  return { request };
})();
