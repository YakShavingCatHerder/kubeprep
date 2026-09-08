# Contributing to KubeCrypt

## Engine

1. Go 1.27 or newer and Docker. `kubecrypt doctor` installs `kind` and
   `kubectl`. CI uses Go 1.27.
2. `make install`
3. `go test ./...`
4. `go test -tags=integration ./tests/integration` for the disposable cluster
   suite.

Grade cluster state, not command history. The runner is a split view with a
real Lab Shell. Do not assume the TUI suspends into a fake shell.

## Labs

See [`contribute/README.md`](../contribute/README.md). Start from
[`example-module.yaml`](../contribute/example-module.yaml).

`make validate-pack` checks `curriculum/` (or `PACK=<directory>` with a
`catalog.yaml`). Lab ids must be unique in that pack.
