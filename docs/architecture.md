# Architecture

The `kubecrypt` process owns learner progress, the curriculum pack embedded
from `curriculum/` at compile time, `kind` lifecycle, state checks, and a
split terminal: scenario pane plus a real PTY Lab Shell.

- `start` reads the embed.
- `lab try` runs a `contribute/` file from a temp overlay (no pack writes).
- `lab publish` writes `./curriculum` and starts that copy from disk. Rebuild
  before `start` sees it.

The Lab Shell gets a dedicated kubeconfig. KubeCrypt mutations verify cluster
ownership themselves; they do not trust the shell or the user's current
context.

## Packages

| Package | Owns |
| --- | --- |
| `internal/curriculum` | schema, pack load, `lab try` / `publish` |
| `internal/cluster` | doctor, kind, kubeconfig, ownership |
| `internal/validator` | observed cluster state |
| `internal/game` | profile and progress files |
| `internal/terminal` | split view and Lab Shell PTY |
| `internal/cli` | commands |

Curriculum and validators do not import Bubble Tea. Overlay drafts, on-disk
`curriculum/`, and the embed use the same schema and safety checks.
