# Security Policy

## Supported version

KubePrep is early. Only the latest commit receives security fixes.

## Reporting

Do not publish vulnerabilities that could cause KubePrep to mutate an
unrelated Kubernetes cluster or escape a scenario environment. Report them
privately to the repository owner.

## Trust boundaries

- Shipped curriculum is trusted after review and automated validation.
- Lab YAML is data. It cannot run host commands.
- KubePrep-owned operations use a dedicated kubeconfig and verify cluster
  identity before mutation or deletion.
- The Lab Shell is the user's real shell. KubePrep aims kubectl at the
  training cluster; it does not sandbox the user.
- Never put production credentials in the KubePrep cluster.
