// Shared helpers for the TRUSS Desktop browser tests.
//
// Each test file starts a real daemon (the built truss binary) with a throwaway
// TRUSS_HOME and HOME, and drives TRUSS Desktop in Chromium through the public
// HTTP API and the page. Nothing here is imported by the product.
"use strict";

const { spawn } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { chromium } = require("playwright");

const exe = process.platform === "win32" ? ".exe" : "";
const TRUSS_BIN = path.resolve(process.env.TRUSS_BIN || path.join(__dirname, "..", "..", "bin", "truss" + exe));

// Pipelines the tests register. Commands work in sh and in cmd.exe.
const NODE_SLEEP = `node -e "setTimeout(() => {}, 120000)"`;
const FIXTURES = {
  "ok.yaml": `version: "1"
pipeline:
  name: ok
steps:
  - id: build
    run: echo building
  - id: test
    needs: [build]
    run: echo testing
  - id: lint
    needs: [build]
    run: echo linting
`,
  "fail.yaml": `version: "1"
pipeline:
  name: broken
steps:
  - id: boom
    run: exit 3
`,
  "slow.yaml": `version: "1"
pipeline:
  name: slow
  overlap: skip
steps:
  - id: first
    run: echo first-line
  - id: wait
    needs: [first]
    run: ${NODE_SLEEP}
`,
  "nightly.yaml": `version: "1"
pipeline:
  name: nightly
on:
  schedule:
    - cron: "0 3 * * *"
steps:
  - id: work
    run: echo nightly
`,
};

// startDaemon boots a daemon and returns what tests need to reach it.
async function startDaemon() {
  if (!fs.existsSync(TRUSS_BIN)) {
    throw new Error(`truss binary not found at ${TRUSS_BIN}; run "go build -o bin/truss ./cmd/truss" or set TRUSS_BIN`);
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "truss-ui-"));
  const homeDir = path.join(root, "home"); // the user's HOME: where the file picker starts
  const trussHome = path.join(root, "truss");
  const pipelines = path.join(homeDir, "pipelines");
  fs.mkdirSync(pipelines, { recursive: true });
  fs.mkdirSync(path.join(homeDir, ".secret-dir"));
  fs.writeFileSync(path.join(homeDir, "notes.txt"), "not a pipeline");
  for (const [name, body] of Object.entries(FIXTURES)) fs.writeFileSync(path.join(pipelines, name), body);

  const env = { ...process.env, HOME: homeDir, USERPROFILE: homeDir, TRUSS_HOME: trussHome };
  const child = spawn(TRUSS_BIN, ["daemon", "run", "--address", "127.0.0.1:0"], { env, stdio: ["ignore", "ignore", "pipe"] });
  let stderr = "";
  child.stderr.on("data", (d) => { stderr += d; });
  const exited = new Promise((resolve) => child.on("exit", resolve));

  const endpointFile = path.join(trussHome, "daemon.json");
  let endpoint;
  for (let i = 0; i < 200 && !endpoint; i++) {
    try { endpoint = JSON.parse(fs.readFileSync(endpointFile, "utf8")); } catch { await sleep(50); }
  }
  if (!endpoint) {
    child.kill();
    throw new Error("the daemon did not start:\n" + stderr);
  }
  const base = "http://" + endpoint.address;
  const call = async (method, p, body) => {
    const res = await fetch(base + "/api/v1" + p, {
      method,
      headers: { Authorization: "Bearer " + endpoint.token, ...(body ? { "Content-Type": "application/json" } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const text = await res.text();
    if (!res.ok) throw new Error(`${method} ${p}: ${res.status} ${text}`);
    return text && res.headers.get("content-type")?.includes("json") ? JSON.parse(text) : text;
  };
  return {
    base, token: endpoint.token, homeDir, pipelines, call,
    // The page takes the token from this fragment, then moves to "#/".
    authedUrl: () => `${base}/#token=${endpoint.token}`,
    register: (file, name) => call("POST", "/pipelines", { path: path.join(pipelines, file), name }),
    async waitRun(id, done = (r) => !["queued", "running"].includes(r.status)) {
      return poll(async () => { const r = await call("GET", "/runs/" + id); return done(r) ? r : null; }, `run ${id}`);
    },
    async stop() {
      try { await call("POST", "/shutdown"); } catch { /* already gone */ }
      const timer = setTimeout(() => child.kill(), 5000);
      await exited;
      clearTimeout(timer);
      fs.rmSync(root, { recursive: true, force: true, maxRetries: 5 });
    },
  };
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// poll calls fn until it returns something truthy, or fails after the timeout.
async function poll(fn, what, timeout = 20000) {
  const end = Date.now() + timeout;
  for (;;) {
    const v = await fn();
    if (v) return v;
    if (Date.now() > end) throw new Error("timed out waiting for " + what);
    await sleep(100);
  }
}

// openBrowser returns a browser plus a function that makes pages which collect
// console errors, page errors and failed requests into page.problems.
async function openBrowser() {
  const browser = await chromium.launch(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {});
  async function newPage(options = {}) {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, ...options });
    const page = await context.newPage();
    page.problems = [];
    page.on("console", (m) => { if (m.type() === "error") page.problems.push("console: " + m.text()); });
    page.on("pageerror", (e) => page.problems.push("pageerror: " + e.message));
    page.on("requestfailed", (r) => {
      const why = r.failure()?.errorText;
      // Chromium reports a bodiless 204 answer to DELETE as ERR_ABORTED although it succeeded.
      if (r.method() === "DELETE" && why === "net::ERR_ABORTED") return;
      page.problems.push(`requestfailed: ${r.method()} ${r.url()} ${why}`);
    });
    return page;
  }
  return { browser, newPage, close: () => browser.close() };
}

// tile returns the number shown on a dashboard counter.
const tile = (page, label) => page.locator(".tile", { hasText: label }).locator(".num");

module.exports = { startDaemon, openBrowser, poll, sleep, tile, FIXTURES };
