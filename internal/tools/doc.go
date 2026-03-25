// Package tools defines LLM tool handlers for the game-master engine.
//
// # Tool Inventory
//
// Phase 2 (core gameplay):
//   - skill_check: Resolve d20+modifier vs DC. Store: StatModifierResolver.
//   - create_language: Create a world language. Store: LanguageStore. Optional: Embedder, MemoryStore.
//
// Phase 2 Track B (planned):
//   - describe_scene
//   - npc_dialogue
//   - present_choices
//   - move_player
//   - update_npc
//   - add_item / remove_item
//   - roll_dice
//
// # Tool Handler Pattern
//
// Each tool follows a consistent structure:
//
//  1. Tool definition function: XxxTool() llm.Tool — returns JSON schema
//  2. Store interface(s): defined in the tool file, using domain types only (uuid.UUID, json.RawMessage, etc.)
//  3. Handler struct: XxxHandler with Handle(ctx, args) (map[string]any, error)
//  4. Constructor: NewXxxHandler(...) *XxxHandler
//  5. Registration: RegisterXxx(reg *Registry, ...) error
//
// Store interfaces use domain types. Adapter implementations that convert
// to/from sqlc types live in the game package.
//
// Arg parsing helpers are consolidated in args.go.
package tools
