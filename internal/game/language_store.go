package game

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// languageStore adapts statedb.Querier to the tools.LanguageStore interface.
type languageStore struct {
	queries statedb.Querier
}

// NewLanguageStore creates a tools.LanguageStore backed by the given Querier.
func NewLanguageStore(q statedb.Querier) tools.LanguageStore {
	return &languageStore{queries: q}
}

func (s *languageStore) CreateLanguage(ctx context.Context, params tools.CreateLanguageParams) (uuid.UUID, error) {
	lang, err := s.queries.CreateLanguage(ctx, statedb.CreateLanguageParams{
		CampaignID:         dbutil.ToPgtype(params.CampaignID),
		Name:               params.Name,
		Description:        pgtype.Text{String: params.Description, Valid: true},
		Phonology:          params.PhonologicalRules,
		Naming:             params.NamingConventions,
		Vocabulary:         params.SampleVocabulary,
		SpokenByFactionIds: dbutil.UUIDsToPgtype(params.SpokenByFactionIDs),
		SpokenByCultureIds: dbutil.UUIDsToPgtype(params.SpokenByCultureIDs),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(lang.ID), nil
}

func (s *languageStore) FactionBelongsToCampaign(ctx context.Context, factionID, campaignID uuid.UUID) (bool, error) {
	faction, err := s.queries.GetFactionByID(ctx, dbutil.ToPgtype(factionID))
	if err != nil {
		return false, fmt.Errorf("get faction: %w", err)
	}
	return faction.CampaignID == dbutil.ToPgtype(campaignID), nil
}

func (s *languageStore) CultureBelongsToCampaign(ctx context.Context, cultureID, campaignID uuid.UUID) (bool, error) {
	culture, err := s.queries.GetCultureByID(ctx, dbutil.ToPgtype(cultureID))
	if err != nil {
		return false, fmt.Errorf("get culture: %w", err)
	}
	return culture.CampaignID == dbutil.ToPgtype(campaignID), nil
}
