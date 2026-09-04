package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YakShavingCatHerder/kubecrypt/internal/game"
)

func TestExperienceModes(t *testing.T) {
	t.Parallel()

	valid := []game.Experience{
		game.ExperienceBeginner,
		game.ExperienceCKACandidate,
		game.ExperienceCKADCandidate,
	}
	for _, experience := range valid {
		if err := experience.Validate(); err != nil {
			t.Errorf("%q should be valid: %v", experience, err)
		}
	}
	if err := game.Experience("expert").Validate(); err == nil {
		t.Fatal("unsupported experience should be rejected")
	}
}

func TestMissingStateReturnsDefaults(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	profile, err := store.LoadProfile()
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}
	if profile.Experience != game.ExperienceBeginner || profile.OnboardingComplete {
		t.Fatalf("LoadProfile() = %#v, want incomplete beginner", profile)
	}

	progress, err := store.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	if progress.Scenarios == nil || len(progress.Scenarios) != 0 {
		t.Fatalf("LoadProgress().Scenarios = %#v, want empty map", progress.Scenarios)
	}
}

func TestProfileRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 3, 19, 54, 0, 0, time.FixedZone("EDT", -4*60*60))
	store := newTestStore(t, now)
	want := game.Profile{
		Experience:         game.ExperienceCKADCandidate,
		OnboardingComplete: true,
	}
	if err := store.SaveProfile(want); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	got, err := store.LoadProfile()
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}
	if got.Experience != want.Experience || !got.OnboardingComplete {
		t.Fatalf("LoadProfile() = %#v", got)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("profile timestamps = %v, %v; want %v", got.CreatedAt, got.UpdatedAt, now.UTC())
	}
	assertOwnerOnly(t, store.Dir(), filepath.Join(store.Dir(), "profile.json"))
}

func TestProgressRoundTripAndMutations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	store := newTestStore(t, now)
	if err := store.RecordHint("cluster-components", 1); err != nil {
		t.Fatalf("RecordHint() error = %v", err)
	}
	if err := store.RecordHint("cluster-components", 1); err != nil {
		t.Fatalf("duplicate RecordHint() error = %v", err)
	}
	if err := store.RecordHint("cluster-components", 3); err != nil {
		t.Fatalf("RecordHint() error = %v", err)
	}
	if err := store.CompleteScenario("cluster-components"); err != nil {
		t.Fatalf("CompleteScenario() error = %v", err)
	}

	got, err := store.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress() error = %v", err)
	}
	scenario := got.Scenarios["cluster-components"]
	if scenario.CompletedAt == nil || !scenario.CompletedAt.Equal(now) {
		t.Fatalf("CompletedAt = %v, want %v", scenario.CompletedAt, now)
	}
	if len(scenario.HintsUsed) != 2 || scenario.HintsUsed[0] != 1 || scenario.HintsUsed[1] != 3 {
		t.Fatalf("HintsUsed = %v, want [1 3]", scenario.HintsUsed)
	}
	assertOwnerOnly(t, store.Dir(), filepath.Join(store.Dir(), "progress.json"))
}

func TestRetainScenariosRemovesUnavailableProgress(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	if err := store.RecordHint("installed-scenario", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordHint("removed-scenario", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.SelectScenario("removed-scenario"); err != nil {
		t.Fatal(err)
	}
	if err := store.RetainScenarios([]string{"installed-scenario"}); err != nil {
		t.Fatal(err)
	}
	progress, err := store.LoadProgress()
	if err != nil {
		t.Fatal(err)
	}
	if progress.CurrentScenarioID != "" {
		t.Fatalf("current scenario = %q, want empty", progress.CurrentScenarioID)
	}
	if _, exists := progress.Scenarios["removed-scenario"]; exists {
		t.Fatal("removed scenario progress was retained")
	}
	if _, exists := progress.Scenarios["installed-scenario"]; !exists {
		t.Fatal("active scenario progress was removed")
	}
}

func TestLegacyProgressStartsWithCleanScenarioState(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	if err := os.MkdirAll(store.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"version":1,"currentEncounterId":"old-scenario","encounters":{"old-scenario":{"hintsUsed":[1]}}}`
	if err := os.WriteFile(filepath.Join(store.Dir(), "progress.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	progress, err := store.LoadProgress()
	if err != nil {
		t.Fatal(err)
	}
	if progress.CurrentScenarioID != "" || len(progress.Scenarios) != 0 {
		t.Fatalf("legacy progress was not cleared: %#v", progress)
	}
}

func TestClearLearnerStateRemovesProfileAndProgress(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	if err := store.SaveProfile(game.Profile{
		Experience:         game.ExperienceCKADCandidate,
		OnboardingComplete: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordHint("shell-orientation", 1); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(store.Dir(), "cluster-ownership.json")
	if err := os.WriteFile(unrelated, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.ClearLearnerState(); err != nil {
		t.Fatalf("ClearLearnerState() error = %v", err)
	}
	profile, err := store.LoadProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.OnboardingComplete {
		t.Fatal("profile remained after clear")
	}
	progress, err := store.LoadProgress()
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.Scenarios) != 0 {
		t.Fatalf("progress remained after clear: %#v", progress.Scenarios)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated cluster state was removed: %v", err)
	}
	if err := store.ClearLearnerState(); err != nil {
		t.Fatalf("idempotent ClearLearnerState() error = %v", err)
	}
}

func TestInvalidExperienceAndCorruptFiles(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	if err := store.SaveProfile(game.Profile{Experience: game.Experience("expert")}); err == nil {
		t.Fatal("SaveProfile() accepted invalid experience")
	}

	if err := os.MkdirAll(store.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(store.Dir(), "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"version":1,"experience":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadProfile(); err == nil || !strings.Contains(err.Error(), profilePath) {
		t.Fatalf("LoadProfile() error = %v, want clear path-bearing corruption error", err)
	}

	progressPath := filepath.Join(store.Dir(), "progress.json")
	if err := os.WriteFile(progressPath, []byte(`{"version":1,"scenarios":{"cluster-components":{"hintsUsed":[0]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadProgress(); err == nil || !strings.Contains(err.Error(), "invalid hint") {
		t.Fatalf("LoadProgress() error = %v, want validation error", err)
	}
}

func TestAtomicWritesNeverExposePartialJSON(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, time.Now())
	if err := store.SaveProfile(game.Profile{Experience: game.ExperienceBeginner}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Dir(), "profile.json")

	var readers sync.WaitGroup
	stop := make(chan struct{})
	readErr := make(chan error, 1)
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
				data, err := os.ReadFile(path)
				if err != nil {
					select {
					case readErr <- err:
					default:
					}
					return
				}
				var profile game.Profile
				if err := json.Unmarshal(data, &profile); err != nil {
					select {
					case readErr <- err:
					default:
					}
					return
				}
			}
		}
	}()

	for i := 0; i < 50; i++ {
		experience := game.ExperienceBeginner
		if i%2 == 0 {
			experience = game.ExperienceCKACandidate
		}
		if err := store.SaveProfile(game.Profile{Experience: experience}); err != nil {
			close(stop)
			readers.Wait()
			t.Fatalf("SaveProfile() error = %v", err)
		}
	}
	close(stop)
	readers.Wait()
	select {
	case err := <-readErr:
		t.Fatalf("reader observed partial state: %v", err)
	default:
	}

	matches, err := filepath.Glob(filepath.Join(store.Dir(), ".profile.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after successful writes: %v", matches)
	}
}

func newTestStore(t *testing.T, now time.Time) *game.Store {
	t.Helper()

	store, err := game.NewStore(
		game.WithConfigDir(t.TempDir()),
		game.WithClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func assertOwnerOnly(t *testing.T, dir, file string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %o, want 600", got)
	}
}
