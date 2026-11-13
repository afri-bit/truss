"use strict";
// The page shell from spec 014: sidebar, header, settings, finder, and the simulated-figures switch.
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const { startDaemon, openBrowser, poll, sleep } = require("./support");

let d, b;
before(async () => {
  d = await startDaemon();
  b = await openBrowser();
  await d.register("ok.yaml");
  await d.register("slow.yaml");
});
after(async () => { await b?.close(); await d?.stop(); });

const active = (page) => page.evaluate(() => {
  const el = document.activeElement;
  return el ? (el.id || el.getAttribute("href") || el.textContent.trim()) : "";
});

test("with simulated figures off, no simulated number or label is shown", async () => {
  const run = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(run.id, (r) => r.status === "running");
  const page = await b.newPage();
  await page.addInitScript(() => localStorage.setItem("truss-settings", JSON.stringify({ sim: false })));
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await page.locator("section", { hasText: "System" }).getByText("Simulated figures are off").waitFor();
    for (const hash of ["#/", "#/monitor", "#/runs/" + run.id]) {
      await page.evaluate((h) => { location.hash = h; }, hash);
      await page.waitForTimeout(800);
      assert.equal(await page.locator(".sim").count(), 0, `simulated label on ${hash}`);
      assert.equal(await page.locator(".chart, .core, .meter").count(), 0, `simulated figure on ${hash}`);
    }
    // The daemon's process row shows its real pid and says the rest is not reported.
    await page.evaluate(() => { location.hash = "#/monitor"; });
    const status = await d.call("GET", "/status");
    await page.locator("section", { hasText: "Processes" }).locator("td", { hasText: String(status.pid) }).waitFor();
    await page.locator("section", { hasText: "Processes" }).getByText("not reported", { exact: true }).waitFor();
    assert.deepEqual(page.problems, []);
  } finally {
    await d.call("POST", `/runs/${run.id}/cancel`);
    await d.waitRun(run.id); // slow.yaml skips overlapping runs: let this one end first
    await page.context().close();
  }
});

test("with simulated figures on, every simulated panel is labeled", async () => {
  const run = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(run.id, (r) => r.status === "running");
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/monitor"; });
    await page.locator("h1", { hasText: "Monitor" }).waitFor();
    for (const title of ["CPU", "Memory", "Cores", "Processes"]) {
      await page.locator(".panel-head", { has: page.locator("h2", { hasText: new RegExp(`^${title}$`) }) }).locator(".sim").waitFor();
    }
    assert.deepEqual(page.problems, []);
  } finally {
    await d.call("POST", `/runs/${run.id}/cancel`);
    await d.waitRun(run.id); // slow.yaml skips overlapping runs: let this one end first
    await page.context().close();
  }
});

test("refreshes keep keyboard focus on a sidebar run and on a breadcrumb", async () => {
  const run = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(run.id, (r) => r.status === "running");
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    const link = page.locator(".side-run").first();
    await link.waitFor();
    await link.focus();
    const want = await active(page);
    await sleep(3500);
    assert.equal(await active(page), want, "focus left the sidebar run");

    await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
    await page.locator(".head .badge.s-running").waitFor(); // navigation itself moves focus to the new page
    await page.locator("#crumbs a", { hasText: "slow" }).waitFor();
    await page.locator("#crumbs a", { hasText: "Runs" }).focus();
    await sleep(3500);
    assert.equal(await active(page), "#/runs", "focus left the breadcrumb");
  } finally {
    await d.call("POST", `/runs/${run.id}/cancel`);
    await d.waitRun(run.id); // slow.yaml skips overlapping runs: let this one end first
    await page.context().close();
  }
});

test("the run page keeps refreshing while Find in log has focus", async () => {
  const run = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(run.id, (r) => r.status === "running");
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
    await page.locator(".head .badge.s-running").waitFor();
    const find = page.getByRole("searchbox", { name: "Find in log" });
    await find.fill("first");
    await d.call("POST", `/runs/${run.id}/cancel`);
    await d.waitRun(run.id);
    await page.locator(".head .badge.s-cancelled").waitFor();
    assert.equal(await find.inputValue(), "first");
    assert.equal(await active(page), "log-find", "focus stays in the find field");
    assert.ok(await find.evaluate((el) => el === document.activeElement));
  } finally {
    await page.context().close();
  }
});

test("typing in the Runs filter keeps focus when a new status chip appears", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/runs"; });
    await page.locator("h1", { hasText: "Runs" }).waitFor();
    const filter = page.locator("#runs-filter");
    await filter.fill("o");
    const run = await d.call("POST", "/pipelines/ok/runs"); // brings in or updates status chips above the field
    await d.waitRun(run.id);
    await sleep(3500);
    assert.ok(await filter.evaluate((el) => el === document.activeElement), "focus left the filter");
    await page.keyboard.type("k");
    assert.equal(await filter.inputValue(), "ok");
  } finally {
    await page.context().close();
  }
});

test("settings menu: theme and density stay open, persist, and Escape returns focus", async () => {
  const page = await b.newPage({ colorScheme: "light" });
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await page.locator("#settings-btn").click();
    const menu = page.locator("#settings-menu");
    await menu.waitFor();
    await menu.getByRole("button", { name: "Dark", exact: true }).click();
    assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), "dark");
    assert.ok(await menu.isVisible(), "the menu stays open after choosing a theme");
    assert.ok(await menu.evaluate((m) => m.contains(document.activeElement)), "focus stays in the menu");
    await menu.getByRole("button", { name: "Compact", exact: true }).click();
    assert.equal(await page.evaluate(() => document.documentElement.dataset.density), "compact");
    await page.keyboard.press("Escape");
    assert.ok(await menu.isHidden());
    assert.equal(await active(page), "settings-btn");

    await page.reload();
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), "dark", "theme persists");
    assert.equal(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), "rgb(12, 14, 18)");
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});

test("sidebar collapses with the button and with [", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    const state = () => page.evaluate(() => document.documentElement.dataset.sidebar || "expanded");
    await page.locator("#collapse").click();
    assert.equal(await state(), "collapsed");
    assert.equal(await page.locator("#collapse").getAttribute("aria-expanded"), "false");
    await page.locator("#view").focus();
    await page.keyboard.press("[");
    assert.equal(await state(), "expanded");
  } finally {
    await page.context().close();
  }
});

test("Jump to finds a pipeline and opens it with Enter", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await poll(async () => (await page.locator(".side-label").count()) > 0, "shell poll");
    await page.locator("#view").focus();
    await page.keyboard.press("/");
    await page.locator("#finder[open]").waitFor();
    await page.keyboard.type("slow");
    await page.locator("#finder [role=option]", { hasText: "slow" }).first().waitFor();
    await page.keyboard.press("ArrowDown"); // Pages match nothing; the first option is the pipeline
    await page.keyboard.press("ArrowUp");
    await page.keyboard.press("Enter");
    await page.waitForFunction(() => location.hash === "#/pipelines/slow");
    await page.locator("h1", { hasText: "slow" }).waitFor();
  } finally {
    await page.context().close();
  }
});

test("Forget token removes the token and shows the terminal hint", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/settings"; });
    page.once("dialog", (dlg) => dlg.accept());
    await page.getByRole("button", { name: "Forget token" }).click();
    await page.locator(".gate h1", { hasText: "Open TRUSS Desktop from your terminal" }).waitFor();
    assert.equal(await page.evaluate(() => localStorage.getItem("truss-token")), null);
  } finally {
    await page.context().close();
  }
});
