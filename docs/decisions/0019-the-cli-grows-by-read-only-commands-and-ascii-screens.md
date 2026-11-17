# 0019. The CLI grows by read-only commands and ASCII screens

- Status: Proposed
- Date: 2026-11-17

## Context

The CLI could run, register and trigger, but it could not answer simple questions: what is the state
of the daemon and of the last run, which flows a project has, what a flow needs, which schedules are
coming. The maintainer asked for a layered CLI with those answers and a small ASCII screen. The CLI is
a public contract (constitution V), and v0.1.0's flat commands exist in scripts and in the docs.

## Decision

1. **Add, never rename.** Every command of v0.1.0 keeps its name, flags, output and exit codes. New
   capability arrives as new top-level nouns (`status`, `dashboard`, `project`, `projects`, `flows`,
   `schedules`, `env`) and as sub-commands of existing nouns (`pipelines show`, `runs show`,
   `daemon logs`). A sub-command word is accepted only where the old command printed usage, plus
   `runs show` alone (a pipeline may be called `show`). `help <x>` for an unknown `x` stays a usage
   error. Flat and layered forms are not duplicated: there is no second spelling of an old command.
2. **Read-only.** The new commands change nothing and never start or stop the daemon. `status`
   exits `0` with or without a daemon, because it answers a question; `daemon status` keeps exit `1`
   for "not running", so scripts that test it keep working.
3. **Files first, daemon optional.** What can be answered from files (`flows`, `project`, `env`,
   `daemon logs`) works without a daemon and shows `unknown` where only the daemon knows.
4. **JSON for scripts.** `--json` prints the daemon's wire types unchanged where one exists, and a
   small documented document otherwise. Human output is not a contract for new commands beyond what
   the spec lists; scripts should use `--json`.
5. **ASCII only.** Screens use `+ - |`, `#` and `.`, no colour, no box-drawing characters. The width
   comes from `COLUMNS`, not from asking the terminal, so there is one code path on every OS. The
   dashboard redraws with an ANSI home-and-clear only on a Linux or macOS terminal. The Windows
   console draws it once, because enabling virtual-terminal mode is not worth a platform file here.
6. **Names, not values.** `flows show` lists environment variable and secret names; values are never
   printed. `status` never prints the Desktop token; `truss desktop` does, as before.
7. **Screens are safe to read.** The new commands exist to inspect files that may come from someone
   else, so a control character from a file (an escape sequence that retitles the window, or a
   carriage return that hides the rest of a `run` line) is shown as `?`. A multi-line `run` is marked
   `(+N lines)`. `daemon logs` reads regular files only. The older `pipelines` and `runs` tables are
   not changed here; that is a follow-up.
8. **No new dependency.** Rendering is a few small functions in `cmd/truss`.

## Consequences

- Scripts and docs of v0.1.0 are unaffected. The new JSON documents become contracts once released.
- A pipeline named `show` or `list` stays usable, at the cost of the one special case in item 1.
- Two vocabularies stay visible, as in the glossary: `flows` reads files, `pipelines` reads the
  daemon's registry. Each command's help says which.

## Alternatives considered

- **A Cobra-style rewrite with `truss <noun> <verb>` only.** Cleaner, but breaks every script.
  Rejected under item 1.
- **Colour and box-drawing characters.** Prettier, but they break in logs, on legacy consoles and
  with some fonts. Rejected under item 5.
- **A third-party TUI library.** Needs a new dependency for a screen this small. Rejected under
  item 7; a full interactive TUI can come later with its own decision.
