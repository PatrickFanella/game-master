// Package tui provides the root Bubble Tea application model and shared TUI
// infrastructure (router, view interface) for the game-master terminal UI.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/engine"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/tui/character"
	"github.com/PatrickFanella/game-master/tui/inventory"
	"github.com/PatrickFanella/game-master/tui/narrative"
	"github.com/PatrickFanella/game-master/tui/quest"
	"github.com/PatrickFanella/game-master/tui/styles"
)

// ViewState identifies which sub-view is currently active.
type ViewState int

const (
	ViewNarrative      ViewState = iota // 0 – main story / conversation log
	ViewCharacterSheet                  // 1 – player attributes and stats
	ViewInventory                       // 2 – carried items and gold
	ViewQuestLog                        // 3 – active and completed quests
)

const statusBarHints = "1-4: switch view | tab/shift+tab/right/left/h/l: cycle | q: quit"
const statusBarSectionSeparator = "  ·  "
const statusBarViewSeparator = " | "
const narrativeChunkSize = 24 // small chunks keep the streamed narrative feeling responsive in the viewport
const narrativeChunkDelay = 20 * time.Millisecond

type turnProcessedMsg struct {
	result *engine.TurnResult
	err    error
}

type narrativeChunkMsg struct {
	chunk     string
	remaining string
	choices   []engine.Choice
}

type narrativeStreamDoneMsg struct {
	choices []engine.Choice
}

// App is the root Bubble Tea model for Game Master. It tracks the active
// ViewState and delegates Init/Update/View to the appropriate sub-model via
// the embedded Router. Global key bindings (quit, view-switching) are handled
// here before any message is forwarded to the active sub-model.
// Sub-view state is preserved across view switches because each view is stored
// independently and only the active index changes.
type App struct {
	cfg       config.Config
	ctx       context.Context
	engine    engine.GameEngine
	campaign  statedb.Campaign
	router    *Router
	viewState ViewState
	width     int
	height    int
	turnBusy  bool
}

// NewApp creates and initialises the root App model with all four sub-views
// registered. The narrative log is pre-seeded with welcome messages.
// campaign is the currently active campaign; its name is shown in the title bar.
func NewApp(cfg config.Config, campaign statedb.Campaign) App {
	return NewAppWithEngine(cfg, campaign, context.Background(), nil)
}

// NewAppWithEngine creates a root App that sends narrative turns through the engine.
func NewAppWithEngine(cfg config.Config, campaign statedb.Campaign, ctx context.Context, gameEngine engine.GameEngine) App {
	if ctx == nil {
		ctx = context.Background()
	}

	router := NewRouter()

	nv := narrative.New()
	cv := character.New()
	iv := inventory.New()
	qv := quest.New()

	router.Register("Narrative", &nv)
	router.Register("Character", &cv)
	router.Register("Inventory", &iv)
	router.Register("Quests", &qv)

	// Seed the narrative log with a welcome message for the selected campaign.
	nv.AddEntry(narrative.Entry{
		Kind: narrative.KindSystem,
		Text: fmt.Sprintf("Welcome to Game Master  ·  Provider: %s", cfg.LLM.Provider),
	})
	if campaign.Name != "" {
		nv.AddEntry(narrative.Entry{
			Kind: narrative.KindSystem,
			Text: fmt.Sprintf("Campaign: %s", campaign.Name),
		})
	}

	return App{
		cfg:       cfg,
		ctx:       ctx,
		engine:    gameEngine,
		campaign:  campaign,
		router:    router,
		viewState: ViewNarrative,
	}
}

// ActiveViewState returns the currently active ViewState.
func (a App) ActiveViewState() ViewState {
	return a.viewState
}

// Init implements tea.Model. No start-up commands are needed.
func (a App) Init() tea.Cmd { return nil }

// Update implements tea.Model. It handles global key bindings (quit and view
// switching) and forwards all other messages to the active sub-model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.propagateSizes()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "tab", "right", "l":
			a.router.NextTab()
			a.viewState = ViewState(a.router.ActiveTab())
		case "shift+tab", "left", "h":
			a.router.PrevTab()
			a.viewState = ViewState(a.router.ActiveTab())
		case "1", "2", "3", "4":
			idx := int(msg.String()[0] - '1')
			a.router.GoToTab(idx)
			a.viewState = ViewState(a.router.ActiveTab())
		default:
			cmd := a.router.Update(msg)
			return a, cmd
		}

	case spinner.TickMsg:
		if a.turnBusy {
			if nv := a.narrativeView(); nv != nil {
				updated, cmd := nv.Update(msg)
				if view, ok := updated.(*narrative.Model); ok {
					a.router.tabs[int(ViewNarrative)].View = view
				}
				return a, cmd
			}
		}
		return a, nil

	case narrative.SubmitMsg:
		if a.turnBusy {
			return a, nil
		}
		if nv := a.narrativeView(); nv != nil {
			nv.AddEntry(narrative.Entry{Kind: narrative.KindPlayer, Text: msg.Input})
			nv.ClearChoices()
			if a.engine == nil {
				return a, nil
			}
			a.turnBusy = true
			return a, tea.Batch(
				nv.SetLoading(true),
				a.processTurn(msg.Input),
			)
		}
		return a, nil

	case turnProcessedMsg:
		nv := a.narrativeView()
		if nv == nil {
			a.turnBusy = false
			return a, nil
		}
		// Turning loading off updates the narrative view state synchronously; it
		// does not need to schedule another spinner tick command.
		_ = nv.SetLoading(false)

		if msg.err != nil {
			a.turnBusy = false
			nv.AddEntry(narrative.Entry{
				Kind: narrative.KindSystem,
				Text: fmt.Sprintf("Error: %v", msg.err),
			})
			return a, nil
		}

		if msg.result == nil {
			a.turnBusy = false
			return a, nil
		}

		if msg.result.Narrative == "" {
			a.turnBusy = false
			nv.SetChoices(msg.result.Choices)
			return a, nil
		}

		nv.BeginStreamingNPCEntry()
		return a, a.streamNarrative(msg.result.Narrative, msg.result.Choices)

	case narrativeChunkMsg:
		if nv := a.narrativeView(); nv != nil {
			nv.AppendToLastEntry(msg.chunk)
		}
		if msg.remaining == "" {
			return a, func() tea.Msg {
				return narrativeStreamDoneMsg{choices: msg.choices}
			}
		}
		return a, a.streamNarrative(msg.remaining, msg.choices)

	case narrativeStreamDoneMsg:
		a.turnBusy = false
		if nv := a.narrativeView(); nv != nil {
			nv.SetChoices(msg.choices)
		}
		return a, nil

	case character.NavigateBackMsg:
		a.router.GoToTab(int(ViewNarrative))
		a.viewState = ViewNarrative

	case inventory.NavigateBackMsg:
		a.router.GoToTab(int(ViewNarrative))
		a.viewState = ViewNarrative

	case quest.NavigateBackMsg:
		a.router.GoToTab(int(ViewNarrative))
		a.viewState = ViewNarrative

	default:
		// Forward any other message types (e.g. commands produced by sub-views)
		// to the active sub-model so they are never silently dropped.
		cmd := a.router.Update(msg)
		return a, cmd
	}
	return a, nil
}

// View implements tea.Model and renders the full TUI chrome plus the active
// sub-view.
func (a App) View() string {
	titleBar, statusBar := a.chrome()
	activeView := lipgloss.NewStyle().Width(a.width).Render(a.router.View())
	return styles.JoinVertical(titleBar, activeView, statusBar)
}

// chrome renders the title bar and status bar at the current width.
func (a App) chrome() (titleBar, statusBar string) {
	title := "⚔  Game Master"
	if a.campaign.Name != "" {
		title += "  ·  " + a.campaign.Name
	}
	titleBar = styles.TitleBar.Width(a.width).Render(
		title + styles.Muted.Render(
			fmt.Sprintf("  ·  %s", a.cfg.LLM.Provider),
		),
	)
	statusViews := a.renderStatusViews()
	hints := styles.Muted.Render(statusBarHints)
	statusBar = styles.StatusBar.Width(a.width).Render(styles.JoinHorizontal(
		statusViews,
		styles.Muted.Render(statusBarSectionSeparator),
		hints,
	))
	return
}

// propagateSizes pushes the current terminal dimensions down to all sub-views,
// accounting for the vertical space consumed by the chrome.
func (a App) propagateSizes() {
	titleBar, statusBar := a.chrome()

	reserved := lipgloss.Height(titleBar) + lipgloss.Height(statusBar)
	viewHeight := a.height - reserved
	if viewHeight < 1 {
		viewHeight = 1
	}

	a.router.SetSize(a.width, viewHeight)
}

// renderStatusViews builds the status-bar view list and highlights the active view.
func (a App) renderStatusViews() string {
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent)
	inactiveStyle := lipgloss.NewStyle().Foreground(styles.ColorMuted)

	var tabs []string
	for i, tab := range a.router.Tabs() {
		label := tab.Name
		if i == a.router.ActiveTab() {
			tabs = append(tabs, activeStyle.Render("["+label+"]"))
		} else {
			tabs = append(tabs, inactiveStyle.Render(label))
		}
	}
	sep := styles.Muted.Render(statusBarViewSeparator)
	return styles.Muted.Render("Views: ") + strings.Join(tabs, sep)
}

func (a App) narrativeView() *narrative.Model {
	if len(a.router.tabs) <= int(ViewNarrative) {
		return nil
	}
	view, _ := a.router.tabs[int(ViewNarrative)].View.(*narrative.Model)
	return view
}

func (a App) processTurn(input string) tea.Cmd {
	return func() tea.Msg {
		result, err := a.engine.ProcessTurn(a.ctx, dbutil.FromPgtype(a.campaign.ID), input)
		return turnProcessedMsg{result: result, err: err}
	}
}

func (a App) streamNarrative(text string, choices []engine.Choice) tea.Cmd {
	return tea.Tick(narrativeChunkDelay, func(time.Time) tea.Msg {
		if text == "" {
			return narrativeStreamDoneMsg{choices: choices}
		}
		chunk, remaining := nextNarrativeChunk(text)
		return narrativeChunkMsg{
			chunk:     chunk,
			remaining: remaining,
			choices:   choices,
		}
	})
}

func nextNarrativeChunk(text string) (chunk, remaining string) {
	runes := []rune(text)
	if len(runes) <= narrativeChunkSize {
		return text, ""
	}
	return string(runes[:narrativeChunkSize]), string(runes[narrativeChunkSize:])
}
