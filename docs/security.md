# Security

KubePrep drives a local [`kind`](https://kind.sigs.k8s.io/) cluster named
`kubeprep`. It is a training tool on your machine, not a sandbox and not a
place for any credentials.

## What KubePrep will not do

`start`, `reset`, and `destroy` use a kubeconfig that KubePrep writes under
your OS user config directory. Before those commands create, reset, or delete
a cluster they check:

- cluster name (`kubeprep`)
- API server URL
- CA certificate fingerprint
- a local ownership file

If that is not the KubePrep cluster, they refuse. They do not switch your
current kubectl context.

Lab YAML is Kubernetes objects and scenario copy. There is no field that runs
a host shell command. `authoring.desiredClusterConfiguration` is never
applied. Pack validation rejects `hostPath`, privileged containers, host
namespaces, and cluster-scoped kinds such as `ClusterRole`. Namespaces must
start with `kubeprep-`.

## What KubePrep does not stop

The Lab Shell is your real `$SHELL`. KubePrep sets `KUBECONFIG` to the
training cluster so `kubectl` aims there by default. You can still run any
command your user can run, including `kubectl` against another cluster if you
change that environment.

Docker and `kind` start containers on this host. Treat the training cluster
as untrusted. Do not put production secrets in it.

A lab file from someone else is only as safe as pack validation plus review.
Do not apply untrusted YAML with extra privileges.

## Reporting

If a bug would let KubePrep mutate a cluster it does not own, report it
privately to the repository owner. Do not open a public issue for that class
of bug.
