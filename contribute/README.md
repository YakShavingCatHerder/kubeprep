# Contribute a KubeCrypt module

Thank you for considering a contribution. You do not need to write Go.

KubeCrypt labs are declarative YAML. Learners fix a real `kind` cluster in a
split view with a real shell. We grade the resulting cluster state—not the
commands they typed—so every legitimate `kubectl` path can succeed.

Your work can ship to every learner. A merged core module is bundled into the
**next official KubeCrypt release** and becomes part of `kubecrypt start`.

## Land in the next official release

KubeCrypt is early, and the bundled curriculum is still growing on purpose.
**v0.1** is the framework release: doctor, start, the split Lab Shell, and the
first beginner lab. Each official release after that adds a chapter of real
practice.

When a pull request is reviewed and merged into `curriculum/`, it is core
curriculum. The following tagged release embeds it in the binary. Anyone who
installs that version gets your labs with no extra packs and no rebuild of
their own.

That is the difference from `--pack`: local packs are perfect for drafting,
classrooms, and personal content, and they work the same day. Core publication
is how a lab becomes the shared CKA/CKAD path.

We merge in catalog order when we can. Careful labs in `pods` and other early
sections help the most right now. One well-taught scenario is more useful
than an unfinished chapter. First-time authors are welcome; review exists so
bundled YAML stays trusted.

## 1. Start with the lab file

Copy [`example-module.yaml`](example-module.yaml) and fill it in. `authoring:`
holds notes for humans and agents (intent, section, guidance, desired YAML,
accept/reject). The rest of the file is the loadable Scenario: start YAML,
checks, hints, completion, and debrief. The runner grades `checks` only and
never applies `desiredClusterConfiguration`.

Set `authoring.section` to the domain folder (`pods`, `scheduling`, `rbac`).
Set `module:` to that same section id. Learner playlists (`beginner`, `cka`,
`ckad`) live in `catalog.yaml`, not in the lab file. A lab is one file and can
appear on more than one path.

Labs are challenges: setup applies a broken or incomplete state, and reset
restores that same start—not a healthy cluster.

## 2. Choose how you want to ship it

**Try it locally first.** A pack is just a directory with `catalog.yaml`,
scenario files, and manifests. Nothing is compiled in.

```sh
make validate-pack PACK=./my-pack
kubecrypt --pack ./my-pack start
```

**Publish into core** when the module should ship in the next official
release. Canonical files live under `curriculum/<section>/`. The binary embeds a
copy from `internal/curriculum/bundled/`, and tests require those two trees to
match byte-for-byte. After you edit `curriculum/`, register the lab id under
the right path and section in `curriculum/catalog.yaml` and sync:

```sh
make bundle-lesson pods/pod-creation
```

That command fails if the catalog has no matching lab id. When the catalog
lists the lab, it copies both the scenario file and `catalog.yaml`.

| | Local pack | Published in core |
| --- | --- | --- |
| Learners get it | `kubecrypt --pack ./my-pack start` | Next official `kubecrypt` release |
| Catalog | `my-pack/catalog.yaml` | `curriculum/catalog.yaml` |
| Mirror the files | No | Yes — `internal/curriculum/bundled/` |
| Validate | `make validate-pack PACK=./my-pack` | `make validate-pack` |

Scenario IDs must be unique across the bundled pack and any `--pack` you load.
Do not add this `contribute/` directory to a catalog.

## 3. Register the labs

The catalog is a nested playlist. Lab files are `{section}/{id}.yaml` with no
numbers. List order under a path is play order. The same id may appear on
more than one path; it is still one file.

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

`module:` inside the scenario must match the section id (`pods`). Use the same
catalog shape in a local pack.

## 4. What a strong lab looks like

- The start is intentionally incomplete or broken.
- “Done” is a measurable cluster state.
- Scale, edit, patch, apply, or recreate all pass if the state is right.
- Exactly three hints: conceptual, then what to inspect, then a concrete next step.
- The debrief explains the real Kubernetes mechanism.
- Declare `namespace: kubecrypt-<id>` on the scenario when the lab needs a
  workspace Namespace. The runner creates it on start and recreates it on reset.
  Extra broken objects still belong in setup/reset manifest files.

Checks the runner can evaluate today: `objectExists`, `fieldEquals`,
`deploymentAvailable`, `podReady`, `containersHealthy`, `nodeTopology`.

Keep resources in a `kubecrypt-*` namespace. Skip host commands, `hostPath`,
privileged containers, and cluster-scoped kinds such as `ClusterRole`.

Published labs should also have a test that setup succeeds, the start is
broken, the target is accepted, and reset restores the start—plus one
alternate valid fix when you can.

Open a pull request when you are ready. After review and merge, the work is
queued for the next official release that covers that section. Questions and
first-time labs are welcome.
