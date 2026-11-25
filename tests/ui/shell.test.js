"use strict";
// The page shell from spec 014 and the resource figures from spec 015: sidebar, header,
// settings, finder, and real figures from GET /metrics.
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

test("resource figures are the daemon's own: no sample labels, real step and daemon processes", async () => {
  const run = await d.call("POST", "/pipelines/slow/runs");
  // "wait" is slow.yaml's long step; "first" ends at once.
  const running = await d.waitRun(run.id, (r) => r.steps.some((s) => s.id === "wait" && s.status === "running" && s.pid > 0));
  const step = running.steps.find((s) => s.id === "wait");
  const metrics = await d.call("GET", "/metrics");
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    for (const hash of ["#/", "#/monitor", "#/runs/" + run.id]) {
      await page.evaluate((h) => { location.hash = h; }, hash);
      await page.waitForTimeout(800);
      assert.equal(await page.locator(".sim").count(), 0, `sample label on ${hash}: figures must be real`);
    }
    const status = await d.call("GET", "/status");
    await page.evaluate(() => { location.hash = "#/monitor"; });
    const table = page.locator("section", { hasText: "Processes" });
    if (metrics.supported) {
      // The daemon row and the step row both carry measured CPU, and the step's pid is the run's.
      await table.locator("tr", { hasText: "truss daemon" }).locator(".meter").waitFor();
      await table.locator("tr", { hasText: step.id }).locator("td", { hasText: String(step.pid) }).waitFor({ timeout: 10000 });
      await table.locator("tr", { hasText: step.id }).locator(".meter").waitFor({ timeout: 10000 });
      await page.locator("section", { hasText: "Cores" }).locator(".core").first().waitFor();
      await page.locator(".strip", { hasText: "Memory" }).getByText(/GiB/).waitFor();
    } else {
      await table.locator("td", { hasText: String(status.pid) }).waitFor();
      await page.getByText(`does not collect resource figures on ${metrics.os}`).first().waitFor();
    }
    // The run page lists the step's real pid.
    await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
    await page.locator(".steps li", { hasText: "pid " + step.pid }).waitFor();
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

test("Jump to offers the Flows page and groups pipelines under Flows", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await poll(async () => (await page.locator(".side-label").count()) > 0, "shell poll");
    await page.locator("#view").focus();
    await page.keyboard.press("/");
    await page.locator("#finder[open]").waitFor();
    await page.keyboard.type("flows");
    await page.locator("#finder [role=option]", { hasText: "Flows" }).first().waitFor();
    assert.equal(await page.locator("#finder [role=option]", { hasText: "Pipelines" }).count(), 0);
    await page.keyboard.press("Escape");
    await page.keyboard.press("/");
    await page.locator("#finder[open]").waitFor();
    await page.keyboard.type("slow");
    await page.locator("#finder [role=option]", { hasText: "slow" }).first().waitFor();
    const text = await page.locator("#finder").textContent();
    assert.ok(text.includes("Flows") && !text.includes("Pipelines"), `finder groups: ${text}`);
    assert.deepEqual(page.problems, []);
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

test("#demo on a real daemon's page shows real data; demo data exists only on the demo server", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl()); // the token is stored
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await page.goto(d.base + "/#demo");
    await page.locator("h1", { hasText: "Dashboard" }).waitFor();
    await page.locator("#conn", { hasText: "connected" }).waitFor();
    assert.equal(await page.locator("#conn", { hasText: "Demo data" }).count(), 0);
    assert.equal(await page.evaluate(() => sessionStorage.getItem("truss-demo")), null, "the demo flag is dropped");
    const names = await page.locator(".tile", { hasText: "Flows registered" }).locator(".num").textContent();
    assert.equal(names, String((await d.call("GET", "/pipelines")).length), "counts come from the daemon");
  } finally {
    await page.context().close();
  }
});

test("a token link turns demo mode off in the same tab", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.base + "/");
    await page.evaluate(() => sessionStorage.setItem("truss-demo", "1")); // as if the tab had been in demo mode
    await page.goto(d.authedUrl());
    await page.locator("#conn", { hasText: "connected" }).waitFor();
    assert.equal(await page.evaluate(() => sessionStorage.getItem("truss-demo")), null);
  } finally {
    await page.context().close();
  }
});

test("the sidebar calls the pipeline list Flows", async () => {
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    const nav = page.locator("#nav");
    await nav.getByRole("link", { name: "Flows" }).click();
    await page.locator("h1", { hasText: "Flows" }).waitFor();
    assert.equal(await nav.getByRole("link", { name: "Pipelines" }).count(), 0);
    assert.ok(page.url().endsWith("#/pipelines"), "the address of the page does not change");
    const crumbs = await page.locator("#crumbs").textContent();
    assert.ok(crumbs.includes("Flows") && !crumbs.includes("Pipelines"), `breadcrumbs: ${crumbs}`);
    await page.evaluate(() => { location.hash = "#/pipelines/slow"; });
    await page.locator("h1", { hasText: "slow" }).waitFor();
    await page.locator("#crumbs").getByRole("link", { name: "Flows" }).waitFor();
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});

test("the run page's memory line stays inside the resource chart", async (t) => {
  const metrics = await d.call("GET", "/metrics");
  if (!metrics.supported) return t.skip("the daemon reports no resource figures on this system");
  const run = await d.call("POST", "/pipelines/slow/runs");
  await d.waitRun(run.id, (r) => r.steps.some((s) => s.id === "wait" && s.status === "running" && s.pid > 0));
  const page = await b.newPage();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
    // The chart appears once the page holds two samples.
    const resources = page.locator("section", { hasText: "Resources" });
    const chart = resources.locator("svg.chart");
    await chart.waitFor({ timeout: 20000 });
    await resources.getByText("mem, scaled", { exact: true }).waitFor();
    const lines = await chart.locator("path.line").evaluateAll((paths) => paths.map((p) => {
      const box = p.getBBox(), view = p.ownerSVGElement.viewBox.baseVal;
      return { top: box.y, bottom: box.y + box.height, height: view.height };
    }));
    assert.equal(lines.length, 2, "one CPU line and one memory line");
    for (const l of lines) assert.ok(l.top >= 0 && l.bottom <= l.height, `a line leaves the chart: ${JSON.stringify(l)}`);
    assert.deepEqual(page.problems, []);
  } finally {
    await d.call("POST", `/runs/${run.id}/cancel`);
    await d.waitRun(run.id);
    await page.context().close();
  }
});
