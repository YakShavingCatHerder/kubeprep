package game

import (
	"fmt"
	"time"
)

const stateVersion = 1

// Experience identifies the learner's selected learning or certification route.
type Experience string

const (
	ExperienceBeginner      Experience = "beginner"
	ExperienceCKACandidate  Experience = "cka"
	ExperienceCKADCandidate Experience = "ckad"
)

// Validate reports whether the experience is supported.
func (e Experience) Validate() error {
	switch e {
	case ExperienceBeginner, ExperienceCKACandidate, ExperienceCKADCandidate:
		return nil
	default:
		return fmt.Errorf("invalid experience %q: must be beginner, cka, or ckad", e)
	}
}

// Profile contains learner onboarding state.
type Profile struct {
	Version            int        `json:"version"`
	Experience         Experience `json:"experience"`
	OnboardingComplete bool       `json:"onboardingComplete"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// Validate checks that a profile can be persisted.
func (p Profile) Validate() error {
	if p.Version != 0 && p.Version != stateVersion {
		return fmt.Errorf("unsupported profile version %d", p.Version)
	}
	if err := p.Experience.Validate(); err != nil {
		return err
	}
	return nil
}

// ScenarioProgress records state-derived completion and requested hints.
// It deliberately contains no command or shell history.
type ScenarioProgress struct {
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	HintsUsed   []int      `json:"hintsUsed,omitempty"`
}

// Progress contains learner progress indexed by scenario ID.
type Progress struct {
	Version           int                         `json:"version"`
	CurrentScenarioID string                      `json:"currentScenarioId,omitempty"`
	Scenarios         map[string]ScenarioProgress `json:"scenarios"`
	UpdatedAt         time.Time                   `json:"updatedAt"`
}

// Validate checks that progress is structurally valid.
func (p Progress) Validate() error {
	if p.Version != 0 && p.Version != stateVersion {
		return fmt.Errorf("unsupported progress version %d", p.Version)
	}
	if p.CurrentScenarioID != "" {
		for _, character := range p.CurrentScenarioID {
			if character != '-' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
				return fmt.Errorf("current scenario ID %q must be lowercase kebab-case", p.CurrentScenarioID)
			}
		}
	}
	for scenarioID, scenario := range p.Scenarios {
		if scenarioID == "" {
			return fmt.Errorf("scenario ID must not be empty")
		}
		seen := make(map[int]struct{}, len(scenario.HintsUsed))
		for _, hint := range scenario.HintsUsed {
			if hint < 1 {
				return fmt.Errorf("scenario %q has invalid hint number %d", scenarioID, hint)
			}
			if _, ok := seen[hint]; ok {
				return fmt.Errorf("scenario %q contains duplicate hint number %d", scenarioID, hint)
			}
			seen[hint] = struct{}{}
		}
	}
	return nil
}
