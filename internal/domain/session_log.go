package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type InputType string

const (
	GameAction InputType = "game_action"
	MetaAction InputType = "meta_action"
	Narrative  InputType = "narrative"

	InputTypeGameAction InputType = GameAction
	InputTypeMeta       InputType = MetaAction
	InputTypeNarrative  InputType = Narrative
)

type SessionLog struct {
	ID           uuid.UUID
	CampaignID   uuid.UUID
	TurnNumber   int
	PlayerInput  string
	InputType    InputType
	LLMResponse  string
	ToolCalls    json.RawMessage
	LocationID   *uuid.UUID
	NPCsInvolved []uuid.UUID
	CreatedAt    time.Time
}

func (sl *SessionLog) Validate() error {
	if sl.CampaignID == uuid.Nil {
		return errors.New("session log campaign_id is required")
	}
	if sl.TurnNumber < 1 {
		return errors.New("session log turn_number must be positive")
	}
	if sl.PlayerInput == "" {
		return errors.New("session log player_input is required")
	}
	return nil
}
