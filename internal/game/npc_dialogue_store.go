package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// npcDialogueStore adapts statedb.Querier to the tools.NPCDialogueStore interface.
type npcDialogueStore struct {
	queries statedb.Querier
}

// NewNPCDialogueStore creates a tools.NPCDialogueStore backed by the given Querier.
func NewNPCDialogueStore(q statedb.Querier) tools.NPCDialogueStore {
	return &npcDialogueStore{queries: q}
}

func (s *npcDialogueStore) GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error) {
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

func (s *npcDialogueStore) LogNPCDialogue(ctx context.Context, entry tools.NPCDialogueLogEntry) error {
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
