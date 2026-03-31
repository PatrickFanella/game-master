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

type stubUpdateQuestStore struct {
	questsByID             map[[16]byte]statedb.Quest
	objectivesByQuestID    map[[16]byte][]statedb.QuestObjective
	subquestsByParentID    map[[16]byte][]statedb.Quest
	getQuestErr            error
	updateQuestErr         error
	updateQuestStatusErr   error
	listObjectivesErr      error
	createObjectiveErr     error
	listQuestsErr          error
	updateQuestCalls       []statedb.UpdateQuestParams
	updateQuestStatusCalls []statedb.UpdateQuestStatusParams
	createObjectiveCalls   []statedb.CreateObjectiveParams
	listQuestsCalls        []pgtype.UUID
}

func (s *stubUpdateQuestStore) GetQuestByID(_ context.Context, id pgtype.UUID) (statedb.Quest, error) {
	if s.getQuestErr != nil {
		return statedb.Quest{}, s.getQuestErr
	}
	quest, ok := s.questsByID[id.Bytes]
	if !ok {
		return statedb.Quest{}, pgx.ErrNoRows
	}
	return quest, nil
}

func (s *stubUpdateQuestStore) UpdateQuest(_ context.Context, arg statedb.UpdateQuestParams) (statedb.Quest, error) {
	if s.updateQuestErr != nil {
		return statedb.Quest{}, s.updateQuestErr
	}
	s.updateQuestCalls = append(s.updateQuestCalls, arg)
	quest := s.questsByID[arg.ID.Bytes]
	quest.ParentQuestID = arg.ParentQuestID
	quest.Title = arg.Title
	quest.Description = arg.Description
	quest.QuestType = arg.QuestType
	quest.Status = arg.Status
	s.questsByID[arg.ID.Bytes] = quest
	return quest, nil
}

func (s *stubUpdateQuestStore) UpdateQuestStatus(_ context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	if s.updateQuestStatusErr != nil {
		return statedb.Quest{}, s.updateQuestStatusErr
	}
	s.updateQuestStatusCalls = append(s.updateQuestStatusCalls, arg)

	quest, ok := s.questsByID[arg.ID.Bytes]
	if ok {
		quest.Status = arg.Status
		s.questsByID[arg.ID.Bytes] = quest
		return quest, nil
	}

	for parentID, subquests := range s.subquestsByParentID {
		for i := range subquests {
			if subquests[i].ID == arg.ID {
				subquests[i].Status = arg.Status
				s.subquestsByParentID[parentID] = subquests
				return subquests[i], nil
			}
		}
	}

	return statedb.Quest{}, errors.New("quest not found for status update")
}

func (s *stubUpdateQuestStore) ListObjectivesByQuest(_ context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	if s.listObjectivesErr != nil {
		return nil, s.listObjectivesErr
	}
	return append([]statedb.QuestObjective(nil), s.objectivesByQuestID[questID.Bytes]...), nil
}

func (s *stubUpdateQuestStore) CreateObjective(_ context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	if s.createObjectiveErr != nil {
		return statedb.QuestObjective{}, s.createObjectiveErr
	}
	s.createObjectiveCalls = append(s.createObjectiveCalls, arg)
	created := statedb.QuestObjective{
		ID:          dbutil.ToPgtype(uuid.New()),
		QuestID:     arg.QuestID,
		Description: arg.Description,
		Completed:   arg.Completed,
		OrderIndex:  arg.OrderIndex,
	}
	questKey := arg.QuestID.Bytes
	s.objectivesByQuestID[questKey] = append(s.objectivesByQuestID[questKey], created)
	return created, nil
}

func (s *stubUpdateQuestStore) ListQuestsByCampaign(_ context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error) {
	if s.listQuestsErr != nil {
		return nil, s.listQuestsErr
	}
	s.listQuestsCalls = append(s.listQuestsCalls, campaignID)

	var quests []statedb.Quest
	for _, quest := range s.questsByID {
		if quest.CampaignID == campaignID {
			quests = append(quests, quest)
		}
	}
	for _, children := range s.subquestsByParentID {
		for _, quest := range children {
			if quest.CampaignID == campaignID {
				quests = append(quests, quest)
			}
		}
	}
	return quests, nil
}

func TestRegisterUpdateQuest(t *testing.T) {
	reg := NewRegistry()
	store := &stubUpdateQuestStore{}

	if err := RegisterUpdateQuest(reg, store); err != nil {
		t.Fatalf("register update_quest: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != updateQuestToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, updateQuestToolName)
	}
	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	if len(required) != 1 || required[0] != "quest_id" {
		t.Fatalf("required schema = %#v, want [quest_id]", required)
	}
}

func TestUpdateQuestHandleDescriptionAndObjectives(t *testing.T) {
	questID := uuid.New()
	questKey := dbutil.ToPgtype(questID).Bytes
	store := &stubUpdateQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			questKey: {
				ID:          dbutil.ToPgtype(questID),
				CampaignID:  dbutil.ToPgtype(uuid.New()),
				Title:       "Recover Relic",
				Description: pgtype.Text{String: "Old description", Valid: true},
				QuestType:   string(domain.QuestTypeShortTerm),
				Status:      string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			questKey: {
				{
					ID:          dbutil.ToPgtype(uuid.New()),
					QuestID:     dbutil.ToPgtype(questID),
					Description: "Find map",
					Completed:   false,
					OrderIndex:  2,
				},
			},
		},
		subquestsByParentID: map[[16]byte][]statedb.Quest{},
	}

	h := NewUpdateQuestHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id":           questID.String(),
		"description_update": "New mission briefing",
		"new_objectives": []any{
			"Travel to ruins",
			"Secure the artifact",
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.updateQuestCalls) != 1 {
		t.Fatalf("UpdateQuest call count = %d, want 1", len(store.updateQuestCalls))
	}
	if store.updateQuestCalls[0].Description.String != "New mission briefing" {
		t.Fatalf("updated description = %q, want New mission briefing", store.updateQuestCalls[0].Description.String)
	}

	if len(store.createObjectiveCalls) != 2 {
		t.Fatalf("CreateObjective call count = %d, want 2", len(store.createObjectiveCalls))
	}
	if store.createObjectiveCalls[0].OrderIndex != 3 || store.createObjectiveCalls[1].OrderIndex != 4 {
		t.Fatalf("objective order indices = [%d,%d], want [3,4]", store.createObjectiveCalls[0].OrderIndex, store.createObjectiveCalls[1].OrderIndex)
	}

	if got.Data["description"] != "New mission briefing" {
		t.Fatalf("result description = %v, want New mission briefing", got.Data["description"])
	}
	added, ok := got.Data["added_objectives"].([]map[string]any)
	if !ok {
		t.Fatalf("added_objectives type = %T, want []map[string]any", got.Data["added_objectives"])
	}
	if len(added) != 2 {
		t.Fatalf("added_objectives length = %d, want 2", len(added))
	}
}

func TestUpdateQuestHandleCascadeOnCompletedParent(t *testing.T) {
	parentQuestID := uuid.New()
	childActiveID := uuid.New()
	childDoneID := uuid.New()
	grandchildActiveID := uuid.New()
	campaignID := uuid.New()

	parentKey := dbutil.ToPgtype(parentQuestID).Bytes
	campaignPg := dbutil.ToPgtype(campaignID)
	childActiveKey := dbutil.ToPgtype(childActiveID).Bytes
	store := &stubUpdateQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			parentKey: {
				ID:          dbutil.ToPgtype(parentQuestID),
				CampaignID:  campaignPg,
				Title:       "Main Quest",
				Description: pgtype.Text{String: "Primary objective", Valid: true},
				QuestType:   string(domain.QuestTypeLongTerm),
				Status:      string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			parentKey: {},
		},
		subquestsByParentID: map[[16]byte][]statedb.Quest{
			parentKey: {
				{
					ID:            dbutil.ToPgtype(childActiveID),
					CampaignID:    campaignPg,
					ParentQuestID: pgtype.UUID{Bytes: parentKey, Valid: true},
					Title:         "Active Child",
					Status:        string(domain.QuestStatusActive),
				},
				{
					ID:            dbutil.ToPgtype(childDoneID),
					CampaignID:    campaignPg,
					ParentQuestID: pgtype.UUID{Bytes: parentKey, Valid: true},
					Title:         "Completed Child",
					Status:        string(domain.QuestStatusCompleted),
				},
			},
			childActiveKey: {
				{
					ID:            dbutil.ToPgtype(grandchildActiveID),
					CampaignID:    campaignPg,
					ParentQuestID: pgtype.UUID{Bytes: childActiveKey, Valid: true},
					Title:         "Grandchild Active",
					Status:        string(domain.QuestStatusActive),
				},
			},
		},
	}

	h := NewUpdateQuestHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id": parentQuestID.String(),
		"status":   "completed",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.updateQuestStatusCalls) != 3 {
		t.Fatalf("UpdateQuestStatus call count = %d, want 3", len(store.updateQuestStatusCalls))
	}
	if len(store.listQuestsCalls) != 1 {
		t.Fatalf("ListQuestsByCampaign call count = %d, want 1", len(store.listQuestsCalls))
	}
	if store.listQuestsCalls[0] != campaignPg {
		t.Fatalf("ListQuestsByCampaign campaign id = %v, want %v", store.listQuestsCalls[0], campaignPg)
	}
	if store.updateQuestStatusCalls[0].Status != string(domain.QuestStatusCompleted) {
		t.Fatalf("parent status update = %q, want completed", store.updateQuestStatusCalls[0].Status)
	}
	if store.updateQuestStatusCalls[1].Status != string(domain.QuestStatusAbandoned) {
		t.Fatalf("child cascade status = %q, want abandoned", store.updateQuestStatusCalls[1].Status)
	}
	if store.updateQuestStatusCalls[2].Status != string(domain.QuestStatusAbandoned) {
		t.Fatalf("grandchild cascade status = %q, want abandoned", store.updateQuestStatusCalls[2].Status)
	}

	cascaded, ok := got.Data["cascaded_subquests"].([]map[string]any)
	if !ok {
		t.Fatalf("cascaded_subquests type = %T, want []map[string]any", got.Data["cascaded_subquests"])
	}
	if len(cascaded) != 2 {
		t.Fatalf("cascaded_subquests len = %d, want 2", len(cascaded))
	}
}

func TestUpdateQuestHandleCascadeOnFailedParent(t *testing.T) {
	parentQuestID := uuid.New()
	childActiveID := uuid.New()
	campaignID := uuid.New()
	parentKey := dbutil.ToPgtype(parentQuestID).Bytes
	campaignPg := dbutil.ToPgtype(campaignID)

	store := &stubUpdateQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			parentKey: {
				ID:          dbutil.ToPgtype(parentQuestID),
				CampaignID:  campaignPg,
				Title:       "Main Quest",
				Description: pgtype.Text{String: "Primary objective", Valid: true},
				QuestType:   string(domain.QuestTypeLongTerm),
				Status:      string(domain.QuestStatusActive),
			},
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			parentKey: {},
		},
		subquestsByParentID: map[[16]byte][]statedb.Quest{
			parentKey: {
				{
					ID:            dbutil.ToPgtype(childActiveID),
					CampaignID:    campaignPg,
					ParentQuestID: pgtype.UUID{Bytes: parentKey, Valid: true},
					Title:         "Active Child",
					Status:        string(domain.QuestStatusActive),
				},
			},
		},
	}

	h := NewUpdateQuestHandler(store)
	got, err := h.Handle(context.Background(), map[string]any{
		"quest_id": parentQuestID.String(),
		"status":   "failed",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.updateQuestStatusCalls) != 2 {
		t.Fatalf("UpdateQuestStatus call count = %d, want 2", len(store.updateQuestStatusCalls))
	}
	if len(store.listQuestsCalls) != 1 {
		t.Fatalf("ListQuestsByCampaign call count = %d, want 1", len(store.listQuestsCalls))
	}
	if store.listQuestsCalls[0] != campaignPg {
		t.Fatalf("ListQuestsByCampaign campaign id = %v, want %v", store.listQuestsCalls[0], campaignPg)
	}
	if store.updateQuestStatusCalls[1].Status != string(domain.QuestStatusFailed) {
		t.Fatalf("child cascade status = %q, want failed", store.updateQuestStatusCalls[1].Status)
	}
	if got.Data["status"] != string(domain.QuestStatusFailed) {
		t.Fatalf("result status = %v, want failed", got.Data["status"])
	}
}

func TestUpdateQuestValidationAndErrors(t *testing.T) {
	questID := uuid.New()
	baseQuest := statedb.Quest{
		ID:          dbutil.ToPgtype(questID),
		CampaignID:  dbutil.ToPgtype(uuid.New()),
		Title:       "Quest",
		Description: pgtype.Text{String: "desc", Valid: true},
		QuestType:   string(domain.QuestTypeMediumTerm),
		Status:      string(domain.QuestStatusActive),
	}
	baseStore := &stubUpdateQuestStore{
		questsByID: map[[16]byte]statedb.Quest{
			dbutil.ToPgtype(questID).Bytes: baseQuest,
		},
		objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
			dbutil.ToPgtype(questID).Bytes: {},
		},
		subquestsByParentID: map[[16]byte][]statedb.Quest{},
	}
	h := NewUpdateQuestHandler(baseStore)

	t.Run("quest not found", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id": uuid.New().String(),
			"status":   "active",
		})
		if err == nil || !strings.Contains(err.Error(), "quest_id does not reference an existing quest") {
			t.Fatalf("error = %v, want not found validation", err)
		}
	})

	t.Run("requires at least one change field", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id": questID.String(),
		})
		if err == nil || !strings.Contains(err.Error(), "at least one of status, description_update, or new_objectives is required") {
			t.Fatalf("error = %v, want missing change field validation", err)
		}
	})

	t.Run("status validation", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id": questID.String(),
			"status":   "paused",
		})
		if err == nil || !strings.Contains(err.Error(), "status must be one of: active, completed, failed, abandoned") {
			t.Fatalf("error = %v, want status validation", err)
		}
	})

	t.Run("description whitespace validation", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id":           questID.String(),
			"description_update": "   ",
		})
		if err == nil || !strings.Contains(err.Error(), "description_update must not be empty or whitespace") {
			t.Fatalf("error = %v, want description whitespace validation", err)
		}
	})

	t.Run("new_objectives item validation", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id": questID.String(),
			"new_objectives": []any{
				"valid",
				"",
			},
		})
		if err == nil || !strings.Contains(err.Error(), "new_objectives[1] must be a non-empty string") {
			t.Fatalf("error = %v, want objective item validation", err)
		}
	})

	t.Run("new_objectives empty array validation", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id":       questID.String(),
			"new_objectives": []any{},
		})
		if err == nil || !strings.Contains(err.Error(), "new_objectives must contain at least one objective") {
			t.Fatalf("error = %v, want empty objectives validation", err)
		}
	})

	t.Run("no-op status does not cascade", func(t *testing.T) {
		activeChildID := uuid.New()
		campaignID := uuid.New()
		campaignPg := dbutil.ToPgtype(campaignID)
		parentKey := dbutil.ToPgtype(questID).Bytes

		store := &stubUpdateQuestStore{
			questsByID: map[[16]byte]statedb.Quest{
				parentKey: {
					ID:          dbutil.ToPgtype(questID),
					CampaignID:  campaignPg,
					Title:       "Quest",
					Description: pgtype.Text{String: "desc", Valid: true},
					QuestType:   string(domain.QuestTypeMediumTerm),
					Status:      string(domain.QuestStatusCompleted),
				},
			},
			objectivesByQuestID: map[[16]byte][]statedb.QuestObjective{
				parentKey: {},
			},
			subquestsByParentID: map[[16]byte][]statedb.Quest{
				parentKey: {
					{
						ID:            dbutil.ToPgtype(activeChildID),
						CampaignID:    campaignPg,
						ParentQuestID: pgtype.UUID{Bytes: parentKey, Valid: true},
						Title:         "Active Child",
						Status:        string(domain.QuestStatusActive),
					},
				},
			},
		}
		h := NewUpdateQuestHandler(store)
		_, err := h.Handle(context.Background(), map[string]any{
			"quest_id": questID.String(),
			"status":   "completed",
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(store.updateQuestStatusCalls) != 0 {
			t.Fatalf("UpdateQuestStatus call count = %d, want 0 for no-op status", len(store.updateQuestStatusCalls))
		}
		if len(store.listQuestsCalls) != 0 {
			t.Fatalf("ListQuestsByCampaign call count = %d, want 0 for no-op status", len(store.listQuestsCalls))
		}
	})
}

var _ UpdateQuestStore = (*stubUpdateQuestStore)(nil)
