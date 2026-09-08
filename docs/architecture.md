# KubeCrypt architecture

KubeCrypt separates scenario intent from where a scenario runs.

## Local v0.1

The `kubecrypt` process owns:

- learner onboarding and progress,
- the core curriculum pack embedded from `curriculum/` at compile time,
- KubeCrypt cluster lifecycle,
- state validation,
- the Bubble Tea scenario view with a permanent split: scenario pane plus a
  real PTY Lab Shell.

Authors run a contribute file with `lab try` (temp overlay, no pack writes)
or `lab publish` (write `./curriculum`, then start that copy from disk).
`start` does not load extra directories and does not provide `--pack`.

The Lab Shell receives a dedicated kubeconfig through its process environment.
KubeCrypt's own mutating operations independently verify cluster ownership;
they never trust the shell or the user's global current context.

## Runtime boundaries

- `internal/curriculum` loads packs and describes setup, typed checks, and reset.
- `internal/cluster` owns environment checks, identity, and lifecycle.
- `internal/validator` observes Kubernetes state.
- `internal/game` owns learner profile and progress.
- `internal/terminal` renders the split scenario view and hosts the Lab Shell PTY.

Neither curriculum nor validators depend on Bubble Tea. Overlay drafts,
on-disk `curriculum/`, and the embedded pack follow the same schema and
safety validation.

## Future hosted mode

A hosted version should put the blog in front of—not inside—the execution
plane. Each learner would receive an expiring isolated VM or virtual cluster,
with browser terminal traffic passing through an authenticated WebSocket
gateway. Quotas, restricted egress, admission policy, idle expiry, and
guaranteed cleanup are required.

This infrastructure is deliberately outside v0.1. An initial hosted experiment
should use an established workshop platform such as Educates rather than
exposing a shell from the personal blog server.
