# Contributing to KubeCrypt

KubeCrypt welcomes scenario-pack and engine contributions.

## Development

1. Install Go 1.27 or newer and Docker. `kind` and `kubectl` are installed by
   `kubecrypt doctor` when you run the CLI. CI uses Go 1.27 for `gofmt` and tests.
2. Run `make install` so `kubecrypt` is on your `PATH`.
3. Run `go test ./...`.
4. Run `go test -tags=integration ./tests/integration` for the disposable
   cluster suite.

Keep scenario behavior declarative. Validators must judge resulting Kubernetes state
rather than command history. The runner presents scenarios in a permanent split
with a real Lab Shell; do not assume the TUI suspends into a shell.

## Scenario submissions

- Start a new module from [`contribute/`](contribute/). For a single extra
  lab, start from the closest scenario under `curriculum/`.
- Include metadata, an objective, setup, typed checks, progressive hints,
  completion text, a technical debrief, capability requirements, and
  deterministic reset data. Observe-only labs may set `observeDelay`
  (for example `60s`) to pause automatic validation.
- Add an explicit scenario ID and path to `curriculum/catalog.yaml`.
- Sync the embedded copy with `make bundle-lesson 01-foundations/01-pod-creation`.
  That checks the catalog `path:` entry and copies both the scenario and
  `catalog.yaml`. Tests reject drift between `curriculum/` and
  `internal/curriculum/bundled/`.
- Do not include host shell commands, privileged workloads, host namespaces,
  or `hostPath` volumes.
- Run `make validate-pack`.
- Add a scenario test proving the initial state, target state, and
  reset behavior.

Local packs can be loaded with `kubecrypt --pack <directory> start`. Scenario
IDs must be unique across all active packs. All contributions are reviewed
before they become bundled trusted curriculum.
