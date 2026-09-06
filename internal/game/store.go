package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	profileFileName  = "profile.json"
	progressFileName = "progress.json"
)

var stateMu sync.Mutex

// StoreOption customizes a Store.
type StoreOption func(*storeOptions)

type storeOptions struct {
	configDir string
	now       func() time.Time
}

// WithConfigDir stores state directly in dir. It is primarily useful for
// isolated tests and portable installations.
func WithConfigDir(dir string) StoreOption {
	return func(options *storeOptions) {
		options.configDir = dir
	}
}

// WithClock supplies the current time used by persistence operations.
func WithClock(now func() time.Time) StoreOption {
	return func(options *storeOptions) {
		options.now = now
	}
}

// Store persists the learner profile and progress.
type Store struct {
	dir string
	now func() time.Time
}

// NewStore creates a learner state store. By default, files live in the
// operating system's user config directory under "kubecrypt".
func NewStore(options ...StoreOption) (*Store, error) {
	settings := storeOptions{
		now: time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(&settings)
		}
	}

	if settings.configDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("locate user config directory: %w", err)
		}
		settings.configDir = filepath.Join(configDir, "kubecrypt")
	}
	if settings.now == nil {
		return nil, errors.New("create learner state store: clock must not be nil")
	}

	return &Store{
		dir: filepath.Clean(settings.configDir),
		now: settings.now,
	}, nil
}

// Dir returns the directory containing learner state.
func (s *Store) Dir() string {
	return s.dir
}

// LoadProfile loads the learner profile. A missing file returns a beginner
// profile with onboarding incomplete.
func (s *Store) LoadProfile() (Profile, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	var profile Profile
	err := readJSON(filepath.Join(s.dir, profileFileName), &profile)
	if errors.Is(err, os.ErrNotExist) {
		return Profile{
			Version:    stateVersion,
			Experience: ExperienceBeginner,
		}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	if err := profile.Validate(); err != nil {
		return Profile{}, fmt.Errorf("load profile %q: %w", filepath.Join(s.dir, profileFileName), err)
	}
	return profile, nil
}

// SaveProfile validates and atomically persists the learner profile.
func (s *Store) SaveProfile(profile Profile) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	if err := profile.Validate(); err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	now := s.now().UTC()
	profile.Version = stateVersion
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now

	if err := writeJSONAtomic(s.dir, profileFileName, profile); err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

// LoadProgress loads scenario progress. A missing file returns empty progress.
func (s *Store) LoadProgress() (Progress, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	return s.loadProgress()
}

func (s *Store) loadProgress() (Progress, error) {
	var persisted struct {
		Version           int                         `json:"version"`
		CurrentScenarioID string                      `json:"currentScenarioId,omitempty"`
		Scenarios         map[string]ScenarioProgress `json:"scenarios"`
		LegacyCurrentID   string                      `json:"currentEncounterId,omitempty"`
		LegacyScenarios   map[string]ScenarioProgress `json:"encounters,omitempty"`
		UpdatedAt         time.Time                   `json:"updatedAt"`
	}
	path := filepath.Join(s.dir, progressFileName)
	err := readJSON(path, &persisted)
	if errors.Is(err, os.ErrNotExist) {
		return Progress{
			Version:   stateVersion,
			Scenarios: make(map[string]ScenarioProgress),
		}, nil
	}
	if err != nil {
		return Progress{}, err
	}
	progress := Progress{
		Version:           persisted.Version,
		CurrentScenarioID: persisted.CurrentScenarioID,
		Scenarios:         persisted.Scenarios,
		UpdatedAt:         persisted.UpdatedAt,
	}
	// Preserve the profile but start progress cleanly when the previous schema
	// is encountered.
	if persisted.LegacyScenarios != nil || persisted.LegacyCurrentID != "" {
		progress.CurrentScenarioID = ""
		progress.Scenarios = make(map[string]ScenarioProgress)
	}
	if err := progress.Validate(); err != nil {
		return Progress{}, fmt.Errorf("load progress %q: %w", path, err)
	}
	if progress.Scenarios == nil {
		progress.Scenarios = make(map[string]ScenarioProgress)
	}
	return progress, nil
}

// SaveProgress validates and atomically persists all scenario progress.
func (s *Store) SaveProgress(progress Progress) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	return s.saveProgress(progress)
}

func (s *Store) saveProgress(progress Progress) error {
	if progress.Scenarios == nil {
		progress.Scenarios = make(map[string]ScenarioProgress)
	}
	if err := progress.Validate(); err != nil {
		return fmt.Errorf("save progress: %w", err)
	}
	progress.Version = stateVersion
	progress.UpdatedAt = s.now().UTC()

	if err := writeJSONAtomic(s.dir, progressFileName, progress); err != nil {
		return fmt.Errorf("save progress: %w", err)
	}
	return nil
}

// RecordHint records a requested one-based hint number once for a scenario.
func (s *Store) RecordHint(scenarioID string, hint int) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	if scenarioID == "" {
		return errors.New("record hint: scenario ID must not be empty")
	}
	if hint < 1 {
		return fmt.Errorf("record hint: hint number must be at least 1, got %d", hint)
	}

	progress, err := s.loadProgress()
	if err != nil {
		return err
	}
	scenario := progress.Scenarios[scenarioID]
	for _, used := range scenario.HintsUsed {
		if used == hint {
			return nil
		}
	}
	scenario.HintsUsed = append(scenario.HintsUsed, hint)
	progress.Scenarios[scenarioID] = scenario
	return s.saveProgress(progress)
}

// ResetScenarioProgress clears hints and completion for one scenario and
// makes it the current lab. Other labs are left alone.
func (s *Store) ResetScenarioProgress(scenarioID string) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	if scenarioID == "" {
		return errors.New("reset scenario progress: scenario ID must not be empty")
	}
	progress, err := s.loadProgress()
	if err != nil {
		return err
	}
	progress.Scenarios[scenarioID] = ScenarioProgress{}
	progress.CurrentScenarioID = scenarioID
	return s.saveProgress(progress)
}

// SelectScenario records which authored scenario should be resumed.
func (s *Store) SelectScenario(scenarioID string) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	progress, err := s.loadProgress()
	if err != nil {
		return err
	}
	progress.CurrentScenarioID = scenarioID
	return s.saveProgress(progress)
}

// RetainScenarios removes progress for scenarios that are not present in the
// active registry. This keeps removed local packs from blocking resume.
func (s *Store) RetainScenarios(validIDs []string) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	valid := make(map[string]struct{}, len(validIDs))
	for _, id := range validIDs {
		valid[id] = struct{}{}
	}
	progress, err := s.loadProgress()
	if err != nil {
		return err
	}
	changed := false
	for id := range progress.Scenarios {
		if _, ok := valid[id]; !ok {
			delete(progress.Scenarios, id)
			changed = true
		}
	}
	if progress.CurrentScenarioID != "" {
		if _, ok := valid[progress.CurrentScenarioID]; !ok {
			progress.CurrentScenarioID = ""
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveProgress(progress)
}

// ClearLearnerState removes the learner profile and scenario progress while
// leaving cluster identity and kubeconfig files untouched.
func (s *Store) ClearLearnerState() error {
	stateMu.Lock()
	defer stateMu.Unlock()

	for _, name := range []string{profileFileName, progressFileName} {
		path := filepath.Join(s.dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clear learner state %q: %w", path, err)
		}
	}
	return nil
}

// CompleteScenario records completion at the injected current time. Existing
// completion timestamps are retained so repeated validation is idempotent.
func (s *Store) CompleteScenario(scenarioID string) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	if scenarioID == "" {
		return errors.New("complete scenario: scenario ID must not be empty")
	}

	progress, err := s.loadProgress()
	if err != nil {
		return err
	}
	scenario := progress.Scenarios[scenarioID]
	if scenario.CompletedAt == nil {
		completedAt := s.now().UTC()
		scenario.CompletedAt = &completedAt
		progress.Scenarios[scenarioID] = scenario
	}
	if progress.CurrentScenarioID == scenarioID {
		progress.CurrentScenarioID = ""
	}
	return s.saveProgress(progress)
}

func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read learner state %q: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode learner state %q: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fmt.Errorf("decode learner state %q: trailing content: %w", path, err)
	}
	return nil
}

func writeJSONAtomic(dir, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode learner state: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create learner state directory %q: %w", dir, err)
	}
	if err := ownerOnly(dir, 0o700); err != nil {
		return fmt.Errorf("secure learner state directory %q: %w", dir, err)
	}

	temp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary learner state file: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		_ = temp.Close()
		return fmt.Errorf("secure temporary learner state file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary learner state file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary learner state file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary learner state file: %w", err)
	}

	target := filepath.Join(dir, name)
	if err := os.Rename(tempPath, target); err != nil {
		return fmt.Errorf("replace learner state file %q: %w", target, err)
	}
	removeTemp = false
	if err := ownerOnly(target, 0o600); err != nil {
		return fmt.Errorf("secure learner state file %q: %w", target, err)
	}

	if runtime.GOOS != "windows" {
		directory, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("open learner state directory for sync: %w", err)
		}
		syncErr := directory.Sync()
		closeErr := directory.Close()
		if syncErr != nil {
			return fmt.Errorf("sync learner state directory: %w", syncErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close learner state directory: %w", closeErr)
		}
	}
	return nil
}

func ownerOnly(path string, mode os.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, mode)
}
