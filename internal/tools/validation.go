package tools

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// EntityLookup provides read-only access to existing campaign entities
// for post-hoc validation.
type EntityLookup interface {
	// NPCByName returns the ID of an existing NPC with the given name,
	// or (uuid.Nil, false) if no such NPC exists.
	NPCByName(name string) (uuid.UUID, bool)

	// NPCExists reports whether an NPC with the given ID exists in the campaign.
	NPCExists(id uuid.UUID) bool

	// LocationExists reports whether a location with the given ID exists.
	LocationExists(id uuid.UUID) bool

	// QuestExists reports whether a quest with the given ID exists.
	QuestExists(id uuid.UUID) bool

	// FactionExists reports whether a faction with the given ID exists.
	FactionExists(id uuid.UUID) bool
}

// Validator validates tool calls against the registered tool schemas and
// known game-world entities.
type Validator struct {
	// schemas is a pre-built map from tool name to its Parameters schema,
	// populated at construction time to avoid repeated deep-copies during
	// validation.
	schemas map[string]map[string]any
}

// NewValidator creates a Validator backed by the given registry.
// It snapshots the current registry contents into an O(1) lookup table.
func NewValidator(reg *Registry) *Validator {
	schemas := make(map[string]map[string]any)
	if reg != nil {
		for _, t := range reg.List() {
			schemas[t.Name] = t.Parameters
		}
	}
	return &Validator{schemas: schemas}
}

// ValidatePreExecution validates a tool call before execution. It verifies that:
//  1. the tool name is registered,
//  2. all required arguments are present, and
//  3. argument types match the tool's JSON Schema.
//
// Returns the first error found, or nil if the call is valid.
func (v *Validator) ValidatePreExecution(call llm.ToolCall) error {
	if v == nil || v.schemas == nil {
		return fmt.Errorf("validator registry is nil")
	}

	// 1. Check tool name is registered and retrieve its schema.
	schema := v.schemas[call.Name]
	if schema == nil {
		return fmt.Errorf("tool %q is not registered", call.Name)
	}

	// 2. Check required arguments.
	if err := checkRequiredArgs(call.Name, call.Arguments, schema); err != nil {
		return err
	}

	// 3. Check argument types against the schema.
	if err := checkArgTypes(call.Name, call.Arguments, schema); err != nil {
		return err
	}

	return nil
}

// PosthocResult is the result of post-hoc validation for a single tool call.
type PosthocResult struct {
	// Call is the (potentially fixed) tool call. Arguments may have been
	// clamped or supplemented with deduplication data.
	Call llm.ToolCall
	// Fixes is a list of human-readable descriptions of silent fixes that
	// were applied to the call. These are intended for internal logging
	// only and must not be surfaced to the player.
	Fixes []string
	// Err is a non-fixable validation error, or nil if the call is valid.
	Err error
}

// ValidatePosthoc runs post-hoc validation on a tool call using the given
// entity lookup. It:
//   - clamps numeric values to valid ranges (disposition -100..100, hp/max_hp >= 0),
//   - deduplicates NPC names by detecting existing NPCs with the same name and
//     annotating the call with the existing NPC's ID,
//   - checks referential integrity for well-known ID arguments.
//
// Fixable issues are applied silently and recorded in PosthocResult.Fixes.
// Unfixable errors are returned in PosthocResult.Err.
// If lookup is nil, only range clamping is performed.
func (v *Validator) ValidatePosthoc(call llm.ToolCall, lookup EntityLookup) PosthocResult {
	// Deep-copy arguments so we never mutate the caller's original map.
	args := deepCopyMap(call.Arguments)
	var fixes []string

	// Range clamping (no DB access required).
	fixes = append(fixes, clampDisposition(args)...)
	fixes = append(fixes, clampHP(args)...)

	if lookup != nil {
		// NPC name deduplication.
		dedupFixes, err := deduplicateNPCName(call.Name, args, lookup)
		if err != nil {
			return PosthocResult{
				Call:  llm.ToolCall{ID: call.ID, Name: call.Name, Arguments: args},
				Fixes: fixes,
				Err:   err,
			}
		}
		fixes = append(fixes, dedupFixes...)

		// Referential integrity checks.
		if err := checkReferentialIntegrity(call.Name, args, lookup); err != nil {
			return PosthocResult{
				Call:  llm.ToolCall{ID: call.ID, Name: call.Name, Arguments: args},
				Fixes: fixes,
				Err:   err,
			}
		}
	}

	fixedCall := llm.ToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: args,
	}
	return PosthocResult{Call: fixedCall, Fixes: fixes}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
// checkRequiredArgs verifies that all fields listed under "required" in
// schema are present in args.
func checkRequiredArgs(toolName string, args map[string]any, schema map[string]any) error {
	required, ok := schema["required"]
	if !ok {
		return nil
	}
	switch r := required.(type) {
	case []any:
		for _, item := range r {
			key, _ := item.(string)
			if key == "" {
				continue
			}
			if _, present := args[key]; !present {
				return fmt.Errorf("tool %q: required argument %q is missing", toolName, key)
			}
		}
	case []string:
		for _, key := range r {
			if _, present := args[key]; !present {
				return fmt.Errorf("tool %q: required argument %q is missing", toolName, key)
			}
		}
	}
	return nil
}

// checkArgTypes verifies that each argument value in args matches the JSON
// Schema type declared in schema["properties"].
func checkArgTypes(toolName string, args map[string]any, schema map[string]any) error {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	for key, val := range args {
		propSchema, ok := props[key].(map[string]any)
		if !ok {
			continue
		}
		expectedType, _ := propSchema["type"].(string)
		if expectedType == "" {
			continue
		}
		if err := checkSingleArgType(toolName, key, val, expectedType); err != nil {
			return err
		}
	}
	return nil
}

// checkSingleArgType validates that val matches the expected JSON Schema type.
func checkSingleArgType(toolName, key string, val any, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("tool %q: argument %q must be a string, got %T", toolName, key, val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("tool %q: argument %q must be a boolean, got %T", toolName, key, val)
		}
	case "integer":
		if !isIntegerValue(val) {
			return fmt.Errorf("tool %q: argument %q must be an integer, got %T", toolName, key, val)
		}
	case "number":
		if !isNumberValue(val) {
			return fmt.Errorf("tool %q: argument %q must be a number, got %T", toolName, key, val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("tool %q: argument %q must be an object, got %T", toolName, key, val)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("tool %q: argument %q must be an array, got %T", toolName, key, val)
		}
	}
	return nil
}

// isIntegerValue reports whether val can be treated as an integer.
func isIntegerValue(val any) bool {
	switch val.(type) {
	case int, int8, int16, int32, int64:
		return true
	}
	f, ok := val.(float64)
	if !ok {
		return false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return false
	}
	return math.Abs(f-math.Round(f)) <= floatIntegerTolerance
}

// isNumberValue reports whether val is any numeric type.
func isNumberValue(val any) bool {
	switch val.(type) {
	case int, int8, int16, int32, int64, float32, float64:
		return true
	}
	return false
}

// clampDisposition clamps the "disposition" argument in args to [-100, 100].
// Returns a list of fix descriptions for any value that was clamped.
func clampDisposition(args map[string]any) []string {
	const key = "disposition"
	raw, ok := args[key]
	if !ok {
		return nil
	}
	v, ok := toFloat64(raw)
	if !ok {
		return nil
	}
	clamped := math.Max(-100, math.Min(100, v))
	if clamped != v {
		args[key] = clamped
		return []string{fmt.Sprintf("disposition clamped from %.0f to %.0f", v, clamped)}
	}
	return nil
}

// clampHP clamps "hp" and "max_hp" arguments in args to >= 0.
// Returns a list of fix descriptions for any value that was clamped.
func clampHP(args map[string]any) []string {
	var fixes []string
	for _, key := range []string{"hp", "max_hp"} {
		raw, ok := args[key]
		if !ok {
			continue
		}
		v, ok := toFloat64(raw)
		if !ok {
			continue
		}
		if v < 0 {
			args[key] = float64(0)
			fixes = append(fixes, fmt.Sprintf("%s clamped from %.0f to 0", key, v))
		}
	}
	return fixes
}

// toFloat64 converts common numeric types to float64.
func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

// deduplicateNPCName checks whether a tool call that contains a "name"
// argument is attempting to create an NPC that already exists. When an
// existing NPC is found the call is annotated with its ID via "npc_id".
// Only applies to tools whose names contain "npc".
func deduplicateNPCName(toolName string, args map[string]any, lookup EntityLookup) ([]string, error) {
	if !strings.Contains(strings.ToLower(toolName), "npc") {
		return nil, nil
	}
	name, _ := args["name"].(string)
	if name == "" {
		return nil, nil
	}
	existingID, found := lookup.NPCByName(name)
	if !found {
		return nil, nil
	}
	// Only inject npc_id when the caller has not already provided one,
	// so an explicit reference is never silently overwritten.
	if existing, _ := args["npc_id"].(string); existing != "" {
		return nil, nil
	}
	args["npc_id"] = existingID.String()
	return []string{
		fmt.Sprintf("NPC name %q already exists; reusing existing ID %s", name, existingID),
	}, nil
}

// checkReferentialIntegrity verifies that well-known ID arguments reference
// entities that exist in the campaign. Returns an error for the first
// reference that cannot be resolved.
func checkReferentialIntegrity(toolName string, args map[string]any, lookup EntityLookup) error {
	checks := []struct {
		key    string
		exists func(uuid.UUID) bool
	}{
		{"location_id", lookup.LocationExists},
		{"npc_id", lookup.NPCExists},
		{"quest_id", lookup.QuestExists},
		{"faction_id", lookup.FactionExists},
	}
	for _, c := range checks {
		raw, ok := args[c.key]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok || s == "" {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return fmt.Errorf("tool %q: argument %q must be a valid UUID", toolName, c.key)
		}
		if !c.exists(id) {
			return fmt.Errorf("tool %q: argument %q references non-existent entity %s", toolName, c.key, id)
		}
	}
	return nil
}
