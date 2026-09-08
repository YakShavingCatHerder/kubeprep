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
  deterministic reset data. Labs that need the cluster to settle may set
  `observeDelay` (for example `60s`) to pause the first automatic check.
- Run `kubecrypt lab try test-lab.yaml` from the repository root (filename
  inside `contribute/`). That validates the file and starts that lab without
  writing `./curriculum`. When the draft is ready, `kubecrypt lab publish
  test-lab.yaml` copies it into `./curriculum` and lists it in `catalog.yaml`.
  The binary embeds `curriculum/` at compile time for `kubecrypt start`.
- Do not include host shell commands, privileged workloads, host namespaces,
  or `hostPath` volumes.
- Add a scenario test proving the initial state, target state, and
  reset behavior.

Local packs can be validated with `make validate-pack PACK=<directory>`.
Scenario IDs must be unique in a pack. All contributions are reviewed
before they become core curriculum.
