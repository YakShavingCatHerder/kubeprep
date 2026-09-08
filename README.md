# KubeCrypt

KubeCrypt is a terminal app for practicing Kubernetes on a dedicated local
[`kind`](https://kind.sigs.k8s.io/) cluster. You type real `kubectl` in a real
shell. It grades the cluster state that results, not the commands you used.

This is early. The binary ships **one lab**: [First API Object](curriculum/pods/pod-creation.yaml)
(`pod-creation`). Tracks (Beginner, CKA, CKAD) exist; today they all play that
lab. There is no `exam` command and no `--pack` flag.

## Requirements

- macOS or Linux (amd64 or arm64)
- Docker, or another daemon `kind` can use

`kubecrypt doctor` installs pinned `kind` and `kubectl` (Kubernetes 1.35) into
the KubeCrypt config directory. You do not install those two yourself.

## Install

Tagged releases publish `kubecrypt` for Linux and macOS (amd64 and arm64) from
[YakShavingCatHerder/kubecrypt](https://github.com/YakShavingCatHerder/kubecrypt).
Download the archive for your platform, unpack it, and put `kubecrypt` on your
`PATH`.

```sh
# Example: macOS Apple Silicon, after downloading the release archive
tar -xzf kubecrypt_*_Darwin_arm64.tar.gz
sudo mv kubecrypt /usr/local/bin/
kubecrypt --version
```

Docker (or a compatible daemon) must already be running.

## Build from source

Go 1.27 or newer (same as CI `gofmt`):

```sh
make install
kubecrypt doctor
kubecrypt start
```

`make install` writes `./bin/kubecrypt` and copies it to `$(go env GOPATH)/bin`
(or `GOBIN`). If that directory is not on your `PATH`, the target prints the
`export PATH=...` line to add.

## Train

```sh
kubecrypt doctor   # install kind/kubectl, check OS and Docker
kubecrypt start    # create the cluster if needed, open the current lab
```

First start asks which track you are on unless you pass `--track=beginner`,
`--track=cka`, or `--track=ckad`.

`start` splits the terminal: the scenario stays visible while a Lab Shell runs
beside it with a session-only `KUBECONFIG` for the KubeCrypt cluster. After you
complete a lab you can continue to the next one on that track, or leave.

| Key | Action |
| --- | --- |
| `?` or `F1` | hint (`?` if the editor steals `F1`) |
| `F2` | check cluster state |
| `F11` | zoom the Lab Shell |
| `F10` | leave |

```sh
kubecrypt              # help
kubecrypt status       # track, current lab, completion, cluster
kubecrypt reset        # restore the current lab only
kubecrypt destroy      # delete the kind cluster; keep progress
kubecrypt destroy --all
```

`reset` and `destroy` ask for confirmation. Scripts must pass `--force`.

`start` reads the curriculum compiled into the binary. Editing YAML on disk
does not change `start` until you rebuild (`make install`).

## Author a lab

`lab try` and `lab publish` only work from a checkout of this repository, with
a file in `contribute/`. They are not available as a way to load extra packs
into an installed binary.

```sh
kubecrypt lab try test-lab.yaml      # validate and run; does not write curriculum/
kubecrypt lab publish test-lab.yaml  # copy into curriculum/ and catalog.yaml
```

See [contribute/README.md](contribute/README.md).

## Safety

KubeCrypt writes its own kubeconfig under your OS user config directory. Before
it mutates or destroys a cluster it checks API server, CA fingerprint, cluster
name, and an ownership marker. It never grades shell history. The Lab Shell is
your real shell; it is not a sandbox.

## Contributing

Engine and curriculum notes: [CONTRIBUTING.md](docs/CONTRIBUTING.md).
Security: [SECURITY.md](docs/SECURITY.md).
