package campaign_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/tui/campaign"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// makeUUID builds a deterministic pgtype.UUID for tests.
func makeUUID(b byte) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{b}, Valid: true}
}

func makeCampaign(id byte, name, status string) statedb.Campaign {
	return statedb.Campaign{
		ID:     makeUUID(id),
		Name:   name,
		Status: status,
		CreatedBy: makeUUID(99),
	}
}

// compile-time check: Model must satisfy tea.Model.
var _ tea.Model = campaign.Model{}

func TestNew_ListContainsAllCampaignsPlusNewOption(t *testing.T) {
	campaigns := []statedb.Campaign{
		makeCampaign(1, "Alpha", "active"),
		makeCampaign(2, "Beta", "active"),
	}
	m := campaign.New(campaigns)
	m.SetSize(120, 40)

	view := m.View()
	if view == "" {
		t.Fatal("View() should return non-empty string")
	}
	// The model must include both campaign names and the new-campaign option.
	for _, name := range []string{"Alpha", "Beta", "New campaign"} {
		if !containsSubstr(view, name) {
			t.Errorf("expected %q to appear in the view", name)
		}
	}
}

func TestNew_EmptyListShowsNewCampaignOnly(t *testing.T) {
	m := campaign.New(nil)
	m.SetSize(120, 40)
	view := m.View()
	if !containsSubstr(view, "New campaign") {
		t.Error("expected 'New campaign' in view with empty campaign list")
	}
}

func TestInit_ReturnsNil(t *testing.T) {
	m := campaign.New(nil)
	if m.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestSetSize_DoesNotPanic(t *testing.T) {
	m := campaign.New(nil)
	m.SetSize(120, 40)
	// Should not panic.
}

func TestUpdate_SelectExistingCampaign(t *testing.T) {
	c := makeCampaign(1, "My Campaign", "active")
	m := campaign.New([]statedb.Campaign{c})

	// Press Enter to select the first item (the existing campaign).
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model == nil {
		t.Fatal("Update returned nil model")
	}
	if cmd == nil {
		t.Fatal("expected a command after selecting a campaign, got nil")
	}

	msg := cmd()
	sel, ok := msg.(campaign.SelectedMsg)
	if !ok {
		t.Fatalf("expected SelectedMsg, got %T", msg)
	}
	if sel.Campaign.ID != c.ID {
		t.Errorf("expected campaign ID %v, got %v", c.ID, sel.Campaign.ID)
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := campaign.New(nil)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m2 == nil {
		t.Fatal("Update returned nil model for WindowSizeMsg")
	}
	_ = cmd
}

// containsSubstr reports whether sub is a substring of s.
func containsSubstr(s, sub string) bool {
	return strings.Contains(s, sub)
}
