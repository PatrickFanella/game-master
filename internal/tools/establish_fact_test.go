package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubEstablishFactStore struct {
	currentLocation statedb.Location
	lastCreateFact  statedb.CreateFactParams
	createFactErr   error
	getLocationErr  error
}

func (s *stubEstablishFactStore) CreateFact(_ context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	if s.createFactErr != nil {
		return statedb.WorldFact{}, s.createFactErr
	}
	s.lastCreateFact = arg
	return statedb.WorldFact{
		ID:         dbutil.ToPgtype(uuid.New()),
		CampaignID: arg.CampaignID,
		Fact:       arg.Fact,
		Category:   arg.Category,
		Source:     arg.Source,
	}, nil
}

func (s *stubEstablishFactStore) GetLocationByID(_ context.Context, _ pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	return s.currentLocation, nil
}

func TestRegisterEstablishFact(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterEstablishFact(reg, &stubEstablishFactStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}}); err != nil {
		t.Fatalf("register establish_fact: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != establishFactToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, establishFactToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"fact", "category"} {
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestEstablishFactHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()

	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}

	h := NewEstablishFactHandler(store, memStore, embedder)
	ctx := WithCurrentLocationID(context.Background(), locationID)

	result, err := h.Handle(ctx, map[string]any{
		"fact":     "The ancient dragon sleeps beneath the mountain.",
		"category": "history",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}

	// Check CreateFact was called with correct params.
	if store.lastCreateFact.Fact != "The ancient dragon sleeps beneath the mountain." {
		t.Fatalf("CreateFact.Fact = %q, want correct text", store.lastCreateFact.Fact)
	}
	if store.lastCreateFact.Category != "history" {
		t.Fatalf("CreateFact.Category = %q, want %q", store.lastCreateFact.Category, "history")
	}
	if store.lastCreateFact.Source != establishedSource {
		t.Fatalf("CreateFact.Source = %q, want %q", store.lastCreateFact.Source, establishedSource)
	}
	if store.lastCreateFact.CampaignID != dbutil.ToPgtype(campaignID) {
		t.Fatal("CreateFact.CampaignID does not match campaign")
	}

	// Check data in result.
	if result.Data["fact"] != "The ancient dragon sleeps beneath the mountain." {
		t.Fatalf("result.Data[fact] = %v, want correct text", result.Data["fact"])
	}
	if result.Data["source"] != establishedSource {
		t.Fatalf("result.Data[source] = %v, want %q", result.Data["source"], establishedSource)
	}

	// Check memory was embedded.
	if !memStore.called {
		t.Fatal("expected CreateMemory to be called")
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
	if metadata["category"] != "history" {
		t.Fatalf("metadata[category] = %v, want %q", metadata["category"], "history")
	}
	if metadata["source"] != establishedSource {
		t.Fatalf("metadata[source] = %v, want %q", metadata["source"], establishedSource)
	}
}

func TestEstablishFactHandleNoEmbedder(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()

	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}

	h := NewEstablishFactHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)

	result, err := h.Handle(ctx, map[string]any{
		"fact":     "The kingdom has stood for 500 years.",
		"category": "history",
	})
	if err != nil {
		t.Fatalf("Handle without embedder returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestEstablishFactHandleMissingLocationContext(t *testing.T) {
	h := NewEstablishFactHandler(&stubEstablishFactStore{}, nil, nil)
	_, err := h.Handle(context.Background(), map[string]any{
		"fact":     "Some fact.",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for missing location context")
	}
}

func TestEstablishFactHandleMissingFact(t *testing.T) {
	locationID := uuid.New()
	h := NewEstablishFactHandler(&stubEstablishFactStore{}, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{"category": "history"})
	if err == nil {
		t.Fatal("expected error for missing fact")
	}
}

func TestEstablishFactHandleMissingCategory(t *testing.T) {
	locationID := uuid.New()
	h := NewEstablishFactHandler(&stubEstablishFactStore{}, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{"fact": "Some fact."})
	if err == nil {
		t.Fatal("expected error for missing category")
	}
}

func TestEstablishFactHandleGetLocationError(t *testing.T) {
	locationID := uuid.New()
	store := &stubEstablishFactStore{getLocationErr: errors.New("db error")}
	h := NewEstablishFactHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{"fact": "Some fact.", "category": "history"})
	if err == nil {
		t.Fatal("expected error when GetLocationByID fails")
	}
}

func TestEstablishFactHandleCreateFactError(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
		createFactErr: errors.New("db error"),
	}
	h := NewEstablishFactHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{"fact": "Some fact.", "category": "history"})
	if err == nil {
		t.Fatal("expected error when CreateFact fails")
	}
}

func TestEstablishFactHandleEmbedError(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	embedder := &stubEmbedder{err: errors.New("embed error")}
	h := NewEstablishFactHandler(store, &stubMemoryStore{}, embedder)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{"fact": "Some fact.", "category": "history"})
	if err == nil {
		t.Fatal("expected error when Embed fails")
	}
}

func TestRegisterEstablishFactNilStore(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterEstablishFact(reg, nil, nil, nil); err == nil {
		t.Fatal("expected error when registering with nil factStore")
	}
}

func TestEstablishFactHandleNilHandler(t *testing.T) {
	var h *EstablishFactHandler
	_, err := h.Handle(context.Background(), map[string]any{
		"fact":     "Some fact.",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestEstablishFactHandleNilFactStore(t *testing.T) {
	h := &EstablishFactHandler{}
	ctx := WithCurrentLocationID(context.Background(), uuid.New())
	_, err := h.Handle(ctx, map[string]any{
		"fact":     "Some fact.",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for nil factStore")
	}
}


func TestEstablishFactHandleEmptyFact(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	h := NewEstablishFactHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{
		"fact":     "",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for empty fact string")
	}
	if !strings.Contains(err.Error(), "fact must be a non-empty string") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "fact must be a non-empty string")
	}
}

func TestEstablishFactHandleArbitraryCategoryAccepted(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	h := NewEstablishFactHandler(store, nil, nil)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	result, err := h.Handle(ctx, map[string]any{
		"fact":     "Stars are suns.",
		"category": "science",
	})
	if err != nil {
		t.Fatalf("Handle returned unexpected error for arbitrary category: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success for arbitrary category")
	}
	if store.lastCreateFact.Category != "science" {
		t.Fatalf("CreateFact.Category = %q, want %q", store.lastCreateFact.Category, "science")
	}
}

func TestEstablishFactHandleMemoryStoreError(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubEstablishFactStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	memStore := &stubMemoryStore{err: errors.New("mem error")}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}
	h := NewEstablishFactHandler(store, memStore, embedder)
	ctx := WithCurrentLocationID(context.Background(), locationID)
	_, err := h.Handle(ctx, map[string]any{
		"fact":     "The forge is ancient.",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error when memory store CreateMemory fails")
	}
	if !strings.Contains(err.Error(), "mem error") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "mem error")
	}
}