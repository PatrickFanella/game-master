// Package tui – launcher model.
//
// Launcher is the initial Bubble Tea model. It:
//  1. Connects to the database and runs the bootstrap sequence
//     (creates the default "Player" user if missing).
//  2. Displays the campaign-selection view.
//  3. After the player chooses (or creates) a campaign it loads the campaign
//     in the engine and transitions to the main App.
package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/engine"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/tui/campaign"
	"github.com/PatrickFanella/game-master/tui/styles"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// bootstrapDoneMsg carries the result of the DB bootstrap step.
type bootstrapDoneMsg struct {
	result bootstrap.Result
	err    error
}

// campaignCreatedMsg is sent after a new campaign has been persisted.
type campaignCreatedMsg struct {
	c   statedb.Campaign
	err error
}

// campaignLoadedMsg is sent after a campaign has been loaded in the engine.
type campaignLoadedMsg struct {
	c   statedb.Campaign
	err error
}

// ---------------------------------------------------------------------------
// Launcher
// ---------------------------------------------------------------------------

// launcherState is the internal phase of the Launcher model.
type launcherState int

const (
	launcherLoading         launcherState = iota // running DB bootstrap
	launcherSelecting                            // showing campaign-selection list
	launcherCreating                             // creating a new campaign in the DB
	launcherLoadingCampaign                      // loading selected campaign into engine
)

// Launcher is the root Bubble Tea model during start-up.
type Launcher struct {
	cfg     config.Config
	ctx     context.Context
	engine  engine.GameEngine
	queries statedb.Querier
	user    statedb.User
	state   launcherState
	picker  campaign.Model
	spinner spinner.Model
	errMsg  string
	width   int
	height  int
}

// NewLauncher creates the Launcher model. ctx is used for all DB operations so
// they can be cancelled on SIGTERM/ctrl+c. queries must already be open and
// ready.
func NewLauncher(cfg config.Config, ctx context.Context, queries statedb.Querier) Launcher {
	return NewLauncherWithEngine(cfg, ctx, queries, nil)
}

// NewLauncherWithEngine creates the Launcher model with a game engine dependency.
func NewLauncherWithEngine(cfg config.Config, ctx context.Context, queries statedb.Querier, gameEngine engine.GameEngine) Launcher {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	return Launcher{
		cfg:     cfg,
		ctx:     ctx,
		engine:  gameEngine,
		queries: queries,
		state:   launcherLoading,
		spinner: sp,
	}
}

// Init implements tea.Model.  It fires the DB bootstrap command immediately.
func (l Launcher) Init() tea.Cmd {
	return tea.Batch(l.spinner.Tick, l.runBootstrap())
}

// Update implements tea.Model.
func (l Launcher) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		// Only resize the picker when it has been initialised.
		if l.state == launcherSelecting {
			l.picker.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return l, tea.Quit
		}

	case spinner.TickMsg:
		// Only advance the spinner while it is actually visible (loading or
		// creating/loading-campaign states). In the selecting state the spinner is not shown so
		// we drop the tick to avoid a perpetual background tick loop.
		if l.state == launcherLoading || l.state == launcherCreating || l.state == launcherLoadingCampaign {
			var cmd tea.Cmd
			l.spinner, cmd = l.spinner.Update(msg)
			return l, cmd
		}
		return l, nil

	case bootstrapDoneMsg:
		if msg.err != nil {
			l.errMsg = fmt.Sprintf("Bootstrap failed: %v", msg.err)
			return l, nil
		}
		l.errMsg = ""
		l.user = msg.result.User
		campaigns := msg.result.Campaigns
		l.picker = campaign.New(campaigns)
		l.picker.SetSize(l.width, l.height)
		l.state = launcherSelecting
		return l, nil

	case campaign.SelectedMsg:
		l.errMsg = ""
		l.state = launcherLoadingCampaign
		return l, tea.Batch(l.spinner.Tick, l.runLoadCampaign(msg.Campaign))

	case campaign.NewCampaignNameMsg:
		l.errMsg = ""
		l.state = launcherCreating
		return l, tea.Batch(l.spinner.Tick, l.runCreateCampaign(msg.Name))

	case campaignCreatedMsg:
		if msg.err != nil {
			l.errMsg = fmt.Sprintf("Create campaign failed: %v", msg.err)
			l.state = launcherSelecting
			return l, nil
		}
		l.errMsg = ""
		l.state = launcherLoadingCampaign
		return l, tea.Batch(l.spinner.Tick, l.runLoadCampaign(msg.c))

	case campaignLoadedMsg:
		if msg.err != nil {
			l.errMsg = fmt.Sprintf("Load campaign failed: %v", msg.err)
			l.state = launcherSelecting
			return l, nil
		}
		l.errMsg = ""
		return l.transitionToApp(msg.c)
	}

	// Forward messages to the campaign picker when it's active.
	if l.state == launcherSelecting {
		updated, cmd := l.picker.Update(msg)
		if m, ok := updated.(campaign.Model); ok {
			l.picker = m
		}
		return l, cmd
	}

	return l, nil
}

// View implements tea.Model.
func (l Launcher) View() string {
	switch l.state {
	case launcherSelecting:
		if l.errMsg != "" {
			return styles.JoinVertical(
				styles.StatusError.Render("⚠  "+l.errMsg),
				l.picker.View(),
			)
		}
		return l.picker.View()

	case launcherCreating:
		return l.loadingView("Creating campaign…")

	case launcherLoadingCampaign:
		return l.loadingView("Loading campaign…")

	default: // launcherLoading
		if l.errMsg != "" {
			return styles.StatusError.Render("⚠  " + l.errMsg)
		}
		return l.loadingView("Connecting to database…")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadingView renders a centred spinner with a message.
func (l Launcher) loadingView(msg string) string {
	line := l.spinner.View() + "  " + styles.Body.Render(msg)
	if l.width > 0 && l.height > 0 {
		return styles.Place(l.width, l.height, line)
	}
	return line
}

// transitionToApp creates the main App model and returns it as the new model.
func (l Launcher) transitionToApp(c statedb.Campaign) (tea.Model, tea.Cmd) {
	app := NewAppWithEngine(l.cfg, c, l.ctx, l.engine)
	return app, app.Init()
}

// runBootstrap returns a tea.Cmd that runs the DB bootstrap asynchronously.
// It uses the launcher's context so the operation is cancelled on shutdown.
func (l Launcher) runBootstrap() tea.Cmd {
	ctx := l.ctx
	queries := l.queries
	return func() tea.Msg {
		result, err := bootstrap.Run(ctx, queries)
		return bootstrapDoneMsg{result: result, err: err}
	}
}

// runCreateCampaign returns a tea.Cmd that creates a new campaign in the DB.
// It uses the launcher's context so the operation is cancelled on shutdown.
func (l Launcher) runCreateCampaign(name string) tea.Cmd {
	ctx := l.ctx
	queries := l.queries
	userID := l.user.ID
	return func() tea.Msg {
		c, err := bootstrap.CreateCampaign(ctx, queries, userID, name)
		return campaignCreatedMsg{c: c, err: err}
	}
}

// runLoadCampaign returns a tea.Cmd that loads a campaign in the game engine
// before transitioning to the main app.
func (l Launcher) runLoadCampaign(c statedb.Campaign) tea.Cmd {
	ctx := l.ctx
	gameEngine := l.engine
	return func() tea.Msg {
		if gameEngine == nil {
			return campaignLoadedMsg{c: c, err: nil}
		}
		err := gameEngine.LoadCampaign(ctx, dbutil.FromPgtype(c.ID))
		return campaignLoadedMsg{c: c, err: err}
	}
}
