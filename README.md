# KubePrep

Practice [Kubernetes](https://kubernetes.io/) in a split terminal: the scenario
stays on one side, a real Lab Shell on the other, against a dedicated
[`kind`](https://kind.sigs.k8s.io/) cluster. It grades cluster state, not the
commands you typed.

[![CI](https://github.com/YakShavingCatHerder/kubeprep/actions/workflows/ci.yml/badge.svg)](https://github.com/YakShavingCatHerder/kubeprep/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Status: early. There is no `exam` command and no `--pack` flag.

![Split TUI: scenario pane beside a real Lab Shell](docs/demo.gif)

## Quick start

Requires macOS or Linux (amd64 or arm64) and
[Docker](https://docs.docker.com/get-docker/) (or another daemon `kind` can
use). `kubeprep doctor` installs pinned `kind` and
[`kubectl`](https://kubernetes.io/docs/reference/kubectl/)
([Kubernetes](https://kubernetes.io/releases/) 1.35) into the KubePrep config
directory.

### Release binary

bash and zsh download the **same** archive for a given machine. The installer
is a bash script; run it with bash even if your login shell is zsh. Do not
pipe it to zsh.

```sh
curl -fsSL https://raw.githubusercontent.com/YakShavingCatHerder/kubeprep/main/scripts/install.sh | bash
kubeprep --version
kubeprep doctor
kubeprep start --track=beginner
```

`PREFIX` and `--no-sudo` override the install path. Pin a tag with
`KUBEPREP_VERSION=v0.1.1`. The script maps `uname`, verifies `checksums.txt`,
and extracts into a temp directory.

macOS vs Linux **does** change the file you download from
[Releases](https://github.com/YakShavingCatHerder/kubeprep/releases). Names are
lowercase. `uname -m` of `x86_64` or `aarch64` is **not** the archive arch:

| Host | Archive |
| --- | --- |
| macOS Apple Silicon (`arm64`) | `kubeprep_<version>_darwin_arm64.tar.gz` |
| macOS Intel (`x86_64`) | `kubeprep_<version>_darwin_amd64.tar.gz` |
| Linux x86_64 | `kubeprep_<version>_linux_amd64.tar.gz` |
| Linux ARM (`aarch64` or `arm64`) | `kubeprep_<version>_linux_arm64.tar.gz` |

Unpack by hand into a temp directory (the archive also contains `LICENSE` and
`README.md`). Checksums: `shasum -a 256` on macOS, `sha256sum` on Linux.

```sh
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; esac
tmpdir=$(mktemp -d)
tar -xzf kubeprep_*_${os}_${arch}.tar.gz -C "$tmpdir"
sudo install -m 755 "$tmpdir/kubeprep" /usr/local/bin/kubeprep
```

### From source

[Go](https://go.dev/dl/) 1.27 or newer (same as CI):

```sh
make install
kubeprep doctor
kubeprep start --track=beginner
```

`make install` writes `./bin/kubeprep` and copies it to `$(go env GOPATH)/bin`
(or `GOBIN`). If that directory is not on your `PATH`, the target prints the
`export PATH=...` line to add.

## Train

```sh
kubeprep doctor          # kind, kubectl, OS, Docker, config dir
kubeprep start           # create the cluster if needed; open the current lab
kubeprep status          # track, current lab, completion
kubeprep reset           # restore the current lab only
kubeprep destroy         # delete the kind cluster; keep progress
kubeprep destroy --all   # cluster and learner progress
```

First `start` asks for a track unless you pass `--track=beginner`,
`--track=cka`, or `--track=ckad`. `start` uses the curriculum compiled into the
binary; YAML on disk does not change it until you `make install`.

The session is a split view with a dedicated `KUBECONFIG` for the KubePrep
cluster. Ungraded labs use `F2` to continue; graded labs use `F2` to check
cluster state.

| Key | Action |
| --- | --- |
| `?` or `F1` | hint (`?` if the editor steals `F1`) |
| `F2` | check cluster state, or continue on an ungraded lab |
| `F11` | zoom the Lab Shell |
| `Alt+←` / `Alt+→` | previous / next scenario page (or `Ctrl+G` then `p`/`n`) |
| `F10` | leave |

`reset` and `destroy` ask for confirmation. Scripts must pass `--force`.

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
