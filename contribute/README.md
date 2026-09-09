# Author a lab

Labs are YAML. Learners use a real shell on a dedicated `kind` cluster. The
runner grades cluster state, so any legitimate `kubectl` path can pass. You do
not need to write Go.

This directory is a scratch pad. `kubeprep start` does not load it. A lab
shows up in `start` only after it is under `curriculum/` **and** you rebuild.
Until then: `lab try` (draft) or `lab publish` (write the pack, run that copy).

From the repository root, pass the filename only (`test-lab.yaml`), not
`contribute/test-lab.yaml`.

## Write one file

Copy [`example-module.yaml`](example-module.yaml). Comments on each field say
what belongs there.

- `authoring` is notes for reviewers. It is not shown to learners and is never
  applied. `desiredClusterConfiguration` is a review fixture. Pass/fail is
  `checks`.
- The rest is the Scenario: start state, checks, hints, completion, debrief.

A line that is exactly `::page::` in `description`, `objective`, `completion`,
or `debrief.explanation` starts a new page in the left-hand scenario pane.
Learners never see the marker. A page that is still too long for the pane
keeps auto-splitting.

Wrap copy-paste commands in fenced blocks: a line of three backticks, the
command, then a closing line of three backticks. Inline backticks highlight
a token in a sentence (for example `READY`). Keep both indented inside the
`|` block, same as `::page::`.

Set `authoring.section` and `module` to the same section id (`welcome`, `pods`,
`rbac`, …). The published path is `{section}/{id}.yaml`. `tracks` lists which
playlists may include the lab. Play order is `catalog.yaml`, not the lab file.

Start from a broken or incomplete cluster. Reset must restore that same start.

## Try

```sh
kubeprep lab try test-lab.yaml
```

Validates the file, checks sidecar manifests, builds a one-lab overlay in a
temp directory, and opens that lab. Does not change `curriculum/` or
`catalog.yaml`. Other labs' progress stays; the previous current lab is
restored when you leave.

## Publish

```sh
kubeprep lab publish test-lab.yaml
```

Copies to `curriculum/{section}/{id}.yaml`, appends the id under each track in
`catalog.yaml`, validates `./curriculum`, and starts that lab from disk.
Missing `./curriculum` is an error. Does not rebuild the binary.

`start` still uses the last `make install`. Publish again to refresh the
on-disk pack; rebuild when you want `start` to include the lab.

The shipped `pod-creation` lab has an integration test under
`tests/integration`. New published labs should prove setup, incomplete start,
accepted target state, and reset. That is not a `lab publish` gate yet.

## Validate

```sh
kubeprep lab validate
make validate-lab
```

`make validate-lab` (default `CURRICULUM=curriculum`) runs
`kubeprep lab validate` on a directory with `catalog.yaml`. That checks the
labs. It does not load them into `start`.

## Catalog

Nested playlist: path → section → lab ids. No numeric prefixes on files. List
order under a path is play order.

```yaml
paths:
  - id: beginner
    title: Beginner
    sections:
      - id: welcome
        labs:
          - kubectl-basics
      - id: pods
        labs:
          - pod-creation
```

Paths are `beginner`, `cka`, and `ckad`. The same id may appear on more than
one path; it is still one file. `lab publish` appends the id to each track in
`tracks`. It does not reorder existing lists.

## Checks

Any `kubectl` path that produces the graded state passes.

Orientation labs may set `ungraded: true` and `checks: []`. F2 then completes
the lab without inspecting cluster state. Other labs still need at least one
typed check.

Types that run (unknown names fail lab validation):

- `objectExists` — `kind`, `namespace`, `name` (namespace is per-check, not inherited from the lab)
- `fieldEquals` — those plus `field` and `value`
- `deploymentAvailable` — `namespace`, `name`; optional `replicas` (minimum Available)
- `podReady` — `namespace`, `selector` (label selector; `name` is ignored). Optional `replicas` is the minimum Ready count (default 1)
- `containersHealthy` — `namespace`, `selector`
- `nodeTopology` — `count`, `controlPlanes`, `workers`

Hints must be exactly three: concept, what to inspect, then one concrete
command or next step. The debrief should name the real mechanism and objects.

Give the lab a `kubeprep-*` namespace. The runner creates it on start and
recreates it on reset. Extra broken objects go in
`startingClusterConfiguration` or in setup/reset YAML next to the lab file.

Do not use host commands, `hostPath`, privileged containers, or cluster-scoped
kinds such as `ClusterRole`.
