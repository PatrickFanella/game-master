package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// npcService consolidates NPC-related persistence for both the update_npc and
// npc_dialogue tools.
type npcService struct {
	queries statedb.Querier
}

// NewNPCService creates a service that satisfies both tools.UpdateNPCStore and
// tools.NPCDialogueStore.
func NewNPCService(q statedb.Querier) *npcService {
	return &npcService{queries: q}
}

// --- shared method ---

func (s *npcService) GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error) {
	npc, err := s.queries.GetNPCByID(ctx, dbutil.ToPgtype(npcID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	domainNPC := npcToDomain(npc)
	return &domainNPC, nil
}

// --- tools.UpdateNPCStore methods ---

func (s *npcService) LocationExistsInCampaign(ctx context.Context, locationID, campaignID uuid.UUID) (bool, error) {
	location, err := s.queries.GetLocationByID(ctx, dbutil.ToPgtype(locationID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return dbutil.FromPgtype(location.CampaignID) == campaignID, nil
}

func (s *npcService) UpdateNPC(ctx context.Context, npc domain.NPC) (*domain.NPC, error) {
	updated, err := s.queries.UpdateNPC(ctx, statedb.UpdateNPCParams{
		Name:        npc.Name,
		Description: stringToPgText(npc.Description),
		Personality: stringToPgText(npc.Personality),
		Disposition: int32(npc.Disposition),
		LocationID:  dbutil.ToPgtype(uuidOrNil(npc.LocationID)),
		FactionID:   dbutil.ToPgtype(uuidOrNil(npc.FactionID)),
		Alive:       npc.Alive,
		Hp:          intOrNullInt4(npc.HP),
		Stats:       npc.Stats,
		Properties:  npc.Properties,
		ID:          dbutil.ToPgtype(npc.ID),
	})
	if err != nil {
		return nil, err
	}
	domainNPC := npcToDomain(updated)
	return &domainNPC, nil
}

func (s *npcService) GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error) {
	playerCharacter, err := s.queries.GetPlayerCharacterByID(ctx, dbutil.ToPgtype(playerCharacterID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	domainPlayerCharacter := playerCharacterToDomain(playerCharacter)
	return &domainPlayerCharacter, nil
}

func (s *npcService) CreateNPC(ctx context.Context, params tools.CreateNPCParams) (*domain.NPC, error) {
	created, err := s.queries.CreateNPC(ctx, statedb.CreateNPCParams{
		CampaignID:  dbutil.ToPgtype(params.CampaignID),
		Name:        params.Name,
		Description: stringToPgText(params.Description),
		Personality: stringToPgText(params.Personality),
		Disposition: int32(params.Disposition),
		LocationID:  dbutil.ToPgtype(uuidOrNil(params.LocationID)),
		FactionID:   dbutil.ToPgtype(uuidOrNil(params.FactionID)),
		Alive:       true,
		Stats:       params.Stats,
		Properties:  params.Properties,
	})
	if err != nil {
		return nil, err
	}
	domainNPC := npcToDomain(created)
	return &domainNPC, nil
}

func (s *npcService) ListNPCsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]domain.NPC, error) {
	npcs, err := s.queries.ListNPCsByCampaign(ctx, dbutil.ToPgtype(campaignID))
	if err != nil {
		return nil, err
	}
	out := make([]domain.NPC, 0, len(npcs))
	for _, npc := range npcs {
		out = append(out, npcToDomain(npc))
	}
	return out, nil
}

// --- tools.NPCDialogueStore methods ---

func (s *npcService) LogNPCDialogue(ctx context.Context, entry tools.NPCDialogueLogEntry) error {
	recentLogs, err := s.queries.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: dbutil.ToPgtype(entry.CampaignID),
		LimitCount: 1,
	})
	if err != nil {
		return fmt.Errorf("list recent session logs: %w", err)
	}

	turnNumber := int32(1)
	if len(recentLogs) > 0 {
		turnNumber = recentLogs[0].TurnNumber + 1
	}

	_, err = s.queries.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:   dbutil.ToPgtype(entry.CampaignID),
		TurnNumber:   turnNumber,
		PlayerInput:  entry.FormattedDialogue,
		InputType:    string(domain.Narrative),
		LlmResponse:  entry.FormattedDialogue,
		ToolCalls:    []byte("[]"),
		LocationID:   dbutil.ToPgtype(entry.LocationID),
		NpcsInvolved: dbutil.UUIDsToPgtype([]uuid.UUID{entry.NPCID}),
	})
	if err != nil {
		return fmt.Errorf("create session log: %w", err)
	}
	return nil
}

// --- helpers ---

func intOrNullInt4(value *int) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*value), Valid: true}
}

func stringToPgText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}
