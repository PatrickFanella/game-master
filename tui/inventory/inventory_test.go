package inventory

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

// Compile-time checks: *Model must satisfy tea.Model and the sub-model
// interface (tea.Model + SetSize) expected by the Router.
var _ tea.Model = (*Model)(nil)
var _ interface {
	tea.Model
	SetSize(width, height int)
} = (*Model)(nil)

func testItems() []domain.Item {
	return []domain.Item{
		{ID: uuid.New(), CampaignID: uuid.New(), Name: "Iron Sword", ItemType: domain.ItemTypeWeapon, Equipped: true, Quantity: 1, Description: "A sturdy blade.", Rarity: "common"},
		{ID: uuid.New(), CampaignID: uuid.New(), Name: "Health Potion", ItemType: domain.ItemTypeConsumable, Equipped: false, Quantity: 3, Description: "Restores health.", Rarity: "common"},
		{ID: uuid.New(), CampaignID: uuid.New(), Name: "Old Map", ItemType: domain.ItemTypeQuest, Equipped: false, Quantity: 1, Description: "A weathered map.", Rarity: "uncommon"},
	}
}

func modelWithItems(t *testing.T) Model {
	t.Helper()
	m := New()
	m.SetSize(80, 24)
	updated, _ := m.Update(UpdateMsg{Items: testItems()})
	return *(updated.(*Model))
}

func TestNew_ReturnsModel(t *testing.T) {
	m := New()
	if m.width != 0 || m.height != 0 {
		t.Fatal("expected zero dimensions on a freshly created model")
	}
	if m.loaded {
		t.Fatal("model should not be loaded initially")
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", m.width, m.height)
	}
}

func TestInit_ReturnsNil(t *testing.T) {
	m := New()
	if m.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestUpdate_EscEmitsNavigateBackMsg(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command on Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(NavigateBackMsg); !ok {
		t.Fatalf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestView_EmptyState(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	// Loaded but no items.
	updated, _ := m.Update(UpdateMsg{Items: nil})
	got := updated.(*Model).View()
	if !strings.Contains(got, "empty") {
		t.Fatalf("expected empty inventory message, got:\n%s", got)
	}
}

func TestView_WithItems(t *testing.T) {
	m := modelWithItems(t)
	got := m.View()

	for _, want := range []string{"Iron Sword", "Health Potion", "Old Map"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected View to contain %q, got:\n%s", want, got)
		}
	}
}

func TestView_EquippedGrouping(t *testing.T) {
	m := modelWithItems(t)
	got := m.View()

	if !strings.Contains(got, "Equipped") {
		t.Fatal("expected Equipped section header")
	}
	if !strings.Contains(got, "Backpack") {
		t.Fatal("expected Backpack section header")
	}
	// Equipped items should have ★ indicator.
	if !strings.Contains(got, "★") {
		t.Fatal("expected ★ indicator for equipped items")
	}
}

func TestView_CursorNavigation(t *testing.T) {
	m := modelWithItems(t)
	// Initial cursor at 0. Move down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m2 := updated.(*Model)
	if m2.cursor != 1 {
		t.Fatalf("expected cursor at 1 after j, got %d", m2.cursor)
	}

	// Move up wraps to last.
	m3 := New()
	m3.SetSize(80, 24)
	up, _ := m3.Update(UpdateMsg{Items: testItems()})
	m3p := up.(*Model)
	upd, _ := m3p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m4 := upd.(*Model)
	if m4.cursor != len(testItems())-1 {
		t.Fatalf("expected cursor to wrap to %d, got %d", len(testItems())-1, m4.cursor)
	}
}

func TestView_ItemDetail(t *testing.T) {
	m := modelWithItems(t)
	got := m.View()

	// First item (cursor=0) is the equipped Iron Sword.
	if !strings.Contains(got, "A sturdy blade") {
		t.Fatalf("expected selected item description in detail section, got:\n%s", got)
	}
	if !strings.Contains(got, "Selected") {
		t.Fatalf("expected 'Selected' separator, got:\n%s", got)
	}
}

func TestView_QuantityDisplay(t *testing.T) {
	m := modelWithItems(t)
	got := m.View()

	if !strings.Contains(got, "×3") {
		t.Fatalf("expected '×3' for Health Potion quantity, got:\n%s", got)
	}
}

func TestView_ContainsInventoryHeader(t *testing.T) {
	m := modelWithItems(t)
	got := m.View()
	if !strings.Contains(got, "Inventory") {
		t.Fatalf("expected View to contain 'Inventory', got:\n%s", got)
	}
}

func TestView_ContainsEscHint(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Esc") {
		t.Fatalf("expected View to contain Esc hint, got:\n%s", got)
	}
}

func TestView_OnlyEquipped(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	items := []domain.Item{
		{ID: uuid.New(), CampaignID: uuid.New(), Name: "Shield", ItemType: domain.ItemTypeArmor, Equipped: true, Quantity: 1},
	}
	updated, _ := m.Update(UpdateMsg{Items: items})
	got := updated.(*Model).View()
	if !strings.Contains(got, "Equipped") {
		t.Fatal("expected Equipped section")
	}
	// Should not render Backpack section when all items are equipped.
	if strings.Contains(got, "Backpack") {
		t.Fatal("did not expect Backpack section when all items equipped")
	}
}
