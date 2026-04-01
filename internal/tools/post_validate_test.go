package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// --- stub store ---

type stubPostValidationStore struct {
	npcID      *uuid.UUID
	npcErr     error
	locID      *uuid.UUID
	locErr     error
	entityOK   bool
	entityErr  error
}

func (s *stubPostValidationStore) FindNPCByNameAndLocation(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) (*uuid.UUID, error) {
	return s.npcID, s.npcErr
}

func (s *stubPostValidationStore) FindLocationByNameAndRegion(_ context.Context, _ uuid.UUID, _ string, _ string) (*uuid.UUID, error) {
	return s.locID, s.locErr
}

func (s *stubPostValidationStore) EntityExists(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
	return s.entityOK, s.entityErr
}

// --- helpers ---

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func mustResult(data map[string]any) *ToolResult {
	return &ToolResult{Success: true, Data: data}
}

// --- tests ---

func TestPostValidator_NPCDedup(t *testing.T) {
	existing := uuid.New()
	campID := uuid.New()
	store := &stubPostValidationStore{npcID: ptrUUID(existing)}
	pv := NewPostValidator(store, nil)

	result := mustResult(map[string]any{
		"campaign_id": campID.String(),
		"name":        "Gareth",
		"location_id": uuid.New().String(),
	})

	got, err := pv.Validate(context.Background(), "create_npc", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data["id"] != existing.String() {
		t.Errorf("id = %v, want %v", got.Data["id"], existing.String())
	}
	if got.Data["deduplicated"] != true {
		t.Error("expected deduplicated = true")
	}
}

func TestPostValidator_NPCNoDup(t *testing.T) {
	campID := uuid.New()
	origID := uuid.New().String()
	store := &stubPostValidationStore{npcID: nil}
	pv := NewPostValidator(store, nil)

	result := mustResult(map[string]any{
		"campaign_id": campID.String(),
		"name":        "Gareth",
		"id":          origID,
	})

	got, err := pv.Validate(context.Background(), "create_npc", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data["id"] != origID {
		t.Errorf("id changed: got %v, want %v", got.Data["id"], origID)
	}
	if _, ok := got.Data["deduplicated"]; ok {
		t.Error("deduplicated should not be set")
	}
}

func TestPostValidator_LocationDedup(t *testing.T) {
	existing := uuid.New()
	campID := uuid.New()
	store := &stubPostValidationStore{locID: ptrUUID(existing)}
	pv := NewPostValidator(store, nil)

	result := mustResult(map[string]any{
		"campaign_id": campID.String(),
		"name":        "Thornwall",
		"region":      "Northern Reaches",
	})

	got, err := pv.Validate(context.Background(), "create_location", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data["id"] != existing.String() {
		t.Errorf("id = %v, want %v", got.Data["id"], existing.String())
	}
	if got.Data["deduplicated"] != true {
		t.Error("expected deduplicated = true")
	}
}

func TestPostValidator_FillDefaults_NPC(t *testing.T) {
	campID := uuid.New()
	store := &stubPostValidationStore{npcID: nil}
	pv := NewPostValidator(store, nil)

	result := mustResult(map[string]any{
		"campaign_id": campID.String(),
		"name":        "Gareth",
	})

	got, err := pv.Validate(context.Background(), "create_npc", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data["disposition"] != 0 {
		t.Errorf("disposition = %v, want 0", got.Data["disposition"])
	}
	if got.Data["alive"] != true {
		t.Errorf("alive = %v, want true", got.Data["alive"])
	}
	if got.Data["properties"] == nil {
		t.Error("properties should be empty map, got nil")
	}
}

func TestPostValidator_UnknownTool(t *testing.T) {
	store := &stubPostValidationStore{}
	pv := NewPostValidator(store, nil)

	original := mustResult(map[string]any{"foo": "bar"})
	got, err := pv.Validate(context.Background(), "create_spell", original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != original {
		t.Error("expected same pointer returned for unknown tool")
	}
}

func TestPostValidator_HandlerError(t *testing.T) {
	store := &stubPostValidationStore{}
	pv := NewPostValidator(store, nil)

	handlerErr := errors.New("db down")
	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return nil, handlerErr
	}

	wrapped := pv.Wrap("create_npc", handler)
	result, err := wrapped(context.Background(), nil)
	if !errors.Is(err, handlerErr) {
		t.Fatalf("err = %v, want %v", err, handlerErr)
	}
	if result != nil {
		t.Error("expected nil result on handler error")
	}
}

func TestPostValidator_StoreError(t *testing.T) {
	campID := uuid.New()
	store := &stubPostValidationStore{npcErr: errors.New("store exploded")}
	pv := NewPostValidator(store, nil)

	original := mustResult(map[string]any{
		"campaign_id": campID.String(),
		"name":        "Gareth",
	})

	// Use Wrap so the store error is logged-and-swallowed, not returned.
	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return original, nil
	}
	wrapped := pv.Wrap("create_npc", handler)
	got, err := wrapped(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Original result returned despite store failure.
	if got != original {
		t.Error("expected original result when store errors")
	}
}

func TestPostValidator_Wrap(t *testing.T) {
	existing := uuid.New()
	campID := uuid.New()
	store := &stubPostValidationStore{npcID: ptrUUID(existing)}
	pv := NewPostValidator(store, nil)

	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return mustResult(map[string]any{
			"campaign_id": campID.String(),
			"name":        "Gareth",
		}), nil
	}

	wrapped := pv.Wrap("create_npc", handler)
	got, err := wrapped(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Data["id"] != existing.String() {
		t.Errorf("id = %v, want %v", got.Data["id"], existing.String())
	}
	if got.Data["deduplicated"] != true {
		t.Error("expected deduplicated = true")
	}
	// Defaults also filled via the full chain.
	if got.Data["alive"] != true {
		t.Error("expected alive default filled")
	}
}
