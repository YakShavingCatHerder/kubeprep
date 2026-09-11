# Contributing to KubePrep

## Write a lab

From the repository root:

1. Copy [`contribute/example-module.yaml`](../contribute/example-module.yaml)
   to a new file into `contribute/`, named for the lab
   (`your_lab.yaml`).
2. Fill in the questionnaire under `authoring:`. Reviewers read this; learners
   never see it, and KubePrep never applies it.
3. Fill in the Scenario body: start state, checks, copy, hints, debrief.
4. Try it, then publish when it is ready.

```sh
kubeprep lab try your_lab.yaml       # filename only, from the repo root
kubeprep lab publish your_lab.yaml
```

`lab try` does not write `curriculum/`. `lab publish` copies the file to
`curriculum/{section}/{id}.yaml` and lists it in `catalog.yaml`. `start` still
uses the last `make install` until you rebuild.

Details for fences, check types, and the catalog:
[`contribute/README.md`](../contribute/README.md).

### Questionnaire (`authoring`)

| Field | What to write |
| --- | --- |
| `section` | Folder and catalog section (`welcome`, `pods`, `deployments`, …). Must match `module`. |
| `guidance` | How much help: `guided`, `assisted`, `operator`, or `exam`. |
| `intent.teach` | The one skill this lab proves. |
| `intent.wrongIdea` | The misconception to break. |
| `intent.mentalModel` | The real mechanism, in one or two sentences. |
| `intent.why` | Why this lab sits here on the playlist. |
| `intent.notes` | Scope, start-state rationale, what not to treat as a second objective. |
| `desiredClusterConfiguration` | Healthy target YAML for reviewers. **Never applied.** Pass/fail is `checks`. |
| `accept` | Legitimate ways to reach that state (`kubectl apply`, `edit`, `patch`, …). |
| `reject` | Near-misses that must fail (wrong kind, namespace, image). |
| `proveAlternate` | One other valid path, so the grader is not locked to a single command. |

### Scenario body

| Field | What it does |
| --- | --- |
| `id`, `title` | Stable kebab-case id; short learner-facing title. Published path is `{section}/{id}.yaml`. |
| `module`, `tracks` | Section id (same as `authoring.section`). Playlists that may list this lab. Play order is `catalog.yaml`. |
| `namespace` | Must be `kubeprep-*`. Created on start; wiped and recreated on reset. Repeat it on each check. |
| `startingClusterConfiguration` | YAML applied when the lab starts and when it resets. Empty means namespace only. Prefer a broken or missing object, not a healthy cluster. |
| `setup` / `reset` `manifests` | Extra files next to the lab YAML, applied after the start blob. Use the same list on reset. |
| `description`, `objective` | Shown in the scenario pane. Objective is the target state, not a command recipe. |
| `checks` | Graded cluster state. Any legitimate `kubectl` path that reaches it passes. Labs may set `ungraded: true` and `checks: []`. if you do not want to set any graded states but be aware that any F2 will skip remaining lab pages and go straight to the lab debrief. |
| `hints` | Exactly three, in order: concept, what to inspect, then one concrete next step. |
| `completion` | Short confirmation of the proven state. |
| `debrief.explanation` | Shown after success: the real mechanism and named objects. `debrief.commands`, `concepts`, and `domain` are YAML metadata, not UI. |

Reset must restore the same start as `startingClusterConfiguration`. Do not
treat `desiredClusterConfiguration` as a second start state.
