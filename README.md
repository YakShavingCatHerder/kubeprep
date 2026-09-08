# KubeCrypt

KubeCrypt is a terminal-native framework for Kubernetes certification training.
It runs declarative scenarios against a disposable local
[`kind`](https://kind.sigs.k8s.io/) cluster and evaluates the resulting cluster
state—not the commands used to reach it.

The bundled starter pack contains:

1. **Create a Pod** — submit a Pod to the API server and confirm the stored
   object in the isolated training cluster.

Draft labs in [`contribute/`](contribute/) with `kubecrypt lab try`. Publish
them into `curriculum/` with `kubecrypt lab publish`. `kubecrypt start` uses
the pack embedded at compile time.

## Requirements

- macOS or Linux (amd64 or arm64)
- Docker (or a Docker-compatible runtime usable by `kind`)

`kind` and `kubectl` are installed by `kubecrypt doctor` into the KubeCrypt
config directory (pinned to Kubernetes 1.35). You do not need to install them
yourself.

## Install

Tagged releases publish `kubecrypt` binaries for Linux and macOS (amd64 and
arm64) from [YakShavingCatHerder/kubecrypt](https://github.com/YakShavingCatHerder/kubecrypt).
Download the archive for your platform from GitHub Releases, unpack it, and
put `kubecrypt` on your `PATH`.

```sh
# Example: macOS Apple Silicon, after downloading the release archive
tar -xzf kubecrypt_*_Darwin_arm64.tar.gz
sudo mv kubecrypt /usr/local/bin/
kubecrypt --version
```

You still need Docker (or a compatible daemon) running. `kubecrypt doctor`
installs pinned `kind` and `kubectl` into the KubeCrypt config directory.

## Build and set up

From source (Go 1.27 or newer; matches CI `gofmt`):

```sh
make install
kubecrypt doctor
kubecrypt start
```

`make build` writes `./bin/kubecrypt`. `make install` copies that binary into
`$(go env GOPATH)/bin` (or `GOBIN`) so you can run `kubecrypt` without a path
prefix. If that directory is not already on your `PATH`, the install target
prints the `export PATH=...` line to add.

`start` creates an isolated three-node `kubecrypt` cluster if needed and opens
the current lab. On first start it asks which track you are following
(Beginner, CKA, or CKAD). For scripts, pass `--track=beginner`,
`--track=cka`, or `--track=ckad`.

`start` opens a permanent split view: the scenario stays on screen
while a real Lab Shell runs in the other pane with a session-only `KUBECONFIG`.
Press `?` or `F1` for a hint, `F2` to validate cluster state, `F11` to zoom the
shell, and `F10` to leave. After a scenario is completed you are asked whether
to continue; `y` opens the next scenario in the same session, and `F10` (or `n`)
ends training. In the Cursor/VS Code terminal, `F1` is often captured
by the editor; use `?`.

Useful commands:

```sh
kubecrypt
kubecrypt start
kubecrypt lab try <file>
kubecrypt lab publish <file>
kubecrypt status
kubecrypt reset
kubecrypt destroy
kubecrypt destroy --all
```

`kubecrypt` with no arguments prints help. `lab try test-lab.yaml` validates a
lab from `contribute/` and starts it without writing `./curriculum`.
`lab publish test-lab.yaml` installs that file into `./curriculum` and starts
it. Neither rebuilds the binary. `reset` restores the current lab
only. `destroy` removes the cluster and keeps progress; `destroy --all` also
clears learner data. Destructive commands require confirmation; scripts must
pass `--force`.

## Safety model

KubeCrypt writes a dedicated kubeconfig under your OS user configuration
directory. Its own mutating and destructive operations verify the API server,
CA fingerprint, cluster name, and an ownership marker before proceeding.
KubeCrypt never reads or grades shell history.

## Scenario packs

Scenarios are declarative YAML and cannot execute arbitrary host commands.
Each pack contains a `catalog.yaml`, scenario definitions, and any referenced
Kubernetes manifests. New modules should start from
[`contribute/`](contribute/). Validate a pack before using or contributing it:

```sh
make validate-pack PACK=path/to/pack
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
