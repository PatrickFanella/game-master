package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// ---------------------------------------------------------------------------
// Helpers shared by tests
// ---------------------------------------------------------------------------

// buildTestRegistry creates a Registry with two tools for use in tests.
//
// "mock_tool" has a string arg "name" (required) and an integer arg "count".
// "mock_npc_create" has string args "name" and "campaign_id" (both required),
// plus integer "disposition" and "hp".
func buildTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()

	noop := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return &ToolResult{Success: true}, nil
	}

	mustRegister := func(tool llm.Tool) {
		t.Helper()
		if err := reg.Register(tool, noop); err != nil {
			t.Fatalf("Register %q: %v", tool.Name, err)
		}
	}

	mustRegister(llm.Tool{
		Name: "mock_tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":  map[string]any{"type": "string"},
				"count": map[string]any{"type": "integer"},
			},
			"required": []any{"name"},
		},
	})

	mustRegister(llm.Tool{
		Name: "mock_npc_create",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"campaign_id": map[string]any{"type": "string"},
				"disposition": map[string]any{"type": "integer"},
				"hp":          map[string]any{"type": "integer"},
				"max_hp":      map[string]any{"type": "integer"},
				"location_id": map[string]any{"type": "string"},
				"faction_id":  map[string]any{"type": "string"},
				"npc_id":      map[string]any{"type": "string"},
				"quest_id":    map[string]any{"type": "string"},
			},
			"required": []any{"name", "campaign_id"},
		},
	})

	return reg
}

// mockLookup is a test double for EntityLookup.
type mockLookup struct {
	npcsByName  map[string]uuid.UUID
	npcIDs      map[uuid.UUID]bool
	locationIDs map[uuid.UUID]bool
	questIDs    map[uuid.UUID]bool
	factionIDs  map[uuid.UUID]bool
}

func (m *mockLookup) NPCByName(name string) (uuid.UUID, bool) {
	id, ok := m.npcsByName[name]
	return id, ok
}
func (m *mockLookup) NPCExists(id uuid.UUID) bool      { return m.npcIDs[id] }
func (m *mockLookup) LocationExists(id uuid.UUID) bool  { return m.locationIDs[id] }
func (m *mockLookup) QuestExists(id uuid.UUID) bool     { return m.questIDs[id] }
func (m *mockLookup) FactionExists(id uuid.UUID) bool   { return m.factionIDs[id] }

func emptyLookup() *mockLookup {
	return &mockLookup{
		npcsByName:  make(map[string]uuid.UUID),
		npcIDs:      make(map[uuid.UUID]bool),
		locationIDs: make(map[uuid.UUID]bool),
		questIDs:    make(map[uuid.UUID]bool),
		factionIDs:  make(map[uuid.UUID]bool),
	}
}

// ---------------------------------------------------------------------------
// NewValidator
// ---------------------------------------------------------------------------

func TestNewValidator(t *testing.T) {
	reg := NewRegistry()
	v := NewValidator(reg)
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
}

// ---------------------------------------------------------------------------
// ValidatePreExecution – tool name
// ---------------------------------------------------------------------------

func TestPreExecution_UnknownTool(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "nonexistent_tool",
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool") {
		t.Errorf("error %q should mention the tool name", err.Error())
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error %q should say 'not registered'", err.Error())
	}
}

func TestPreExecution_KnownTool_Valid(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Alice"},
	})
	if err != nil {
		t.Fatalf("unexpected error for valid call: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidatePreExecution – required arguments
// ---------------------------------------------------------------------------

func TestPreExecution_MissingRequired(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error for missing required arg, got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error %q should mention missing field 'name'", err.Error())
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q should mention 'missing'", err.Error())
	}
}

func TestPreExecution_AllRequiredPresent(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Bob", "count": float64(3)},
	})
	if err != nil {
		t.Fatalf("unexpected error when all required args present: %v", err)
	}
}

func TestPreExecution_OptionalArgAbsent_OK(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	// "count" is optional; only "name" is required.
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Carol"},
	})
	if err != nil {
		t.Fatalf("unexpected error when optional arg absent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidatePreExecution – argument types
// ---------------------------------------------------------------------------

func TestPreExecution_WrongType_StringExpected(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": 42}, // should be string
	})
	if err == nil {
		t.Fatal("expected type error, got nil")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error %q should mention expected type 'string'", err.Error())
	}
}

func TestPreExecution_WrongType_IntegerExpected(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Alice", "count": "three"}, // should be integer
	})
	if err == nil {
		t.Fatal("expected type error, got nil")
	}
	if !strings.Contains(err.Error(), "integer") {
		t.Errorf("error %q should mention expected type 'integer'", err.Error())
	}
}

func TestPreExecution_IntegerAsFloat64_OK(t *testing.T) {
	// JSON numbers decode as float64; whole-number floats are valid integers.
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Alice", "count": float64(5)},
	})
	if err != nil {
		t.Fatalf("whole-number float64 should be accepted for integer arg: %v", err)
	}
}

func TestPreExecution_FractionalFloat_NotInteger(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Alice", "count": float64(1.5)},
	})
	if err == nil {
		t.Fatal("fractional float should be rejected for integer arg")
	}
}

// buildTypeTestRegistry creates a Registry that exposes one arg of every
// supported JSON Schema type so that type-check tests can cover all branches.
func buildTypeTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	noop := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return &ToolResult{Success: true}, nil
	}
	if err := reg.Register(llm.Tool{
		Name: "typed_tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"flag":   map[string]any{"type": "boolean"},
				"score":  map[string]any{"type": "number"},
				"meta":   map[string]any{"type": "object"},
				"tags":   map[string]any{"type": "array"},
				"label":  map[string]any{"type": "string"},
				"amount": map[string]any{"type": "integer"},
			},
		},
	}, noop); err != nil {
		t.Fatalf("Register typed_tool: %v", err)
	}
	return reg
}

func TestPreExecution_WrongType_BooleanExpected(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"flag": "yes"}, // should be boolean
	})
	if err == nil {
		t.Fatal("expected type error for non-boolean, got nil")
	}
	if !strings.Contains(err.Error(), "boolean") {
		t.Errorf("error %q should mention 'boolean'", err.Error())
	}
}

func TestPreExecution_CorrectType_Boolean(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	if err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"flag": true},
	}); err != nil {
		t.Fatalf("unexpected error for valid boolean: %v", err)
	}
}

func TestPreExecution_WrongType_NumberExpected(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"score": "high"}, // should be number
	})
	if err == nil {
		t.Fatal("expected type error for non-number, got nil")
	}
	if !strings.Contains(err.Error(), "number") {
		t.Errorf("error %q should mention 'number'", err.Error())
	}
}

func TestPreExecution_CorrectType_Number(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	if err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"score": float64(9.5)},
	}); err != nil {
		t.Fatalf("unexpected error for valid number: %v", err)
	}
}

func TestPreExecution_WrongType_ObjectExpected(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"meta": "not-an-object"}, // should be object
	})
	if err == nil {
		t.Fatal("expected type error for non-object, got nil")
	}
	if !strings.Contains(err.Error(), "object") {
		t.Errorf("error %q should mention 'object'", err.Error())
	}
}

func TestPreExecution_CorrectType_Object(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	if err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"meta": map[string]any{"key": "val"}},
	}); err != nil {
		t.Fatalf("unexpected error for valid object: %v", err)
	}
}

func TestPreExecution_WrongType_ArrayExpected(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"tags": "not-an-array"}, // should be array
	})
	if err == nil {
		t.Fatal("expected type error for non-array, got nil")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Errorf("error %q should mention 'array'", err.Error())
	}
}

func TestPreExecution_CorrectType_Array(t *testing.T) {
	v := NewValidator(buildTypeTestRegistry(t))
	if err := v.ValidatePreExecution(llm.ToolCall{
		Name:      "typed_tool",
		Arguments: map[string]any{"tags": []any{"a", "b"}},
	}); err != nil {
		t.Fatalf("unexpected error for valid array: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – range clamping
// ---------------------------------------------------------------------------

func TestPosthoc_DispositionClamped_TooHigh(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Goblin", "campaign_id": "x", "disposition": float64(150)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) == 0 {
		t.Fatal("expected a fix record for clamped disposition")
	}
	got, _ := toFloat64(result.Call.Arguments["disposition"])
	if got != 100 {
		t.Errorf("disposition = %.0f, want 100", got)
	}
}

func TestPosthoc_DispositionClamped_TooLow(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Vampire", "campaign_id": "x", "disposition": float64(-200)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	got, _ := toFloat64(result.Call.Arguments["disposition"])
	if got != -100 {
		t.Errorf("disposition = %.0f, want -100", got)
	}
}

func TestPosthoc_DispositionInRange_NoClamp(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Innkeeper", "campaign_id": "x", "disposition": float64(50)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) != 0 {
		t.Errorf("expected no fixes for in-range disposition, got: %v", result.Fixes)
	}
	got, _ := toFloat64(result.Call.Arguments["disposition"])
	if got != 50 {
		t.Errorf("disposition = %.0f, want 50", got)
	}
}

func TestPosthoc_HPClamped_Negative(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Zombie", "campaign_id": "x", "hp": float64(-5)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) == 0 {
		t.Fatal("expected a fix record for clamped hp")
	}
	got, _ := toFloat64(result.Call.Arguments["hp"])
	if got != 0 {
		t.Errorf("hp = %.0f, want 0", got)
	}
}

func TestPosthoc_MaxHPClamped_Negative(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Ghost", "campaign_id": "x", "max_hp": float64(-1)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	got, _ := toFloat64(result.Call.Arguments["max_hp"])
	if got != 0 {
		t.Errorf("max_hp = %.0f, want 0", got)
	}
}

func TestPosthoc_HPZero_NoClamp(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Dead", "campaign_id": "x", "hp": float64(0)},
	}
	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) != 0 {
		t.Errorf("expected no fixes for hp=0, got: %v", result.Fixes)
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – NPC name deduplication
// ---------------------------------------------------------------------------

func TestPosthoc_NPCDedup_NameExists(t *testing.T) {
	existingID := uuid.New()
	lookup := emptyLookup()
	lookup.npcsByName["Goblin Scout"] = existingID
	lookup.npcIDs[existingID] = true

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Goblin Scout", "campaign_id": uuid.New().String()},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) == 0 {
		t.Fatal("expected a fix record for NPC deduplication")
	}
	// The existing NPC's ID should be injected as "npc_id".
	npcID, ok := result.Call.Arguments["npc_id"].(string)
	if !ok {
		t.Fatal("expected npc_id to be set as string in fixed call")
	}
	if npcID != existingID.String() {
		t.Errorf("npc_id = %q, want %q", npcID, existingID.String())
	}
}

func TestPosthoc_NPCDedup_NewName_NoFix(t *testing.T) {
	lookup := emptyLookup()

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: map[string]any{"name": "Brand New NPC", "campaign_id": uuid.New().String()},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Fixes) != 0 {
		t.Errorf("expected no dedup fix for new NPC name, got: %v", result.Fixes)
	}
	if _, ok := result.Call.Arguments["npc_id"]; ok {
		t.Error("expected no npc_id injected for new NPC name")
	}
}

func TestPosthoc_NPCDedup_NonNPCTool_Ignored(t *testing.T) {
	existingID := uuid.New()
	lookup := emptyLookup()
	lookup.npcsByName["Alice"] = existingID

	v := NewValidator(buildTestRegistry(t))
	// "mock_tool" does not contain "npc"; dedup should not apply.
	call := llm.ToolCall{
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "Alice"},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if _, ok := result.Call.Arguments["npc_id"]; ok {
		t.Error("npc_id should not be injected for non-NPC tool")
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – referential integrity
// ---------------------------------------------------------------------------

func TestPosthoc_RefIntegrity_ValidLocationID(t *testing.T) {
	locID := uuid.New()
	lookup := emptyLookup()
	lookup.locationIDs[locID] = true

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Guard",
			"campaign_id": uuid.New().String(),
			"location_id": locID.String(),
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error for valid location_id: %v", result.Err)
	}
}

func TestPosthoc_RefIntegrity_InvalidLocationID(t *testing.T) {
	lookup := emptyLookup() // no locations registered

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Guard",
			"campaign_id": uuid.New().String(),
			"location_id": uuid.New().String(), // not in lookup
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected referential integrity error for unknown location_id, got nil")
	}
	if !strings.Contains(result.Err.Error(), "location_id") {
		t.Errorf("error %q should mention 'location_id'", result.Err.Error())
	}
	if !strings.Contains(result.Err.Error(), "non-existent") {
		t.Errorf("error %q should mention 'non-existent'", result.Err.Error())
	}
}

func TestPosthoc_RefIntegrity_InvalidNPCID(t *testing.T) {
	lookup := emptyLookup()

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Ally",
			"campaign_id": uuid.New().String(),
			"npc_id":      uuid.New().String(), // not in lookup
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected referential integrity error for unknown npc_id, got nil")
	}
}

func TestPosthoc_RefIntegrity_InvalidQuestID(t *testing.T) {
	lookup := emptyLookup()

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Questgiver",
			"campaign_id": uuid.New().String(),
			"quest_id":    uuid.New().String(),
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected referential integrity error for unknown quest_id, got nil")
	}
}

func TestPosthoc_RefIntegrity_InvalidFactionID(t *testing.T) {
	lookup := emptyLookup()

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Soldier",
			"campaign_id": uuid.New().String(),
			"faction_id":  uuid.New().String(),
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected referential integrity error for unknown faction_id, got nil")
	}
}

func TestPosthoc_RefIntegrity_MalformedUUID(t *testing.T) {
	lookup := emptyLookup()

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Bandit",
			"campaign_id": uuid.New().String(),
			"location_id": "not-a-uuid",
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected error for malformed UUID in location_id, got nil")
	}
	if !strings.Contains(result.Err.Error(), "valid UUID") {
		t.Errorf("error %q should mention 'valid UUID'", result.Err.Error())
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – original call not mutated
// ---------------------------------------------------------------------------

func TestPosthoc_OriginalCallNotMutated(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	origArgs := map[string]any{
		"name":        "Test NPC",
		"campaign_id": "some-id",
		"disposition": float64(150), // will be clamped in the fix
	}
	call := llm.ToolCall{
		Name:      "mock_npc_create",
		Arguments: origArgs,
	}

	result := v.ValidatePosthoc(call, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Original args must not have been modified.
	if origArgs["disposition"] != float64(150) {
		t.Errorf("original disposition mutated: got %v, want 150", origArgs["disposition"])
	}
	// Fixed call should have the clamped value.
	got, _ := toFloat64(result.Call.Arguments["disposition"])
	if got != 100 {
		t.Errorf("fixed disposition = %.0f, want 100", got)
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – nil lookup skips entity checks
// ---------------------------------------------------------------------------

func TestPosthoc_NilLookup_SkipsEntityChecks(t *testing.T) {
	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Anyone",
			"campaign_id": uuid.New().String(),
			"location_id": uuid.New().String(), // would fail if lookup were not nil
		},
	}
	result := v.ValidatePosthoc(call, nil) // nil lookup → skip entity checks
	if result.Err != nil {
		t.Fatalf("nil lookup should skip referential integrity: %v", result.Err)
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – combined fixes
// ---------------------------------------------------------------------------

func TestPosthoc_CombinedFixes(t *testing.T) {
	existingNPCID := uuid.New()
	locID := uuid.New()
	lookup := emptyLookup()
	lookup.npcsByName["Dragon"] = existingNPCID
	lookup.npcIDs[existingNPCID] = true
	lookup.locationIDs[locID] = true

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Dragon",
			"campaign_id": uuid.New().String(),
			"disposition": float64(999),  // will be clamped to 100
			"hp":          float64(-10),  // will be clamped to 0
			"location_id": locID.String(), // valid
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// Expect fixes: disposition clamp + hp clamp + NPC dedup.
	if len(result.Fixes) < 3 {
		t.Errorf("expected at least 3 fixes, got %d: %v", len(result.Fixes), result.Fixes)
	}

	disp, _ := toFloat64(result.Call.Arguments["disposition"])
	if disp != 100 {
		t.Errorf("disposition = %.0f, want 100", disp)
	}
	hp, _ := toFloat64(result.Call.Arguments["hp"])
	if hp != 0 {
		t.Errorf("hp = %.0f, want 0", hp)
	}
	if result.Call.Arguments["npc_id"] != existingNPCID.String() {
		t.Errorf("npc_id = %v, want %s", result.Call.Arguments["npc_id"], existingNPCID)
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – patched call returned even on error
// ---------------------------------------------------------------------------

func TestPosthoc_PatchedCallReturnedOnRefIntegrityError(t *testing.T) {
	// Disposition will be clamped (a fix) before the referential integrity
	// error is raised. The returned Call should carry the clamped value even
	// though Err is non-nil.
	lookup := emptyLookup() // location_id will not be found

	v := NewValidator(buildTestRegistry(t))
	locID := uuid.New()
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Guard",
			"campaign_id": uuid.New().String(),
			"disposition": float64(200),  // will be clamped to 100
			"location_id": locID.String(), // unknown → integrity error
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err == nil {
		t.Fatal("expected referential integrity error, got nil")
	}
	// Clamping fix must still appear in Fixes.
	if len(result.Fixes) == 0 {
		t.Fatal("expected at least one fix (disposition clamp) even on error")
	}
	// result.Call should contain the patched (clamped) disposition.
	got, _ := toFloat64(result.Call.Arguments["disposition"])
	if got != 100 {
		t.Errorf("patched Call disposition = %.0f, want 100", got)
	}
	// Original args must be unchanged.
	if call.Arguments["disposition"] != float64(200) {
		t.Errorf("original disposition mutated: got %v, want 200", call.Arguments["disposition"])
	}
}

// ---------------------------------------------------------------------------
// ValidatePosthoc – NPC dedup does not overwrite explicit npc_id
// ---------------------------------------------------------------------------

func TestPosthoc_NPCDedup_ExplicitNPCIDNotOverwritten(t *testing.T) {
	existingID := uuid.New()
	explicitID := uuid.New()
	lookup := emptyLookup()
	lookup.npcsByName["Guard"] = existingID
	lookup.npcIDs[existingID] = true
	lookup.npcIDs[explicitID] = true

	v := NewValidator(buildTestRegistry(t))
	call := llm.ToolCall{
		Name: "mock_npc_create",
		Arguments: map[string]any{
			"name":        "Guard",
			"campaign_id": uuid.New().String(),
			"npc_id":      explicitID.String(), // caller already knows the ID
		},
	}
	result := v.ValidatePosthoc(call, lookup)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// The explicit npc_id must not be replaced by the dedup-detected ID.
	if result.Call.Arguments["npc_id"] != explicitID.String() {
		t.Errorf("npc_id = %v, want %s (explicit ID must not be overwritten)", result.Call.Arguments["npc_id"], explicitID)
	}
	// No dedup fix should be recorded.
	for _, fix := range result.Fixes {
		if strings.Contains(fix, "reusing existing") {
			t.Errorf("unexpected dedup fix when npc_id was already set: %q", fix)
		}
	}
}
