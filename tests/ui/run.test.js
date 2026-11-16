"use strict";
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const { startDaemon, openBrowser, poll } = require("./support");

let d, b, page;
before(async () => { d = await startDaemon(); b = await openBrowser(); page = await b.newPage(); });
after(async () => { await b?.close(); await d?.stop(); });

const log = () => page.locator("#log");

test("run page: step graph, step filter, then re-run", async () => {
  await d.register("ok.yaml");
  const run = await d.call("POST", "/pipelines/ok/runs");
  await d.waitRun(run.id);
  await page.goto(d.authedUrl());
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);

  await page.locator(".graph .node").first().waitFor();
  assert.equal(await page.locator(".graph .node").count(), 3);
  assert.equal(await page.locator(".graph .node.s-succeeded").count(), 3);
  assert.equal(await page.locator(".graph .edge").count(), 2);
  await poll(async () => (await log().textContent()).includes("linting"), "log text");
  const full = await log().textContent();
  for (const word of ["building", "testing", "linting"]) assert.ok(full.includes(word), word);

  // Selecting a step filters the log to it.
  await page.locator(".graph .node", { hasText: "test" }).click();
  await poll(async () => { const t = await log().textContent(); return t.includes("testing") && !t.includes("linting") && !t.includes("building"); }, "filtered log");
  await page.getByRole("button", { name: "Show all" }).click();
  await poll(async () => (await log().textContent()).includes("building"), "unfiltered log");

  // The same filter works from the keyboard on the step list.
  await page.locator(".steps li", { hasText: "lint" }).focus();
  await page.keyboard.press("Enter");
  await poll(async () => { const t = await log().textContent(); return t.includes("linting") && !t.includes("testing"); }, "keyboard filter");

  await page.getByRole("button", { name: "Re-run" }).click();
  await page.waitForFunction((id) => !location.hash.endsWith(id), run.id);
  await page.locator(".badge.s-succeeded").first().waitFor();
  assert.deepEqual(page.problems, []);
});

test("run page: live log while running, then cancel", async () => {
  await d.register("slow.yaml");
  const run = await d.call("POST", "/pipelines/slow/runs");
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
  await page.locator(".log .bar", { hasText: "live" }).waitFor();
  await poll(async () => (await log().textContent()).includes("first-line"), "live log line");
  await page.locator(".head .badge.s-running").waitFor();
  await page.locator(".graph .node.s-running").waitFor();
  assert.equal(await page.getByRole("button", { name: "Re-run" }).count(), 0);

  await page.getByRole("button", { name: "Cancel run" }).click();
  await page.locator(".head .badge.s-cancelled").waitFor();
  await page.locator(".log .bar", { hasText: "complete" }).waitFor();
  await poll(async () => (await log().textContent()).includes("summary cancelled"), "cancel summary");
  await page.getByRole("button", { name: "Re-run" }).waitFor();
  assert.deepEqual(page.problems, []);
});

test("run page: a failed run shows the exit code and the reason", async () => {
  await d.register("fail.yaml");
  const run = await d.call("POST", "/pipelines/broken/runs");
  await d.waitRun(run.id);
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
  await page.locator(".head .badge.s-failed").waitFor();
  await page.locator(".steps li", { hasText: "exit 3" }).waitFor();
  assert.deepEqual(page.problems, []);
});

test("run page: an unknown run says so", async () => {
  await page.evaluate(() => { location.hash = "#/runs/does-not-exist"; });
  await page.locator(".gate h1", { hasText: "Not found" }).waitFor();
});
