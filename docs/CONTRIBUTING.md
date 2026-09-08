# Contributing to KubeCrypt

KubeCrypt welcomes engine changes and new labs.

## Development

1. Install Go 1.27 or newer and Docker. `kind` and `kubectl` are installed by
   `kubecrypt doctor` when you run the CLI. CI uses Go 1.27 for `gofmt` and tests.
2. Run `make install` so `kubecrypt` is on your `PATH`.
3. Run `go test ./...`.
4. Run `go test -tags=integration ./tests/integration` for the disposable
   cluster suite.

Keep scenario behavior declarative. Validators must judge resulting Kubernetes
state rather than command history. The runner presents scenarios in a permanent
split with a real Lab Shell; do not assume the TUI suspends into a shell.

## Scenario submissions

Authoring details live in [`contribute/README.md`](../contribute/README.md).

- Start from [`contribute/example-module.yaml`](../contribute/example-module.yaml).
  For a single extra lab, you can also start from the closest file under
  `curriculum/`.
- Include metadata, an objective, setup, typed checks, progressive hints,
  completion text, a technical debrief, capability requirements, and
  deterministic reset data. Labs that need the cluster to settle may set
  `observeDelay` (for example `60s`) to pause the first automatic check.
- From the repository root, `kubecrypt lab try test-lab.yaml` validates and
  runs a filename in `contribute/` without writing `./curriculum`.
  `kubecrypt lab publish test-lab.yaml` copies it into `./curriculum` and
  lists it in `catalog.yaml`. `kubecrypt start` uses the embed from the last
  build until you `make install` again.
- Do not include host shell commands, privileged workloads, host namespaces,
  or `hostPath` volumes.
- Published labs should have a test that setup succeeds, the start is
  incomplete or broken, the target state is accepted, and reset restores the
  start. That is required for the shipped `pod-creation` lab; it is not yet
  enforced for every file `lab publish` writes.

`make validate-pack` checks `curriculum/` (or `PACK=<directory>` with a
`catalog.yaml`). Lab ids must be unique in that pack. There is no `--pack`
flag on the learner CLI. All contributions are reviewed before they become
core curriculum.
