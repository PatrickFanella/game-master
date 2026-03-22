package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/tui/character"
	"github.com/PatrickFanella/game-master/tui/inventory"
	"github.com/PatrickFanella/game-master/tui/narrative"
	"github.com/PatrickFanella/game-master/tui/quest"
	"github.com/PatrickFanella/game-master/tui/styles"
)

// tab identifies which view is currently active.
type tab int

const (
	tabNarrative tab = iota
	tabCharacter
	tabInventory
	tabQuest
	tabCount
)

var tabNames = [tabCount]string{"Narrative", "Character", "Inventory", "Quests"}

// appModel is the root Bubble Tea model that hosts all sub-views and handles
// global key bindings (tab switching, quit).
type appModel struct {
	cfg       config.Config
	activeTab tab
	width     int
	height    int

	narrativeView narrative.Model
	characterView character.Model
	inventoryView inventory.Model
	questView     quest.Model
}

func newApp(cfg config.Config) appModel {
	m := appModel{
		cfg:           cfg,
		narrativeView: narrative.New(),
		characterView: character.New(),
		inventoryView: inventory.New(),
		questView:     quest.New(),
	}

	// Seed the narrative log with example entries.
	m.narrativeView.AddEntry(narrative.Entry{
		Kind: narrative.KindSystem,
		Text: fmt.Sprintf("Welcome to Game Master  ·  Provider: %s", cfg.LLM.Provider),
	})
	m.narrativeView.AddEntry(narrative.Entry{
		Kind:    narrative.KindNPC,
		Speaker: "Innkeeper Brynn",
		Text:    "\"Ah, a traveller! You've arrived just in time — there's trouble on the east road.\"",
	})
	m.narrativeView.AddEntry(narrative.Entry{
		Kind: narrative.KindPlayer,
		Text: "What kind of trouble?",
	})
	m.narrativeView.AddEntry(narrative.Entry{
		Kind:    narrative.KindNPC,
		Speaker: "Innkeeper Brynn",
		Text:    "\"A merchant went missing three days ago. Cargo and all. Sheriff won't lift a finger.\"",
	})

	return m
}

func (m appModel) Init() tea.Cmd { return nil }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.activeTab = (m.activeTab + 1) % tabCount
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
		case "1":
			m.activeTab = tabNarrative
		case "2":
			m.activeTab = tabCharacter
		case "3":
			m.activeTab = tabInventory
		case "4":
			m.activeTab = tabQuest
		}
	}
	return m, nil
}

// chrome renders the title bar, tab bar, and status bar at the current width.
// Both propagateSizes and View call this to share a single rendering code path.
func (m *appModel) chrome() (titleBar, tabBar, statusBar string) {
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

// propagateSizes pushes the current terminal size down to all sub-views.
// It measures the actual rendered height of the title bar, tab bar, and status
// bar so the active view always fills the remaining space precisely.
func (m *appModel) propagateSizes() {
	titleBar, tabBar, statusBar := m.chrome()

	reserved := lipgloss.Height(titleBar) + lipgloss.Height(tabBar) + lipgloss.Height(statusBar)
	viewHeight := m.height - reserved
	if viewHeight < 1 {
		viewHeight = 1
	}

	m.narrativeView.SetSize(m.width, viewHeight)
	m.characterView.SetSize(m.width, viewHeight)
	m.inventoryView.SetSize(m.width, viewHeight)
	m.questView.SetSize(m.width, viewHeight)
}

func (m appModel) View() string {
	titleBar, tabBar, statusBar := m.chrome()

	var activeView string
	switch m.activeTab {
	case tabNarrative:
		activeView = m.narrativeView.View()
	case tabCharacter:
		activeView = m.characterView.View()
	case tabInventory:
		activeView = m.inventoryView.View()
	case tabQuest:
		activeView = m.questView.View()
	}

	// Pad/trim view to fill available width.
	activeView = lipgloss.NewStyle().Width(m.width).Render(activeView)

	return styles.JoinVertical(titleBar, tabBar, activeView, statusBar)
}

func (m appModel) renderTabs() string {
	var tabs []string
	for i := tab(0); i < tabCount; i++ {
		label := fmt.Sprintf("%d %s", i+1, tabNames[i])
		if i == m.activeTab {
			tabs = append(tabs, styles.ActiveTab.Render(label))
		} else {
			tabs = append(tabs, styles.Tab.Render(label))
		}
	}
	return styles.JoinHorizontal(tabs...)
}

func main() {
	cfg, err := config.Load(os.Getenv("GM_CONFIG"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		newApp(cfg),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
