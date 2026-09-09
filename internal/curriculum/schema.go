package curriculum

const (
	APIVersionV1Alpha1 = "kubeprep.io/v1alpha1"
)

// Scenario is a versioned, declarative Kubernetes training scenario.
type Scenario struct {
	APIVersion string `yaml:"apiVersion"`
	ID         string `yaml:"id"`
	Mode       string `yaml:"mode"`
	// ObserveDelay is an optional Go duration (for example "60s"). When set,
	// the training view waits this long before the first automatic check.
	ObserveDelay string                  `yaml:"observeDelay,omitempty"`
	Title        string                  `yaml:"title"`
	Description  string                  `yaml:"description"`
	Revision     string                  `yaml:"revision"`
	Module       string                  `yaml:"module"`
	Difficulty   int                     `yaml:"difficulty"`
	Kubernetes   KubernetesCompatibility `yaml:"kubernetes"`
	Requires     []string                `yaml:"requires,omitempty"`
	Tracks       []string                `yaml:"tracks"`
	Objective    string                  `yaml:"objective"`
	Concepts     []string                `yaml:"concepts"`
	Namespace    string                  `yaml:"namespace,omitempty"`
	// StartingClusterConfiguration is inline Kubernetes YAML applied on setup
	// and reset. Empty means the declared namespace is the only start state.
	StartingClusterConfiguration string      `yaml:"startingClusterConfiguration,omitempty"`
	Setup                        ResourceSet `yaml:"setup"`
	// Ungraded labs have no target cluster state. F2 records completion.
	Ungraded   bool        `yaml:"ungraded,omitempty"`
	Checks     []Check     `yaml:"checks"`
	Hints      []string    `yaml:"hints"`
	Completion string      `yaml:"completion"`
	Debrief    Debrief     `yaml:"debrief"`
	Reset      ResourceSet `yaml:"reset"`
}

// KubernetesCompatibility declares the Kubernetes versions for which a scenario
// is authored. Versions are major.minor strings (for example, "1.35").
type KubernetesCompatibility struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

// ResourceSet contains references to Kubernetes manifests. It intentionally
// cannot express host commands.
type ResourceSet struct {
	Manifests []string `yaml:"manifests"`
}

type CheckType string

const (
	CheckObjectExists        CheckType = "objectExists"
	CheckFieldEquals         CheckType = "fieldEquals"
	CheckPodReady            CheckType = "podReady"
	CheckDeploymentAvailable CheckType = "deploymentAvailable"
	CheckContainersHealthy   CheckType = "containersHealthy"
	CheckNodeTopology        CheckType = "nodeTopology"
)

// Check is a typed state observation. Optional fields are interpreted by the
// validator implementing Type; curriculum loading never executes commands.
type Check struct {
	Type          CheckType `yaml:"type"`
	Kind          string    `yaml:"kind,omitempty"`
	Namespace     string    `yaml:"namespace,omitempty"`
	Name          string    `yaml:"name,omitempty"`
	Selector      string    `yaml:"selector,omitempty"`
	Container     string    `yaml:"container,omitempty"`
	Field         string    `yaml:"field,omitempty"`
	Value         string    `yaml:"value,omitempty"`
	Condition     string    `yaml:"condition,omitempty"`
	Status        string    `yaml:"status,omitempty"`
	Text          string    `yaml:"text,omitempty"`
	Path          string    `yaml:"path,omitempty"`
	URL           string    `yaml:"url,omitempty"`
	Verb          string    `yaml:"verb,omitempty"`
	Resource      string    `yaml:"resource,omitempty"`
	Replicas      *int      `yaml:"replicas,omitempty"`
	Count         *int      `yaml:"count,omitempty"`
	ControlPlanes *int      `yaml:"controlPlanes,omitempty"`
	Workers       *int      `yaml:"workers,omitempty"`
	Code          *int      `yaml:"code,omitempty"`
}

type Debrief struct {
	Explanation string   `yaml:"explanation"`
	Commands    []string `yaml:"commands"`
	Concepts    []string `yaml:"concepts"`
	Domain      string   `yaml:"domain"`
}

// Document is one scenario file: optional author notes plus the loadable
// Scenario. Validate and the runner use Scenario only.
type Document struct {
	Authoring Authoring `yaml:"authoring,omitempty"`
	Scenario  `yaml:",inline"`
}

// Authoring is human/agent notes. It is not graded and is not applied.
type Authoring struct {
	Section                     string   `yaml:"section,omitempty"`
	Guidance                    string   `yaml:"guidance,omitempty"`
	Intent                      Intent   `yaml:"intent,omitempty"`
	DesiredClusterConfiguration string   `yaml:"desiredClusterConfiguration,omitempty"`
	Accept                      []string `yaml:"accept,omitempty"`
	Reject                      []string `yaml:"reject,omitempty"`
	ProveAlternate              string   `yaml:"proveAlternate,omitempty"`
}

// Intent records why the lab exists. It is not shown in the learner UI.
type Intent struct {
	Teach       string `yaml:"teach,omitempty"`
	WrongIdea   string `yaml:"wrongIdea,omitempty"`
	MentalModel string `yaml:"mentalModel,omitempty"`
	Why         string `yaml:"why,omitempty"`
	Notes       string `yaml:"notes,omitempty"`
}
