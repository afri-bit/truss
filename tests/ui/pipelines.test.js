"use strict";
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { startDaemon, openBrowser } = require("./support");

let d, b, page;
before(async () => { d = await startDaemon(); b = await openBrowser(); page = await b.newPage(); });
after(async () => { await b?.close(); await d?.stop(); });

const card = (name) => page.locator("article.card", { has: page.getByRole("link", { name, exact: true }) });

test("register through the file picker, then run, pause, resume and unregister", async () => {
  await page.goto(d.authedUrl());
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("h1", { hasText: "Flows" }).waitFor();
  await page.getByText("No pipelines registered yet").waitFor();

  // The picker opens at the user's home and lists names only.
  await page.locator("#browse").click();
  await page.locator("#picker[open]").waitFor();
  assert.equal(await page.locator("#picker-path").textContent(), d.homeDir);
  const names = await page.locator("#picker-list button.entry").allTextContents();
  assert.ok(names.some((n) => n.includes("pipelines")), names.join("|"));
  assert.ok(!names.some((n) => n.includes("notes.txt") || n.includes(".secret-dir")), "listed a hidden or non-YAML name: " + names);
  await page.locator("#picker-hidden").check();
  await page.locator("#picker-list").getByText(".secret-dir").waitFor();
  await page.locator("#picker-hidden").uncheck();
  await page.locator("#picker-list").getByText(".secret-dir").waitFor({ state: "detached" });

  await page.locator("#picker-list button.entry", { hasText: "pipelines" }).click();
  await page.locator("#picker-path", { hasText: "pipelines" }).waitFor();
  await page.locator("#picker-list button.entry", { hasText: "nightly.yaml" }).click();
  await page.locator("#picker[open]").waitFor({ state: "hidden" });
  assert.equal(await page.locator("#picker").evaluate((el) => el.open), false);
  assert.ok((await page.locator("#register-path").inputValue()).endsWith("nightly.yaml"));
  assert.ok(await page.locator("#register-path").evaluate((el) => el === document.activeElement), "focus returns to the path field");

  await page.getByRole("button", { name: "Register", exact: true }).click();
  await card("nightly").waitFor();
  assert.equal(await page.locator("#register-path").inputValue(), "", "form is cleared after registering");

  // Typing a path by hand works too.
  await page.locator("#register-path").fill(require("node:path").join(d.pipelines, "ok.yaml"));
  await page.locator("#register-name").fill("ok-by-hand");
  await page.getByRole("button", { name: "Register", exact: true }).click();
  await card("ok-by-hand").waitFor();

  // Pause and resume a scheduled pipeline.
  const nightly = card("nightly");
  await nightly.locator(".badge", { hasText: "ready" }).waitFor();
  await nightly.getByRole("button", { name: "Pause schedule" }).click();
  await nightly.locator(".badge", { hasText: "paused" }).waitFor();
  await nightly.getByRole("button", { name: "Resume schedule" }).click();
  await nightly.locator(".badge", { hasText: "ready" }).waitFor();

  // Run now opens the run page.
  await card("ok-by-hand").getByRole("button", { name: "Run now" }).click();
  await page.waitForURL(/#\/runs\/./);
  await page.locator(".badge.s-succeeded").first().waitFor();

  // Unregister asks first; dismissing keeps it, accepting removes it.
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await card("nightly").waitFor();
  page.once("dialog", (dlg) => dlg.dismiss());
  await card("nightly").getByRole("button", { name: "Unregister" }).click();
  await card("nightly").waitFor();
  page.once("dialog", (dlg) => dlg.accept());
  await card("nightly").getByRole("button", { name: "Unregister" }).click();
  await card("nightly").waitFor({ state: "detached" });
  assert.equal((await d.call("GET", "/pipelines")).length, 1);
  assert.deepEqual(page.problems, []);
});

test("a half-typed register form survives the refresh", async () => {
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("#register-name").fill("draft-name");
  // Move focus to a button: the periodic refresh must not wipe the form.
  await page.locator("#browse").focus();
  await new Promise((r) => setTimeout(r, 3500));
  assert.equal(await page.locator("#register-name").inputValue(), "draft-name");
  assert.ok(await page.locator("#browse").evaluate((el) => el === document.activeElement), "focus is kept across refreshes");
});

test("register form follows a sensible keyboard order", async () => {
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("#register-path").focus();
  const order = [];
  for (let i = 0; i < 4; i++) {
    order.push(await page.evaluate(() => document.activeElement.id || document.activeElement.textContent.trim()));
    await page.keyboard.press("Tab");
  }
  assert.deepEqual(order.slice(0, 3), ["register-path", "browse", "register-name"]);
  assert.equal(order[3], "Register");
});

test("the picker closes on Escape and falls back to home when the typed folder is gone", async () => {
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("#register-path").fill(require("node:path").join(d.homeDir, "gone", "x.yaml")); // folder does not exist: falls back to home
  await page.locator("#browse").click();
  await page.locator("#picker-path").waitFor();
  assert.equal(await page.locator("#picker-path").textContent(), d.homeDir);
  await page.keyboard.press("Escape");
  assert.equal(await page.locator("#picker").evaluate((el) => el.open), false);
  assert.ok(await page.locator("#register-path").evaluate((el) => el === document.activeElement), "focus returns to the path field");
});

test("the picker walks up and home, and submitting with Enter registers once", async () => {
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("#register-path").fill("");
  await page.locator("#browse").click();
  await page.locator("#picker-path").waitFor();
  await page.locator("#picker-list button.entry", { hasText: "pipelines" }).click();
  await page.locator("#picker-path", { hasText: "pipelines" }).waitFor();
  await page.locator("#picker-up").click();
  await page.waitForFunction((home) => document.getElementById("picker-path").textContent === home, d.homeDir);
  await page.locator("#picker-list button.entry", { hasText: "pipelines" }).click();
  await page.locator("#picker-home").click();
  await page.waitForFunction((home) => document.getElementById("picker-path").textContent === home, d.homeDir);
  await page.locator("#picker-list button.entry", { hasText: "pipelines" }).click();
  await page.locator("#picker-list button.entry", { hasText: "slow.yaml" }).click();

  const before = (await d.call("GET", "/pipelines")).length;
  await page.locator("#register-name").fill("slow-enter");
  await page.locator("#register-path").press("Enter");
  await card("slow-enter").waitFor();
  assert.equal(await page.locator("#register-path").inputValue(), "", "the field is cleared after Enter");
  assert.equal(await page.locator("#register-name").inputValue(), "");
  await page.locator("#register-path").press("Enter"); // empty and required: nothing is sent
  await new Promise((r) => setTimeout(r, 300));
  assert.equal((await d.call("GET", "/pipelines")).length, before + 1);
});

test("a flow follows an edit of its file, new name included, and keeps its runs", async () => {
  const file = path.join(d.pipelines, "follow.yaml");
  const body = (name, ...steps) => `version: "1"\npipeline:\n  name: ${name}\nsteps:\n` + steps.map((s) => `  - id: ${s}\n    run: echo ${s}\n`).join("");
  fs.writeFileSync(file, body("before", "one"));
  await d.register("follow.yaml");
  const run = await d.call("POST", "/pipelines/before/runs");
  await d.waitRun(run.id);
  const page = await b.newPage(); // its own page: the shared one may still be on its way to "#/"
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/pipelines/before"; });
    await page.locator("h1", { hasText: "before" }).waitFor();
    await page.locator(".graph .node").first().waitFor();
    assert.equal(await page.locator(".graph .node").count(), 1);

    fs.writeFileSync(file, body("after", "first", "second"));
    // The page refreshes by itself: the new name, the new graph, and the old run in the history.
    await page.locator("h1", { hasText: "after" }).waitFor({ timeout: 15000 });
    await page.waitForFunction(() => document.querySelectorAll(".graph .node").length === 2);
    await page.locator(".facts dd", { hasText: "before" }).waitFor();
    await page.locator(`a[href="#/runs/${run.id}"]`).first().waitFor();

    // Either name still reaches it, and the run page shows the name the file gives.
    assert.equal((await d.call("GET", "/pipelines/before")).name, "after");
    await page.evaluate((id) => { location.hash = "#/runs/" + id; }, run.id);
    await page.locator(".head .sub a", { hasText: "after" }).waitFor();
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});
