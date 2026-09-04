# KubeCrypt

KubeCrypt is a terminal-native framework for Kubernetes certification training.
It runs declarative scenarios against a disposable local
[`kind`](https://kind.sigs.k8s.io/) cluster and evaluates the resulting cluster
state—not the commands used to reach it.

The bundled starter pack contains:

1. **Shell Orientation** — use the permanent split view: scenario on one side,
   a real Lab Shell on the other.
2. **Cluster Components** — inspect nodes, namespaces, control-plane Pods,
   CoreDNS, kube-proxy, and the CNI.

Additional scenario packs can be loaded from local directories without
recompiling KubeCrypt.

## Requirements

- macOS or Linux (amd64 or arm64)
- Docker (or a Docker-compatible runtime usable by `kind`)
- `kind`
- `kubectl`

Run `kubecrypt doctor` for actionable prerequisite checks.

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

You still install Docker, `kind`, and `kubectl` yourself. `kind` and `kubectl`
will be managed by KubeCrypt in a later change.

## Build and set up

From source (Go 1.24 or newer):

```sh
go build -o kubecrypt ./cmd/kubecrypt
./kubecrypt doctor
./kubecrypt setup
```

`setup` creates an isolated three-node `kubecrypt` cluster and opens the first
incomplete scenario in the active catalogs. On first setup it
asks whether you want the introductory tutorial:

- **Yes** starts with shell orientation.
- **No** asks whether you are following the CKA or CKAD track and skips
  tutorial-only scenarios.

For scripts, use `--tutorial=yes` or
`--tutorial=no --track=cka|ckad`.

`setup` and `resume` open a permanent split view: the scenario stays on screen
while a real Lab Shell runs in the other pane with a session-only `KUBECONFIG`.
Press `?` or `F1` for a hint, `F2` to validate cluster state, `F11` to zoom the
shell, and `F10` to leave. After a scenario is completed you are asked whether
to continue; `y` opens the next scenario in the same session, and `F10` (or `n`)
ends training. In the Cursor/VS Code terminal, `F1` is often captured
by the editor; use `?`.

Useful commands:

```sh
kubecrypt resume
kubecrypt status
kubecrypt objective
kubecrypt hint
kubecrypt check
kubecrypt reset
kubecrypt reset cluster-components
kubecrypt reset --all
kubecrypt destroy
kubecrypt --pack ./my-pack resume
kubecrypt pack validate ./my-pack
```

`kubecrypt reset` clears the learner profile and scenario progress while
retaining the verified cluster. `kubecrypt reset <scenario>` restores one
scenario and selects it for replay. `kubecrypt reset --all` destroys the
verified cluster and clears learner data. Destructive resets require
confirmation; scripts must pass `--force`.

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
kubecrypt pack validate path/to/pack
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
