"use strict";
const { test, before, after } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { startDaemon, openBrowser, poll } = require("./support");

let d, b, page;
before(async () => { d = await startDaemon(); b = await openBrowser(); page = await b.newPage(); });
after(async () => { await b?.close(); await d?.stop(); });

const card = (name) => page.locator("article.card", { has: page.getByRole("link", { name, exact: true }) });

test("register through the file picker, then run, pause, resume and unregister", async () => {
  await page.goto(d.authedUrl());
  await page.evaluate(() => { location.hash = "#/pipelines"; });
  await page.locator("h1", { hasText: "Flows" }).waitFor();
  await page.getByText("No pipelines registered yet").waitFor();
  assert.equal(await page.locator("#flows-view-cards").count(), 0, "nothing to switch with no flow registered");

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
    // Re-run acts on the registration the run belongs to.
    await page.getByRole("button", { name: "Re-run" }).click();
    await page.waitForFunction((id) => !location.hash.endsWith(id), run.id);
    assert.equal((await d.call("GET", "/runs?pipeline=after")).length, 2);
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});

test("the Flows page shows cards or a list, and keeps the choice", async () => {
  const bad = path.join(d.pipelines, "toggle-bad.yaml");
  fs.writeFileSync(bad, fs.readFileSync(path.join(d.pipelines, "ok.yaml")));
  await d.register("ok.yaml", "toggle-ok");
  await d.register("nightly.yaml", "toggle-nightly");
  await d.register("toggle-bad.yaml", "toggle-bad");
  fs.writeFileSync(bad, 'version: "1"\nsteps: [\n'); // the file stops loading: the flow stays, as invalid
  await poll(async () => (await d.call("GET", "/pipelines/toggle-bad")).valid === false || null, "toggle-bad invalid");

  const page = await b.newPage(); // its own page: the view choice lives in the browser's storage
  const row = (name) => page.locator("#flows-table tbody tr", { has: page.getByRole("link", { name, exact: true }) });
  const pressed = (label) => page.getByRole("button", { name: label, exact: true }).getAttribute("aria-pressed");
  const stored = () => page.evaluate(() => JSON.parse(localStorage.getItem("truss-settings") || "{}").flowsView);
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/pipelines"; });
    await page.locator("h1", { hasText: "Flows" }).waitFor();
    const total = (await d.call("GET", "/pipelines")).length;

    // Cards are the default and the page shows nothing else.
    await page.locator("article.card").first().waitFor();
    assert.equal(await pressed("Cards"), "true");
    assert.equal(await pressed("List"), "false");
    assert.equal(await page.locator("#flows-table").count(), 0);
    assert.equal(await page.locator("article.card").count(), total);

    // The filter and the count carry over to the list.
    await page.locator("#pipe-filter").fill("toggle-");
    await page.locator("#pipe-count", { hasText: `3 of ${total}` }).waitFor();
    await page.getByRole("button", { name: "List", exact: true }).click();
    await page.locator("#flows-table").waitFor();
    assert.equal(await page.locator("article.card").count(), 0);
    assert.equal(await pressed("List"), "true");
    assert.equal(await page.locator("#pipe-filter").inputValue(), "toggle-");
    assert.equal(await page.locator("#pipe-count").textContent(), `3 of ${total}`);
    assert.equal(await page.locator("#flows-table tbody tr:not([hidden])").count(), 3);
    assert.ok(await page.locator("#flows-view-list").evaluate((el) => el === document.activeElement), "the switch keeps focus");
    assert.equal(await stored(), "list");
    const heads = await page.locator("#flows-table thead th").allTextContents();
    assert.deepEqual(heads, ["State", "Flow", "Schedule", "Next run", "Last run", "Duration", "Shape", "Actions"]);
    await page.locator("#pipe-filter").fill("");
    await page.locator("#pipe-count", { hasText: `${total} of ${total}` }).waitFor();
    assert.equal(await page.locator("#flows-table tbody tr:not([hidden])").count(), total);

    // A row says what a card says.
    const ok = row("toggle-ok");
    assert.ok((await ok.locator(".path").textContent()).endsWith("ok.yaml"));
    await ok.locator(".badge", { hasText: "ready" }).waitFor();
    assert.match(await ok.textContent(), /3 steps/);
    const broken = row("toggle-bad");
    await broken.locator(".badge", { hasText: "invalid" }).waitFor();
    assert.match(await broken.textContent(), /not runnable/);
    assert.ok((await broken.locator("td.error").textContent()).length > 0, "the error is shown");

    // The actions work from a row, under the same names as on a card.
    const nightly = row("toggle-nightly");
    await nightly.getByRole("button", { name: "Pause schedule" }).click();
    await nightly.locator(".badge", { hasText: "paused" }).waitFor();
    await nightly.getByRole("button", { name: "Resume schedule" }).click();
    await nightly.locator(".badge", { hasText: "ready" }).waitFor();
    assert.equal(await ok.getByRole("button", { name: "Run now" }).getAttribute("title"), "Run now", "an icon button has a tooltip");
    assert.equal(await ok.getByRole("button", { name: "Unregister" }).getAttribute("title"), "Unregister toggle-ok");
    assert.equal(await ok.getByRole("button", { name: "Pause schedule" }).isDisabled(), true, "no schedule, nothing to pause");
    assert.equal(await broken.getByRole("button", { name: "Run now" }).isDisabled(), true, "an invalid flow cannot run");

    await ok.getByRole("button", { name: "Run now" }).click();
    await page.waitForURL(/#\/runs\/./);
    await page.locator(".badge.s-succeeded").first().waitFor();
    await page.evaluate(() => { location.hash = "#/pipelines"; });
    await row("toggle-ok").waitFor(); // still a list when the page comes back

    // A row opens its flow; its buttons do not.
    await row("toggle-ok").locator("td").nth(2).click();
    await page.waitForURL(/#\/pipelines\/toggle-ok$/);
    await page.evaluate(() => { location.hash = "#/pipelines"; });

    // Unregister asks first.
    page.once("dialog", (dlg) => dlg.dismiss());
    await row("toggle-nightly").getByRole("button", { name: "Unregister" }).click();
    await row("toggle-nightly").waitFor();
    page.once("dialog", (dlg) => dlg.accept());
    await row("toggle-nightly").getByRole("button", { name: "Unregister" }).click();
    await row("toggle-nightly").waitFor({ state: "detached" });

    // A reload keeps the list; Cards brings the cards back; an unknown stored value means Cards.
    await page.reload();
    await page.locator("#flows-table").waitFor();
    assert.equal(await pressed("List"), "true");
    await page.getByRole("button", { name: "Cards", exact: true }).click();
    await page.locator("article.card").first().waitFor();
    assert.equal(await page.locator("#flows-table").count(), 0);
    assert.equal(await stored(), "cards");
    await page.evaluate(() => localStorage.setItem("truss-settings", JSON.stringify({ flowsView: "bogus" })));
    await page.reload();
    await page.locator("article.card").first().waitFor();
    assert.equal(await pressed("Cards"), "true");
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});

test("switching the Flows view keeps the filter, the order and the register form, and the list follows the daemon", async () => {
  await d.register("nightly.yaml", "keep-a");
  await d.register("ok.yaml", "keep-b");
  const page = await b.newPage();
  const cardNames = () => page.locator("article.card:not([hidden]) > header a").allTextContents();
  const rowNames = () => page.locator("#flows-table tbody tr:not([hidden]) td.flow a.pipe").allTextContents();
  try {
    await page.goto(d.authedUrl());
    await page.evaluate(() => { location.hash = "#/pipelines"; });
    await page.locator("article.card").first().waitFor();
    const total = (await d.call("GET", "/pipelines")).length;

    // The same flows in the same order, and a half-typed form that nothing wipes.
    const cards = await cardNames();
    await page.locator("#register-name").fill("half-typed");
    await page.locator("#flows-view-list").click();
    await page.locator("#flows-table").waitFor();
    assert.deepEqual(await rowNames(), cards);
    assert.equal(await page.locator("#register-name").inputValue(), "half-typed");

    // A filter with a trailing space reads the same when the page is drawn again as when it was typed.
    await page.locator("#pipe-filter").fill("keep-a ");
    await page.locator("#pipe-count", { hasText: `1 of ${total}` }).waitFor();
    await page.getByRole("button", { name: "Cards", exact: true }).click();
    await page.locator("article.card:not([hidden])").first().waitFor();
    assert.equal(await page.locator("#pipe-count").textContent(), `1 of ${total}`);
    assert.deepEqual(await cardNames(), ["keep-a"]);
    await page.getByRole("button", { name: "List", exact: true }).click();
    await page.locator("#flows-table").waitFor();
    assert.deepEqual(await rowNames(), ["keep-a"]);
    await page.locator("#pipe-filter").fill("");
    await page.locator("#pipe-count", { hasText: `${total} of ${total}` }).waitFor();

    // The switch works from the keyboard and keeps focus.
    await page.locator("#flows-view-list").focus();
    await page.keyboard.press("Shift+Tab");
    assert.equal(await page.evaluate(() => document.activeElement.id), "flows-view-cards");
    await page.keyboard.press("Space");
    await page.locator("article.card").first().waitFor();
    assert.equal(await page.locator("#flows-view-cards").getAttribute("aria-pressed"), "true");
    assert.equal(await page.evaluate(() => document.activeElement.id), "flows-view-cards");
    await page.keyboard.press("Tab");
    await page.keyboard.press("Enter");
    await page.locator("#flows-table").waitFor();
    assert.equal(await page.evaluate(() => document.activeElement.id), "flows-view-list");

    // A press beside a button in the actions cell does not leave the page.
    const row = page.locator("#flows-table tbody tr", { has: page.getByRole("link", { name: "keep-a", exact: true }) });
    await row.locator("td.acts").click({ position: { x: 2, y: 2 } });
    await new Promise((r) => setTimeout(r, 300));
    assert.equal(await page.evaluate(() => location.hash), "#/pipelines");

    // The list follows the daemon by itself: nothing is clicked here.
    await row.locator(".badge", { hasText: "ready" }).waitFor();
    await d.call("POST", "/pipelines/keep-a/pause");
    await row.locator(".badge", { hasText: "paused" }).waitFor({ timeout: 10000 });
    assert.deepEqual(page.problems, []);
  } finally {
    await page.context().close();
  }
});
