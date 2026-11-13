# 0010. The register dialog browses files through a names-only listing endpoint

- Status: Proposed
- Date: 2026-11-12

## Context

A browser never reveals the absolute path of a file the user picks, but registering a pipeline
needs one. Typing a path is error-prone. The daemon can see the file system; the page cannot.

## Decision

Add `GET /api/v1/fs?path=&hidden=` to the v1 API (additive). It lists one directory:

- **Names only.** Each entry is `{name, kind, link}`. Directories, and regular files ending in
  `.yaml` or `.yml`. No content, size or time. It cannot read a file.
- **Starts at the user's home** when `path` is empty. Dot-names are hidden unless `hidden=true`.
- **Strict paths.** The path must be absolute. Any `..` element (with `/` or `\`), a NUL byte,
  and on Windows anything but `X:\...` (so no drive-relative, UNC, `\\?\` or `\\.\` paths, no
  `:` after the drive) is a `400`. Missing is `404`, unreadable `403`.
- **Honest symlinks.** The listed directory is resolved first and the answer carries the real
  path, so "up" goes to the real parent. A link is listed with `link: true` as the kind of its
  target; a broken link or a link to something else is left out.
- **Same guards as the rest of the API**: token, local `Host`, loopback bind, plus a loopback
  peer check. `roots` tells the page where to start: `/`, or the existing drive letters.
- At most 5000 entries; at most 50000 names are read from a folder. `truncated` says when there were more.
- A link whose target is a UNC or device path (Windows) is left out without being touched, so
  listing a folder never opens a network connection. Windows device names (`CON`, `NUL`,
  `COM1`...) in a path are refused.

## Consequences

- Status codes (404, 403, 400) tell a token holder whether a path exists. Same boundary.
- Windows drive roots are found by trying `A:\` to `Z:\`; a disconnected network drive can slow that.
- Windows "hidden" and system attributes are not honored; only dot-names are hidden.
- The picker leaks directory and `*.yaml` names to whoever holds the token. That is the same
  trust boundary as the shell the daemon already runs commands in (see `docs/daemon.md`).
- Listing is not confined to the home directory: pipelines legitimately live elsewhere, and
  the user can already reach any path from a shell. Confinement would add surprise, not safety.
- Registering still reads the file only through the existing validation.
- Old clients are unaffected; the endpoint is new.
