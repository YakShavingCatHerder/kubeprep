# Author a lab

KubeCrypt labs are YAML. Learners use a real shell against a dedicated `kind`
cluster. The runner grades the cluster that results, not the commands they
typed, so any legitimate `kubectl` path can pass.

You do not need to write Go.

This directory is a scratch workspace. `kubecrypt start` does not load it. A
lab reaches learners when it is published under `curriculum/`. That directory
is the core pack; the binary embeds it at compile time.

## Write one file

Copy [`example-module.yaml`](example-module.yaml). Each field has a comment
that says what belongs there.

The file has two parts:

- `authoring` is notes for reviewers and agents. It is not shown to learners
  and is never applied. `desiredClusterConfiguration` is a review fixture
  only. Pass and fail come from `checks`.
- The rest is the Scenario the runner loads: start state, checks, hints,
  completion, and debrief.

Set `authoring.section` and `module` to the same section id (`pods`,
`scheduling`, `rbac`, and so on). The published path is
`{section}/{id}.yaml`. `tracks` lists which playlists may include the lab.
Play order lives in `catalog.yaml`, not in the lab file.

Start from a broken or incomplete cluster. Reset must restore that same
start, not a healthy cluster.

## Run it from a local pack

A pack is a directory with `catalog.yaml`, lab files, and any extra
manifests. Nothing is compiled.

```sh
make validate-pack PACK=./my-pack
kubecrypt --pack ./my-pack start
```

Do not point a catalog at this `contribute/` directory. Lab ids must be
unique across the core pack and every `--pack` you load.

## Publish into core

Canonical labs live at `curriculum/{section}/{id}.yaml`. The binary embeds
that tree. There is no second copy to sync.

1. Write the lab under `curriculum/{section}/`.
2. Add the id under the right path and section in `curriculum/catalog.yaml`.
   The same id can appear on more than one path; it is still one file.
3. Run `make validate-pack`.
4. Add a test that setup succeeds, the start is incomplete or broken, the
   target state is accepted, and reset restores the start. Prove one
   alternate valid fix when you can.

`kubecrypt --pack ./curriculum start` runs the pack without rebuilding.
`make install` is only needed when you want the lab inside a plain
`kubecrypt start`.

## Catalog shape

The catalog is a nested playlist: path → section → lab ids. Files have no
numeric prefixes. List order under a path is the order learners play those
labs. It is not the order labs have to be written or merged.

```yaml
paths:
  - id: beginner
    title: Beginner
    sections:
      - id: pods
        labs:
          - pod-creation
  - id: cka
    title: CKA
    sections:
      - id: pods
        labs:
          - pod-creation
```

Paths today are `beginner`, `cka`, and `ckad`. Use the same shape in a
local pack.

## What the runner grades

Checks describe measurable cluster state. `kubectl scale`, `edit`, `patch`,
`apply`, or recreate all pass if that state is right.

Implemented check types: `objectExists`, `fieldEquals`,
`deploymentAvailable`, `podReady`, `containersHealthy`, `nodeTopology`.

Hints must be exactly three, in this order: the concept, what to inspect,
then one concrete command or next step. The debrief should explain the real
Kubernetes mechanism and name the objects involved.

Give the lab a `kubecrypt-*` namespace when it needs a workspace. The
runner creates that namespace on start and recreates it on reset. Extra
broken objects belong in `startingClusterConfiguration` or in setup/reset
manifest files.

Keep resources in a `kubecrypt-*` namespace. Do not use host commands,
`hostPath`, privileged containers, or cluster-scoped kinds such as
`ClusterRole`.
