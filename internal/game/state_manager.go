package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// pgStateManager implements StateManager using pgx and sqlc.
type pgStateManager struct {
	db      statedb.DBTX
	queries statedb.Querier
	loader  *StateLoader
}

// NewStateManager creates a new StateManager backed by the given database connection.
func NewStateManager(db statedb.DBTX) StateManager {
	q := statedb.New(db)
	return &pgStateManager{
		db:      db,
		queries: q,
		loader:  NewStateLoader(q, db),
	}
}

// newStateManagerWithQuerier is used for testing with a mock Querier.
func newStateManagerWithQuerier(q statedb.Querier) *pgStateManager {
	return &pgStateManager{
		queries: q,
		loader:  NewStateLoader(q, nil),
	}
}

func (sm *pgStateManager) GetOrCreateDefaultUser(ctx context.Context) (*domain.User, error) {
	const defaultName = "Player"

	u, err := sm.queries.GetUserByName(ctx, defaultName)
	if err == nil {
		return userToDomain(u), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get default user: %w", err)
	}

	u, err = sm.queries.CreateUser(ctx, defaultName)
	if err != nil {
		return nil, fmt.Errorf("create default user: %w", err)
	}
	return userToDomain(u), nil
}

func (sm *pgStateManager) CreateCampaign(ctx context.Context, params CreateCampaignParams) (*domain.Campaign, error) {
	campaign, err := sm.queries.CreateCampaign(ctx, statedb.CreateCampaignParams{
		Name:        params.Name,
		Description: pgtype.Text{String: params.Description, Valid: params.Description != ""},
		Genre:       pgtype.Text{String: params.Genre, Valid: params.Genre != ""},
		Tone:        pgtype.Text{String: params.Tone, Valid: params.Tone != ""},
		Themes:      params.Themes,
		Status:      "active",
		CreatedBy:   dbutil.ToPgtype(params.UserID),
	})
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	c := campaignToDomain(campaign)
	return &c, nil
}

func (sm *pgStateManager) GatherState(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	return sm.loader.Load(ctx, campaignID)
}

const loadCampaignTimeSQL = `SELECT day, hour, minute FROM campaign_time WHERE campaign_id = $1`

func loadCampaignTime(ctx context.Context, db statedb.DBTX, campaignID pgtype.UUID) (*CampaignTime, error) {
	var ct CampaignTime
	err := db.QueryRow(ctx, loadCampaignTimeSQL, campaignID).Scan(&ct.Day, &ct.Hour, &ct.Minute)
	if err != nil {
		return nil, err
	}
	return &ct, nil
}

func (sm *pgStateManager) SaveSessionLog(ctx context.Context, log domain.SessionLog) error {
	if err := log.Validate(); err != nil {
		return fmt.Errorf("save session log validate: %w", err)
	}

	_, err := sm.queries.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:   dbutil.ToPgtype(log.CampaignID),
		TurnNumber:   int32(log.TurnNumber),
		PlayerInput:  log.PlayerInput,
		InputType:    string(log.InputType),
		LlmResponse:  log.LLMResponse,
		ToolCalls:    log.ToolCalls,
		LocationID:   dbutil.ToPgtype(uuidOrNil(log.LocationID)),
		NpcsInvolved: dbutil.UUIDsToPgtype(log.NPCsInvolved),
	})
	if err != nil {
		return fmt.Errorf("save session log: %w", err)
	}
	return nil
}

func uuidOrNil(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}


func (sm *pgStateManager) ListRecentSessionLogs(ctx context.Context, campaignID uuid.UUID, limit int) ([]domain.SessionLog, error) {
	logs, err := sm.queries.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: dbutil.ToPgtype(campaignID),
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent session logs: %w", err)
	}
	return sessionLogsToDomain(logs), nil
}

func (sm *pgStateManager) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := sm.queries.GetCampaignByID(ctx, dbutil.ToPgtype(campaignID))
	if err != nil {
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	c := campaignToDomain(campaign)
	return &c, nil
}

func sessionLogsToDomain(logs []statedb.SessionLog) []domain.SessionLog {
	if len(logs) == 0 {
		return nil
	}
	result := make([]domain.SessionLog, 0, len(logs))
	for i := len(logs) - 1; i >= 0; i-- {
		l := logs[i]
		result = append(result, domain.SessionLog{
			ID:           dbutil.FromPgtype(l.ID),
			CampaignID:   dbutil.FromPgtype(l.CampaignID),
			TurnNumber:   int(l.TurnNumber),
			PlayerInput:  l.PlayerInput,
			InputType:    domain.InputType(l.InputType),
			LLMResponse:  l.LlmResponse,
			ToolCalls:    append(json.RawMessage(nil), l.ToolCalls...),
			LocationID:   optionalUUID(l.LocationID),
			NPCsInvolved: pgUUIDsToUUIDs(l.NpcsInvolved),
			CreatedAt:    l.CreatedAt.Time,
		})
	}
	return result
}

func optionalUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	value := dbutil.FromPgtype(id)
	return &value
}

func pgUUIDsToUUIDs(ids []pgtype.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, dbutil.FromPgtype(id))
	}
	return out
}