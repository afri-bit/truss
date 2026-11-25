"use strict";
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const { startDaemon, openBrowser, poll, tile } = require("./support");

let d, b, page;
before(async () => { d = await startDaemon(); b = await openBrowser(); page = await b.newPage(); });
after(async () => { await b?.close(); await d?.stop(); });

test("dashboard counters follow what the daemon did", async () => {
  await d.register("ok.yaml");
  await d.register("fail.yaml");
  await d.register("slow.yaml");
  await d.register("nightly.yaml");

  const ok = await d.call("POST", "/pipelines/ok/runs");
  const broken = await d.call("POST", "/pipelines/broken/runs");
  await d.waitRun(ok.id);
  await d.waitRun(broken.id);
  const slow = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(slow.id, (r) => r.status === "running");
  await d.call("POST", "/pipelines/slow/runs"); // overlap: skip -> a skipped run

  await page.goto(d.authedUrl());
  await page.locator("h1", { hasText: "Dashboard" }).waitFor();

  const expected = { "flows registered": "4", "running now": "1", "queued": "0", "succeeded today": "1", "failed today": "1", "skipped today": "1" };
  for (const [label, want] of Object.entries(expected)) {
    await poll(async () => (await tile(page, label).textContent()) === want, `tile "${label}" = ${want}`);
  }
  assert.equal(await page.locator("section", { hasText: "Active runs" }).locator("tbody tr").count(), 1);
  // The scheduled pipeline shows under "Coming up".
  await page.locator("section", { hasText: "Coming up" }).getByRole("link", { name: "nightly" }).waitFor();
  // The token is gone from the address bar once taken.
  assert.ok(!page.url().includes("token"), page.url());
  assert.deepEqual(page.problems, []);
  await d.call("POST", `/runs/${slow.id}/cancel`);
});
