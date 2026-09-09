package curriculum

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	idPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	revisionPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	versionPattern  = regexp.MustCompile(`^(\d+)\.(\d+)$`)
)

var allowedCheckTypes = map[CheckType]struct{}{
	CheckObjectExists: {}, CheckFieldEquals: {}, CheckPodReady: {},
	CheckDeploymentAvailable: {}, CheckContainersHealthy: {}, CheckNodeTopology: {},
}

// Validate checks a scenario independently of where its manifest files live.
// LoadFile and LoadFS additionally verify that every referenced manifest exists.
func Validate(scenario *Scenario) error {
	if scenario == nil {
		return fmt.Errorf("scenario is nil")
	}
	if scenario.APIVersion != APIVersionV1Alpha1 {
		return fmt.Errorf("apiVersion: unsupported value %q (want %q)", scenario.APIVersion, APIVersionV1Alpha1)
	}
	if !idPattern.MatchString(scenario.ID) {
		return fmt.Errorf("id: %q must be lowercase kebab-case", scenario.ID)
	}
	if scenario.Mode != "challenge" {
		return fmt.Errorf("mode: unsupported value %q (want challenge)", scenario.Mode)
	}
	if _, err := ParseObserveDelay(scenario.ObserveDelay); err != nil {
		return err
	}
	if strings.TrimSpace(scenario.Title) == "" {
		return fmt.Errorf("title: must not be empty")
	}
	if strings.TrimSpace(scenario.Description) == "" {
		return fmt.Errorf("description: must not be empty")
	}
	if !revisionPattern.MatchString(scenario.Revision) {
		return fmt.Errorf("revision: %q must use YYYY-MM format", scenario.Revision)
	}
	if !idPattern.MatchString(scenario.Module) {
		return fmt.Errorf("module: %q must be lowercase kebab-case", scenario.Module)
	}
	if scenario.Difficulty < 1 || scenario.Difficulty > 5 {
		return fmt.Errorf("difficulty: %d is outside the supported range 1..5", scenario.Difficulty)
	}
	if err := validateCompatibility(scenario.Kubernetes); err != nil {
		return err
	}
	if strings.TrimSpace(scenario.Objective) == "" {
		return fmt.Errorf("objective: must not be empty")
	}
	if len(scenario.Concepts) == 0 {
		return fmt.Errorf("concepts: must contain at least one concept")
	}
	for i, concept := range scenario.Concepts {
		if strings.TrimSpace(concept) == "" {
			return fmt.Errorf("concepts[%d]: must not be empty", i)
		}
	}
	for i, capability := range scenario.Requires {
		if !idPattern.MatchString(capability) {
			return fmt.Errorf("requires[%d]: %q must be lowercase kebab-case", i, capability)
		}
	}
	if len(scenario.Tracks) == 0 {
		return fmt.Errorf("tracks: must contain at least one track")
	}
	seenTracks := make(map[string]struct{}, len(scenario.Tracks))
	for i, track := range scenario.Tracks {
		switch track {
		case "beginner", "cka", "ckad":
		default:
			return fmt.Errorf("tracks[%d]: unsupported track %q", i, track)
		}
		if _, exists := seenTracks[track]; exists {
			return fmt.Errorf("tracks[%d]: duplicate track %q", i, track)
		}
		seenTracks[track] = struct{}{}
	}
	if err := validateNamespaceName(scenario.Namespace); err != nil {
		return err
	}
	requireManifests := strings.TrimSpace(scenario.Namespace) == ""
	if err := validateManifestRefs("setup.manifests", scenario.Setup.Manifests, requireManifests); err != nil {
		return err
	}
	if scenario.Ungraded {
		if len(scenario.Checks) != 0 {
			return fmt.Errorf("checks: must be empty when ungraded is true")
		}
	} else if len(scenario.Checks) == 0 {
		return fmt.Errorf("checks: must contain at least one check")
	}
	for i, check := range scenario.Checks {
		if _, ok := allowedCheckTypes[check.Type]; !ok {
			return fmt.Errorf("checks[%d].type: unsupported check %q", i, check.Type)
		}
		switch check.Type {
		case CheckPodReady, CheckContainersHealthy:
			if strings.TrimSpace(check.Namespace) == "" {
				return fmt.Errorf("checks[%d]: namespace must not be empty", i)
			}
			if strings.TrimSpace(check.Selector) == "" {
				return fmt.Errorf("checks[%d]: selector must not be empty (%s matches a label selector, not name)", i, check.Type)
			}
		case CheckNodeTopology:
			if check.Count == nil || check.ControlPlanes == nil || check.Workers == nil {
				return fmt.Errorf("checks[%d]: count, controlPlanes, and workers must be set", i)
			}
			if *check.Count < 1 || *check.ControlPlanes < 1 || *check.Workers < 0 ||
				*check.ControlPlanes+*check.Workers != *check.Count {
				return fmt.Errorf("checks[%d]: node topology counts are inconsistent", i)
			}
		case CheckObjectExists, CheckFieldEquals:
			if strings.TrimSpace(check.Kind) == "" || strings.TrimSpace(check.Name) == "" {
				return fmt.Errorf("checks[%d]: kind and name must not be empty", i)
			}
			if strings.TrimSpace(check.Namespace) == "" {
				return fmt.Errorf("checks[%d]: namespace must not be empty", i)
			}
			if check.Type == CheckFieldEquals && strings.TrimSpace(check.Field) == "" {
				return fmt.Errorf("checks[%d].field: must not be empty", i)
			}
			if check.Type == CheckFieldEquals && check.Value == "" {
				return fmt.Errorf("checks[%d].value: must not be empty", i)
			}
		case CheckDeploymentAvailable:
			if strings.TrimSpace(check.Namespace) == "" || strings.TrimSpace(check.Name) == "" {
				return fmt.Errorf("checks[%d]: namespace and name must not be empty", i)
			}
		}
	}
	if len(scenario.Hints) != 3 {
		return fmt.Errorf("hints: must contain exactly 3 hints (got %d)", len(scenario.Hints))
	}
	for i, hint := range scenario.Hints {
		if strings.TrimSpace(hint) == "" {
			return fmt.Errorf("hints[%d]: must not be empty", i)
		}
	}
	if strings.TrimSpace(scenario.Completion) == "" {
		return fmt.Errorf("completion: must not be empty")
	}
	if strings.TrimSpace(scenario.Debrief.Explanation) == "" {
		return fmt.Errorf("debrief.explanation: must not be empty")
	}
	if err := validateManifestRefs("reset.manifests", scenario.Reset.Manifests, requireManifests); err != nil {
		return err
	}
	return nil
}

func validateNamespaceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if !strings.HasPrefix(name, "kubeprep-") || !idPattern.MatchString(name) {
		return fmt.Errorf("namespace: %q must be a kubeprep-* name", name)
	}
	return nil
}

func validateCompatibility(compat KubernetesCompatibility) error {
	min, err := parseVersion(compat.Min)
	if err != nil {
		return fmt.Errorf("kubernetes.min: %w", err)
	}
	max, err := parseVersion(compat.Max)
	if err != nil {
		return fmt.Errorf("kubernetes.max: %w", err)
	}
	if min[0] > max[0] || min[0] == max[0] && min[1] > max[1] {
		return fmt.Errorf("kubernetes: min %q is newer than max %q", compat.Min, compat.Max)
	}
	return nil
}

func parseVersion(value string) ([2]int, error) {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return [2]int{}, fmt.Errorf("%q must use major.minor format", value)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return [2]int{major, minor}, nil
}

func validateManifestRefs(field string, refs []string, required bool) error {
	if required && len(refs) == 0 {
		return fmt.Errorf("%s: must contain at least one manifest", field)
	}
	for i, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("%s[%d]: must not be empty", field, i)
		}
		if !fs.ValidPath(ref) || path.Clean(ref) != ref || strings.HasPrefix(ref, "/") {
			return fmt.Errorf("%s[%d]: %q must be a clean relative path", field, i, ref)
		}
		if ext := path.Ext(ref); ext != ".yaml" && ext != ".yml" {
			return fmt.Errorf("%s[%d]: %q must reference a YAML manifest", field, i, ref)
		}
	}
	return nil
}

// ParseObserveDelay parses an optional Go duration such as "60s".
func ParseObserveDelay(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	delay, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("observeDelay: %q is not a valid duration", raw)
	}
	if delay < 0 {
		return 0, fmt.Errorf("observeDelay: must not be negative")
	}
	if delay > 10*time.Minute {
		return 0, fmt.Errorf("observeDelay: %s exceeds 10m", delay)
	}
	return delay, nil
}
