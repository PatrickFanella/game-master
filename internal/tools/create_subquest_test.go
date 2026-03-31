package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

func TestRegisterCreateSubquest(t *testing.T) {
	reg := NewRegistry()
	store := &stubQuestStore{}

	if err := RegisterCreateSubquest(reg, store); err != nil {
		t.Fatalf("register create_subquest: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createSubquestToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createSubquestToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"parent_quest_id", "title", "description", "quest_type", "objectives"} {
		if _, exists := requiredSet[field]; !exists {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateSubquestHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	parentQuestID := uuid.New()
	subquestID := uuid.New()

	store := &stubQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(parentQuestID).Bytes: {
				ID:          dbutil.ToPgtype(parentQuestID),
				CampaignID:  dbutil.ToPgtype(campaignID),
				Title:       "Main Quest",
				Description: pgtype.Text{String: "Main", Valid: true},
				QuestType:   string(domain.QuestTypeLongTerm),
				Status:      string(domain.QuestStatusActive),
			},
		},
		createdQuest: statedb.Quest{
			ID:            dbutil.ToPgtype(subquestID),
			CampaignID:    dbutil.ToPgtype(campaignID),
			ParentQuestID: pgtype.UUID{Bytes: dbutil.ToPgtype(parentQuestID).Bytes, Valid: true},
			Title:         "Scout the area",
			Description:   pgtype.Text{String: "Find enemy movement.", Valid: true},
			QuestType:     string(domain.QuestTypeShortTerm),
			Status:        string(domain.QuestStatusActive),
		},
	}

	h := NewCreateSubquestHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"parent_quest_id": parentQuestID.String(),
		"title":           "Scout the area",
		"description":     "Find enemy movement.",
		"quest_type":      "short_term",
		"objectives": []any{
			map[string]any{"description": "Report to captain", "order_index": 2},
			map[string]any{"description": "Search forest edge", "order_index": 1},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.createQuestCalls) != 1 {
		t.Fatalf("CreateQuest call count = %d, want 1", len(store.createQuestCalls))
	}
	createCall := store.createQuestCalls[0]
	if !createCall.ParentQuestID.Valid {
		t.Fatal("ParentQuestID.Valid = false, want true")
	}
	if createCall.ParentQuestID.Bytes != dbutil.ToPgtype(parentQuestID).Bytes {
		t.Fatalf("parent quest id = %v, want %v", createCall.ParentQuestID.Bytes, dbutil.ToPgtype(parentQuestID).Bytes)
	}
	if createCall.CampaignID != dbutil.ToPgtype(campaignID) {
		t.Fatalf("campaign id = %v, want %v", createCall.CampaignID, dbutil.ToPgtype(campaignID))
	}

	if len(store.createObjectiveCalls) != 2 {
		t.Fatalf("CreateObjective call count = %d, want 2", len(store.createObjectiveCalls))
	}
	if store.createObjectiveCalls[0].OrderIndex != 1 || store.createObjectiveCalls[1].OrderIndex != 2 {
		t.Fatalf("objective order = [%d,%d], want [1,2]", store.createObjectiveCalls[0].OrderIndex, store.createObjectiveCalls[1].OrderIndex)
	}

	if got.Data["id"] != subquestID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], subquestID)
	}
	if got.Data["parent_quest_id"] != parentQuestID.String() {
		t.Fatalf("result parent_quest_id = %v, want %s", got.Data["parent_quest_id"], parentQuestID)
	}
}

func TestCreateSubquestInvalidParent(t *testing.T) {
	h := NewCreateSubquestHandler(&stubQuestStore{})
	_, err := h.Handle(context.Background(), map[string]any{
		"parent_quest_id": uuid.New().String(),
		"title":           "Scout the area",
		"description":     "Find enemy movement.",
		"quest_type":      "short_term",
		"objectives": []any{
			map[string]any{"description": "Search forest edge", "order_index": 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "parent quest not found") {
		t.Fatalf("error = %v, want parent quest not found", err)
	}
}

func TestCreateSubquestCompletedParent(t *testing.T) {
	parentQuestID := uuid.New()
	store := &stubQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(parentQuestID).Bytes: {
				ID:          dbutil.ToPgtype(parentQuestID),
				CampaignID:  dbutil.ToPgtype(uuid.New()),
				Title:       "Main Quest",
				Description: pgtype.Text{String: "Main", Valid: true},
				QuestType:   string(domain.QuestTypeLongTerm),
				Status:      string(domain.QuestStatusCompleted),
			},
		},
	}

	h := NewCreateSubquestHandler(store)
	_, err := h.Handle(context.Background(), map[string]any{
		"parent_quest_id": parentQuestID.String(),
		"title":           "Scout the area",
		"description":     "Find enemy movement.",
		"quest_type":      "short_term",
		"objectives": []any{
			map[string]any{"description": "Search forest edge", "order_index": 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "parent quest must be active") {
		t.Fatalf("error = %v, want parent quest active error", err)
	}
}

func TestCreateSubquestParentLookupError(t *testing.T) {
	h := NewCreateSubquestHandler(&stubQuestStore{getQuestErr: errors.New("db down")})
	_, err := h.Handle(context.Background(), map[string]any{
		"parent_quest_id": uuid.New().String(),
		"title":           "Scout the area",
		"description":     "Find enemy movement.",
		"quest_type":      "short_term",
		"objectives": []any{
			map[string]any{"description": "Search forest edge", "order_index": 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "parent quest lookup failed") {
		t.Fatalf("error = %v, want parent quest lookup failed wrapping", err)
	}
}

func TestCreateSubquestValidationAndParentErrors(t *testing.T) {
	parentQuestID := uuid.New()
	store := &stubQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(parentQuestID).Bytes: {
				ID:          dbutil.ToPgtype(parentQuestID),
				CampaignID:  dbutil.ToPgtype(uuid.New()),
				Title:       "Main Quest",
				Description: pgtype.Text{String: "Main", Valid: true},
				QuestType:   string(domain.QuestTypeLongTerm),
				Status:      string(domain.QuestStatusActive),
			},
		},
	}
	h := NewCreateSubquestHandler(store)

	baseArgs := map[string]any{
		"parent_quest_id": parentQuestID.String(),
		"title":           "Scout the area",
		"description":     "Find enemy movement.",
		"quest_type":      "short_term",
		"objectives": []any{
			map[string]any{"description": "Search forest edge", "order_index": 1},
		},
	}

	t.Run("parent not found from no rows", func(t *testing.T) {
		h := NewCreateSubquestHandler(&stubQuestStore{getQuestErr: pgx.ErrNoRows})
		_, err := h.Handle(context.Background(), copyQuestArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "parent quest not found") {
			t.Fatalf("error = %v, want parent quest not found", err)
		}
	})

	t.Run("reject whitespace title", func(t *testing.T) {
		args := copyQuestArgs(baseArgs)
		args["title"] = "   "
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "title must not be empty or whitespace") {
			t.Fatalf("error = %v, want whitespace title validation", err)
		}
	})

	t.Run("reject whitespace description", func(t *testing.T) {
		args := copyQuestArgs(baseArgs)
		args["description"] = "   "
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "description must not be empty or whitespace") {
			t.Fatalf("error = %v, want whitespace description validation", err)
		}
	})
}
