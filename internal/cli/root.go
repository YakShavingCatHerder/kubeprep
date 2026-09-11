package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YakShavingCatHerder/kubeprep/internal/cluster"
	"github.com/YakShavingCatHerder/kubeprep/internal/curriculum"
	"github.com/YakShavingCatHerder/kubeprep/internal/game"
	"github.com/YakShavingCatHerder/kubeprep/internal/terminal"
	"github.com/YakShavingCatHerder/kubeprep/internal/validator"
	"github.com/spf13/cobra"
)

// Version is the reported CLI version. Release builds override it with ldflags.
var Version = "dev"

type app struct {
	in          io.Reader
	out         io.Writer
	err         io.Writer
	livePackDir string
	targetLabID string
	preview     bool
}

func Execute() error {
	a := &app{in: os.Stdin, out: os.Stdout, err: os.Stderr}
	return a.rootCommand().ExecuteContext(context.Background())
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "kubeprep",
		Short:         "Run Kubernetes certification training scenarios",
		Version:       Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("{{printf \"kubeprep %s\\n\" .Version}}")
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.err)
	root.AddCommand(
		a.doctorCommand(),
		a.startCommand(),
		a.labCommand(),
		a.statusCommand(),
		a.resetCommand(),
		a.destroyCommand(),
	)
	root.CompletionOptions.HiddenDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Help about any command",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, _, err := cmd.Root().Find(args)
			if err != nil {
				return err
			}
			return target.Help()
		},
	})
	root.SetUsageTemplate(learnerUsageTemplate)
	return root
}

// learnerUsageTemplate is Cobra's default usage text without the special-case
// that always lists a command named "help". Root usage is a single line
// (`kubeprep [command] [flags]`) instead of separate flag and command lines.
const learnerUsageTemplate = `Usage:{{if and .Runnable .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{if .HasAvailableFlags}} [flags]{{end}}{{else if .Runnable}}
  {{.UseLine}}{{else if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) .IsAvailableCommand)}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") .IsAvailableCommand)}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func (a *app) registry() (*curriculum.Registry, error) {
	if a.livePackDir != "" {
		return curriculum.NewRegistryFromDirectory(a.livePackDir)
	}
	return curriculum.NewRegistry()
}

func (a *app) clusterManager() (*cluster.Manager, error) {
	manager, err := cluster.NewManager(cluster.ExecRunner{})
	if err != nil {
		return nil, err
	}
	if err := cluster.RequireTools(manager.Paths()); err != nil {
		return nil, err
	}
	return manager, nil
}

func (a *app) output() io.Writer {
	if a.out != nil {
		return a.out
	}
	return os.Stdout
}

func (a *app) doctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Install pinned kind and kubectl, then check prerequisites",
		RunE: func(cmd *cobra.Command, _ []string) error {
			doctor, err := cluster.NewDoctor(cluster.ExecRunner{})
			if err != nil {
				return err
			}
			if err := waitFor(cmd.OutOrStdout(), "checking pinned kind and kubectl", func() error {
				return cluster.EnsureTools(cmd.Context(), doctor.Paths, cluster.DefaultToolOptions())
			}); err != nil {
				return err
			}
			if writeDoctorReport(cmd.OutOrStdout(), doctor.Check(cmd.Context())) {
				return errors.New("one or more prerequisites are unavailable")
			}
			return nil
		},
	}
}

func (a *app) startCommand() *cobra.Command {
	var track string
	var prepareOnly bool
	command := &cobra.Command{
		Use:   "start",
		Short: "Create the training cluster if needed and continue the current lab",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runStart(cmd, track, prepareOnly)
		},
	}
	command.Flags().StringVar(&track, "track", "", "learning track: beginner, cka, or ckad")
	command.Flags().BoolVar(&prepareOnly, "prepare-only", false, "prepare the current lab without starting the TUI")
	return command
}

func (a *app) labCommand() *cobra.Command {
	lab := &cobra.Command{
		Use:   "lab",
		Short: "Try, publish, or validate labs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	lab.AddCommand(a.labTryCommand(), a.labPublishCommand(), a.labValidateCommand())
	return lab
}

func (a *app) labTryCommand() *cobra.Command {
	var track string
	var prepareOnly bool
	command := &cobra.Command{
		Use:   "try <file>",
		Short: "Validate a contribute/ lab YAML and start it without publishing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := resolveContributeLab(args[0])
			if err != nil {
				return err
			}
			cleanup, err := a.prepareTry(source)
			if err != nil {
				return err
			}
			defer cleanup()
			fmt.Fprintf(cmd.OutOrStdout(), "validated %s (%s)\n", source, a.targetLabID)
			return a.runStart(cmd, track, prepareOnly)
		},
	}
	command.Flags().StringVar(&track, "track", "", "learning track: beginner, cka, or ckad")
	command.Flags().BoolVar(&prepareOnly, "prepare-only", false, "prepare the current lab without starting the TUI")
	return command
}

func (a *app) labPublishCommand() *cobra.Command {
	var track string
	var prepareOnly bool
	command := &cobra.Command{
		Use:   "publish <file>",
		Short: "Install a contribute/ lab YAML into ./curriculum and start it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := resolveContributeLab(args[0])
			if err != nil {
				return err
			}
			if err := a.useLiveCurriculum(); err != nil {
				return err
			}
			install, err := curriculum.InstallLab(source, a.livePackDir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "published %s -> %s\n", source, install.Path)
			a.targetLabID = install.ID
			return a.runStart(cmd, track, prepareOnly)
		},
	}
	command.Flags().StringVar(&track, "track", "", "learning track: beginner, cka, or ckad")
	command.Flags().BoolVar(&prepareOnly, "prepare-only", false, "prepare the current lab without starting the TUI")
	return command
}

func (a *app) labValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [directory]",
		Short: "Validate labs in a curriculum directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := liveCurriculumDir
			if len(args) == 1 {
				dir = args[0]
			}
			catalog, err := curriculum.ValidateLabs(dir)
			if err != nil {
				return err
			}
			count := len(catalog.ScenarioIDs())
			noun := "lab"
			if count != 1 {
				noun = "labs"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "validated %s (%d %s)\n", catalog.Name, count, noun)
			return nil
		},
	}
}

const liveCurriculumDir = "curriculum"
const contributeDir = "contribute"

func resolveContributeLab(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("lab: file is required")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("lab: pass a filename inside contribute/, not an absolute path")
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	clean = strings.TrimPrefix(clean, "./")
	if clean == contributeDir {
		return "", fmt.Errorf("lab: pass a filename inside contribute/")
	}
	if prefix := contributeDir + "/"; strings.HasPrefix(clean, prefix) {
		clean = strings.TrimPrefix(clean, prefix)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("lab: file must stay inside contribute/")
	}
	if strings.TrimSpace(clean) == "" {
		return "", fmt.Errorf("lab: pass a filename inside contribute/")
	}
	return filepath.Join(contributeDir, filepath.FromSlash(clean)), nil
}

func (a *app) prepareTry(source string) (func(), error) {
	packDir, err := os.MkdirTemp("", "kubeprep-try-")
	if err != nil {
		return nil, fmt.Errorf("lab try: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(packDir) }
	install, err := curriculum.MaterializeDraft(source, packDir)
	if err != nil {
		cleanup()
		return nil, err
	}
	a.livePackDir = packDir
	a.targetLabID = install.ID
	a.preview = true
	return cleanup, nil
}

func (a *app) useLiveCurriculum() error {
	info, err := os.Stat(liveCurriculumDir)
	if err != nil {
		return fmt.Errorf("lab publish: %s not found; run this from the repository root", liveCurriculumDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("lab publish: %s is not a directory", liveCurriculumDir)
	}
	a.livePackDir = liveCurriculumDir
	return nil
}

func (a *app) runStart(cmd *cobra.Command, track string, prepareOnly bool) error {
	store, err := game.NewStore()
	if err != nil {
		return err
	}
	if a.preview {
		progress, err := store.LoadProgress()
		if err != nil {
			return err
		}
		previous := progress.CurrentScenarioID
		defer func() { _ = store.SelectScenario(previous) }()
	}
	if _, err := a.ensureProfile(cmd, store, track); err != nil {
		return err
	}
	manager, err := a.clusterManager()
	if err != nil {
		return err
	}
	if err := waitFor(cmd.OutOrStdout(), "Preparing training cluster", func() error {
		_, err := manager.EnsureCluster(cmd.Context())
		return err
	}); err != nil {
		return err
	}
	registry, err := a.registry()
	if err != nil {
		return err
	}
	if a.targetLabID != "" {
		if err := store.SelectScenario(a.targetLabID); err != nil {
			return err
		}
	}
	scenario, err := resolveCurrentScenario(store, registry, !a.preview)
	if err != nil {
		return err
	}
	if err := waitFor(cmd.OutOrStdout(), "Preparing "+scenario.Title, func() error {
		return a.enterScenario(cmd.Context(), scenario, registry, manager, store, wipeIfNewLab(store, scenario.ID))
	}); err != nil {
		return err
	}
	if prepareOnly {
		fmt.Fprintf(cmd.OutOrStdout(), "KubePrep cluster verified and %s prepared.\n", scenario.Title)
		return nil
	}
	return a.runTrainingSession(cmd.Context(), scenario, manager, store)
}

func (a *app) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show learner and cluster status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := game.NewStore()
			if err != nil {
				return err
			}
			profile, err := store.LoadProfile()
			if err != nil {
				return err
			}
			progress, err := store.LoadProgress()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Track: %s\n", profile.Experience)
			registry, err := a.registry()
			if err != nil {
				return err
			}
			active, err := currentScenario(store, registry)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current scenario: %s (%s)\n", active.Title, active.ID)
			pathID := pathIDForExperience(profile.Experience)
			for _, catalog := range registry.Catalogs() {
				fmt.Fprintf(cmd.OutOrStdout(), "Pack: %s (%s)\n", catalog.Title, catalog.Name)
				learningPath := catalog.PathByID(pathID)
				if learningPath == nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", learningPath.Title)
				for _, section := range learningPath.Sections {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", section.ID)
					for _, id := range section.Labs {
						scenario, loadErr := registry.LoadScenario(id)
						if loadErr != nil {
							return loadErr
						}
						scenarioProgress := progress.Scenarios[id]
						if scenarioProgress.CompletedAt == nil {
							fmt.Fprintf(cmd.OutOrStdout(), "    %s: incomplete (hints %v)\n", scenario.Title, scenarioProgress.HintsUsed)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "    %s: completed %s (hints %v)\n",
								scenario.Title, scenarioProgress.CompletedAt.Format(time.RFC3339), scenarioProgress.HintsUsed)
						}
					}
				}
			}
			manager, managerErr := a.clusterManager()
			if managerErr == nil {
				_, managerErr = manager.VerifyOwnership(cmd.Context())
			}
			if managerErr != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Cluster: unavailable or unverified")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Cluster: verified")
			}
			return nil
		},
	}
}

func (a *app) resetCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "reset",
		Short: "Reset the current lab's progress and starting cluster state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := game.NewStore()
			if err != nil {
				return err
			}
			registry, err := a.registry()
			if err != nil {
				return err
			}
			scenario, err := currentScenario(store, registry)
			if err != nil {
				return err
			}
			if err := a.confirm(cmd, yes, fmt.Sprintf("Reset %s to its starting state?", scenario.Title)); err != nil {
				return err
			}
			manager, err := a.clusterManager()
			if err != nil {
				return err
			}
			if _, err := manager.VerifyOwnership(cmd.Context()); err != nil {
				return fmt.Errorf("run kubeprep start before resetting a lab: %w", err)
			}
			if err := waitFor(cmd.OutOrStdout(), "Resetting "+scenario.Title, func() error {
				return restoreLabCluster(cmd.Context(), scenario, registry, manager)
			}); err != nil {
				return err
			}
			if err := store.ResetScenarioProgress(scenario.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s reset. Run kubeprep start to continue.\n", scenario.Title)
			return nil
		},
	}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return command
}

func (a *app) destroyCommand() *cobra.Command {
	var yes bool
	var allFlag bool
	command := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the training cluster",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if allFlag {
				return errors.New("use `kubeprep destroy all` to destroy the cluster and clear learner progress")
			}
			return a.runDestroy(cmd, yes, false)
		},
	}
	command.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	command.Flags().BoolVar(&allFlag, "all", false, "")
	_ = command.Flags().MarkHidden("all")
	command.TraverseChildren = true

	command.AddCommand(&cobra.Command{
		Use:   "all",
		Short: "Destroy the training cluster and clear learner progress",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runDestroy(cmd, yes, true)
		},
	})
	return command
}

func (a *app) runDestroy(cmd *cobra.Command, yes, clearProgress bool) error {
	prompt := "Destroy the KubePrep cluster?"
	if clearProgress {
		prompt = "Destroy the KubePrep cluster and clear all learner progress?"
	}
	if err := a.confirm(cmd, yes, prompt); err != nil {
		return err
	}
	manager, err := a.clusterManager()
	if err != nil {
		return err
	}
	if err := waitFor(cmd.OutOrStdout(), "Destroying training cluster", func() error {
		return manager.Destroy(cmd.Context())
	}); err != nil {
		return err
	}
	if clearProgress {
		store, err := game.NewStore()
		if err != nil {
			return err
		}
		if err := store.ClearLearnerState(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Cluster destroyed. Learner progress was cleared.")
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Cluster destroyed. Learner progress was retained.")
	return nil
}

func (a *app) ensureProfile(cmd *cobra.Command, store *game.Store, requestedTrack string) (game.Profile, error) {
	profile, err := store.LoadProfile()
	if err != nil {
		return game.Profile{}, err
	}
	if profile.OnboardingComplete && requestedTrack == "" {
		return profile, nil
	}

	track := strings.ToLower(strings.TrimSpace(requestedTrack))
	if track == "" {
		if !readerIsTerminal(cmd.InOrStdin()) {
			return game.Profile{}, errors.New("first non-interactive start requires --track=beginner, --track=cka, or --track=ckad")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Which track are you following?")
		fmt.Fprintln(cmd.OutOrStdout(), "  1) Beginner\n  2) CKA\n  3) CKAD")
		fmt.Fprint(cmd.OutOrStdout(), "Choose 1-3: ")
		line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return game.Profile{}, fmt.Errorf("read track: %w", readErr)
		}
		switch strings.TrimSpace(line) {
		case "1":
			track = "beginner"
		case "2":
			track = "cka"
		case "3":
			track = "ckad"
		default:
			return game.Profile{}, errors.New("track must be 1, 2, or 3")
		}
	}
	switch track {
	case "beginner":
		profile.Experience = game.ExperienceBeginner
	case "cka":
		profile.Experience = game.ExperienceCKACandidate
	case "ckad":
		profile.Experience = game.ExperienceCKADCandidate
	default:
		return game.Profile{}, errors.New("--track must be beginner, cka, or ckad")
	}
	if err := profile.Experience.Validate(); err != nil {
		return game.Profile{}, err
	}
	profile.OnboardingComplete = true
	return profile, store.SaveProfile(profile)
}

func (a *app) runTrainingSession(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager, store *game.Store) error {
	registry, err := a.registry()
	if err != nil {
		return err
	}
	for {
		result, err := a.runScenario(ctx, scenario, manager, store)
		if err != nil {
			return err
		}
		if !result.Continue {
			return nil
		}
		next, err := nextIncompleteScenario(store, registry)
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		if err := waitFor(a.output(), "Preparing "+next.Title, func() error {
			return a.enterScenario(ctx, next, registry, manager, store, true)
		}); err != nil {
			return err
		}
		scenario = next
	}
}

func (a *app) runScenario(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager, store *game.Store) (terminal.SessionResult, error) {
	profile, err := store.LoadProfile()
	if err != nil {
		return terminal.SessionResult{}, err
	}
	observeDelay, err := curriculum.ParseObserveDelay(scenario.ObserveDelay)
	if err != nil {
		return terminal.SessionResult{}, err
	}
	registry, err := a.registry()
	if err != nil {
		return terminal.SessionResult{}, err
	}
	next, err := followingIncompleteScenario(store, registry, scenario.ID)
	if err != nil {
		return terminal.SessionResult{}, err
	}
	nextTitle := ""
	if next != nil {
		nextTitle = next.Title
	}
	return terminal.RunScenarioView(ctx, terminal.ScenarioView{
		ScenarioID:          scenario.ID,
		Title:               scenario.Title,
		Module:              scenario.Module,
		Description:         strings.TrimSpace(scenario.Description),
		Objective:           strings.TrimSpace(scenario.Objective),
		Namespace:           scenario.Namespace,
		Experience:          string(profile.Experience),
		Hints:               scenario.Hints,
		Completion:          strings.TrimSpace(scenario.Completion),
		Debrief:             strings.TrimSpace(scenario.Debrief.Explanation),
		Ungraded:            scenario.Ungraded,
		Kubeconfig:          manager.Paths().Kubeconfig,
		ToolBinDir:          manager.Paths().BinDir(),
		ObserveWhileRunning: observeDelay > 0 && !scenario.Ungraded,
		ObserveDelay:        observeDelay,
		HasNext:             next != nil,
		NextTitle:           nextTitle,
		Check: func(checkContext context.Context) (terminal.CheckState, string, error) {
			timeout, cancel := context.WithTimeout(checkContext, 8*time.Second)
			defer cancel()
			result, err := evaluateScenario(timeout, scenario, manager)
			return terminal.CheckState(result.Status.String()), result.Message, err
		},
		UseHint:  func(level int) error { return store.RecordHint(scenario.ID, level) },
		Complete: func() error { return store.CompleteScenario(scenario.ID) },
		Shell:    terminal.NewShellRunner(),
	})
}

func currentScenario(store *game.Store, registry *curriculum.Registry) (*curriculum.Scenario, error) {
	return resolveCurrentScenario(store, registry, true)
}

func resolveCurrentScenario(store *game.Store, registry *curriculum.Registry, retain bool) (*curriculum.Scenario, error) {
	profile, err := store.LoadProfile()
	if err != nil {
		return nil, err
	}
	if retain {
		if err := store.RetainScenarios(registry.ScenarioIDs()); err != nil {
			return nil, fmt.Errorf("sanitize scenario progress: %w", err)
		}
	}
	progress, err := store.LoadProgress()
	if err != nil {
		return nil, err
	}
	if progress.CurrentScenarioID != "" {
		return registry.LoadScenario(progress.CurrentScenarioID)
	}
	ids := registry.PlayOrder(pathIDForExperience(profile.Experience))
	if len(ids) == 0 {
		return nil, fmt.Errorf("active scenario packs contain no scenarios for track %q", profile.Experience)
	}
	var last *curriculum.Scenario
	for _, id := range ids {
		scenario, loadErr := registry.LoadScenario(id)
		if loadErr != nil {
			return nil, loadErr
		}
		last = scenario
		if progress.Scenarios[id].CompletedAt == nil {
			return scenario, nil
		}
	}
	return last, nil
}

func nextIncompleteScenario(store *game.Store, registry *curriculum.Registry) (*curriculum.Scenario, error) {
	return followingIncompleteScenario(store, registry, "")
}

func followingIncompleteScenario(store *game.Store, registry *curriculum.Registry, afterID string) (*curriculum.Scenario, error) {
	profile, err := store.LoadProfile()
	if err != nil {
		return nil, err
	}
	progress, err := store.LoadProgress()
	if err != nil {
		return nil, err
	}
	seenCurrent := afterID == ""
	for _, id := range registry.PlayOrder(pathIDForExperience(profile.Experience)) {
		scenario, loadErr := registry.LoadScenario(id)
		if loadErr != nil {
			return nil, loadErr
		}
		if !seenCurrent {
			if id == afterID {
				seenCurrent = true
			}
			continue
		}
		if progress.Scenarios[id].CompletedAt == nil {
			return scenario, nil
		}
	}
	return nil, nil
}

func pathIDForExperience(experience game.Experience) string {
	return string(experience)
}

func (a *app) enterScenario(ctx context.Context, scenario *curriculum.Scenario, registry *curriculum.Registry, manager *cluster.Manager, store *game.Store, wipe bool) error {
	if err := prepareScenario(ctx, scenario, registry, manager, wipe); err != nil {
		return err
	}
	if err := pinLabWorkspace(ctx, scenario, manager); err != nil {
		return err
	}
	return store.SelectScenario(scenario.ID)
}

func pinLabWorkspace(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager) error {
	return manager.SetContextNamespace(ctx, contextNamespaceForLab(scenario))
}

func contextNamespaceForLab(scenario *curriculum.Scenario) string {
	namespace := strings.TrimSpace(scenario.Namespace)
	if namespace == "" {
		return "default"
	}
	return namespace
}

func wipeIfNewLab(store *game.Store, scenarioID string) bool {
	progress, err := store.LoadProgress()
	if err != nil {
		return true
	}
	return shouldWipeLabWorkspace(progress.CurrentScenarioID, scenarioID)
}

func shouldWipeLabWorkspace(currentScenarioID, enteringScenarioID string) bool {
	return currentScenarioID != enteringScenarioID
}

func wipeLabWorkspace(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager) error {
	namespace := strings.TrimSpace(scenario.Namespace)
	if namespace == "" {
		return nil
	}
	return manager.WipeWorkspace(ctx, namespace)
}

func restoreLabCluster(ctx context.Context, scenario *curriculum.Scenario, registry *curriculum.Registry, manager *cluster.Manager) error {
	if err := wipeLabWorkspace(ctx, scenario, manager); err != nil {
		return err
	}
	manifest, err := scenarioResources(registry, scenario, scenario.Reset)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(manifest))) > 0 {
		if strings.TrimSpace(scenario.Namespace) != "" {
			if err := manager.Apply(ctx, manifest); err != nil {
				return err
			}
		} else if err := manager.Reset(ctx, manifest); err != nil {
			return err
		}
	}
	return pinLabWorkspace(ctx, scenario, manager)
}

func prepareScenario(ctx context.Context, scenario *curriculum.Scenario, registry *curriculum.Registry, manager *cluster.Manager, wipe bool) error {
	for _, capability := range scenario.Requires {
		if capability != "single-node" && capability != "multi-node" {
			return fmt.Errorf("prepare %s: cluster profile does not provide capability %q", scenario.Title, capability)
		}
	}
	if !scenario.Ungraded && len(scenario.Checks) == 0 {
		return fmt.Errorf("prepare %s: scenario has no checks", scenario.Title)
	}
	if wipe {
		if err := wipeLabWorkspace(ctx, scenario, manager); err != nil {
			return fmt.Errorf("prepare %s: %w", scenario.Title, err)
		}
	}
	manifest, err := scenarioResources(registry, scenario, scenario.Setup)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(manifest))) == 0 {
		return nil
	}
	if kind, namespace, name := setupObjectProbe(scenario); !wipe && kind != "" && namespace != "" && name != "" {
		result, err := (validator.ObjectExists{Kind: kind, Namespace: namespace, Name: name}).
			Evaluate(ctx, validator.KubectlRunner{Kubeconfig: manager.Paths().Kubeconfig, Executable: manager.Paths().KubectlExecutable()})
		if err != nil {
			return fmt.Errorf("inspect %s setup: %w", scenario.Title, err)
		}
		if result.Status == validator.Success {
			return nil
		}
	}
	if err := manager.Apply(ctx, manifest); err != nil {
		return fmt.Errorf("prepare %s: %w", scenario.Title, err)
	}
	return nil
}

func setupObjectProbe(scenario *curriculum.Scenario) (kind, namespace, name string) {
	if scenario == nil || len(scenario.Checks) == 0 {
		return "", "", ""
	}
	first := scenario.Checks[0]
	kind = first.Kind
	if kind == "" && first.Type == curriculum.CheckDeploymentAvailable {
		kind = "deployment"
	}
	return kind, first.Namespace, first.Name
}

func scenarioResources(registry *curriculum.Registry, scenario *curriculum.Scenario, resources curriculum.ResourceSet) ([]byte, error) {
	return scenario.ComposeResources(resources, func(reference string) ([]byte, error) {
		return registry.ReadManifest(scenario.ID, reference)
	})
}

func evaluateScenario(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager) (validator.Result, error) {
	if scenario.Ungraded {
		return validator.Result{Status: validator.Success, Message: "This lab does not grade cluster state."}, nil
	}
	checks := make([]validator.Check, 0, len(scenario.Checks))
	for index, authored := range scenario.Checks {
		var definition validator.Definition
		definition.Kind = authored.Kind
		definition.Namespace = authored.Namespace
		definition.Name = authored.Name
		definition.Selector = authored.Selector
		definition.Field = authored.Field
		definition.Value = authored.Value
		if authored.Count != nil {
			definition.MinReady = *authored.Count
		}
		if authored.Replicas != nil {
			definition.MinReady = *authored.Replicas
		}
		if authored.ControlPlanes != nil {
			definition.ControlPlanes = *authored.ControlPlanes
		}
		if authored.Workers != nil {
			definition.Workers = *authored.Workers
		}
		switch authored.Type {
		case curriculum.CheckObjectExists:
			definition.Type = validator.CheckObjectExists
		case curriculum.CheckFieldEquals:
			definition.Type = validator.CheckFieldEquals
		case curriculum.CheckDeploymentAvailable:
			definition.Type = validator.CheckDeploymentAvailable
		case curriculum.CheckPodReady:
			definition.Type = validator.CheckPodReady
		case curriculum.CheckContainersHealthy:
			definition.Type = validator.CheckContainersHealthy
		case curriculum.CheckNodeTopology:
			definition.Type = validator.CheckNodeTopology
		default:
			return validator.Result{}, fmt.Errorf("scenario %s check %d has unsupported type %q", scenario.ID, index+1, authored.Type)
		}
		check, err := validator.NewCheck(definition)
		if err != nil {
			return validator.Result{}, err
		}
		checks = append(checks, check)
	}
	return validator.All(checks...).Evaluate(ctx, validator.KubectlRunner{
		Kubeconfig: manager.Paths().Kubeconfig,
		Executable: manager.Paths().KubectlExecutable(),
	})
}

func (a *app) confirm(cmd *cobra.Command, yes bool, prompt string) error {
	if yes {
		return nil
	}
	if !readerIsTerminal(cmd.InOrStdin()) {
		return errors.New("destructive non-interactive operation requires -y")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", prompt)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return errors.New("operation cancelled")
	}
	return nil
}
