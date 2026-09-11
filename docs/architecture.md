# Architecture

KubePrep is one program: `kubeprep`. It opens a split terminal (scenario on
one side, a real Lab Shell on the other), talks to a dedicated `kind`
cluster, and grades **cluster state**, not the commands you typed.

`start` uses the labs compiled into the binary from `curriculum/`. Changing
YAML on disk does not change `start` until you run `make build` again.

| Command | What it loads |
| --- | --- |
| `start` | Labs embedded in the binary; checks for user progression within `progress.json` to load proper lab|
| `lab try` | Draft lab file within `contribute/`. This does not write anything to `curriculum/` and is for local testing|
| `lab publish` | Copies draft lab file from `contribute/` into `curriculum/` and updates `catalog.yaml`, then runs it from disk |

The Lab Shell gets its own kubeconfig for the KubePrep cluster. Before
KubePrep creates, resets, or deletes anything, it checks that this is still
that cluster other context on your machine will be untouched.

## Packages

| Package | Job |
| --- | --- |
| `internal/cli` | Commands (`start`, `doctor`, `lab`, …) |
| `internal/cluster` | `kind` cluster, kubeconfig, `doctor`, safety checks |
| `internal/curriculum` | Lab YAML schema, catalog, `lab try` / `publish` / `validate` |
| `internal/validator` | “Is the cluster in the target state?” |
| `internal/game` | Learning track, current lab, hints, completion |
| `internal/terminal` | Split view and Lab Shell. No lab logic. |

Drafts, the on-disk `curriculum/` folder, and the embedded pack all use the
same schema. Curriculum and validators do not import the terminal UI which is why labs are set to load automatically
