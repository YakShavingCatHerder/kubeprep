# KubeCrypt

KubeCrypt is a terminal app for practicing Kubernetes on a dedicated local
[`kind`](https://kind.sigs.k8s.io/) cluster. You type real `kubectl` in a real
shell. It grades the cluster that results, not the commands you used.

This is early. The binary ships **one lab**: [First API Object](curriculum/pods/pod-creation.yaml)
(`pod-creation`). Tracks Beginner, CKA, and CKAD exist; today they all play
that lab. There is no `exam` command and no `--pack` flag.

## Requirements

- macOS or Linux (amd64 or arm64)
- Docker, or another daemon `kind` can use

`kubecrypt doctor` installs pinned `kind` and `kubectl` (Kubernetes 1.35) into
the KubeCrypt config directory.

## Install

Tagged releases publish Linux and macOS binaries (amd64 and arm64) from
[YakShavingCatHerder/kubecrypt](https://github.com/YakShavingCatHerder/kubecrypt).
Unpack the archive and put `kubecrypt` on your `PATH`. Docker must already be
running.

```sh
# Example: macOS Apple Silicon
tar -xzf kubecrypt_*_Darwin_arm64.tar.gz
sudo mv kubecrypt /usr/local/bin/
kubecrypt --version
```

## Build from source

Go 1.27 or newer (same as CI):

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
kubecrypt doctor   # install kind/kubectl; check OS, Docker, and the config dir
kubecrypt start    # create the cluster if needed; open the current lab
```

First start asks for a track unless you pass `--track=beginner`, `--track=cka`,
or `--track=ckad`.

`start` splits the terminal: scenario on one side, Lab Shell on the other, with
a session-only `KUBECONFIG` for the KubeCrypt cluster. If the track has another
incomplete lab, you can continue to it.

| Key | Action |
| --- | --- |
| `?` or `F1` | hint (use `?` if the editor steals `F1`) |
| `F2` | check cluster state |
| `F11` | zoom the Lab Shell |
| `F10` | leave |

```sh
kubecrypt              # help
kubecrypt status       # track, current lab, completion, cluster
kubecrypt reset        # restore the current lab only
kubecrypt destroy      # delete the kind cluster; keep progress
kubecrypt destroy --all  # cluster and learner progress
```

`reset` and `destroy` ask for confirmation. Scripts must pass `--force`.

`start` uses the curriculum compiled into the binary. YAML on disk does not
change `start` until you `make install`.

## Author a lab

`lab try` and `lab publish` need a checkout of this repo and a file in
`contribute/`. A release binary has no contribute directory.

```sh
kubecrypt lab try test-lab.yaml      # validate and run; does not write curriculum/
kubecrypt lab publish test-lab.yaml  # copy into curriculum/ and catalog.yaml
```

Details: [contribute/README.md](contribute/README.md).

## Safety

KubeCrypt writes its own kubeconfig under your OS user config directory. Before
it mutates or destroys a cluster it checks API server, CA fingerprint, cluster
name, and an ownership marker. It never grades shell history. The Lab Shell is
your real shell, not a sandbox.

## Contributing

[CONTRIBUTING.md](docs/CONTRIBUTING.md) · [SECURITY.md](docs/SECURITY.md)
