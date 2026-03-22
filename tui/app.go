// Package tui provides the root Bubble Tea application model and shared TUI
// infrastructure (router, view interface) for the game-master terminal UI.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/config"
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

// App is the root Bubble Tea model for Game Master. It tracks the active
// ViewState and delegates Init/Update/View to the appropriate sub-model via
// the embedded Router. Global key bindings (quit, view-switching) are handled
// here before any message is forwarded to the active sub-model.
type App struct {
	cfg       config.Config
	router    *Router
	viewState ViewState
	width     int
	height    int
}

// NewApp creates and initialises the root App model with all four sub-views
// registered. The narrative log is pre-seeded with welcome messages.
func NewApp(cfg config.Config) App {
	router := NewRouter()

	nv := narrative.New()
	cv := character.New()
	iv := inventory.New()
	qv := quest.New()

	router.Register("Narrative", &nv)
	router.Register("Character", &cv)
	router.Register("Inventory", &iv)
	router.Register("Quests", &qv)

	// Seed the narrative log with example entries.
	nv.AddEntry(narrative.Entry{
		Kind: narrative.KindSystem,
		Text: fmt.Sprintf("Welcome to Game Master  ·  Provider: %s", cfg.LLM.Provider),
	})
	nv.AddEntry(narrative.Entry{
		Kind:    narrative.KindNPC,
		Speaker: "Innkeeper Brynn",
		Text:    "\"Ah, a traveller! You've arrived just in time — there's trouble on the east road.\"",
	})
	nv.AddEntry(narrative.Entry{
		Kind: narrative.KindPlayer,
		Text: "What kind of trouble?",
	})
	nv.AddEntry(narrative.Entry{
		Kind:    narrative.KindNPC,
		Speaker: "Innkeeper Brynn",
		Text:    "\"A merchant went missing three days ago. Cargo and all. Sheriff won't lift a finger.\"",
	})

	return App{cfg: cfg, router: router, viewState: ViewNarrative}
}

// ActiveViewState returns the currently active ViewState.
func (m App) ActiveViewState() ViewState {
	return m.viewState
}

// Init implements tea.Model. No start-up commands are needed.
func (m App) Init() tea.Cmd { return nil }

// Update implements tea.Model. It handles global key bindings (quit and view
// switching) and forwards all other messages to the active sub-model.
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.propagateSizes()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.router.NextTab()
			m.viewState = ViewState(m.router.ActiveTab())
		case "shift+tab", "left", "h":
			m.router.PrevTab()
			m.viewState = ViewState(m.router.ActiveTab())
		case "1", "2", "3", "4":
			idx := int(msg.String()[0] - '1')
			m.router.GoToTab(idx)
			m.viewState = ViewState(m.router.ActiveTab())
		default:
			cmd := m.router.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// View implements tea.Model and renders the full TUI chrome plus the active
// sub-view.
func (m App) View() string {
	titleBar, tabBar, statusBar := m.chrome()
	activeView := m.router.View()
	activeView = lipgloss.NewStyle().Width(m.width).Render(activeView)
	return styles.JoinVertical(titleBar, tabBar, activeView, statusBar)
}

// chrome renders the title bar, tab bar, and status bar at the current width.
func (m *App) chrome() (titleBar, tabBar, statusBar string) {
	titleBar = styles.TitleBar.Width(m.width).Render(
		"⚔  Game Master" + styles.Muted.Render(
			fmt.Sprintf("  ·  %s", m.cfg.LLM.Provider),
		),
	)
	tabBar = m.renderTabs()
	hints := styles.Muted.Render("tab/←/→ switch view  ·  1–4 jump to view  ·  q quit")
	statusBar = styles.StatusBar.Width(m.width).Render(hints)
	return
}

// propagateSizes pushes the current terminal dimensions down to all sub-views,
// accounting for the vertical space consumed by the chrome.
func (m *App) propagateSizes() {
	titleBar, tabBar, statusBar := m.chrome()

	reserved := lipgloss.Height(titleBar) + lipgloss.Height(tabBar) + lipgloss.Height(statusBar)
	viewHeight := m.height - reserved
	if viewHeight < 1 {
		viewHeight = 1
	}

	m.router.SetSize(m.width, viewHeight)
}

// renderTabs builds the tab-bar string for the current set of registered views.
func (m App) renderTabs() string {
	var tabs []string
	for i, tab := range m.router.Tabs() {
		label := fmt.Sprintf("%d %s", i+1, tab.Name)
		if i == m.router.ActiveTab() {
			tabs = append(tabs, styles.ActiveTab.Render(label))
		} else {
			tabs = append(tabs, styles.Tab.Render(label))
		}
	}
	return styles.JoinHorizontal(tabs...)
}
