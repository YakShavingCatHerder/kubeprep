# Author a lab

Labs are YAML. Learners use a real shell on a dedicated
[`kind`](https://kind.sigs.k8s.io/) cluster. The runner grades cluster state,
so any legitimate [`kubectl`](https://kubernetes.io/docs/reference/kubectl/)
path can pass. You do not need to write Go.

This directory is a scratch pad. [`kubeprep start`](../README.md#train) does
not load it. A lab shows up in `start` only after it is under
[`curriculum/`](../curriculum/) **and** you rebuild. Until then: `lab try`
(draft) or `lab publish` (write the pack, run that copy).

From the repository root, pass the filename only (`test-lab.yaml`), not
`contribute/test-lab.yaml`.

## Write one file

Copy [`example-module.yaml`](example-module.yaml). Comments on each field say
what belongs there.

- `authoring` is notes for reviewers. It is not shown to learners and is never
  applied. `desiredClusterConfiguration` is a review fixture. Pass/fail is
  `checks`.
- The rest is the Scenario: start state, checks, hints, completion, debrief.
  The pane renders `debrief.explanation`. `debrief.commands` / `concepts` /
  `domain` are metadata, not UI.

Keep `::page::`, fences, and backticks indented inside `|` blocks. A flush-left
marker splits the string and breaks the lab.

| Fence | Renders as |
| --- | --- |
| `kubectl` or `sh` | typed command; the pane draws `$` — do not put `$` in the YAML |
| `diagram` | vertical chain: one node per line, `->` (or `\|` / `v`) between them |
| `text` | glosses and one-line shapes (`kubectl <verb> <resource>`), indented |
| `yaml` | YAML sample, indented |
| unlabeled | indented diagram, not a command |

Inline backticks highlight a token in a sentence (`READY`), not a command to
type. `::page::` starts a new scenario-pane page when the task changes. Do not
use it to hide the objective or checks.

Set `authoring.section` and `module` to the same section id (`welcome`, `pods`,
`rbac`, …). The published path is `{section}/{id}.yaml`. `tracks` lists which
playlists may include the lab. Play order is
[`curriculum/catalog.yaml`](../curriculum/catalog.yaml), not the lab file.

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
[`catalog.yaml`](../curriculum/catalog.yaml), validates `./curriculum`, and
starts that lab from disk. Missing `./curriculum` is an error. Does not rebuild
the binary.

`start` still uses the last `make install`. Publish again to refresh the
on-disk pack; rebuild when you want `start` to include the lab.

Kind integration ([`tests/integration`](../tests/integration)) proves the
runner honors lab YAML keys on a fixture, not a specific shipped lab.
`lab validate` is the publish-time gate for schema, catalog paths, and
manifest safety.

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
order under a path is play order. Current beginner welcome pack:

```yaml
paths:
  - id: beginner
    sections:
      - id: welcome
        labs:
          - kubectl-basics
          - pod-creation
          - pod-inspection
          - pod-exec
          - manifest-basics
          - welcome-summary
```

Paths are `beginner`, `cka`, and `ckad`. The same id may appear on more than
one path; it is still one file. `lab publish` appends the id to each track in
`tracks`. It does not reorder existing lists.

## Checks

Any `kubectl` path that produces the graded state passes.

Orientation labs may set `ungraded: true` and `checks: []`. `F2` then completes
the lab without inspecting cluster state. Other labs still need at least one
typed check.

Types that run (unknown names fail lab validation):

| Type | Required fields |
| --- | --- |
| `objectExists` | `kind`, `namespace`, `name` (namespace is per-check) |
| `fieldEquals` | those plus `field` and `value` |
| `deploymentAvailable` | `namespace`, `name`; optional `replicas` (minimum Available) |
| `podReady` | `namespace`, `selector` (label selector; `name` is ignored). Optional `replicas` is the minimum Ready count (default 1) |
| `containersHealthy` | `namespace`, `selector` |
| `nodeTopology` | `count`, `controlPlanes`, `workers` |

Hints must be exactly three: concept, what to inspect, then one concrete
command or next step. Write `debrief.explanation` as the real mechanism and
named objects.

Give the lab a `kubeprep-*` namespace. The runner creates it on start and
recreates it on reset. Extra broken objects go in
`startingClusterConfiguration` or in setup/reset YAML next to the lab file.

Do not use host commands, `hostPath`, privileged containers, or cluster-scoped
kinds such as `ClusterRole`. See [`docs/SECURITY.md`](../docs/SECURITY.md).
