package game

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// updateNPCStore adapts statedb.Querier to the tools.UpdateNPCStore interface.
type updateNPCStore struct {
	queries statedb.Querier
}

var _ tools.UpdateNPCStore = (*updateNPCStore)(nil)

// NewUpdateNPCStore creates a tools.UpdateNPCStore backed by the given Querier.
func NewUpdateNPCStore(q statedb.Querier) tools.UpdateNPCStore {
	return &updateNPCStore{queries: q}
}

func (s *updateNPCStore) GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error) {
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

func (s *updateNPCStore) LocationExistsInCampaign(ctx context.Context, locationID, campaignID uuid.UUID) (bool, error) {
	location, err := s.queries.GetLocationByID(ctx, dbutil.ToPgtype(locationID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return dbutil.FromPgtype(location.CampaignID) == campaignID, nil
}

func (s *updateNPCStore) UpdateNPC(ctx context.Context, npc domain.NPC) (*domain.NPC, error) {
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
