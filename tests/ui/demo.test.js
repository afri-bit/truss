"use strict";
// `truss desktop --demo`: TRUSS Desktop on sample data, with no daemon (spec 014, FR-010).
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const { spawn } = require("node:child_process");
const path = require("node:path");
const { openBrowser, poll } = require("./support");

const exe = process.platform === "win32" ? ".exe" : "";
const TRUSS_BIN = path.resolve(process.env.TRUSS_BIN || path.join(__dirname, "..", "..", "bin", "truss" + exe));

let child, url, b;
before(async () => {
  child = spawn(TRUSS_BIN, ["desktop", "--demo", "--no-open"], { stdio: ["ignore", "pipe", "pipe"] });
  url = await new Promise((resolve, reject) => {
    let out = "";
    child.stdout.on("data", (d) => {
      out += d;
      const m = /(http:\/\/127\.0\.0\.1:\d+\/#demo)/.exec(out);
      if (m) resolve(m[1]);
    });
    child.on("exit", (code) => reject(new Error("truss desktop --demo exited with " + code)));
    setTimeout(() => reject(new Error("no address printed")), 10000);
  });
  b = await openBrowser();
});
after(async () => {
  await b?.close();
  child?.kill("SIGINT");
});

test("demo mode shows sample data, says so, and never calls an API", async () => {
  const page = await b.newPage();
  const apiCalls = [];
  page.on("request", (r) => { if (new URL(r.url()).pathname.startsWith("/api/")) apiCalls.push(r.url()); });
  try {
    await page.goto(url);
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await page.locator("#conn", { hasText: "Demo data" }).waitFor();
    await page.locator("#daemon", { hasText: "Demo mode" }).waitFor();
    await poll(async () => Number(await page.locator(".tile", { hasText: "Pipelines registered" }).locator(".num").textContent()) >= 9, "sample pipelines");
    assert.ok(Number(await page.locator(".tile", { hasText: "Running now" }).locator(".num").textContent()) >= 1, "a run is live");

    // Pipelines: an invalid file and a paused schedule are part of the sample.
    await page.evaluate(() => { location.hash = "#/pipelines"; });
    await page.locator("article.card", { hasText: "release" }).locator(".badge", { hasText: "invalid" }).waitFor();
    await page.locator("article.card", { hasText: "infra-plan" }).locator(".badge", { hasText: "paused" }).waitFor();

    // A live run: steps run, the log grows, and Cancel works.
    await page.locator("article.card", { has: page.getByRole("link", { name: "web-app", exact: true }) }).getByRole("button", { name: "Run now" }).click();
    await page.waitForURL(/#\/runs\/./);
    await page.locator(".head .badge.s-running").waitFor({ timeout: 10000 });
    await page.locator(".graph .node.s-running").first().waitFor();
    await poll(async () => (await page.locator("#log").textContent()).includes("fetching origin/main"), "live log");
    await page.getByRole("button", { name: "Cancel run" }).click();
    await page.locator(".head .badge.s-cancelled").waitFor();

    // A reload stays in demo mode for this tab.
    await page.reload();
    await page.locator("#conn", { hasText: "Demo data" }).waitFor();
    assert.deepEqual(apiCalls, [], "demo mode must not call the API");
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});

test("Leave demo returns to the normal page, which finds no daemon here", async () => {
  const page = await b.newPage();
  try {
    await page.goto(url);
    await page.locator("#conn", { hasText: "Demo data" }).waitFor();
    await page.locator("#settings-btn").click();
    await page.getByRole("menuitem", { name: "Leave demo" }).click();
    await page.locator(".gate h1").waitFor();
    assert.equal(await page.evaluate(() => sessionStorage.getItem("truss-demo")), null);
    await page.locator("#conn", { hasText: "Demo data" }).waitFor({ state: "detached" }).catch(() => {});
    assert.ok(!(await page.locator("#conn").textContent()).includes("Demo data"));
  } finally {
    await page.context().close();
  }
});
