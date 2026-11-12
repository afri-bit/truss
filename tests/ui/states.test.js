"use strict";
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const { startDaemon, openBrowser } = require("./support");

let d, b;
before(async () => { d = await startDaemon(); b = await openBrowser(); });
after(async () => { await b?.close(); await d?.stop(); });

test("without a token the page tells you to open it from the terminal", async () => {
  const page = await b.newPage();
  await page.goto(d.base + "/");
  await page.locator(".gate h1", { hasText: "Open TRUSS Desktop from your terminal" }).waitFor();
  await page.locator(".gate code", { hasText: "truss desktop" }).waitFor();
  // A wrong token gets the same answer.
  await page.goto(d.base + "/#token=deadbeef");
  await page.locator(".gate h1", { hasText: "Open TRUSS Desktop from your terminal" }).waitFor();
  await page.context().close();
});

for (const scheme of ["light", "dark"]) {
  test(`every page loads without console errors in ${scheme} mode`, async () => {
    await d.register("ok.yaml").catch(() => {}); // the first scheme registers; the second finds it there
    const run = await d.call("POST", "/pipelines/ok/runs");
    await d.waitRun(run.id);
    const page = await b.newPage({ colorScheme: scheme });
    await page.goto(d.authedUrl());
    for (const [hash, heading] of [["#/", "Dashboard"], ["#/pipelines", "Pipelines"], ["#/pipelines/ok", "ok"], ["#/runs", "Runs"], ["#/runs/" + run.id, run.id]]) {
      await page.evaluate((h) => { location.hash = h; }, hash);
      await page.locator("h1", { hasText: heading }).first().waitFor();
      await page.waitForTimeout(300);
    }
    const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    assert.equal(bg, scheme === "dark" ? "rgb(15, 20, 23)" : "rgb(244, 242, 238)");
    assert.deepEqual(page.problems, []);
    await page.context().close();
  });
}

test("the daemon going away shows the unreachable state", async () => {
  const other = await startDaemon();
  const page = await b.newPage();
  await page.goto(other.authedUrl());
  await page.locator("h1", { hasText: "Dashboard" }).waitFor();
  await other.stop();
  await page.locator(".gate.unreachable h1", { hasText: "Daemon unreachable" }).waitFor();
  await page.locator("#daemon.down").waitFor();
  await page.context().close();
});

test("an empty daemon shows helpful empty states", async () => {
  const empty = await startDaemon();
  const page = await b.newPage();
  try {
    await page.goto(empty.authedUrl());
    await page.getByText("Nothing is running.").waitFor();
    await page.getByText("No schedules.").waitFor();
    await page.getByText("No runs yet.").first().waitFor();
    await page.evaluate(() => { location.hash = "#/pipelines"; });
    await page.getByText("No pipelines registered yet").waitFor();
    // Picker over a folder with nothing to show.
    await page.locator("#register-path").fill(require("node:path").join(empty.homeDir, ".secret-dir", "x.yaml"));
    await page.locator("#browse").click();
    await page.locator("#picker-path", { hasText: ".secret-dir" }).waitFor();
    await page.getByText("No folders or .yaml files here.").waitFor();
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
    await empty.stop();
  }
});
