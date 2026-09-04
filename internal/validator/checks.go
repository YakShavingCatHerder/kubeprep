package validator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CheckType identifies a reusable state check in data-oriented definitions.
type CheckType string

const (
	CheckObjectExists        CheckType = "object-exists"
	CheckFieldEquals         CheckType = "field-equals"
	CheckDeploymentExists    CheckType = "deployment-exists"
	CheckDeploymentAvailable CheckType = "deployment-available"
	CheckPodReady            CheckType = "pod-ready"
	CheckContainersHealthy   CheckType = "containers-healthy"
	CheckNodeTopology        CheckType = "node-topology"
)

// Definition is a curriculum-neutral representation that can be populated
// from YAML or another authoring format by the caller.
type Definition struct {
	Type          CheckType
	Kind          string
	Namespace     string
	Name          string
	Selector      string
	Field         string
	Value         string
	MinReady      int
	ControlPlanes int
	Workers       int
}

// NewCheck builds a typed check from a portable definition.
func NewCheck(def Definition) (Check, error) {
	switch def.Type {
	case CheckObjectExists:
		return ObjectExists{Kind: def.Kind, Namespace: def.Namespace, Name: def.Name}, validateGenericRef(def.Kind, def.Namespace, def.Name)
	case CheckFieldEquals:
		if err := validateGenericRef(def.Kind, def.Namespace, def.Name); err != nil {
			return nil, err
		}
		if strings.TrimSpace(def.Field) == "" {
			return nil, errors.New("validator: field is required")
		}
		return FieldEquals{Kind: def.Kind, Namespace: def.Namespace, Name: def.Name, Field: def.Field, Value: def.Value}, nil
	case CheckDeploymentExists:
		return DeploymentExists{Namespace: def.Namespace, Name: def.Name}, validateObjectRef(def.Namespace, def.Name)
	case CheckDeploymentAvailable:
		return DeploymentAvailable{Namespace: def.Namespace, Name: def.Name, RequiredReplicas: def.MinReady}, validateObjectRef(def.Namespace, def.Name)
	case CheckPodReady:
		if err := validateSelector(def.Namespace, def.Selector); err != nil {
			return nil, err
		}
		return PodReady{Namespace: def.Namespace, Selector: def.Selector, MinReady: def.MinReady}, nil
	case CheckContainersHealthy:
		if err := validateSelector(def.Namespace, def.Selector); err != nil {
			return nil, err
		}
		return ContainersHealthy{Namespace: def.Namespace, Selector: def.Selector}, nil
	case CheckNodeTopology:
		if def.MinReady < 1 || def.ControlPlanes < 1 || def.Workers < 0 ||
			def.ControlPlanes+def.Workers != def.MinReady {
			return nil, errors.New("validator: node topology counts are inconsistent")
		}
		return NodeTopology{
			Count: def.MinReady, ControlPlanes: def.ControlPlanes, Workers: def.Workers,
		}, nil
	default:
		return nil, fmt.Errorf("validator: unsupported check type %q", def.Type)
	}
}

// NodeTopology checks the real cluster's node count, roles, and Ready state.
type NodeTopology struct {
	Count         int
	ControlPlanes int
	Workers       int
}

func (c NodeTopology) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if c.Count < 1 || c.ControlPlanes < 1 || c.Workers < 0 ||
		c.ControlPlanes+c.Workers != c.Count {
		return invalidResult(errors.New("validator: node topology counts are inconsistent"))
	}
	output, err := runner.Run(ctx, "get", "nodes", "--output=json")
	if err != nil {
		return commandFailure("read Nodes", err)
	}
	var nodes nodeListJSON
	if err := decodeJSON(output, &nodes); err != nil {
		return malformedResult("Node list", err)
	}
	if nodes.Items == nil {
		return malformedResult("Node list", errors.New("items field is missing or null"))
	}
	controlPlanes := 0
	workers := 0
	ready := 0
	for _, node := range nodes.Items {
		if _, isControlPlane := node.Metadata.Labels["node-role.kubernetes.io/control-plane"]; isControlPlane {
			controlPlanes++
		} else {
			workers++
		}
		if conditionTrue(node.Status.Conditions, "Ready") {
			ready++
		}
	}
	if len(nodes.Items) != c.Count || controlPlanes != c.ControlPlanes || workers != c.Workers {
		return Result{
			Status: Wrong,
			Message: fmt.Sprintf("cluster has %d nodes (%d control-plane, %d workers); expected %d (%d control-plane, %d workers)",
				len(nodes.Items), controlPlanes, workers, c.Count, c.ControlPlanes, c.Workers),
		}, nil
	}
	if ready != c.Count {
		return Result{
			Status:  Converging,
			Message: fmt.Sprintf("%d/%d nodes are Ready", ready, c.Count),
		}, nil
	}
	return Result{
		Status:  Success,
		Message: fmt.Sprintf("%d Ready nodes detected: %d control-plane and %d workers", ready, controlPlanes, workers),
	}, nil
}

// ObjectExists checks for a named namespaced Kubernetes object.
type ObjectExists struct {
	Kind      string
	Namespace string
	Name      string
}

func (c ObjectExists) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateGenericRef(c.Kind, c.Namespace, c.Name); err != nil {
		return invalidResult(err)
	}
	output, err := runner.Run(ctx, objectGetArgs(c.Kind, c.Namespace, c.Name)...)
	if err != nil {
		if isNotFound(output, err) {
			return Result{Status: Wrong, Message: fmt.Sprintf("%s %s does not exist", c.Kind, objectRef(c.Namespace, c.Name))}, nil
		}
		return commandFailure("read "+c.Kind, err)
	}
	var object map[string]any
	if err := decodeJSON(output, &object); err != nil {
		return malformedResult(c.Kind, err)
	}
	return Result{Status: Success, Message: fmt.Sprintf("%s %s exists", c.Kind, objectRef(c.Namespace, c.Name))}, nil
}

// FieldEquals checks a dotted field path on a named namespaced object.
type FieldEquals struct {
	Kind      string
	Namespace string
	Name      string
	Field     string
	Value     string
}

func (c FieldEquals) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateGenericRef(c.Kind, c.Namespace, c.Name); err != nil {
		return invalidResult(err)
	}
	if strings.TrimSpace(c.Field) == "" {
		return invalidResult(errors.New("validator: field is required"))
	}
	output, err := runner.Run(ctx, objectGetArgs(c.Kind, c.Namespace, c.Name)...)
	if err != nil {
		if isNotFound(output, err) {
			return Result{Status: Wrong, Message: fmt.Sprintf("%s %s does not exist", c.Kind, objectRef(c.Namespace, c.Name))}, nil
		}
		return commandFailure("read "+c.Kind, err)
	}
	var object map[string]any
	if err := decodeJSON(output, &object); err != nil {
		return malformedResult(c.Kind, err)
	}
	actual, ok := nestedField(object, strings.Split(c.Field, "."))
	if !ok {
		return Result{Status: Wrong, Message: fmt.Sprintf("%s %s has no field %s", c.Kind, objectRef(c.Namespace, c.Name), c.Field)}, nil
	}
	if fmt.Sprint(actual) != c.Value {
		return Result{
			Status:  Wrong,
			Message: fmt.Sprintf("%s %s field %s is %q, expected %q", c.Kind, objectRef(c.Namespace, c.Name), c.Field, fmt.Sprint(actual), c.Value),
		}, nil
	}
	return Result{
		Status:  Success,
		Message: fmt.Sprintf("%s %s field %s is %q", c.Kind, objectRef(c.Namespace, c.Name), c.Field, c.Value),
	}, nil
}

func nestedField(object map[string]any, path []string) (any, bool) {
	var current any = object
	for _, segment := range path {
		name, indexes, ok := parsePathSegment(segment)
		if !ok {
			return nil, false
		}
		fields, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = fields[name]
		if !ok {
			return nil, false
		}
		for _, index := range indexes {
			list, ok := current.([]any)
			if !ok || index >= len(list) {
				return nil, false
			}
			current = list[index]
		}
	}
	return current, true
}

func parsePathSegment(segment string) (string, []int, bool) {
	start := strings.IndexByte(segment, '[')
	if start == -1 {
		if segment == "" || strings.ContainsAny(segment, "]") {
			return "", nil, false
		}
		return segment, nil, true
	}
	name := segment[:start]
	if name == "" {
		return "", nil, false
	}
	rest := segment[start:]
	var indexes []int
	for len(rest) > 0 {
		if rest[0] != '[' {
			return "", nil, false
		}
		close := strings.IndexByte(rest, ']')
		if close < 2 {
			return "", nil, false
		}
		index, err := strconv.Atoi(rest[1:close])
		if err != nil || index < 0 {
			return "", nil, false
		}
		indexes = append(indexes, index)
		rest = rest[close+1:]
	}
	return name, indexes, true
}

// DeploymentExists checks for a named Deployment.
type DeploymentExists struct {
	Namespace string
	Name      string
}

func (c DeploymentExists) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateObjectRef(c.Namespace, c.Name); err != nil {
		return invalidResult(err)
	}
	var deployment deploymentJSON
	output, err := runner.Run(ctx, "get", "deployment", c.Name, "--namespace", c.Namespace, "--output=json")
	if err != nil {
		if isNotFound(output, err) {
			return Result{Status: Wrong, Message: fmt.Sprintf("Deployment %s/%s does not exist", c.Namespace, c.Name)}, nil
		}
		return commandFailure("read Deployment", err)
	}
	if err := decodeJSON(output, &deployment); err != nil {
		return malformedResult("Deployment", err)
	}
	if deployment.Metadata.Name == "" {
		return malformedResult("Deployment", errors.New("metadata.name is missing"))
	}
	if deployment.Metadata.Name != c.Name {
		return malformedResult("Deployment", fmt.Errorf("metadata.name is %q, expected %q", deployment.Metadata.Name, c.Name))
	}
	return Result{Status: Success, Message: fmt.Sprintf("Deployment %s/%s exists", c.Namespace, c.Name)}, nil
}

// DeploymentAvailable checks that a named Deployment exists, has observed its
// current generation, and has all desired replicas available.
type DeploymentAvailable struct {
	Namespace        string
	Name             string
	RequiredReplicas int
}

func (c DeploymentAvailable) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateObjectRef(c.Namespace, c.Name); err != nil {
		return invalidResult(err)
	}
	var deployment deploymentJSON
	output, err := runner.Run(ctx, "get", "deployment", c.Name, "--namespace", c.Namespace, "--output=json")
	if err != nil {
		if isNotFound(output, err) {
			return Result{Status: Wrong, Message: fmt.Sprintf("Deployment %s/%s does not exist", c.Namespace, c.Name)}, nil
		}
		return commandFailure("read Deployment", err)
	}
	if err := decodeJSON(output, &deployment); err != nil {
		return malformedResult("Deployment", err)
	}
	if deployment.Metadata.Name == "" {
		return malformedResult("Deployment", errors.New("metadata.name is missing"))
	}
	if deployment.Metadata.Name != c.Name {
		return malformedResult("Deployment", fmt.Errorf("metadata.name is %q, expected %q", deployment.Metadata.Name, c.Name))
	}

	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	target := desired
	if c.RequiredReplicas > 0 {
		target = int32(c.RequiredReplicas)
		if desired != target {
			return Result{
				Status: Wrong,
				Message: fmt.Sprintf("Deployment %s/%s requests %d replicas, expected %d",
					c.Namespace, c.Name, desired, target),
			}, nil
		}
	}
	if deployment.Metadata.Generation > 0 && deployment.Status.ObservedGeneration < deployment.Metadata.Generation {
		return Result{
			Status: Converging,
			Message: fmt.Sprintf("Deployment %s/%s has not observed generation %d yet",
				c.Namespace, c.Name, deployment.Metadata.Generation),
		}, nil
	}
	if deployment.Status.AvailableReplicas < target {
		return Result{
			Status: Converging,
			Message: fmt.Sprintf("Deployment %s/%s has %d/%d available replicas",
				c.Namespace, c.Name, deployment.Status.AvailableReplicas, target),
		}, nil
	}
	return Result{
		Status:  Success,
		Message: fmt.Sprintf("Deployment %s/%s has %d/%d available replicas", c.Namespace, c.Name, deployment.Status.AvailableReplicas, target),
	}, nil
}

// PodReady checks that at least MinReady selected Pods are Ready. MinReady
// defaults to one. A failed Pod is wrong; absent or unready Pods are converging.
type PodReady struct {
	Namespace string
	Selector  string
	MinReady  int
}

func (c PodReady) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateSelector(c.Namespace, c.Selector); err != nil {
		return invalidResult(err)
	}
	minReady := c.MinReady
	if minReady == 0 {
		minReady = 1
	}
	if minReady < 0 {
		return invalidResult(errors.New("validator: minimum ready Pods cannot be negative"))
	}

	pods, result, err := getPods(ctx, runner, c.Namespace, c.Selector)
	if err != nil || result.Status == Wrong {
		return result, err
	}
	if len(pods.Items) == 0 {
		return Result{Status: Converging, Message: fmt.Sprintf("no Pods match %q in namespace %s", c.Selector, c.Namespace)}, nil
	}

	ready := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" {
			return Result{Status: Wrong, Message: fmt.Sprintf("Pod %s/%s is Failed", c.Namespace, pod.Metadata.Name)}, nil
		}
		if conditionTrue(pod.Status.Conditions, "Ready") {
			ready++
		}
	}
	if ready < minReady {
		return Result{
			Status:  Converging,
			Message: fmt.Sprintf("%d/%d required Pods matching %q are Ready in namespace %s", ready, minReady, c.Selector, c.Namespace),
		}, nil
	}
	return Result{
		Status:  Success,
		Message: fmt.Sprintf("%d Pods matching %q are Ready in namespace %s", ready, c.Selector, c.Namespace),
	}, nil
}

// ContainersHealthy checks current regular container states for selected Pods.
// Known failure waits and non-zero terminations are wrong; transient waits and
// successful terminations are converging.
type ContainersHealthy struct {
	Namespace string
	Selector  string
}

func (c ContainersHealthy) Evaluate(ctx context.Context, runner Runner) (Result, error) {
	if err := validateSelector(c.Namespace, c.Selector); err != nil {
		return invalidResult(err)
	}
	pods, result, err := getPods(ctx, runner, c.Namespace, c.Selector)
	if err != nil || result.Status == Wrong {
		return result, err
	}
	if len(pods.Items) == 0 {
		return Result{Status: Converging, Message: fmt.Sprintf("no Pods match %q in namespace %s", c.Selector, c.Namespace)}, nil
	}

	var convergingMessage string
	for _, pod := range pods.Items {
		if len(pod.Status.ContainerStatuses) == 0 {
			if convergingMessage == "" {
				convergingMessage = fmt.Sprintf("Pod %s/%s has no reported container state yet", c.Namespace, pod.Metadata.Name)
			}
			continue
		}
		for _, container := range pod.Status.ContainerStatuses {
			state := container.State
			switch {
			case state.Waiting != nil:
				message := fmt.Sprintf("container %s in Pod %s/%s is waiting: %s", container.Name, c.Namespace, pod.Metadata.Name, state.Waiting.Reason)
				if failingWaitReason(state.Waiting.Reason) {
					return Result{Status: Wrong, Message: message}, nil
				}
				if convergingMessage == "" {
					convergingMessage = message
				}
			case state.Terminated != nil:
				message := fmt.Sprintf("container %s in Pod %s/%s is terminated with exit code %d", container.Name, c.Namespace, pod.Metadata.Name, state.Terminated.ExitCode)
				if state.Terminated.ExitCode != 0 {
					return Result{Status: Wrong, Message: message}, nil
				}
				if convergingMessage == "" {
					convergingMessage = message
				}
			case state.Running == nil:
				if convergingMessage == "" {
					convergingMessage = fmt.Sprintf("container %s in Pod %s/%s has no current state", container.Name, c.Namespace, pod.Metadata.Name)
				}
			}
		}
	}
	if convergingMessage != "" {
		return Result{Status: Converging, Message: convergingMessage}, nil
	}
	return Result{Status: Success, Message: fmt.Sprintf("all containers matching %q are running in namespace %s", c.Selector, c.Namespace)}, nil
}

type deploymentJSON struct {
	Metadata struct {
		Generation int64  `json:"generation"`
		Name       string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int32 `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration int64 `json:"observedGeneration"`
		AvailableReplicas  int32 `json:"availableReplicas"`
	} `json:"status"`
}

type podListJSON struct {
	Items []podJSON `json:"items"`
}

type nodeListJSON struct {
	Items []nodeJSON `json:"items"`
}

type nodeJSON struct {
	Metadata struct {
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions []conditionJSON `json:"conditions"`
	} `json:"status"`
}

type podJSON struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Phase             string            `json:"phase"`
		Conditions        []conditionJSON   `json:"conditions"`
		ContainerStatuses []containerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type conditionJSON struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type containerStatus struct {
	Name  string             `json:"name"`
	State containerStateJSON `json:"state"`
}

type containerStateJSON struct {
	Running *struct{} `json:"running"`
	Waiting *struct {
		Reason string `json:"reason"`
	} `json:"waiting"`
	Terminated *struct {
		ExitCode int32 `json:"exitCode"`
	} `json:"terminated"`
}

func getPods(ctx context.Context, runner Runner, namespace, selector string) (podListJSON, Result, error) {
	var pods podListJSON
	output, err := runner.Run(ctx, "get", "pods", "--namespace", namespace, "--selector", selector, "--output=json")
	if err != nil {
		result, wrapped := commandFailure("read Pods", err)
		return pods, result, wrapped
	}
	if err := decodeJSON(output, &pods); err != nil {
		result, wrapped := malformedResult("Pod list", err)
		return pods, result, wrapped
	}
	if pods.Items == nil {
		result, wrapped := malformedResult("Pod list", errors.New("items field is missing or null"))
		return pods, result, wrapped
	}
	return pods, Result{Status: Success}, nil
}

func decodeJSON(output []byte, target any) error {
	if len(strings.TrimSpace(string(output))) == 0 {
		return errors.New("empty JSON output")
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func conditionTrue(conditions []conditionJSON, conditionType string) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == "True"
		}
	}
	return false
}

func failingWaitReason(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "CreateContainerConfigError", "CreateContainerError",
		"ErrImagePull", "ImagePullBackOff", "InvalidImageName", "RunContainerError":
		return true
	default:
		return false
	}
}

func isNotFound(output []byte, err error) bool {
	text := strings.ToLower(string(output) + " " + err.Error())
	return strings.Contains(text, "notfound") || strings.Contains(text, "not found")
}

func validateObjectRef(namespace, name string) error {
	if strings.TrimSpace(namespace) == "" {
		return errors.New("validator: namespace is required")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("validator: object name is required")
	}
	return nil
}

func validateGenericRef(kind, namespace, name string) error {
	if strings.TrimSpace(kind) == "" {
		return errors.New("validator: object kind is required")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("validator: object name is required")
	}
	return nil
}

func objectGetArgs(kind, namespace, name string) []string {
	args := []string{"get", kind, name}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	return append(args, "--output=json")
}

func objectRef(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

func validateSelector(namespace, selector string) error {
	if strings.TrimSpace(namespace) == "" {
		return errors.New("validator: namespace is required")
	}
	if strings.TrimSpace(selector) == "" {
		return errors.New("validator: selector is required")
	}
	return nil
}

func invalidResult(err error) (Result, error) {
	return Result{Status: Wrong, Message: err.Error()}, err
}

func malformedResult(kind string, err error) (Result, error) {
	wrapped := fmt.Errorf("validator: malformed %s output: %w", kind, err)
	return Result{Status: Wrong, Message: wrapped.Error()}, wrapped
}

func commandFailure(action string, err error) (Result, error) {
	wrapped := fmt.Errorf("validator: cannot %s: %w", action, err)
	return Result{Status: Wrong, Message: wrapped.Error()}, wrapped
}
