package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

func TestMarshalAppliedToolCallsNilEncodesArray(t *testing.T) {
	got, err := marshalAppliedToolCalls(nil)
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls(nil) error = %v", err)
	}
	if string(got) != "[]" {
		t.Fatalf("expected empty JSON array, got %s", got)
	}
}

func TestMarshalAppliedToolCallsPreservesEntries(t *testing.T) {
	got, err := marshalAppliedToolCalls([]AppliedToolCall{{
		Tool:      "skill_check",
		Arguments: json.RawMessage(`{"skill":"stealth"}`),
		Result:    json.RawMessage(`{"success":true}`),
	}})
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls() error = %v", err)
	}
	if string(got) == "[]" {
		t.Fatal("expected non-empty marshaled tool calls")
	}
}

type testQuerier struct {
	statedb.Querier
	updateNPCCalled bool
}

func (q *testQuerier) GetLocationByID(_ context.Context, id pgtype.UUID) (statedb.Location, error) {
	return statedb.Location{
		ID: id,
	}, nil
}

func (q *testQuerier) GetNPCByID(_ context.Context, id pgtype.UUID) (statedb.Npc, error) {
	locationID := uuid.New()
	return statedb.Npc{
		ID:          id,
		CampaignID:  dbutil.ToPgtype(uuid.New()),
		Name:        "Runtime NPC",
		Description: pgtype.Text{String: "desc", Valid: true},
		Personality: pgtype.Text{String: "calm", Valid: true},
		Disposition: 0,
		LocationID:  dbutil.ToPgtype(locationID),
		Alive:       true,
	}, nil
}

func (q *testQuerier) UpdateNPC(_ context.Context, arg statedb.UpdateNPCParams) (statedb.Npc, error) {
	q.updateNPCCalled = true
	return statedb.Npc{
		ID:          arg.ID,
		CampaignID:  dbutil.ToPgtype(uuid.New()),
		Name:        arg.Name,
		Description: arg.Description,
		Personality: arg.Personality,
		Disposition: arg.Disposition,
		LocationID:  arg.LocationID,
		FactionID:   arg.FactionID,
		Alive:       arg.Alive,
		Hp:          arg.Hp,
		Stats:       arg.Stats,
		Properties:  arg.Properties,
	}, nil
}

type testProvider struct{}

func (p *testProvider) Complete(_ context.Context, _ []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	var foundUpdateNPC bool
	var foundUpdatePlayerStats bool
	var foundAddExperience bool
	var foundLevelUp bool
	var foundAddAbility bool
	var foundRemoveAbility bool
	for _, tool := range tools {
		if tool.Name == "update_npc" {
			foundUpdateNPC = true
		}
		if tool.Name == "update_player_stats" {
			foundUpdatePlayerStats = true
		}
		if tool.Name == "add_experience" {
			foundAddExperience = true
		}
		if tool.Name == "level_up" {
			foundLevelUp = true
		if tool.Name == "add_ability" {
			foundAddAbility = true
		}
		if tool.Name == "remove_ability" {
			foundRemoveAbility = true
		}
	}
	if !foundUpdateNPC {
		return nil, errors.New("update_npc tool not registered")
	}
	if !foundUpdatePlayerStats {
		return nil, errors.New("update_player_stats tool not registered")
	}
	if !foundAddExperience {
		return nil, errors.New("add_experience tool not registered")
	}
	if !foundLevelUp {
		return nil, errors.New("level_up tool not registered")
	if !foundAddAbility {
		return nil, errors.New("add_ability tool not registered")
	}
	if !foundRemoveAbility {
		return nil, errors.New("remove_ability tool not registered")
	}
	return &llm.Response{
		Content: "",
		ToolCalls: []llm.ToolCall{
			{
				ID:   "1",
				Name: "update_npc",
				Arguments: map[string]any{
					"npc_id":      uuid.New().String(),
					"description": "updated via runtime registration test",
				},
			},
		},
	}, nil
}

func (p *testProvider) Stream(_ context.Context, _ []llm.Message, _ []llm.Tool) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestNewRegistersUpdateNPCTool(t *testing.T) {
	queries := &testQuerier{}
	e := New(nil, queries, &testProvider{})

	_, applied, err := e.processor.ProcessWithRecovery(context.Background(), nil, e.assembler.Tools())
	if err != nil {
		t.Fatalf("ProcessWithRecovery: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("applied count = %d, want 1", len(applied))
	}
	if applied[0].Tool != "update_npc" {
		t.Fatalf("applied tool = %q, want update_npc", applied[0].Tool)
	}
	if !queries.updateNPCCalled {
		t.Fatal("expected UpdateNPC to be called by update_npc handler")
	}
}
