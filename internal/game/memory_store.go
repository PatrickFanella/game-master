package game

import (
	"context"

	pgvector "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// memoryStore adapts statedb.Querier to the tools.MemoryStore interface.
type memoryStore struct {
	queries statedb.Querier
}

// NewMemoryStore creates a tools.MemoryStore backed by the given Querier.
func NewMemoryStore(q statedb.Querier) tools.MemoryStore {
	return &memoryStore{queries: q}
}

func (s *memoryStore) CreateMemory(ctx context.Context, params tools.CreateMemoryParams) error {
	_, err := s.queries.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID: dbutil.ToPgtype(params.CampaignID),
		Content:    params.Content,
		Embedding:  pgvector.NewVector(params.Embedding),
		MemoryType: params.MemoryType,
		Metadata:   params.Metadata,
	})
	return err
}
