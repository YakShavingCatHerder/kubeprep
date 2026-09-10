# Contributing to KubePrep

## Engine

1. Go 1.27 or newer and Docker. `kubeprep doctor` installs `kind` and
   `kubectl`. CI uses Go 1.27.
2. `make install`
3. `make ci` (gofmt, `go vet`, lab validate, `go test -race`). CI runs the same
   target on every PR and before a release tag.
4. `go test -tags=integration ./tests/integration` for the disposable cluster
   suite.

Grade cluster state, not command history. The runner is a split view with a
real Lab Shell. Do not assume the TUI suspends into a fake shell.

## Labs

See [`contribute/README.md`](../contribute/README.md). Start from
[`example-module.yaml`](../contribute/example-module.yaml). Engine overview:
[`architecture.md`](architecture.md).

`make validate-lab` checks `curriculum/` (or `CURRICULUM=<directory>` with a
`catalog.yaml`). Lab ids must be unique in that directory.
