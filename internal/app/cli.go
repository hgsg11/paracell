package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/adapter/agent"
	"github.com/hgsg11/paracell/internal/adapter/celldir"
	"github.com/hgsg11/paracell/internal/adapter/config"
	"github.com/hgsg11/paracell/internal/adapter/id"
	"github.com/hgsg11/paracell/internal/adapter/logging"
	"github.com/hgsg11/paracell/internal/adapter/output"
	"github.com/hgsg11/paracell/internal/adapter/provider"
	"github.com/hgsg11/paracell/internal/adapter/state"
	"github.com/hgsg11/paracell/internal/adapter/system"
	viewadapter "github.com/hgsg11/paracell/internal/adapter/view"
	"github.com/hgsg11/paracell/internal/adapter/workspacefile"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/query"
	"github.com/hgsg11/paracell/internal/usecase"
)

var (
	runView      = viewadapter.Run
	runEnterRoot = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.WorkspaceProviderFactory) error {
		uc := usecase.EnterRootWorkspaceUseCase{
			Config:           cfg,
			WorkspaceFactory: factory,
		}
		return uc.Execute(ctx)
	}
	runExit = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.WorkspaceProviderFactory) error {
		uc := usecase.ExitWorkspaceUseCase{
			Config:           cfg,
			WorkspaceFactory: factory,
		}
		return uc.Execute(ctx)
	}
	runMarkInProgress = func(ctx context.Context, cells usecase.CellPort, cellName string) error {
		uc := usecase.MarkCellInProgressUseCase{Cells: cells}
		_, err := uc.Execute(ctx, usecase.MarkCellInProgressInput{Cell: cellName})
		return err
	}
	runMarkMerged = func(ctx context.Context, cells usecase.CellPort, fork usecase.ForkCellUseCase, cellName string) error {
		uc := usecase.MarkCellMergedUseCase{Cells: cells, Fork: fork}
		_, err := uc.Execute(ctx, usecase.MarkCellMergedInput{Cell: cellName})
		return err
	}
	runMarkAbandoned = func(ctx context.Context, cells usecase.CellPort, fork usecase.ForkCellUseCase, cellName string) error {
		uc := usecase.MarkCellAbandonedUseCase{Cells: cells, Fork: fork}
		_, err := uc.Execute(ctx, usecase.MarkCellAbandonedInput{Cell: cellName})
		return err
	}
	runPending = func(ctx context.Context, cells usecase.CellPort, cellName string) error {
		uc := usecase.MarkCellPendingUseCase{Cells: cells}
		_, err := uc.Execute(ctx, usecase.MarkCellPendingInput{Cell: cellName})
		return err
	}
	runReady = func(ctx context.Context, cells usecase.CellPort, notifications usecase.NotificationProviderFactory, cellName string) error {
		uc := usecase.MarkCellReadyUseCase{Cells: cells, NotificationFactory: notifications}
		_, err := uc.Execute(ctx, usecase.MarkCellReadyInput{Cell: cellName})
		return err
	}
	runEnterCmd = func(ctx context.Context, cells usecase.CellPort, factory usecase.WorkspaceProviderFactory, cellName string) (*exec.Cmd, error) {
		uc := usecase.EnterCellUseCase{Cells: cells, WorkspaceFactory: factory}
		return uc.Execute(ctx, usecase.EnterCellInput{Cell: cellName})
	}
	runFork = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.WorkspaceProviderFactory, cells usecase.CellPort, issue string, template string, prefix string, command string, note *string, root string) (domain.CommanderCell, error) {
		uc := usecase.ForkCellUseCase{
			Config:           cfg,
			Cells:            cells,
			SourceFactory:    source,
			CellDirectory:    celldir.NewFilesystemAdapter(root),
			ContainerFactory: container,
			WorkspaceFactory: session,
			IDs:              id.RandomGenerator{},
		}
		return uc.Execute(ctx, usecase.ForkCellInput{Issue: issue, Template: template, Prefix: prefix, Command: command, Note: note})
	}
)

// newForkCellUseCase builds the fork use case that also starts queued Cells.
func newForkCellUseCase(cfg usecase.ConfigPort, cells usecase.CellPort, factory provider.Factory, root string) usecase.ForkCellUseCase {
	return usecase.ForkCellUseCase{
		Config:           cfg,
		Cells:            cells,
		SourceFactory:    factory,
		CellDirectory:    celldir.NewFilesystemAdapter(root),
		ContainerFactory: factory,
		WorkspaceFactory: factory,
		IDs:              id.RandomGenerator{},
	}
}

var runClean = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.WorkspaceProviderFactory, cells usecase.CellPort, cellName string, root string) error {
	_ = cfg
	uc := usecase.CleanCellUseCase{
		Cells:            cells,
		SourceFactory:    source,
		CellDirectory:    celldir.NewFilesystemAdapter(root),
		ContainerFactory: container,
		WorkspaceFactory: session,
	}
	return uc.Execute(ctx, usecase.CleanCellInput{Cell: cellName})
}

type CommandKind string

const AppName = "paracell"

const (
	CommandInit      CommandKind = "init"
	CommandFork      CommandKind = "fork"
	CommandAnnotate  CommandKind = "annotate"
	CommandClean     CommandKind = "clean"
	CommandList      CommandKind = "ls"
	CommandPending   CommandKind = "pending"
	CommandReady     CommandKind = "ready"
	CommandReview    CommandKind = "review"
	CommandImplement CommandKind = "implement"
	CommandQueue     CommandKind = "queue"
	CommandPlanned   CommandKind = "planned"
	CommandDequeue   CommandKind = "dequeue"
	CommandMerge     CommandKind = "merge"
	CommandMerged    CommandKind = "merged"
	CommandAbandon   CommandKind = "abandon"
	CommandRoot      CommandKind = "root"
	CommandExit      CommandKind = "exit"
	CommandView      CommandKind = "view"
	CommandVersion   CommandKind = "version"
	CommandHelp      CommandKind = "help"
)

const usage = "usage: paracell [init|fork|queue|planned|dequeue|annotate|ls|view|clean|pending|ready|review|implement|merge|merged|abandon|exit|version|help]\n"

const (
	forkUsage     = "usage: paracell fork <issue> --template <template> [--prefix <prefix>] [--command <command>] [--note <note>]"
	queueUsage    = "usage: paracell queue <issue> --template <template> [--prefix <prefix>] [--command <command>] [--note <note>] [--modify <path>]... [--create <path>]..."
	annotateUsage = "usage: paracell annotate <cell> --note <note>"
	reviewUsage   = "usage: paracell review [--title <title> --body-file <file>]"
	mergeUsage    = "usage: paracell merge <cell> [--squash|--rebase]"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

type Command struct {
	Kind     CommandKind
	Issue    string
	Template string
	Prefix   string
	Command  string
	Note     *string
	Cell     string
	Force    bool
	// Modify and Create are the paths declared by queue.
	Modify []string
	Create []string
	// PathPrefix filters the paths listed by planned.
	PathPrefix string
	// Title and BodyFile open the pull requests on the first review.
	Title    *string
	BodyFile *string
	// MergeMethod is how merge merges the pull requests; empty means a merge commit.
	MergeMethod usecase.MergeMethod
}

func ParseCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Kind: CommandRoot}, nil
	}
	switch args[0] {
	case "--version":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell --version")
		}
		return Command{Kind: CommandVersion}, nil
	case "--help", "-h", "help":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell help")
		}
		return Command{Kind: CommandHelp}, nil
	case "init":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell init")
		}
		return Command{Kind: CommandInit}, nil
	case "ls":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell ls")
		}
		return Command{Kind: CommandList}, nil
	case "view":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell view")
		}
		return Command{Kind: CommandView}, nil
	case "pending":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell pending")
		}
		return Command{Kind: CommandPending}, nil
	case "ready":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell ready")
		}
		return Command{Kind: CommandReady}, nil
	case "review":
		return parseReviewCommand(args)
	case "merge":
		return parseMergeCommand(args)
	case "implement":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell implement")
		}
		return Command{Kind: CommandImplement}, nil
	case "merged":
		if len(args) != 2 || args[1] == "" {
			return Command{}, errors.New("usage: paracell merged <cell>")
		}
		return Command{Kind: CommandMerged, Cell: args[1]}, nil
	case "abandon":
		if len(args) != 2 || args[1] == "" {
			return Command{}, errors.New("usage: paracell abandon <cell>")
		}
		return Command{Kind: CommandAbandon, Cell: args[1]}, nil
	case "exit":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell exit")
		}
		return Command{Kind: CommandExit}, nil
	case "version":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell version")
		}
		return Command{Kind: CommandVersion}, nil
	case "fork":
		return parseForkCommand(args)
	case "queue":
		return parseQueueCommand(args)
	case "planned":
		if len(args) > 2 {
			return Command{}, errors.New("usage: paracell planned [prefix]")
		}
		cmd := Command{Kind: CommandPlanned}
		if len(args) == 2 {
			cmd.PathPrefix = args[1]
		}
		return cmd, nil
	case "dequeue":
		if len(args) != 2 || args[1] == "" {
			return Command{}, errors.New("usage: paracell dequeue <issue>")
		}
		return Command{Kind: CommandDequeue, Issue: args[1]}, nil
	case "annotate":
		if len(args) != 4 || args[1] == "" || args[2] != "--note" {
			return Command{}, errors.New(annotateUsage)
		}
		note := args[3]
		return Command{Kind: CommandAnnotate, Cell: args[1], Note: &note}, nil
	case "clean":
		if len(args) != 2 && !(len(args) == 3 && args[2] == "--force") {
			return Command{}, errors.New("usage: paracell clean <cell> [--force]")
		}
		return Command{Kind: CommandClean, Cell: args[1], Force: len(args) == 3}, nil
	default:
		return Command{}, fmt.Errorf("unsupported command %q", args[0])
	}
}

func parseForkCommand(args []string) (Command, error) {
	if len(args) < 4 || args[1] == "" || (len(args)-2)%2 != 0 {
		return Command{}, errors.New(forkUsage)
	}
	cmd := Command{Kind: CommandFork, Issue: args[1]}
	seen := map[string]bool{}
	for i := 2; i < len(args); i += 2 {
		option, value := args[i], args[i+1]
		if seen[option] {
			return Command{}, errors.New(forkUsage)
		}
		seen[option] = true
		switch option {
		case "--template":
			if value == "" {
				return Command{}, errors.New(forkUsage)
			}
			cmd.Template = value
		case "--prefix":
			if value == "" {
				return Command{}, errors.New(forkUsage)
			}
			cmd.Prefix = value
		case "--command":
			cmd.Command = value
		case "--note":
			note := value
			cmd.Note = &note
		default:
			return Command{}, errors.New(forkUsage)
		}
	}
	if cmd.Template == "" {
		return Command{}, errors.New(forkUsage)
	}
	return cmd, nil
}

// parseReviewCommand reads --title and --body-file once each. Both or neither
// must be given.
func parseReviewCommand(args []string) (Command, error) {
	if (len(args)-1)%2 != 0 {
		return Command{}, errors.New(reviewUsage)
	}
	cmd := Command{Kind: CommandReview}
	for i := 1; i < len(args); i += 2 {
		value := args[i+1]
		switch args[i] {
		case "--title":
			if cmd.Title != nil || value == "" {
				return Command{}, errors.New(reviewUsage)
			}
			cmd.Title = &value
		case "--body-file":
			if cmd.BodyFile != nil || value == "" {
				return Command{}, errors.New(reviewUsage)
			}
			cmd.BodyFile = &value
		default:
			return Command{}, errors.New(reviewUsage)
		}
	}
	if (cmd.Title == nil) != (cmd.BodyFile == nil) {
		return Command{}, errors.New(reviewUsage)
	}
	return cmd, nil
}

func parseMergeCommand(args []string) (Command, error) {
	if len(args) < 2 || len(args) > 3 || args[1] == "" {
		return Command{}, errors.New(mergeUsage)
	}
	cmd := Command{Kind: CommandMerge, Cell: args[1]}
	if len(args) == 3 {
		switch args[2] {
		case "--squash":
			cmd.MergeMethod = usecase.MergeSquash
		case "--rebase":
			cmd.MergeMethod = usecase.MergeRebase
		default:
			return Command{}, errors.New(mergeUsage)
		}
	}
	return cmd, nil
}

// parseQueueCommand reads the fork options once each and --modify and --create
// any number of times.
func parseQueueCommand(args []string) (Command, error) {
	if len(args) < 4 || args[1] == "" || (len(args)-2)%2 != 0 {
		return Command{}, errors.New(queueUsage)
	}
	cmd := Command{Kind: CommandQueue, Issue: args[1]}
	seen := map[string]bool{}
	for i := 2; i < len(args); i += 2 {
		option, value := args[i], args[i+1]
		if option != "--modify" && option != "--create" {
			if seen[option] {
				return Command{}, errors.New(queueUsage)
			}
			seen[option] = true
		}
		switch option {
		case "--template":
			if value == "" {
				return Command{}, errors.New(queueUsage)
			}
			cmd.Template = value
		case "--prefix":
			if value == "" {
				return Command{}, errors.New(queueUsage)
			}
			cmd.Prefix = value
		case "--command":
			cmd.Command = value
		case "--note":
			note := value
			cmd.Note = &note
		case "--modify":
			cmd.Modify = append(cmd.Modify, value)
		case "--create":
			cmd.Create = append(cmd.Create, value)
		default:
			return Command{}, errors.New(queueUsage)
		}
	}
	if cmd.Template == "" || len(cmd.Modify)+len(cmd.Create) == 0 {
		return Command{}, errors.New(queueUsage)
	}
	return cmd, nil
}

func Run(ctx context.Context, args []string, workdir string) (runErr error) {
	invocationWorkdir := workdir
	workdir = projectRootForWorkdir(workdir)
	logPath := filepath.Join(workdir, ".paracell", "logs", "paracell.log")
	cmd, err := ParseCommand(args)
	if err != nil {
		logger := logging.NewFile(logPath)
		return errors.Join(err, logger.Write(logging.LevelError, "paracell", "command parse failed: "+err.Error()))
	}
	logger := logging.NewFile(logPath)
	if cmd.Kind == CommandView {
		logger = logging.New(logPath)
	}
	if err := logger.Write(logging.LevelInfo, "paracell", "command "+string(cmd.Kind)+" started"); err != nil {
		return err
	}
	defer func() {
		if runErr != nil {
			runErr = errors.Join(runErr, logger.Write(logging.LevelError, "paracell", "command "+string(cmd.Kind)+" failed: "+runErr.Error()))
			return
		}
		runErr = logger.Write(logging.LevelInfo, "paracell", "command "+string(cmd.Kind)+" completed")
	}()
	runner := system.LoggingRunner{Dir: workdir, Logger: logger, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	quietRunner := system.LoggingRunner{Dir: workdir, Logger: logger}
	configAdapter := config.NewYAMLConfigAdapter(filepath.Join(workdir, "paracell.yaml"))
	cellsAdapter := state.NewSQLiteCellAdapter(filepath.Join(workdir, ".paracell", "state.db"))

	switch cmd.Kind {
	case CommandVersion:
		return writeCLIOutput(logger, formatVersion())
	case CommandHelp:
		return writeCLIOutput(logger, usage)
	case CommandInit:
		uc := usecase.InitProjectUseCase{
			Config:     configAdapter,
			Cells:      cellsAdapter,
			AgentHooks: agent.NewClaudeSettingsAdapter(filepath.Join(workdir, ".claude", "settings.json")),
		}
		_, err := uc.Execute(ctx)
		return err
	case CommandList:
		cells, err := cellsAdapter.ListCellViews(ctx)
		if err != nil {
			return err
		}
		queue, err := cellsAdapter.ListQueueViews(ctx)
		if err != nil {
			return err
		}
		list := output.FormatCellList(cells)
		undecided, err := undecidedReviews(ctx, configAdapter, cellsAdapter, provider.NewFactory(quietRunner, workdir))
		if err != nil {
			list += fmt.Sprintf("WARNING: could not check pull requests: %v\n", err)
		}
		list += output.FormatUndecidedReviews(undecided)
		if waiting := output.FormatWaitingQueue(queue); waiting != "" {
			list += "\n" + waiting
		}
		return writeCLIOutput(logger, list)
	case CommandQueue:
		uc := usecase.EnqueueCellUseCase{
			Config: configAdapter,
			Cells:  cellsAdapter,
			Files:  workspacefile.NewFilesystemAdapter(workdir),
			IDs:    id.RandomGenerator{},
			Fork:   newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(runner, workdir), workdir),
		}
		result, err := uc.Execute(ctx, usecase.EnqueueCellInput{
			Issue: cmd.Issue, Template: cmd.Template, Prefix: cmd.Prefix, Command: cmd.Command, Note: cmd.Note,
			Modify: cmd.Modify, Create: cmd.Create,
		})
		if result.Entry.ID == "" {
			return err
		}
		return errors.Join(writeCLIOutput(logger, output.FormatEnqueueResult(cmd.Issue, result.Started, result.BlockedBy)), err)
	case CommandPlanned:
		paths, err := cellsAdapter.ListPlannedPaths(ctx, cmd.PathPrefix)
		if err != nil {
			return err
		}
		return writeCLIOutput(logger, output.FormatPlannedPaths(paths))
	case CommandDequeue:
		uc := usecase.DequeueCellUseCase{
			Cells: cellsAdapter,
			Fork:  newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(runner, workdir), workdir),
		}
		_, err := uc.Execute(ctx, usecase.DequeueCellInput{Issue: cmd.Issue})
		return err
	case CommandAnnotate:
		uc := usecase.AnnotateCellUseCase{Cells: cellsAdapter, WorkspaceFactory: provider.NewFactory(quietRunner, workdir)}
		_, err = uc.Execute(ctx, usecase.AnnotateCellInput{Cell: cmd.Cell, Note: *cmd.Note})
		return err
	case CommandPending:
		cellName, err := currentCell(invocationWorkdir)
		if err != nil {
			return err
		}
		return runPending(ctx, cellsAdapter, cellName)
	case CommandReady:
		cellName, err := currentCell(invocationWorkdir)
		if err != nil {
			return err
		}
		return runReady(ctx, cellsAdapter, provider.NewFactory(quietRunner, workdir), cellName)
	case CommandReview:
		cellName, err := currentCell(invocationWorkdir)
		if err != nil {
			return err
		}
		input := usecase.RequestReviewInput{Cell: cellName, Title: cmd.Title}
		if cmd.BodyFile != nil {
			body, err := os.ReadFile(resolvePath(invocationWorkdir, *cmd.BodyFile))
			if err != nil {
				return fmt.Errorf("read --body-file: %w", err)
			}
			text := string(body)
			input.Body = &text
		}
		uc := usecase.RequestReviewUseCase{
			Config:        configAdapter,
			Cells:         cellsAdapter,
			ReviewFactory: provider.NewFactory(quietRunner, workdir),
			IDs:           id.RandomGenerator{},
			Fork:          newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(quietRunner, workdir), workdir),
			Report: func(line string) {
				_ = writeCLIOutput(logger, line+"\n")
			},
		}
		message, err := uc.Execute(ctx, input)
		if err != nil || message == "" {
			return err
		}
		return writeCLIOutput(logger, message+"\n")
	case CommandMerge:
		return runMerge(ctx, configAdapter, cellsAdapter, provider.NewFactory(runner, workdir), workdir, cmd.Cell, cmd.MergeMethod)
	case CommandImplement:
		cellName, err := currentCell(invocationWorkdir)
		if err != nil {
			return err
		}
		return runMarkInProgress(ctx, cellsAdapter, cellName)
	case CommandMerged:
		return runMarkMerged(ctx, cellsAdapter, newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(runner, workdir), workdir), cmd.Cell)
	case CommandAbandon:
		return runMarkAbandoned(ctx, cellsAdapter, newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(runner, workdir), workdir), cmd.Cell)
	case CommandRoot:
		return runEnterRoot(ctx, configAdapter, provider.NewFactory(runner, workdir))
	case CommandExit:
		return runExit(ctx, configAdapter, provider.NewFactory(runner, workdir))
	case CommandView:
		viewLogger := logger
		viewRunner := system.LoggingRunner{Dir: workdir, Logger: viewLogger}
		interactiveViewRunner := system.LoggingRunner{Dir: workdir, Logger: viewLogger, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
		viewContext := viewadapter.WithLogger(ctx, viewLogger)
		loaded, err := configAdapter.Load(ctx)
		if err != nil {
			return err
		}
		cells, err := cellsAdapter.ListCellViews(ctx)
		if err != nil {
			return err
		}
		names, err := loaded.SelectableNames()
		if err != nil {
			return err
		}
		reviews := newReviewWatch(func() ([]string, error) {
			return undecidedReviews(ctx, configAdapter, cellsAdapter, provider.NewFactory(viewRunner, workdir))
		})
		_, err = runView(viewContext, reviews.mark(cells), names, os.Getenv("PARACELL_CELL"), func() ([]query.CellView, error) {
			cells, err := cellsAdapter.ListCellViews(ctx)
			return reviews.mark(cells), err
		}, func() ([]query.QueueView, error) {
			return cellsAdapter.ListQueueViews(ctx)
		}, func(cellName string) tea.Cmd {
			cmd, err := runEnterCmd(ctx, cellsAdapter, provider.NewFactory(viewRunner, workdir), cellName)
			if err != nil {
				return viewadapter.EnterFailureCmd(cellName, err)
			}
			return viewadapter.EnterLoggedProcessCmd(cellName, cmd, viewLogger)
		}, func() error {
			return runEnterRoot(ctx, configAdapter, provider.NewFactory(interactiveViewRunner, workdir))
		}, func(cellName string) error {
			factory := provider.NewFactory(viewRunner, workdir)
			return runClean(ctx, configAdapter, factory, factory, factory, cellsAdapter, cellName, workdir)
		}, func(cellName string) error {
			// A Cell in review is merged on the provider, like paracell merge.
			factory := provider.NewFactory(viewRunner, workdir)
			err := runMerge(ctx, configAdapter, cellsAdapter, factory, workdir, cellName, "")
			if !errors.Is(err, usecase.ErrNoReview) {
				return err
			}
			return runMarkMerged(ctx, cellsAdapter, newForkCellUseCase(configAdapter, cellsAdapter, factory, workdir), cellName)
		}, func(cellName string) error {
			return runMarkAbandoned(ctx, cellsAdapter, newForkCellUseCase(configAdapter, cellsAdapter, provider.NewFactory(viewRunner, workdir), workdir), cellName)
		}, func(issue string, template string) tea.Cmd {
			return func() tea.Msg {
				factory := provider.NewFactory(viewRunner, workdir)
				_, err := runFork(ctx, configAdapter, factory, factory, factory, cellsAdapter, issue, template, "", "", nil, workdir)
				return viewadapter.ForkResultCmd(err)()
			}
		})
		if err != nil {
			return err
		}
		return nil
	case CommandFork:
		uc := usecase.ForkCellUseCase{
			Config:           configAdapter,
			Cells:            cellsAdapter,
			SourceFactory:    provider.NewFactory(runner, workdir),
			CellDirectory:    celldir.NewFilesystemAdapter(workdir),
			ContainerFactory: provider.NewFactory(runner, workdir),
			WorkspaceFactory: provider.NewFactory(runner, workdir),
			IDs:              id.RandomGenerator{},
		}
		_, err = uc.Execute(ctx, usecase.ForkCellInput{Issue: cmd.Issue, Template: cmd.Template, Prefix: cmd.Prefix, Command: cmd.Command, Note: cmd.Note})
		return err
	case CommandClean:
		uc := usecase.CleanCellUseCase{
			Cells:            cellsAdapter,
			SourceFactory:    provider.NewFactory(runner, workdir),
			CellDirectory:    celldir.NewFilesystemAdapter(workdir),
			ContainerFactory: provider.NewFactory(runner, workdir),
			WorkspaceFactory: provider.NewFactory(runner, workdir),
		}
		return uc.Execute(ctx, usecase.CleanCellInput{Cell: cmd.Cell})
	default:
		return fmt.Errorf("unsupported command %q", cmd.Kind)
	}
}

// runMerge merges the pull requests of a Cell in review. It fails with
// usecase.ErrNoReview when the Cell has no Review.
var runMerge = func(ctx context.Context, cfg usecase.ConfigPort, cells usecase.CellPort, factory provider.Factory, root string, cellName string, method usecase.MergeMethod) error {
	uc := usecase.MergeCellUseCase{Config: cfg, Cells: cells, ReviewFactory: factory, Fork: newForkCellUseCase(cfg, cells, factory, root)}
	_, err := uc.Execute(ctx, usecase.MergeCellInput{Cell: cellName, Method: method})
	return err
}

// undecidedReviews returns the Cells, in name order, whose pull requests are
// partly merged and partly closed. Without Cells in review it reads no config.
func undecidedReviews(ctx context.Context, cfg usecase.ConfigPort, cells usecase.CellPort, factory usecase.ReviewProviderFactory) ([]string, error) {
	outcomes, err := (usecase.CheckReviewsUseCase{Config: cfg, Cells: cells, ReviewFactory: factory}).Execute(ctx)
	var names []string
	for name, outcome := range outcomes {
		if outcome == domain.ReviewMixed {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, err
}

func resolvePath(workdir string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workdir, path)
}

func formatVersion() string {
	return fmt.Sprintf("%s %s\ncommit: %s\nbuilt: %s\n", AppName, Version, Commit, BuildDate)
}

func currentCell(workdir string) (string, error) {
	cell := os.Getenv("PARACELL_CELL")
	if cell != "" {
		return cell, nil
	}
	if cell, _, ok := cellForWorkdir(workdir); ok {
		return cell, nil
	}
	return "", errors.New("PARACELL_CELL is required")
}

func projectRootForWorkdir(workdir string) string {
	if root := os.Getenv("PARACELL_ROOT"); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return abs
		}
		return filepath.Clean(root)
	}
	if _, root, ok := cellForWorkdir(workdir); ok {
		return root
	}
	return workdir
}

// cellForWorkdir finds the Cell directory .paracell/cells/<cell> that contains
// workdir, and returns the Cell name and the project root, the parent of
// .paracell. The nearest such ancestor wins.
func cellForWorkdir(workdir string) (string, string, bool) {
	for dir := filepath.Clean(workdir); ; dir = filepath.Dir(dir) {
		cellsDir := filepath.Dir(dir)
		paracellDir := filepath.Dir(cellsDir)
		if filepath.Base(cellsDir) == "cells" && filepath.Base(paracellDir) == ".paracell" && dir != cellsDir {
			return filepath.Base(dir), filepath.Dir(paracellDir), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
	}
}

func writeCLIOutput(logger *logging.Logger, value string) error {
	content := strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r")
	var logErr error
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		logErr = errors.Join(logErr, logger.Write(logging.LevelInfo, "paracell", "stdout: "+strings.TrimSuffix(line, "\r")))
	}
	_, outputErr := os.Stdout.WriteString(value)
	return errors.Join(outputErr, logErr)
}
