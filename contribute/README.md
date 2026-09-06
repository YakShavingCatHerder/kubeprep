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
orientation labs. Each official release after that adds a chapter of real
practice.

When a pull request is reviewed and merged into `curriculum/`, it is core
curriculum. The following tagged release embeds it in the binary. Anyone who
installs that version gets your labs with no extra packs and no rebuild of
their own.

That is the difference from `--pack`: local packs are perfect for drafting,
classrooms, and personal content, and they work the same day. Core publication
is how a lab becomes the shared CKA/CKAD path.

We merge in catalog order when we can. Careful labs in `01-foundations` and
`02-workloads` help the most right now. One well-taught scenario is more useful
than an unfinished chapter. First-time authors are welcome; review exists so
bundled YAML stays trusted.

## 1. Start with the questionnaire

Copy [`example-module.yaml`](example-module.yaml) and fill it in. That file is
the design brief: what you are teaching, the broken starting state, the done
condition, hints, and the debrief.

Pick a **slot** so the lab sits in the right chapter and the right release:

| Slot | Teaches | Official release |
| --- | --- | --- |
| `00-orientation` | Lab shell, kubeconfig, cluster components | v0.1 — in tree now |
| `01-foundations` | kubectl, contexts, namespaces, API resources, YAML | Next after v0.1 |
| `02-workloads` | Pods, Jobs, CronJobs, Deployments, DaemonSets, StatefulSets | v0.2 |
| `03-scheduling` | Labels, selectors, taints, tolerations, affinity, resources | v0.3 |
| `04-storage` | PVs, PVCs, StorageClasses, reclaim behavior | v0.4 |
| `05-networking` | Services, EndpointSlices, DNS, NetworkPolicy, ingress | v0.5 |
| `06-cluster-admin` | RBAC, ServiceAccounts, Helm, Kustomize, CRDs | v0.6 |
| `07-troubleshooting` | Mixed-domain failures, less guidance | v0.7 |

Exam mode is v1.0. It is not a numbered slot; later mixed labs feed it.

From the slot we infer module id, catalog order, domain, tracks, Kubernetes
1.35, namespace `kubecrypt-<id>`, and shrinking guidance (first lab more
helped, last lab more independent). Use `overrides` in the questionnaire only
when those defaults are wrong.

`00-orientation` is observe-only. Other slots are challenges: setup applies a
broken or incomplete state, and reset restores that same start—not a healthy
cluster.

## 2. Choose how you want to ship it

**Try it locally first.** A pack is just a directory with `catalog.yaml`,
scenario files, and manifests. Nothing is compiled in.

```sh
make validate-pack PACK=./my-pack
kubecrypt --pack ./my-pack start
```

**Publish into core** when the module should ship in the next official
release. Canonical files live under `curriculum/<slot>/`. The binary embeds a
copy from `internal/curriculum/bundled/`, and tests require those two trees to
match byte-for-byte. After you edit `curriculum/`, register the lab in
`curriculum/catalog.yaml` and sync the embedded copy:

```sh
make bundle-lesson 01-foundations/01-pod-creation
```

That command fails if the catalog has no matching `path:` entry. When the
catalog lists the lab, it copies both the scenario file and `catalog.yaml`.

| | Local pack | Published in core |
| --- | --- | --- |
| Learners get it | `kubecrypt --pack ./my-pack start` | Next official `kubecrypt` release |
| Catalog | `my-pack/catalog.yaml` | `curriculum/catalog.yaml` |
| Mirror the files | No | Yes — `internal/curriculum/bundled/` |
| Validate | `make validate-pack PACK=./my-pack` | `make validate-pack` |

Scenario IDs must be unique across the bundled pack and any `--pack` you load.
Do not add this `contribute/` directory to a catalog.

## 3. Register the labs

Module `id` is the slot without the number (`01-foundations` → `foundations`).
Each scenario `id` and `path` must match the file, and `module:` inside the
scenario must match the catalog module id. Lab files are numbered inside the
slot so play order is visible: `01-foundations/01-pod-creation.yaml` is always
the first foundations lab. Catalog list order must match those numbers.

```yaml
  - id: foundations
    title: Foundations
    scenarios:
      - id: inspect-namespace
        path: 01-foundations/01-inspect-namespace.yaml
```

Use the same shape in a local pack’s `catalog.yaml`, with paths relative to
that pack.

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
queued for the next official release that covers that slot. Questions and
first-time modules are welcome.
