package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubReviseFactStore struct {
	factsByID       map[[16]byte]statedb.WorldFact
	supersedeCalled bool
	lastSupersede   statedb.SupersedeFactParams
	supersedeFact   statedb.WorldFact
	getFactErr      error
	supersedeErr    error
}

func (s *stubReviseFactStore) GetFactByID(_ context.Context, id pgtype.UUID) (statedb.WorldFact, error) {
	if s.getFactErr != nil {
		return statedb.WorldFact{}, s.getFactErr
	}
	if fact, ok := s.factsByID[id.Bytes]; ok {
		return fact, nil
	}
	return statedb.WorldFact{}, errors.New("fact not found")
}

func (s *stubReviseFactStore) SupersedeFact(_ context.Context, arg statedb.SupersedeFactParams) (statedb.WorldFact, error) {
	if s.supersedeErr != nil {
		return statedb.WorldFact{}, s.supersedeErr
	}
	s.supersedeCalled = true
	s.lastSupersede = arg
	if s.supersedeFact.ID.Valid {
		return s.supersedeFact, nil
	}
	// Build a plausible new fact from the old one.
	old := s.factsByID[arg.OldFactID.Bytes]
	newFactID := uuid.New()
	return statedb.WorldFact{
		ID:         dbutil.ToPgtype(newFactID),
		CampaignID: old.CampaignID,
		Fact:       arg.Fact,
		Category:   arg.Category,
		Source:     arg.Source,
	}, nil
}

func TestRegisterReviseFact(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterReviseFact(reg, &stubReviseFactStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}}); err != nil {
		t.Fatalf("register revise_fact: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != reviseFactToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, reviseFactToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"fact_id", "new_fact"} {
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestReviseFactHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	oldFactID := uuid.New()

	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:         dbutil.ToPgtype(oldFactID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Fact:       "The dragon is dormant.",
				Category:   "history",
				Source:     establishedSource,
			},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.3, 0.4}}

	h := NewReviseFactHandler(store, memStore, embedder)

	result, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "The dragon has awakened and threatens the region.",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}

	// Check SupersedeFact was called with correct params.
	if !store.supersedeCalled {
		t.Fatal("expected SupersedeFact to be called")
	}
	if store.lastSupersede.Fact != "The dragon has awakened and threatens the region." {
		t.Fatalf("SupersedeFact.Fact = %q, want revised text", store.lastSupersede.Fact)
	}
	if store.lastSupersede.Category != "history" {
		t.Fatalf("SupersedeFact.Category = %q, want %q inherited from old fact", store.lastSupersede.Category, "history")
	}
	if store.lastSupersede.Source != reviseFactToolName {
		t.Fatalf("SupersedeFact.Source = %q, want %q", store.lastSupersede.Source, reviseFactToolName)
	}
	if store.lastSupersede.OldFactID != dbutil.ToPgtype(oldFactID) {
		t.Fatal("SupersedeFact.OldFactID mismatch")
	}

	// Check result data links old and new facts.
	if result.Data["old_fact_id"] != oldFactID.String() {
		t.Fatalf("result.Data[old_fact_id] = %v, want %s", result.Data["old_fact_id"], oldFactID)
	}
	if result.Data["supersedes"] != oldFactID.String() {
		t.Fatalf("result.Data[supersedes] = %v, want %s", result.Data["supersedes"], oldFactID)
	}

	// Check memory was embedded for new fact.
	if !memStore.called {
		t.Fatal("expected CreateMemory to be called for new fact")
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if memStore.lastParams.CampaignID != campaignID {
		t.Fatal("CreateMemory CampaignID mismatch")
	}

	// Validate metadata JSON.
	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["old_fact_id"] != oldFactID.String() {
		t.Fatalf("metadata[old_fact_id] = %v, want %s", metadata["old_fact_id"], oldFactID)
	}
	if metadata["supersedes"] != oldFactID.String() {
		t.Fatalf("metadata[supersedes] = %v, want %s", metadata["supersedes"], oldFactID)
	}
}

func TestReviseFactHandleNoEmbedder(t *testing.T) {
	campaignID := uuid.New()
	oldFactID := uuid.New()

	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:         dbutil.ToPgtype(oldFactID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Fact:       "The king rules alone.",
				Category:   "politics",
				Source:     establishedSource,
			},
		},
	}

	h := NewReviseFactHandler(store, nil, nil)

	result, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "The king has been deposed by a council.",
	})
	if err != nil {
		t.Fatalf("Handle without embedder returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestReviseFactHandleAlreadySuperseded(t *testing.T) {
	oldFactID := uuid.New()
	supersedingID := uuid.New()

	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:           dbutil.ToPgtype(oldFactID),
				CampaignID:   dbutil.ToPgtype(uuid.New()),
				Fact:         "Old fact.",
				Category:     "history",
				Source:       establishedSource,
				SupersededBy: dbutil.ToPgtype(supersedingID),
			},
		},
	}

	h := NewReviseFactHandler(store, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "Attempted revision.",
	})
	if err == nil {
		t.Fatal("expected error when fact is already superseded")
	}
}

func TestReviseFactHandleMissingFactID(t *testing.T) {
	h := NewReviseFactHandler(&stubReviseFactStore{}, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{"new_fact": "Some text."})
	if err == nil {
		t.Fatal("expected error for missing fact_id")
	}
}

func TestReviseFactHandleMissingNewFact(t *testing.T) {
	h := NewReviseFactHandler(&stubReviseFactStore{}, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{"fact_id": uuid.New().String()})
	if err == nil {
		t.Fatal("expected error for missing new_fact")
	}
}

func TestReviseFactHandleGetFactError(t *testing.T) {
	h := NewReviseFactHandler(&stubReviseFactStore{getFactErr: errors.New("db error")}, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  uuid.New().String(),
		"new_fact": "Some text.",
	})
	if err == nil {
		t.Fatal("expected error when GetFactByID fails")
	}
}

func TestReviseFactHandleSupersedeError(t *testing.T) {
	oldFactID := uuid.New()
	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:       dbutil.ToPgtype(oldFactID),
				Fact:     "Old fact.",
				Category: "history",
				Source:   establishedSource,
			},
		},
		supersedeErr: errors.New("db error"),
	}

	h := NewReviseFactHandler(store, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "New fact.",
	})
	if err == nil {
		t.Fatal("expected error when SupersedeFact fails")
	}
}

func TestReviseFactHandleEmbedError(t *testing.T) {
	campaignID := uuid.New()
	oldFactID := uuid.New()
	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:         dbutil.ToPgtype(oldFactID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Fact:       "Old fact.",
				Category:   "history",
				Source:     establishedSource,
			},
		},
	}
	embedder := &stubEmbedder{err: errors.New("embed error")}

	h := NewReviseFactHandler(store, &stubMemoryStore{}, embedder)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "New fact.",
	})
	if err == nil {
		t.Fatal("expected error when Embed fails")
	}
}

func TestReviseFactRevisionChain(t *testing.T) {
	// Verify a chain: fact1 -> fact2 -> fact3 (each supersedes the previous)
	campaignID := uuid.New()
	fact1ID := uuid.New()
	fact2ID := uuid.New()

	// fact1 is already superseded by fact2.
	// fact2 is active (not yet superseded).
	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(fact1ID).Bytes: {
				ID:           dbutil.ToPgtype(fact1ID),
				CampaignID:   dbutil.ToPgtype(campaignID),
				Fact:         "Fact 1.",
				Category:     "history",
				Source:       establishedSource,
				SupersededBy: dbutil.ToPgtype(fact2ID),
			},
			dbutil.ToPgtype(fact2ID).Bytes: {
				ID:         dbutil.ToPgtype(fact2ID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Fact:       "Fact 2.",
				Category:   "history",
				Source:     establishedSource,
			},
		},
	}

	h := NewReviseFactHandler(store, nil, nil)

	// Attempting to revise fact1 (already superseded) should fail.
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  fact1ID.String(),
		"new_fact": "Fact 3.",
	})
	if err == nil {
		t.Fatal("expected error when trying to revise already-superseded fact1")
	}

	// Revising fact2 (active) should succeed.
	result, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  fact2ID.String(),
		"new_fact": "Fact 3.",
	})
	if err != nil {
		t.Fatalf("revise active fact2: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success revising fact2")
	}
	if result.Data["old_fact_id"] != fact2ID.String() {
		t.Fatalf("old_fact_id = %v, want %s", result.Data["old_fact_id"], fact2ID)
	}
}

func TestRegisterReviseFactNilStore(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterReviseFact(reg, nil, nil, nil); err == nil {
		t.Fatal("expected error when registering with nil factStore")
	}
}

func TestReviseFactHandleNilHandler(t *testing.T) {
	var h *ReviseFactHandler
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  uuid.New().String(),
		"new_fact": "some text",
	})
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestReviseFactHandleNilFactStore(t *testing.T) {
	h := &ReviseFactHandler{}
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  uuid.New().String(),
		"new_fact": "some text",
	})
	if err == nil {
		t.Fatal("expected error for nil factStore")
	}
}

func TestReviseFactHandleFactNotFound(t *testing.T) {
	store := &stubReviseFactStore{getFactErr: pgx.ErrNoRows}
	h := NewReviseFactHandler(store, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  uuid.New().String(),
		"new_fact": "revised text",
	})
	if err == nil {
		t.Fatal("expected error when fact does not exist")
	}
	if err.Error() != "fact_id does not reference an existing world fact" {
		t.Fatalf("error = %q, want user-facing not-found message", err.Error())
	}
}

func TestReviseFactHandleSupersedeNoRows(t *testing.T) {
	// SupersedeFact returning ErrNoRows means the fact was superseded concurrently.
	oldFactID := uuid.New()
	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:       dbutil.ToPgtype(oldFactID),
				Fact:     "some fact",
				Category: "history",
				Source:   establishedSource,
			},
		},
		supersedeErr: pgx.ErrNoRows,
	}

	h := NewReviseFactHandler(store, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "revised text",
	})
	if err == nil {
		t.Fatal("expected error when SupersedeFact returns no rows")
	}
	if err.Error() == pgx.ErrNoRows.Error() {
		t.Fatalf("error message should be user-facing, got raw pgx error: %v", err)
	}
}

func TestReviseFactSourceIsReviseFactTool(t *testing.T) {
	// Verify that revise_fact uses reviseFactToolName as source, not "established".
	campaignID := uuid.New()
	oldFactID := uuid.New()
	store := &stubReviseFactStore{
		factsByID: map[[16]byte]statedb.WorldFact{
			dbutil.ToPgtype(oldFactID).Bytes: {
				ID:         dbutil.ToPgtype(oldFactID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Fact:       "Old fact.",
				Category:   "politics",
				Source:     establishedSource,
			},
		},
	}

	h := NewReviseFactHandler(store, nil, nil)
	result, err := h.Handle(context.Background(), map[string]any{
		"fact_id":  oldFactID.String(),
		"new_fact": "New fact.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastSupersede.Source != reviseFactToolName {
		t.Fatalf("SupersedeFact.Source = %q, want %q", store.lastSupersede.Source, reviseFactToolName)
	}
	if result.Data["source"] != reviseFactToolName {
		t.Fatalf("result.Data[source] = %v, want %q", result.Data["source"], reviseFactToolName)
	}
}
