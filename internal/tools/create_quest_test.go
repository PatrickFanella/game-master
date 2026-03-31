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

type stubQuestStore struct {
	locationsByID           map[[16]byte]statedb.Location
	questsByID              map[[16]byte]statedb.Quest
	createdQuest            statedb.Quest
	getLocationErr          error
	getQuestErr             error
	createQuestErr          error
	createObjectiveErr      error
	createRelationshipErr   error
	createQuestCalls        []statedb.CreateQuestParams
	createObjectiveCalls    []statedb.CreateObjectiveParams
	createRelationshipCalls []statedb.CreateRelationshipParams
}

func (s *stubQuestStore) GetLocationByID(_ context.Context, id pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	location, ok := s.locationsByID[id.Bytes]
	if !ok {
		return statedb.Location{}, errors.New("location not found")
	}
	return location, nil
}

func (s *stubQuestStore) CreateQuest(_ context.Context, arg statedb.CreateQuestParams) (statedb.Quest, error) {
	if s.createQuestErr != nil {
		return statedb.Quest{}, s.createQuestErr
	}
	s.createQuestCalls = append(s.createQuestCalls, arg)
	if s.createdQuest.ID.Valid {
		return s.createdQuest, nil
	}
	return statedb.Quest{
		ID:          dbutil.ToPgtype(uuid.New()),
		CampaignID:  arg.CampaignID,
		Title:       arg.Title,
		Description: arg.Description,
		QuestType:   arg.QuestType,
		Status:      arg.Status,
	}, nil
}

func (s *stubQuestStore) GetQuestByID(_ context.Context, id pgtype.UUID) (statedb.Quest, error) {
	if s.getQuestErr != nil {
		return statedb.Quest{}, s.getQuestErr
	}
	quest, ok := s.questsByID[id.Bytes]
	if !ok {
		return statedb.Quest{}, pgx.ErrNoRows
	}
	return quest, nil
}

func (s *stubQuestStore) CreateObjective(_ context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	if s.createObjectiveErr != nil {
		return statedb.QuestObjective{}, s.createObjectiveErr
	}
	s.createObjectiveCalls = append(s.createObjectiveCalls, arg)
	return statedb.QuestObjective{
		ID:          dbutil.ToPgtype(uuid.New()),
		QuestID:     arg.QuestID,
		Description: arg.Description,
		Completed:   arg.Completed,
		OrderIndex:  arg.OrderIndex,
	}, nil
}

func (s *stubQuestStore) CreateRelationship(_ context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	if s.createRelationshipErr != nil {
		return statedb.EntityRelationship{}, s.createRelationshipErr
	}
	s.createRelationshipCalls = append(s.createRelationshipCalls, arg)
	return statedb.EntityRelationship{
		ID:               dbutil.ToPgtype(uuid.New()),
		CampaignID:       arg.CampaignID,
		SourceEntityType: arg.SourceEntityType,
		SourceEntityID:   arg.SourceEntityID,
		TargetEntityType: arg.TargetEntityType,
		TargetEntityID:   arg.TargetEntityID,
		RelationshipType: arg.RelationshipType,
	}, nil
}

func TestRegisterCreateQuest(t *testing.T) {
	reg := NewRegistry()
	store := &stubQuestStore{}

	if err := RegisterCreateQuest(reg, store); err != nil {
		t.Fatalf("register create_quest: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createQuestToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createQuestToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"title", "description", "quest_type", "objectives"} {
		if _, exists := requiredSet[field]; !exists {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateQuestHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	questID := uuid.New()
	relatedEntityID := uuid.New()

	store := &stubQuestStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current Location",
			},
		},
		createdQuest: statedb.Quest{
			ID:          dbutil.ToPgtype(questID),
			CampaignID:  dbutil.ToPgtype(campaignID),
			Title:       "Secure the Outpost",
			Description: pgtype.Text{String: "Reinforce defenses.", Valid: true},
			QuestType:   string(domain.QuestTypeShortTerm),
			Status:      string(domain.QuestStatusActive),
		},
	}

	h := NewCreateQuestHandler(store)
	got, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), map[string]any{
		"title":       "Secure the Outpost",
		"description": "Reinforce defenses.",
		"quest_type":  "short_term",
		"objectives": []any{
			map[string]any{"description": "Fortify the walls", "order_index": 20},
			map[string]any{"description": "Gather timber", "order_index": 0},
			map[string]any{"description": "Recruit builders", "order_index": 10},
		},
		"related_entities": []any{
			map[string]any{"entity_type": "npc", "entity_id": relatedEntityID.String()},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.createQuestCalls) != 1 {
		t.Fatalf("CreateQuest call count = %d, want 1", len(store.createQuestCalls))
	}
	if store.createQuestCalls[0].Status != string(domain.QuestStatusActive) {
		t.Fatalf("quest status = %q, want active", store.createQuestCalls[0].Status)
	}
	if store.createQuestCalls[0].QuestType != string(domain.QuestTypeShortTerm) {
		t.Fatalf("quest type = %q, want short_term", store.createQuestCalls[0].QuestType)
	}

	if len(store.createObjectiveCalls) != 3 {
		t.Fatalf("CreateObjective call count = %d, want 3", len(store.createObjectiveCalls))
	}
	wantOrder := []int32{0, 10, 20}
	for i, call := range store.createObjectiveCalls {
		if call.OrderIndex != wantOrder[i] {
			t.Fatalf("createObjectiveCalls[%d].OrderIndex = %d, want %d", i, call.OrderIndex, wantOrder[i])
		}
	}

	if len(store.createRelationshipCalls) != 1 {
		t.Fatalf("CreateRelationship call count = %d, want 1", len(store.createRelationshipCalls))
	}
	rel := store.createRelationshipCalls[0]
	if rel.SourceEntityType != "location" {
		t.Fatalf("source_entity_type = %q, want location", rel.SourceEntityType)
	}
	if rel.TargetEntityType != "npc" {
		t.Fatalf("target_entity_type = %q, want npc", rel.TargetEntityType)
	}
	if rel.RelationshipType != "quest_related" {
		t.Fatalf("relationship_type = %q, want quest_related", rel.RelationshipType)
	}

	if got.Data["id"] != questID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], questID)
	}
	if got.Data["status"] != string(domain.QuestStatusActive) {
		t.Fatalf("result status = %v, want active", got.Data["status"])
	}
	if !strings.Contains(got.Narrative, "Quest \"Secure the Outpost\"") {
		t.Fatalf("narrative = %q, want quest summary", got.Narrative)
	}
}

func TestCreateQuestHandleQuestTypeVariants(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	store := &stubQuestStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
			},
		},
	}
	h := NewCreateQuestHandler(store)

	for _, questType := range []string{"short_term", "medium_term", "long_term"} {
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), map[string]any{
			"title":       "Quest " + questType,
			"description": "Description",
			"quest_type":  questType,
			"objectives": []any{
				map[string]any{"description": "Objective", "order_index": 0},
			},
		})
		if err != nil {
			t.Fatalf("quest_type %q: Handle error = %v", questType, err)
		}
	}
}

func TestCreateQuestValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	baseStore := &stubQuestStore{
		locationsByID: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(currentLocationID).Bytes: {
				ID:         dbutil.ToPgtype(currentLocationID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Current",
			},
		},
	}
	h := NewCreateQuestHandler(baseStore)

	baseArgs := map[string]any{
		"title":       "Escort the caravan",
		"description": "Get the caravan safely to the city.",
		"quest_type":  "medium_term",
		"objectives": []any{
			map[string]any{"description": "Leave at dawn", "order_index": 0},
		},
	}

	t.Run("missing required fields", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			want  string
		}{
			{field: "title", want: "title is required"},
			{field: "description", want: "description is required"},
			{field: "quest_type", want: "quest_type is required"},
			{field: "objectives", want: "objectives is required"},
		} {
			args := copyQuestArgs(baseArgs)
			delete(args, tc.field)

			_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("field %s: error = %v, want %q", tc.field, err, tc.want)
			}
		}
	})

	t.Run("invalid quest type", func(t *testing.T) {
		args := copyQuestArgs(baseArgs)
		args["quest_type"] = "epic"
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "quest_type must be one of") {
			t.Fatalf("error = %v, want invalid quest_type error", err)
		}
	})

	t.Run("missing current location context", func(t *testing.T) {
		_, err := h.Handle(context.Background(), copyQuestArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "requires current location id in context") {
			t.Fatalf("error = %v, want missing context error", err)
		}
	})

	t.Run("invalid related entity type", func(t *testing.T) {
		args := copyQuestArgs(baseArgs)
		args["related_entities"] = []any{
			map[string]any{"entity_type": "dragon", "entity_id": uuid.New().String()},
		}
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "related_entities[0].entity_type must be one of") {
			t.Fatalf("error = %v, want related entity type validation", err)
		}
	})
}

var _ QuestStore = (*stubQuestStore)(nil)

func copyQuestArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
