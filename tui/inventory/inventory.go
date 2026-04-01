// Package inventory provides the inventory view for the TUI.
// It renders the player's items grouped by equipped/backpack with cursor
// navigation and item detail expansion.
package inventory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/tui/styles"
)

const (
	containerHPad = 4
	containerVPad = 2
)

// NavigateBackMsg is emitted when the user presses Escape to return to the
// narrative view.
type NavigateBackMsg struct{}

// UpdateMsg delivers fresh inventory data to the view.
type UpdateMsg struct {
	Items []domain.Item
}

// Model is the Bubble Tea model for the inventory view.
type Model struct {
	width, height int
	items         []domain.Item
	cursor        int
	loaded        bool
	viewport      viewport.Model
}

// New returns a freshly initialised inventory Model.
func New() Model {
	vp := viewport.New(40, 1)
	return Model{viewport: vp}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	vpWidth := width - containerHPad
	vpHeight := height - containerVPad - 2
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
	if m.loaded {
		m.refreshContent()
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UpdateMsg:
		m.items = msg.Items
		m.loaded = true
		ordered := m.allItemsOrdered()
		if m.cursor >= len(ordered) {
			m.cursor = max(0, len(ordered)-1)
		}
		m.refreshContent()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return NavigateBackMsg{} }
		case "j", "down":
			ordered := m.allItemsOrdered()
			if len(ordered) > 0 {
				m.cursor = (m.cursor + 1) % len(ordered)
				m.refreshContent()
			}
			return m, nil
		case "k", "up":
			ordered := m.allItemsOrdered()
			if len(ordered) > 0 {
				m.cursor = (m.cursor - 1 + len(ordered)) % len(ordered)
				m.refreshContent()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model and renders the inventory.
func (m Model) View() string {
	title := styles.Header.Render("🎒 Inventory")
	if m.loaded && len(m.items) > 0 {
		title = styles.Header.Render(fmt.Sprintf("🎒 Inventory (%d items)", len(m.items)))
	}

	if !m.loaded || len(m.items) == 0 {
		msg := "Your inventory is empty."
		if !m.loaded {
			msg = "No inventory data available."
		}
		placeholder := styles.SystemMessage.Render(msg)
		hint := styles.Muted.Render("Press Esc to return to the narrative view.")
		content := styles.JoinVertical(placeholder, "", hint)
		return styles.Container.Width(m.width).Render(
			styles.JoinVertical(title, "", content),
		)
	}

	hint := styles.Muted.Render("↑/↓ navigate · Esc return")
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", m.viewport.View(), "", hint),
	)
}

// refreshContent rebuilds the viewport content from current item data.
func (m *Model) refreshContent() {
	equipped, backpack := m.splitItems()
	ordered := m.allItemsOrdered()
	var lines []string
	idx := 0

	if len(equipped) > 0 {
		lines = append(lines, styles.SubHeader.Render("◆ Equipped"))
		for _, item := range equipped {
			lines = append(lines, m.renderItemLine(item, idx, true))
			idx++
		}
		lines = append(lines, "")
	}

	if len(backpack) > 0 {
		lines = append(lines, styles.SubHeader.Render("◆ Backpack"))
		for _, item := range backpack {
			lines = append(lines, m.renderItemLine(item, idx, false))
			idx++
		}
		lines = append(lines, "")
	}

	// Selected item detail.
	if m.cursor >= 0 && m.cursor < len(ordered) {
		selected := ordered[m.cursor]
		lines = append(lines, styles.Muted.Render("─── Selected ───"))
		lines = append(lines, styles.SubHeader.Render(selected.Name))
		if selected.Description != "" {
			lines = append(lines, styles.Body.Render(selected.Description))
		}
		if selected.Rarity != "" {
			lines = append(lines, fmt.Sprintf("Rarity: %s", styles.Muted.Render(selected.Rarity)))
		}
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *Model) renderItemLine(item domain.Item, idx int, equipped bool) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = "> "
	}

	prefix := "  "
	if equipped {
		prefix = styles.StatusWarning.Render("★ ")
	}

	name := item.Name
	if item.Quantity > 1 {
		name = fmt.Sprintf("%s ×%d", item.Name, item.Quantity)
	}

	badge := itemTypeBadge(item.ItemType)
	return fmt.Sprintf("%s%s%s  %s", cursor, prefix, name, badge)
}

// splitItems separates items into equipped and backpack groups, sorted by type then name.
func (m *Model) splitItems() (equipped, backpack []domain.Item) {
	for _, item := range m.items {
		if item.Equipped {
			equipped = append(equipped, item)
		} else {
			backpack = append(backpack, item)
		}
	}
	sortItems(equipped)
	sortItems(backpack)
	return
}

// allItemsOrdered returns all items in display order: equipped first, then backpack.
func (m *Model) allItemsOrdered() []domain.Item {
	equipped, backpack := m.splitItems()
	return append(equipped, backpack...)
}

func sortItems(items []domain.Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ItemType != items[j].ItemType {
			return items[i].ItemType < items[j].ItemType
		}
		return items[i].Name < items[j].Name
	})
}

// itemTypeBadge returns a colored badge like [weap] for the item type.
func itemTypeBadge(t domain.ItemType) string {
	var label string
	var color lipgloss.AdaptiveColor
	switch t {
	case domain.ItemTypeWeapon:
		label, color = "weap", styles.ColorError
	case domain.ItemTypeArmor:
		label, color = "armor", styles.ColorInfo
	case domain.ItemTypeConsumable:
		label, color = "cons", styles.ColorSuccess
	case domain.ItemTypeQuest:
		label, color = "quest", styles.ColorWarning
	default:
		label, color = "misc", styles.ColorMuted
	}
	return lipgloss.NewStyle().Foreground(color).Render("[" + label + "]")
}
