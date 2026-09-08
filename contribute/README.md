# Author a lab

Labs are YAML. Learners use a real shell on a dedicated `kind` cluster. The
runner grades the cluster that results, not the commands they typed, so any
legitimate `kubectl` path can pass.

You do not need to write Go.

This directory is a scratch pad. `kubecrypt start` does not load it. A lab
reaches `start` when it lives under `curriculum/` **and** you rebuild the
binary. Until then, use `lab try` (draft) or `lab publish` (write the pack on
disk and run that copy).

Run these commands from the repository root. Pass the filename inside
`contribute/` only (`test-lab.yaml`), not `contribute/test-lab.yaml`.

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

## Try (does not write the pack)

```sh
kubecrypt lab try test-lab.yaml
```

That schema-validates the file, checks sidecar manifests, builds a one-lab
overlay in a temp directory, and opens that lab. It does not change
`curriculum/` or `catalog.yaml`. Other labs' progress is left in place; the
previous current lab is restored when you leave.

## Publish (writes the pack)

```sh
kubecrypt lab publish test-lab.yaml
```

That copies the file to `curriculum/{section}/{id}.yaml`, lists the id in
`catalog.yaml` under the lab's `tracks`, validates `./curriculum`, and starts
that lab from disk. Missing `./curriculum` is an error. It does not rebuild
the binary.

`kubecrypt start` still uses the embed from the last `make install`. Publish
again after edits if you want the on-disk pack updated; rebuild when you want
`start` to include the lab.

The shipped `pod-creation` lab has an integration test under
`tests/integration`. New published labs should prove setup, incomplete start,
accepted target state, and reset. That is not a `lab publish` gate yet.

## Catalog shape

The catalog is a nested playlist: path → section → lab ids. Files have no
numeric prefixes. List order under a path is the order learners play those
labs.

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

Paths today are `beginner`, `cka`, and `ckad`. The same id may appear on more
than one path; it is still one file. `lab publish` appends the id to each
track in `tracks`. It does not reorder existing lists.

`make validate-pack` (default `PACK=curriculum`) runs the hidden
`kubecrypt pack validate` command on a directory that contains `catalog.yaml`.
That checks the pack. It does not load the pack into `start`.

## What the runner grades

Checks describe measurable cluster state. `kubectl scale`, `edit`, `patch`,
`apply`, or recreate all pass if that state is right.

Types that actually run:

- `objectExists`
- `fieldEquals`
- `deploymentAvailable`
- `podReady`
- `containersHealthy`
- `nodeTopology`

Other type names may pass file validation and then fail when the lab is
checked (`F2`). Do not use them.

Hints must be exactly three, in this order: the concept, what to inspect,
then one concrete command or next step. The debrief should explain the real
Kubernetes mechanism and name the objects involved.

Give the lab a `kubecrypt-*` namespace when it needs a workspace. The
runner creates that namespace on start and recreates it on reset. Extra
broken objects belong in `startingClusterConfiguration` or in setup/reset
manifest files next to the lab YAML.

Keep resources in a `kubecrypt-*` namespace. Do not use host commands,
`hostPath`, privileged containers, or cluster-scoped kinds such as
`ClusterRole`.
