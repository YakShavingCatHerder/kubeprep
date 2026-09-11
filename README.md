# KubePrep

Practice [Kubernetes](https://kubernetes.io/) in a split terminal: the scenario
stays on one side, a real Lab Shell on the other, against a dedicated
[`kind`](https://kind.sigs.k8s.io/) cluster. It grades cluster state, not the
commands you typed.

[![CI](https://github.com/YakShavingCatHerder/kubeprep/actions/workflows/ci.yml/badge.svg)](https://github.com/YakShavingCatHerder/kubeprep/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

![Split TUI: scenario pane beside a real Lab Shell](docs/demo.gif)

## Quick start

Requires macOS or Linux (amd64 or arm64) and
[Docker](https://docs.docker.com/get-docker/) (or another daemon `kind` can
use). `kubeprep doctor` installs pinned `kind` and
[`kubectl`](https://kubernetes.io/docs/reference/kubectl/)
([Kubernetes](https://kubernetes.io/releases/) 1.35) into the KubePrep config
directory.

### Install via script

```sh
curl -fsSL https://raw.githubusercontent.com/YakShavingCatHerder/kubeprep/main/scripts/install.sh | bash
kubeprep --version     #checks that kubeprep is in PATH, if not doctor will help fix
kubeprep doctor 
kubeprep start
```

### Install from source

[Go](https://go.dev/dl/) 1.27 or newer (same as CI):

```sh
make install
kubeprep doctor
kubeprep start --track=beginner
```

`make install` writes `./bin/kubeprep` and copies it to `$(go env GOPATH)/bin`
(or `GOBIN`). If that directory is not on your `PATH`, the target prints the
`export PATH=...` line to add.

## KubePrep commands

| Command | What it does |
| --- | --- |
| `kubeprep doctor` | Install pinned `kind` and `kubectl`, then check OS, Docker, and the config directory |
| `kubeprep start` | Create the training cluster if needed and open the current lab |
| `kubeprep start --track=beginner` | Same, and pick a track (`beginner`, `cka`, or `ckad`) without a prompt |
| `kubeprep status` | Track, current lab, completion, and hint use |
| `kubeprep reset` | Restore the current lab only (cluster stays) |
| `kubeprep destroy` | Delete the `kind` cluster; keep progress |
| `kubeprep destroy all` | Delete the cluster and clear all progress |

First `start` asks for a track unless you pass `--track`. `start` uses labs
compiled into the binary; YAML on disk does not change it until you
`make install`.

`reset` and `destroy` ask for confirmation. Scripts skip that with `-y`
(`kubeprep destroy all -y`).

The session is a split view. The Lab Shell uses a dedicated `KUBECONFIG` for
the KubePrep cluster. Ungraded labs: `F2` continues. Graded labs: `F2` checks
cluster state.

| Key | Action |
| --- | --- |
| `?` or `F1` | hint (`?` if the editor steals `F1`) |
| `F2` | check cluster state, or continue on an ungraded lab |
| `F11` | zoom the Lab Shell |
| `Alt+←` / `Alt+→` | previous / next scenario page (or `Ctrl+G` then `p`/`n`) |
| `F10` | leave |

### Curriculum

Play order lives in [`curriculum/catalog.yaml`](curriculum/catalog.yaml).
Beginner currently runs the welcome pack:

| Lab | File |
| --- | --- |
| meet kubectl | [`kubectl-basics.yaml`](curriculum/welcome/kubectl-basics.yaml) |
| create your first pod | [`pod-creation.yaml`](curriculum/welcome/pod-creation.yaml) |
| inspect a running pod | [`pod-inspection.yaml`](curriculum/welcome/pod-inspection.yaml) |
| run commands inside a pod | [`pod-exec.yaml`](curriculum/welcome/pod-exec.yaml) |
| create a pod from yaml | [`manifest-basics.yaml`](curriculum/welcome/manifest-basics.yaml) |
| Review the Kubernetes Foundations | [`welcome-summary.yaml`](curriculum/welcome/welcome-summary.yaml) |

[CKA](https://training.linuxfoundation.org/certification/certified-kubernetes-administrator-cka/)
and
[CKAD](https://training.linuxfoundation.org/certification/certified-kubernetes-application-developer-ckad/)
tracks currently start with meet kubectl.

## Author a lab

`lab try` and `lab publish` need a checkout of this repo and a file in
[`contribute/`](contribute/). A release binary has no contribute directory.

```sh
kubeprep lab try test-lab.yaml       # validate and run; does not write curriculum/
kubeprep lab publish test-lab.yaml   # install into curriculum/ and catalog.yaml
kubeprep lab validate                # check ./curriculum
```

See [`contribute/README.md`](contribute/README.md).

## Safety

KubePrep writes its own kubeconfig under your OS user config directory. Before
it mutates or destroys a cluster it checks API server, CA fingerprint, cluster
name, and an ownership marker. It never grades shell history. The Lab Shell is
your real shell, not a sandbox. Details:
[`docs/security.md`](docs/security.md).

## Docs

- [`docs/architecture.md`](docs/architecture.md) — packages and pack load
- [`docs/contributing.md`](docs/contributing.md) — engine and lab workflow
- [`docs/security.md`](docs/security.md) — trust boundaries
- [`contribute/README.md`](contribute/README.md) — authoring labs

## License

[Apache License 2.0](LICENSE)
