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

test("run page: the log is one collapsible card per step", async () => {
  await d.register("ok.yaml");
  const run = await d.call("POST", "/pipelines/ok/runs");
  await d.waitRun(run.id);
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
  await page.locator("#log details.sec").first().waitFor();
  await poll(async () => (await page.locator("#log details.sec").count()) === 5, "setup, three steps and the summary");

  const card = (key) => page.locator(`#log details[data-sec="${key}"]`);
  const keys = await page.locator("#log details.sec").evaluateAll((els) => els.map((e) => e.dataset.sec));
  assert.deepEqual([...keys].sort(), ["setup", "step:build", "step:lint", "step:test", "summary"]);
  assert.equal(keys[0], "setup");
  assert.equal(keys.at(-1), "summary");

  // The setup card says which environment the steps ran in, even when closed.
  const setup = await card("setup").locator("summary").textContent();
  assert.match(setup, /Setup/);
  assert.match(setup, /^Setup.+ · .+ · .+/, "the OS, architecture and shell are named on the header");
  assert.equal(await card("setup").evaluate((e) => e.open), false);

  // Steps that succeeded are closed; their header names the step and how long it took.
  const head = await card("step:build").locator("summary").textContent();
  assert.match(head, /build/);
  assert.match(head, /\d+\.\d{3}s/);
  assert.equal(await card("step:build").evaluate((e) => e.open), false);
  assert.equal(await card("step:build").locator(".ln", { hasText: "building" }).isVisible(), false);
  await card("step:build").locator("summary").click();
  await card("step:build").locator(".ln", { hasText: "building" }).waitFor({ state: "visible" });

  // The summary stays open, and no heading text or label prefix leaks into the cards.
  assert.equal(await card("summary").evaluate((e) => e.open), true);
  const text = await log().textContent();
  assert.ok(!text.includes("===="), "headings are not shown, the cards replace them");
  assert.ok(!text.includes("step build |"), "output lines are not prefixed by their step");

  // Finding text opens the card that has it and hides the others.
  await page.locator("#log-find").fill("linting");
  await poll(async () => (await page.locator("#log details.sec").count()) === 1, "only the matching card");
  assert.equal(await card("step:lint").evaluate((e) => e.open), true);
  await page.locator("#log-find").fill("");
  await poll(async () => (await page.locator("#log details.sec").count()) === 5, "all cards again");
  assert.deepEqual(page.problems, []);
});

test("run page: a failed step is open, and a card the reader closed stays closed", async () => {
  await d.register("fail.yaml");
  const bad = await d.call("POST", "/pipelines/broken/runs");
  await d.waitRun(bad.id);
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, bad.id);
  const boom = page.locator('#log details[data-sec="step:boom"]');
  await boom.waitFor();
  assert.equal(await boom.evaluate((e) => e.open), true);
  assert.ok((await boom.getAttribute("class")).includes("st-failed"));
  assert.match(await page.locator('#log details[data-sec="summary"]').textContent(), /broken failed at boom/);

  await d.register("slow.yaml");
  const run = await d.call("POST", "/pipelines/slow/runs");
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
  const wait = page.locator('#log details[data-sec="step:wait"]');
  await wait.waitFor();
  assert.equal(await wait.evaluate((e) => e.open), true, "a running step is open");
  assert.ok((await wait.getAttribute("class")).includes("st-running"));
  await wait.locator("summary").click();
  assert.equal(await wait.evaluate((e) => e.open), false);
  await page.locator("#follow").click();
  assert.equal(await wait.evaluate((e) => e.open), false, "repainting keeps the reader's choice");
  await page.getByRole("button", { name: "Cancel run" }).click();
  await page.locator(".head .badge.s-cancelled").waitFor();
  assert.deepEqual(page.problems, []);
});

test("run page: a log written before section headings groups the same way", async () => {
  await page.goto(d.authedUrl());
  await page.getByRole("link", { name: "Flows" }).waitFor();
  const text = [
    "2026-11-12 09:00:00 setup environment ready",
    "2026-11-12 09:00:00 setup os linux",
    "2026-11-12 09:00:00 setup shell /bin/sh",
    "2026-11-12 09:00:00 step a started",
    "2026-11-12 09:00:00 step b started",
    "2026-11-12 09:00:01 step b | from b",
    "2026-11-12 09:00:01 step a | from a",
    "2026-11-12 09:00:01 step a finished duration 1.000s exit 0",
    "2026-11-12 09:00:02 step b finished duration 2.000s exit 4",
    "2026-11-12 09:00:02 summary failed steps 2 duration 2.100s",
    "2026-11-12 09:00:02 error p failed at b (exit 4)",
    "",
  ].join("\n");
  const cards = await page.evaluate((t) => parseLog(t, true).map((c) => ({ key: c.key, state: c.state, rows: c.rows.length, dur: c.dur })), text);
  assert.deepEqual(cards, [
    { key: "setup", state: "succeeded", rows: 3, dur: undefined },
    { key: "step:a", state: "succeeded", rows: 3, dur: "1.000s" },
    { key: "step:b", state: "failed", rows: 3, dur: "2.000s" },
    { key: "summary", state: "failed", rows: 2, dur: undefined },
  ]);
});

test("run page: output with a carriage return stays in its step", async () => {
  await page.goto(d.authedUrl());
  await page.getByRole("link", { name: "Flows" }).waitFor();
  const text = [
    "2026-11-12 09:00:00 step a started",
    "2026-11-12 09:00:01 step a | progress 10%\rprogress 100%",
    "2026-11-12 09:00:01 step a | line\u2028separator",
    "2026-11-12 09:00:02 step a finished duration 2.000s exit 0",
    "",
  ].join("\n");
  const cards = await page.evaluate((t) => parseLog(t, true).map((c) => ({ key: c.key, state: c.state, rows: c.rows.length })), text);
  assert.deepEqual(cards, [{ key: "step:a", state: "succeeded", rows: 4 }]);
});

test("run page: the cards follow the log grammar", async () => {
  await page.goto(d.authedUrl());
  await page.getByRole("link", { name: "Flows" }).waitFor();
  const T = (s) => `2026-11-12 09:00:0${s}`;
  const body = [
    `${T(0)} setup environment ready`,
    `${T(0)} setup os linux`,
    `${T(0)} step flaky started`,
    `${T(1)} step flaky finished duration 1.000s exit 1`,
    `${T(1)} step flaky retry 2 of 3 in 1.000s`,
    `${T(1)} step flaky started`,
    `${T(2)} step flaky finished duration 1.000s exit 0`,
    `${T(2)} step build app started`,
    `${T(2)} step build app | hi | there`,
    `${T(2)} step build app finished duration 0.100s exit 0`,
    `${T(2)} step lint started`,
    `${T(3)} step lint finished duration 0.100s exit 1`,
    `${T(3)} step lint failed but the run continues (continue-on-error)`,
    `${T(3)} step docs skipped because its if condition is false`,
    `${T(3)} step x | y started`,
    `${T(3)} step x | y finished duration 0.100s error start command: not found`,
    `${T(3)} step a/b started`,
    `${T(3)} step a/b stopped because step x | y failed`,
    `${T(3)} step a/b finished duration 0.100s exit 1`,
    `${T(3)} summary failed steps 5 duration 3.000s`,
    `${T(3)} error p failed at x | y: start command: not found`,
  ];
  const headed = [body[0].replace("setup environment ready", "==== Setup: the environment the steps run in ===="), ...body]
    .concat([`${T(3)} ==== Summary ====`]);
  const cards = (lines, ids) => page.evaluate(([t, ids]) => parseLog(t, true, ids).map((c) => [c.key, c.state, c.dur || "", c.note || "", c.rows.length]), [lines.join("\n") + "\n", ids]);
  const ids = ["flaky", "build app", "lint", "docs", "x | y", "a/b"];
  const got = await cards(body, ids);
  assert.deepEqual(got, [
    ["setup", "succeeded", "", "linux", 2],
    ["step:flaky", "succeeded", "1.000s", "attempt 2", 5],
    ["step:build app", "succeeded", "0.100s", "", 3],
    ["step:lint", "tolerated", "0.100s", "", 3],
    ["step:docs", "skipped", "", "its if condition is false", 1],
    ["step:x | y", "failed", "0.100s", "", 2],
    ["step:a/b", "cancelled", "0.100s", "stopped because step x | y failed", 3],
    ["summary", "failed", "", "failed · 5 steps · 3.000s", 2],
  ]);
  // The same log with its headings gives the same cards.
  const withHeadings = body.flatMap((l) => (l.endsWith("setup environment ready") ? [`${T(0)} ==== Setup: the environment the steps run in ====`, l] : l.endsWith("summary failed steps 5 duration 3.000s") ? [`${T(3)} ==== Summary ====`, l] : /step (flaky|build app|lint|docs|a\/b) started$/.test(l) ? [`${T(0)} ==== Step ${l.split(" ")[3] === "build" ? "build app" : l.split(" ")[3]} ====`, l] : l));
  assert.deepEqual(await cards(withHeadings, ids), got);
  // A step cut off by a cancel is cancelled, not failed.
  const cut = await cards([`${T(0)} step w started`, `${T(1)} step w finished duration 1.000s error terminated by signal: killed`, `${T(1)} summary cancelled steps 1 duration 1.100s`], ["w"]);
  assert.deepEqual(cut.map((c) => [c[0], c[1]]), [["step:w", "cancelled"], ["summary", "cancelled"]]);
  // Half a line at the end of a log that is still arriving waits for its end.
  const partial = await page.evaluate(() => parseLog("2026-11-12 09:00:00 step a started\n2026-11-12 09:00:01 st", false, ["a"]).map((c) => c.key));
  assert.deepEqual(partial, ["step:a"]);
});

test("run page: keyboard focus stays on a card header while the log updates", async () => {
  await d.register("slow.yaml");
  const run = await d.call("POST", "/pipelines/slow/runs");
  await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
  const head = page.locator('#log details[data-sec="step:wait"] > summary');
  await head.waitFor();
  await head.focus();
  await page.evaluate(() => { logState.text += ""; paintLog(); });
  assert.equal(await page.evaluate(() => document.activeElement.closest("details")?.dataset.sec), "step:wait");
  await page.getByRole("button", { name: "Cancel run" }).click();
  await page.locator(".head .badge.s-cancelled").waitFor();
  const card = page.locator('#log details[data-sec="step:wait"]');
  await poll(async () => (await card.getAttribute("class")).includes("st-cancelled"), "the cut-off step is shown as cancelled");
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
