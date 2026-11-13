# Third-party design skills

Installed for UI design work on TRUSS Desktop. Text files only (`SKILL.md`, references, licenses);
the upstream helper scripts, binaries and image assets are left out, so commands in these skills
that call a `scripts/` launcher do not run here. Read the references directly instead.

| Folder | Upstream | Commit | License |
|---|---|---|---|
| `hallmark/` | github.com/nutlope/hallmark (`skills/hallmark`) | `04977e1` | MIT |
| `taste-skill/` | github.com/leonxlnx/taste-skill (`skills/taste-skill`) | `f6b7657` | MIT |
| `impeccable/` | github.com/pbakaus/impeccable (`.claude/skills/impeccable`) | `b4bff16` | Apache-2.0 |

They are agent tooling, not part of the product, and nothing in `truss` loads them at run time.
