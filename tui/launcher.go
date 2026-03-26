// Package tui – launcher model.
//
// Launcher is the initial Bubble Tea model. It:
//  1. Connects to the database and runs the bootstrap sequence
//     (creates the default "Player" user and a starter campaign when
//     none exist).
//  2. If exactly one campaign is available, it auto-selects it and
//     transitions to the main App immediately.
//  3. If multiple campaigns exist it displays the campaign-selection view
//     and waits for the player to choose.
//  4. After the player chooses (or creates) a campaign it transitions to
//     the main App, passing in the selected Campaign.
package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
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

// ---------------------------------------------------------------------------
// Launcher
// ---------------------------------------------------------------------------

// launcherState is the internal phase of the Launcher model.
type launcherState int

const (
	launcherLoading   launcherState = iota // running DB bootstrap
	launcherSelecting                      // showing campaign-selection list
	launcherCreating                       // creating a new campaign in the DB
)

// Launcher is the root Bubble Tea model during start-up.
type Launcher struct {
	cfg      config.Config
	pool     *pgxpool.Pool
	queries  statedb.Querier
	user     statedb.User
	state    launcherState
	picker   campaign.Model
	spinner  spinner.Model
	errMsg   string
	width    int
	height   int
}

// NewLauncher creates the Launcher model.  pool and queries must already be
// open and ready; they are used throughout the bootstrap and campaign-creation
// phases.
func NewLauncher(cfg config.Config, pool *pgxpool.Pool, queries statedb.Querier) Launcher {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	return Launcher{
		cfg:     cfg,
		pool:    pool,
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
		l.picker.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return l, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		l.spinner, cmd = l.spinner.Update(msg)
		return l, cmd

	case bootstrapDoneMsg:
		if msg.err != nil {
			l.errMsg = fmt.Sprintf("Bootstrap failed: %v", msg.err)
			return l, nil
		}
		l.user = msg.result.User
		campaigns := msg.result.Campaigns
		// Single campaign (first boot or only one) → auto-select.
		if len(campaigns) == 1 {
			return l.transitionToApp(campaigns[0])
		}
		// Multiple campaigns → show selection.
		l.picker = campaign.New(campaigns)
		l.picker.SetSize(l.width, l.height)
		l.state = launcherSelecting
		return l, nil

	case campaign.SelectedMsg:
		return l.transitionToApp(msg.Campaign)

	case campaign.NewCampaignNameMsg:
		l.state = launcherCreating
		return l, tea.Batch(l.spinner.Tick, l.runCreateCampaign(msg.Name))

	case campaignCreatedMsg:
		if msg.err != nil {
			l.errMsg = fmt.Sprintf("Create campaign failed: %v", msg.err)
			l.state = launcherSelecting
			return l, nil
		}
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
		return l.picker.View()

	case launcherCreating:
		return l.loadingView("Creating campaign…")

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
	app := NewApp(l.cfg, c)
	return app, app.Init()
}

// runBootstrap returns a tea.Cmd that runs the DB bootstrap asynchronously.
func (l Launcher) runBootstrap() tea.Cmd {
	return func() tea.Msg {
		result, err := bootstrap.Run(context.Background(), l.queries)
		return bootstrapDoneMsg{result: result, err: err}
	}
}

// runCreateCampaign returns a tea.Cmd that creates a new campaign in the DB.
func (l Launcher) runCreateCampaign(name string) tea.Cmd {
	return func() tea.Msg {
		c, err := bootstrap.CreateCampaign(context.Background(), l.queries, l.user.ID, name)
		return campaignCreatedMsg{c: c, err: err}
	}
}
