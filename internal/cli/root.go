package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/YakShavingCatHerder/kubecrypt/internal/cluster"
	"github.com/YakShavingCatHerder/kubecrypt/internal/curriculum"
	"github.com/YakShavingCatHerder/kubecrypt/internal/game"
	"github.com/YakShavingCatHerder/kubecrypt/internal/terminal"
	"github.com/YakShavingCatHerder/kubecrypt/internal/validator"
	"github.com/spf13/cobra"
)

// Version is the reported CLI version. Release builds override it with ldflags.
var Version = "dev"

type app struct {
	in       io.Reader
	out      io.Writer
	err      io.Writer
	packDirs []string
}

func Execute() error {
	a := &app{in: os.Stdin, out: os.Stdout, err: os.Stderr, packDirs: packDirectoriesFromEnvironment()}
	return a.rootCommand().ExecuteContext(context.Background())
}

func packDirectoriesFromEnvironment() []string {
	value := strings.TrimSpace(os.Getenv("KUBECRYPT_PACKS"))
	if value == "" {
		return nil
	}
	return filepath.SplitList(value)
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "kubecrypt",
		Short:         "Run Kubernetes certification training scenarios",
		Version:       Version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("{{printf \"kubecrypt %s\\n\" .Version}}")
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.err)
	root.AddCommand(
		a.doctorCommand(),
		a.startCommand(),
		a.statusCommand(),
		a.resetCommand(),
		a.destroyCommand(),
		a.packCommand(),
	)
	root.PersistentFlags().StringSliceVar(&a.packDirs, "pack", append([]string(nil), a.packDirs...), "load an additional local scenario pack directory")
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
// that always lists a command named "help".
const learnerUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
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
	return curriculum.NewRegistry(a.packDirs...)
}

func (a *app) clusterManager(ctx context.Context) (*cluster.Manager, error) {
	manager, err := cluster.NewManager(cluster.ExecRunner{})
	if err != nil {
		return nil, err
	}
	if err := cluster.EnsureTools(ctx, manager.Paths(), cluster.DefaultToolOptions()); err != nil {
		return nil, err
	}
	return manager, nil
}

func (a *app) doctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local prerequisites",
		RunE: func(cmd *cobra.Command, _ []string) error {
			doctor, err := cluster.NewDoctor(cluster.ExecRunner{})
			if err != nil {
				return err
			}
			failed := false
			for _, result := range doctor.Check(cmd.Context()) {
				marker := "ok"
				if !result.OK {
					marker = "fail"
					failed = true
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", marker, result.Name, result.Detail)
				if result.Remediation != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "       %s\n", result.Remediation)
				}
			}
			if failed {
				return errors.New("one or more prerequisites are unavailable")
			}
			return nil
		},
	}
}

func (a *app) startCommand() *cobra.Command {
	var tutorial string
	var track string
	var prepareOnly bool
	command := &cobra.Command{
		Use:   "start",
		Short: "Create the training cluster if needed and continue the current lab",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := game.NewStore()
			if err != nil {
				return err
			}
			if _, err := a.ensureProfile(cmd, store, tutorial, track); err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Ensuring pinned kind and kubectl...")
			manager, err := a.clusterManager(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := manager.CheckIn(cmd.Context()); err != nil {
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
			if err := a.enterScenario(cmd.Context(), scenario, registry, manager, store, wipeIfNewLab(store, scenario.ID)); err != nil {
				return err
			}
			if prepareOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "KubeCrypt cluster verified and %s prepared.\n", scenario.Title)
				return nil
			}
			return a.runTrainingSession(cmd.Context(), scenario, manager, store)
		},
	}
	command.Flags().StringVar(&tutorial, "tutorial", "", "introductory tutorial: yes or no")
	command.Flags().StringVar(&track, "track", "", "certification track when skipping the tutorial: cka or ckad")
	command.Flags().BoolVar(&prepareOnly, "prepare-only", false, "prepare the current lab without starting the TUI")
	return command
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
			for _, catalog := range registry.Catalogs() {
				fmt.Fprintf(cmd.OutOrStdout(), "Pack: %s (%s)\n", catalog.Title, catalog.Name)
				for _, module := range catalog.Modules {
					var moduleStatus strings.Builder
					for _, ref := range module.Scenarios {
						scenario, loadErr := registry.LoadScenario(ref.ID)
						if loadErr != nil {
							return loadErr
						}
						if !scenarioSupportsExperience(scenario, profile.Experience) {
							continue
						}
						scenarioProgress := progress.Scenarios[ref.ID]
						if scenarioProgress.CompletedAt == nil {
							fmt.Fprintf(&moduleStatus, "  %s: incomplete (hints %v)\n", scenario.Title, scenarioProgress.HintsUsed)
						} else {
							fmt.Fprintf(&moduleStatus, "  %s: completed %s (hints %v)\n",
								scenario.Title, scenarioProgress.CompletedAt.Format(time.RFC3339), scenarioProgress.HintsUsed)
						}
					}
					if moduleStatus.Len() > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s", module.Title, moduleStatus.String())
					}
				}
			}
			manager, managerErr := a.clusterManager(cmd.Context())
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
	var force bool
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
			if err := a.confirm(cmd, force, fmt.Sprintf("Reset %s to its starting state?", scenario.Title)); err != nil {
				return err
			}
			manager, err := a.clusterManager(cmd.Context())
			if err != nil {
				return err
			}
			if _, err := manager.VerifyOwnership(cmd.Context()); err != nil {
				return fmt.Errorf("run kubecrypt start before resetting a lab: %w", err)
			}
			if err := restoreLabCluster(cmd.Context(), scenario, registry, manager); err != nil {
				return err
			}
			if err := store.ResetScenarioProgress(scenario.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s reset. Run kubecrypt start to continue.\n", scenario.Title)
			return nil
		},
	}
	command.Flags().BoolVar(&force, "force", false, "confirm a non-interactive reset")
	return command
}

func (a *app) destroyCommand() *cobra.Command {
	var force bool
	var all bool
	command := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the training cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			prompt := "Destroy the KubeCrypt cluster?"
			if all {
				prompt = "Destroy the KubeCrypt cluster and clear all learner progress?"
			}
			if err := a.confirm(cmd, force, prompt); err != nil {
				return err
			}
			manager, err := a.clusterManager(cmd.Context())
			if err != nil {
				return err
			}
			if err := manager.Destroy(cmd.Context()); err != nil {
				return err
			}
			if all {
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
		},
	}
	command.Flags().BoolVar(&force, "force", false, "confirm a non-interactive destroy")
	command.Flags().BoolVar(&all, "all", false, "also clear learner progress")
	return command
}

func (a *app) packCommand() *cobra.Command {
	pack := &cobra.Command{
		Use:    "pack",
		Short:  "Scenario-pack authoring tools",
		Hidden: true,
	}
	pack.AddCommand(&cobra.Command{
		Use:   "validate <directory>",
		Short: "Validate a local scenario pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := curriculum.ValidatePack(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "valid pack: %s (%d scenarios)\n", catalog.Name, len(catalog.ScenarioIDs()))
			return nil
		},
	})
	return pack
}

func (a *app) ensureProfile(cmd *cobra.Command, store *game.Store, tutorialChoice, requestedTrack string) (game.Profile, error) {
	profile, err := store.LoadProfile()
	if err != nil {
		return game.Profile{}, err
	}
	input := bufio.NewReader(cmd.InOrStdin())
	if profile.OnboardingComplete && tutorialChoice == "" && requestedTrack == "" {
		return profile, nil
	}

	if tutorialChoice == "" && requestedTrack != "" {
		tutorialChoice = "no"
	}
	if tutorialChoice == "" {
		if !readerIsTerminal(cmd.InOrStdin()) {
			return game.Profile{}, errors.New("first non-interactive start requires --tutorial=yes, or --tutorial=no with --track=cka|ckad")
		}
		fmt.Fprint(cmd.OutOrStdout(), "Would you like the introductory tutorial? [Y/n]: ")
		line, readErr := input.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return game.Profile{}, fmt.Errorf("read tutorial choice: %w", readErr)
		}
		tutorialChoice = strings.TrimSpace(strings.ToLower(line))
		if tutorialChoice == "" {
			tutorialChoice = "yes"
		}
	}

	switch strings.ToLower(strings.TrimSpace(tutorialChoice)) {
	case "yes", "y", "true":
		if requestedTrack != "" {
			return game.Profile{}, errors.New("--track cannot be used when the introductory tutorial is enabled")
		}
		profile.Experience = game.ExperienceBeginner
	case "no", "n", "false":
		track := strings.ToLower(strings.TrimSpace(requestedTrack))
		if track == "" {
			if !readerIsTerminal(cmd.InOrStdin()) {
				return game.Profile{}, errors.New("skipping the tutorial requires --track=cka or --track=ckad")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Which certification track are you preparing for?")
			fmt.Fprintln(cmd.OutOrStdout(), "  1) CKA\n  2) CKAD")
			fmt.Fprint(cmd.OutOrStdout(), "Choose 1-2: ")
			line, readErr := input.ReadString('\n')
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return game.Profile{}, fmt.Errorf("read certification track: %w", readErr)
			}
			switch strings.TrimSpace(line) {
			case "1":
				track = "cka"
			case "2":
				track = "ckad"
			default:
				return game.Profile{}, errors.New("certification track must be 1 or 2")
			}
		}
		profile.Experience = game.Experience(track)
	default:
		return game.Profile{}, errors.New("tutorial choice must be yes or no")
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
		if err := a.enterScenario(ctx, next, registry, manager, store, true); err != nil {
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
	namespace, resource := scenarioTarget(scenario)
	if scenario.Namespace != "" {
		namespace = scenario.Namespace
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
		Namespace:           namespace,
		Resource:            resource,
		Experience:          string(profile.Experience),
		Hints:               scenario.Hints,
		Completion:          strings.TrimSpace(scenario.Completion),
		Debrief:             strings.TrimSpace(scenario.Debrief.Explanation),
		Kubeconfig:          manager.Paths().Kubeconfig,
		ToolBinDir:          manager.Paths().BinDir(),
		PackDirectories:     append([]string(nil), a.packDirs...),
		ObserveWhileRunning: scenario.Mode == "orientation",
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
	profile, err := store.LoadProfile()
	if err != nil {
		return nil, err
	}
	ids := registry.ScenarioIDs()
	if len(ids) == 0 {
		return nil, errors.New("active scenario packs contain no scenarios")
	}
	if err := store.RetainScenarios(ids); err != nil {
		return nil, fmt.Errorf("sanitize scenario progress: %w", err)
	}
	progress, err := store.LoadProgress()
	if err != nil {
		return nil, err
	}
	if progress.CurrentScenarioID != "" {
		return registry.LoadScenario(progress.CurrentScenarioID)
	}
	var lastApplicable *curriculum.Scenario
	for _, id := range ids {
		scenario, loadErr := registry.LoadScenario(id)
		if loadErr != nil {
			return nil, loadErr
		}
		if !scenarioSupportsExperience(scenario, profile.Experience) {
			continue
		}
		lastApplicable = scenario
		if progress.Scenarios[id].CompletedAt == nil {
			return scenario, nil
		}
	}
	if lastApplicable == nil {
		return nil, fmt.Errorf("active scenario packs contain no scenarios for track %q", profile.Experience)
	}
	return lastApplicable, nil
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
	for _, id := range registry.ScenarioIDs() {
		scenario, loadErr := registry.LoadScenario(id)
		if loadErr != nil {
			return nil, loadErr
		}
		if !scenarioSupportsExperience(scenario, profile.Experience) {
			continue
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

func scenarioSupportsExperience(scenario *curriculum.Scenario, experience game.Experience) bool {
	return slices.Contains(scenario.Tracks, string(experience))
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
	if len(scenario.Checks) == 0 {
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
	first := scenario.Checks[0]
	kind := first.Kind
	if kind == "" && first.Type == curriculum.CheckDeploymentAvailable {
		kind = "deployment"
	}
	if !wipe && kind != "" && first.Namespace != "" && first.Name != "" {
		result, err := (validator.ObjectExists{Kind: kind, Namespace: first.Namespace, Name: first.Name}).
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

func scenarioResources(registry *curriculum.Registry, scenario *curriculum.Scenario, resources curriculum.ResourceSet) ([]byte, error) {
	return scenario.ComposeResources(resources, func(reference string) ([]byte, error) {
		return registry.ReadManifest(scenario.ID, reference)
	})
}

func scenarioTarget(scenario *curriculum.Scenario) (string, string) {
	if len(scenario.Checks) == 0 {
		return "", ""
	}
	check := scenario.Checks[0]
	kind := check.Kind
	if kind == "" {
		switch check.Type {
		case curriculum.CheckDeploymentAvailable:
			kind = "deployment"
		case curriculum.CheckPodReady:
			kind = "pod"
		case curriculum.CheckNodeTopology:
			return "", "cluster nodes"
		}
	}
	resource := kind
	if check.Name != "" {
		resource += "/" + check.Name
	}
	return check.Namespace, resource
}

func evaluateScenario(ctx context.Context, scenario *curriculum.Scenario, manager *cluster.Manager) (validator.Result, error) {
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

func (a *app) confirm(cmd *cobra.Command, force bool, prompt string) error {
	if force {
		return nil
	}
	if !readerIsTerminal(cmd.InOrStdin()) {
		return errors.New("destructive non-interactive operation requires --force")
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

func readerIsTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
