package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	pgvector_go "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// --- stubs ---

type stubSearchEmbedder struct {
	err error
}

func (s *stubSearchEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	return make([]float32, DefaultVectorDimension), nil
}

func (s *stubSearchEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = make([]float32, DefaultVectorDimension)
	}
	return out, nil
}

type stubMemorySearchStore struct {
	params statedb.SearchMemoriesBySimilarityParams
	rows   []statedb.SearchMemoriesBySimilarityRow
	err    error
}

func (s *stubMemorySearchStore) SearchMemoriesBySimilarity(_ context.Context, arg statedb.SearchMemoriesBySimilarityParams) ([]statedb.SearchMemoriesBySimilarityRow, error) {
	s.params = arg
	if s.err != nil {
		return nil, s.err
	}
	return s.rows, nil
}

type stubFilteredMemorySearchStore struct {
	params statedb.SearchMemoriesWithFiltersParams
	rows   []statedb.SearchMemoriesWithFiltersRow
	err    error
}

func (s *stubFilteredMemorySearchStore) SearchMemoriesWithFilters(_ context.Context, arg statedb.SearchMemoriesWithFiltersParams) ([]statedb.SearchMemoriesWithFiltersRow, error) {
	s.params = arg
	if s.err != nil {
		return nil, s.err
	}
	return s.rows, nil
}

// --- helpers ---

func makeRow(id, campID uuid.UUID, content, mtype string, dist float64, ts time.Time) statedb.SearchMemoriesBySimilarityRow {
	return statedb.SearchMemoriesBySimilarityRow{
		ID:         dbutil.ToPgtype(id),
		CampaignID: dbutil.ToPgtype(campID),
		Content:    content,
		MemoryType: mtype,
		Embedding:  pgvector_go.NewVector(make([]float32, DefaultVectorDimension)),
		LocationID: pgtype.UUID{},
		InGameTime: pgtype.Text{},
		CreatedAt:  pgtype.Timestamptz{Time: ts, Valid: true},
		Distance:   dist,
	}
}

func makeFilteredRow(id, campID uuid.UUID, content, mtype string, dist float64, ts time.Time) statedb.SearchMemoriesWithFiltersRow {
	return statedb.SearchMemoriesWithFiltersRow{
		ID:         dbutil.ToPgtype(id),
		CampaignID: dbutil.ToPgtype(campID),
		Content:    content,
		MemoryType: mtype,
		Embedding:  pgvector_go.NewVector(make([]float32, DefaultVectorDimension)),
		CreatedAt:  pgtype.Timestamptz{Time: ts, Valid: true},
		Distance:   dist,
	}
}

// --- tests ---

func TestSearchSimilar_Success(t *testing.T) {
	campID := uuid.New()
	id1, id2 := uuid.New(), uuid.New()
	now := time.Now().Truncate(time.Microsecond)

	store := &stubMemorySearchStore{
		rows: []statedb.SearchMemoriesBySimilarityRow{
			makeRow(id1, campID, "battle at dawn", "event", 0.1, now),
			makeRow(id2, campID, "meeting the elder", "dialogue", 0.3, now.Add(-time.Hour)),
		},
	}
	s := NewSearcher(&stubSearchEmbedder{}, store)

	results, err := s.SearchSimilar(context.Background(), campID, "fight", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != id1 {
		t.Errorf("results[0].ID = %v, want %v", results[0].ID, id1)
	}
	if results[0].Content != "battle at dawn" {
		t.Errorf("results[0].Content = %q, want %q", results[0].Content, "battle at dawn")
	}
	if results[0].Distance != 0.1 {
		t.Errorf("results[0].Distance = %f, want 0.1", results[0].Distance)
	}
	if !results[0].CreatedAt.Equal(now) {
		t.Errorf("results[0].CreatedAt = %v, want %v", results[0].CreatedAt, now)
	}
	if results[1].MemoryType != "dialogue" {
		t.Errorf("results[1].MemoryType = %q, want %q", results[1].MemoryType, "dialogue")
	}
	// Verify params passed to store.
	if store.params.LimitCount != 10 {
		t.Errorf("store LimitCount = %d, want 10", store.params.LimitCount)
	}
}

func TestSearchSimilar_EmptyQuery(t *testing.T) {
	s := NewSearcher(&stubSearchEmbedder{}, &stubMemorySearchStore{})
	_, err := s.SearchSimilar(context.Background(), uuid.New(), "", 5)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	var emptyErr *ErrEmptyInput
	if !errors.As(err, &emptyErr) {
		t.Errorf("expected ErrEmptyInput, got %T: %v", err, err)
	}
}

func TestSearchSimilar_EmbedError(t *testing.T) {
	embedErr := errors.New("provider down")
	s := NewSearcher(&stubSearchEmbedder{err: embedErr}, &stubMemorySearchStore{})
	_, err := s.SearchSimilar(context.Background(), uuid.New(), "hello", 5)
	if err == nil {
		t.Fatal("expected error from embedder")
	}
	if !errors.Is(err, embedErr) {
		t.Errorf("expected wrapped embedErr, got %v", err)
	}
}

func TestSearchSimilar_StoreError(t *testing.T) {
	storeErr := errors.New("db timeout")
	store := &stubMemorySearchStore{err: storeErr}
	s := NewSearcher(&stubSearchEmbedder{}, store)
	_, err := s.SearchSimilar(context.Background(), uuid.New(), "hello", 5)
	if err == nil {
		t.Fatal("expected error from store")
	}
	if !errors.Is(err, storeErr) {
		t.Errorf("expected wrapped storeErr, got %v", err)
	}
}

func TestSearchSimilar_DefaultLimit(t *testing.T) {
	store := &stubMemorySearchStore{}
	s := NewSearcher(&stubSearchEmbedder{}, store)

	// limit = 0 should default to 5
	_, err := s.SearchSimilar(context.Background(), uuid.New(), "query", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.params.LimitCount != 5 {
		t.Errorf("LimitCount = %d, want 5 (default)", store.params.LimitCount)
	}

	// limit = -1 should also default to 5
	_, err = s.SearchSimilar(context.Background(), uuid.New(), "query", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.params.LimitCount != 5 {
		t.Errorf("LimitCount = %d, want 5 (default)", store.params.LimitCount)
	}
}

func TestSearchSimilar_EmptyResults(t *testing.T) {
	store := &stubMemorySearchStore{
		rows: []statedb.SearchMemoriesBySimilarityRow{}, // explicitly empty
	}
	s := NewSearcher(&stubSearchEmbedder{}, store)

	results, err := s.SearchSimilar(context.Background(), uuid.New(), "obscure", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}


func TestSearchWithFilters_AllFilters(t *testing.T) {
	campID := uuid.New()
	id1 := uuid.New()
	now := time.Now().Truncate(time.Microsecond)
	locID := uuid.New()
	npcID := uuid.New()
	start := now.Add(-24 * time.Hour)
	end := now

	fStore := &stubFilteredMemorySearchStore{
		rows: []statedb.SearchMemoriesWithFiltersRow{
			makeFilteredRow(id1, campID, "ambush in forest", "event", 0.2, now),
		},
	}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	results, err := s.SearchWithFilters(context.Background(), campID, "forest battle", SearchFilters{
		MemoryType: "event",
		LocationID: &locID,
		NPCID:      &npcID,
		StartTime:  &start,
		EndTime:    &end,
	}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != id1 {
		t.Errorf("result ID = %v, want %v", results[0].ID, id1)
	}
	// Verify all filter params were passed.
	if !fStore.params.MemoryType.Valid || fStore.params.MemoryType.String != "event" {
		t.Errorf("MemoryType = %v, want valid 'event'", fStore.params.MemoryType)
	}
	if !fStore.params.LocationID.Valid {
		t.Error("LocationID should be valid")
	}
	if dbutil.FromPgtype(fStore.params.LocationID) != locID {
		t.Errorf("LocationID = %v, want %v", dbutil.FromPgtype(fStore.params.LocationID), locID)
	}
	if !fStore.params.NpcID.Valid {
		t.Error("NpcID should be valid")
	}
	if dbutil.FromPgtype(fStore.params.NpcID) != npcID {
		t.Errorf("NpcID = %v, want %v", dbutil.FromPgtype(fStore.params.NpcID), npcID)
	}
	if !fStore.params.StartTime.Valid || !fStore.params.StartTime.Time.Equal(start) {
		t.Errorf("StartTime = %v, want %v", fStore.params.StartTime, start)
	}
	if !fStore.params.EndTime.Valid || !fStore.params.EndTime.Time.Equal(end) {
		t.Errorf("EndTime = %v, want %v", fStore.params.EndTime, end)
	}
	if fStore.params.LimitCount != 10 {
		t.Errorf("LimitCount = %d, want 10", fStore.params.LimitCount)
	}
}

func TestSearchWithFilters_NoFilters(t *testing.T) {
	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "query", SearchFilters{}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fStore.params.MemoryType.Valid {
		t.Error("MemoryType should be invalid (NULL)")
	}
	if fStore.params.LocationID.Valid {
		t.Error("LocationID should be invalid (NULL)")
	}
	if fStore.params.NpcID.Valid {
		t.Error("NpcID should be invalid (NULL)")
	}
	if fStore.params.StartTime.Valid {
		t.Error("StartTime should be invalid (NULL)")
	}
	if fStore.params.EndTime.Valid {
		t.Error("EndTime should be invalid (NULL)")
	}
}

func TestSearchWithFilters_MemoryTypeOnly(t *testing.T) {
	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "query", SearchFilters{
		MemoryType: "dialogue",
	}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fStore.params.MemoryType.Valid || fStore.params.MemoryType.String != "dialogue" {
		t.Errorf("MemoryType = %v, want valid 'dialogue'", fStore.params.MemoryType)
	}
	if fStore.params.LocationID.Valid {
		t.Error("LocationID should be invalid (NULL)")
	}
	if fStore.params.NpcID.Valid {
		t.Error("NpcID should be invalid (NULL)")
	}
}

func TestSearchWithFilters_CombinedFilters(t *testing.T) {
	locID := uuid.New()
	start := time.Now().Add(-48 * time.Hour).Truncate(time.Microsecond)
	end := time.Now().Truncate(time.Microsecond)

	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "query", SearchFilters{
		LocationID: &locID,
		StartTime:  &start,
		EndTime:    &end,
	}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fStore.params.LocationID.Valid {
		t.Error("LocationID should be valid")
	}
	if dbutil.FromPgtype(fStore.params.LocationID) != locID {
		t.Errorf("LocationID = %v, want %v", dbutil.FromPgtype(fStore.params.LocationID), locID)
	}
	if fStore.params.MemoryType.Valid {
		t.Error("MemoryType should be invalid (NULL)")
	}
	if fStore.params.NpcID.Valid {
		t.Error("NpcID should be invalid (NULL)")
	}
	if !fStore.params.StartTime.Valid || !fStore.params.StartTime.Time.Equal(start) {
		t.Errorf("StartTime = %v, want %v", fStore.params.StartTime, start)
	}
	if !fStore.params.EndTime.Valid || !fStore.params.EndTime.Time.Equal(end) {
		t.Errorf("EndTime = %v, want %v", fStore.params.EndTime, end)
	}
	if fStore.params.LimitCount != 3 {
		t.Errorf("LimitCount = %d, want 3", fStore.params.LimitCount)
	}
}

func TestSearchWithFilters_EmptyQuery(t *testing.T) {
	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "", SearchFilters{}, 5)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	var emptyErr *ErrEmptyInput
	if !errors.As(err, &emptyErr) {
		t.Errorf("expected ErrEmptyInput, got %T: %v", err, err)
	}
}

func TestSearchWithFilters_EmbedError(t *testing.T) {
	embedErr := errors.New("provider down")
	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{err: embedErr}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "hello", SearchFilters{}, 5)
	if err == nil {
		t.Fatal("expected error from embedder")
	}
	if !errors.Is(err, embedErr) {
		t.Errorf("expected wrapped embedErr, got %v", err)
	}
}

func TestSearchWithFilters_EmptyResults(t *testing.T) {
	fStore := &stubFilteredMemorySearchStore{
		rows: []statedb.SearchMemoriesWithFiltersRow{},
	}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	results, err := s.SearchWithFilters(context.Background(), uuid.New(), "obscure", SearchFilters{}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchWithFilters_NilFilteredStore(t *testing.T) {
	s := NewSearcher(&stubSearchEmbedder{}, &stubMemorySearchStore{})

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "hello", SearchFilters{}, 5)
	if err == nil {
		t.Fatal("expected error for nil filteredStore")
	}
	if got := err.Error(); got != "memory search: filtered store not configured" {
		t.Errorf("error = %q, want 'memory search: filtered store not configured'", got)
	}
}

func TestSearchWithFilters_DefaultLimit(t *testing.T) {
	fStore := &stubFilteredMemorySearchStore{}
	s := NewSearcherWithFilters(&stubSearchEmbedder{}, &stubMemorySearchStore{}, fStore)

	_, err := s.SearchWithFilters(context.Background(), uuid.New(), "query", SearchFilters{}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fStore.params.LimitCount != 5 {
		t.Errorf("LimitCount = %d, want 5 (default)", fStore.params.LimitCount)
	}
}