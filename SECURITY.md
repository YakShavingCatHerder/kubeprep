# Security Policy

## Supported version

KubeCrypt is currently a proof of concept. Only the latest commit receives
security fixes.

## Reporting

Do not publish vulnerabilities that could cause KubeCrypt to mutate an
unrelated Kubernetes cluster or escape a scenario environment. Report them
privately to the repository owner.

## Trust boundaries

- Bundled curriculum is trusted only after repository review and automated
  validation.
- External scenario YAML is data, not executable host code.
- KubeCrypt-owned operations use a dedicated kubeconfig and verify cluster
  identity before mutation or deletion.
- The learner shell is intentionally unrestricted local user activity.
  KubeCrypt scopes its default Kubernetes target but does not sandbox the user.
- Never place production credentials in the KubeCrypt cluster.
