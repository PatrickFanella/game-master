package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubCompleteObjectiveStore struct {
	questsByID             map[[16]byte]statedb.Quest
	objectivesByQuestID    map[[16]byte][]statedb.QuestObjective
	getQuestErr            error
	listObjectivesErr      error
	completeObjectiveErr   error
	updateQuestStatusErr   error
	completeObjectiveCalls []pgtype.UUID
	updateQuestStatusCalls []statedb.UpdateQuestStatusParams
}

func (s *stubCompleteObjectiveStore) GetQuestByID(_ context.Context, id pgtype.UUID) (statedb.Quest, error) {
	if s.getQuestErr != nil {
		return statedb.Quest{}, s.getQuestErr
	}
	quest, ok := s.questsByID[id.Bytes]
	if !ok {
		return statedb.Quest{}, errors.New("quest not found")
	}
	return quest, nil
}

func (s *stubCompleteObjectiveStore) ListObjectivesByQuest(_ context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	if s.listObjectivesErr != nil {
		return nil, s.listObjectivesErr
	}
	return append([]statedb.QuestObjective(nil), s.objectivesByQuestID[questID.Bytes]...), nil
}

func (s *stubCompleteObjectiveStore) CompleteObjective(_ context.Context, id pgtype.UUID) (statedb.QuestObjective, error) {
	if s.completeObjectiveErr != nil {
		return statedb.QuestObjective{}, s.completeObjectiveErr
	}
	s.completeObjectiveCalls = append(s.completeObjectiveCalls, id)
	for questID, objectives := range s.objectivesByQuestID {
		for i := range objectives {
			if objectives[i].ID == id {
				objectives[i].Completed = true
				s.objectivesByQuestID[questID] = objectives
				return objectives[i], nil
			}
		}
	}
	return statedb.QuestObjective{}, errors.New("objective not found")
}

func (s *stubCompleteObjectiveStore) UpdateQuestStatus(_ context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	if s.updateQuestStatusErr != nil {
		return statedb.Quest{}, s.updateQuestStatusErr
	}
	s.updateQuestStatusCalls = append(s.updateQuestStatusCalls, arg)
	quest := s.questsByID[arg.ID.Bytes]
	quest.Status = arg.Status
	s.questsByID[arg.ID.Bytes] = quest
	return quest, nil
}

func TestRegisterCompleteObjective(t *testing.T) {
	reg := NewRegistry()
	store := &stubCompleteObjectiveStore{}

	if err := RegisterCompleteObjective(reg, store); err != nil {
		t.Fatalf("register complete_objective: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != completeObjectiveToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, completeObjectiveToolName)
	}
}

func TestCompleteObjectiveByIDPartialProgress(t *testing.T) {
	questID := uuid.New()
	obj1ID := uuid.New()
	obj2ID := uuid.New()
	store := &stubCompleteObjectiveStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(questID).Bytes: {
				ID:     dbutil.ToPgtype(questID),
				Title:  "Find the Relic",
				Status: string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			dbutil.ToPgtype(questID).Bytes: {
				{
					ID:          dbutil.ToPgtype(obj1ID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Search the ruins",
					Completed:   false,
					OrderIndex:  0,
				},
				{
					ID:          dbutil.ToPgtype(obj2ID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Return to camp",
					Completed:   false,
					OrderIndex:  1,
				},
			},
		},
	}

	h := NewCompleteObjectiveHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id":     questID.String(),
		"objective_id": obj1ID.String(),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.completeObjectiveCalls) != 1 {
		t.Fatalf("CompleteObjective call count = %d, want 1", len(store.completeObjectiveCalls))
	}
	if got.Data["progress"] != "1/2" {
		t.Fatalf("progress = %v, want 1/2", got.Data["progress"])
	}
	if got.Data["all_objectives_complete"] != false {
		t.Fatalf("all_objectives_complete = %v, want false", got.Data["all_objectives_complete"])
	}
	if got.Data["quest_auto_completed"] != false {
		t.Fatalf("quest_auto_completed = %v, want false", got.Data["quest_auto_completed"])
	}
}

func TestCompleteObjectiveByDescriptionAutoCompleteQuest(t *testing.T) {
	questID := uuid.New()
	objID := uuid.New()
	store := &stubCompleteObjectiveStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(questID).Bytes: {
				ID:     dbutil.ToPgtype(questID),
				Title:  "Seal the Gate",
				Status: string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			dbutil.ToPgtype(questID).Bytes: {
				{
					ID:          dbutil.ToPgtype(objID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Defeat gate guardian",
					Completed:   false,
					OrderIndex:  0,
				},
			},
		},
	}

	h := NewCompleteObjectiveHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id":              questID.String(),
		"objective_description": "gate guardian",
		"auto_complete_quest":   true,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.updateQuestStatusCalls) != 1 {
		t.Fatalf("UpdateQuestStatus call count = %d, want 1", len(store.updateQuestStatusCalls))
	}
	if store.updateQuestStatusCalls[0].Status != string(domain.QuestStatusCompleted) {
		t.Fatalf("updated status = %q, want completed", store.updateQuestStatusCalls[0].Status)
	}
	if got.Data["quest_status"] != string(domain.QuestStatusCompleted) {
		t.Fatalf("quest_status = %v, want completed", got.Data["quest_status"])
	}
	if got.Data["all_objectives_complete"] != true {
		t.Fatalf("all_objectives_complete = %v, want true", got.Data["all_objectives_complete"])
	}
}

func TestCompleteObjectiveAllCompleteReadyWithoutAutoComplete(t *testing.T) {
	questID := uuid.New()
	objID := uuid.New()
	store := &stubCompleteObjectiveStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(questID).Bytes: {
				ID:     dbutil.ToPgtype(questID),
				Title:  "Rekindle Beacon",
				Status: string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			dbutil.ToPgtype(questID).Bytes: {
				{
					ID:          dbutil.ToPgtype(objID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Light the beacon",
					Completed:   false,
					OrderIndex:  0,
				},
			},
		},
	}

	h := NewCompleteObjectiveHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id":     questID.String(),
		"objective_id": objID.String(),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.updateQuestStatusCalls) != 0 {
		t.Fatalf("UpdateQuestStatus call count = %d, want 0", len(store.updateQuestStatusCalls))
	}
	if got.Data["quest_ready_for_completion"] != true {
		t.Fatalf("quest_ready_for_completion = %v, want true", got.Data["quest_ready_for_completion"])
	}
	if !strings.Contains(got.Narrative, "ready for completion") {
		t.Fatalf("narrative = %q, want ready for completion message", got.Narrative)
	}
}

func TestCompleteObjectiveValidationErrors(t *testing.T) {
	questID := uuid.New()
	objID := uuid.New()
	obj2ID := uuid.New()
	store := &stubCompleteObjectiveStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(questID).Bytes: {
				ID:     dbutil.ToPgtype(questID),
				Title:  "Validation Test Quest",
				Status: string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			dbutil.ToPgtype(questID).Bytes: {
				{
					ID:          dbutil.ToPgtype(objID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "First objective",
					Completed:   false,
					OrderIndex:  0,
				},
				{
					ID:          dbutil.ToPgtype(obj2ID),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Second objective",
					Completed:   false,
					OrderIndex:  1,
				},
			},
		},
	}
	h := NewCompleteObjectiveHandler(store)

	_, err := h.Handle(context.Background(), map[string]any{
		"quest_id":              questID.String(),
		"objective_id":          objID.String(),
		"objective_description": "First objective",
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one of objective_id or objective_description must be provided") {
		t.Fatalf("error = %v, want mutual exclusivity error", err)
	}

	_, err = h.Handle(context.Background(), map[string]any{
		"quest_id":              questID.String(),
		"objective_description": "objective",
	})
	if err == nil || !strings.Contains(err.Error(), "matches multiple objectives") {
		t.Fatalf("error = %v, want ambiguous description error", err)
	}
}
