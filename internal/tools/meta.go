package tools

import "github.com/PatrickFanella/game-master/internal/llm"

// ToolCategory classifies tools for phase-based filtering.
type ToolCategory string

const (
	CategoryBase        ToolCategory = "base"
	CategoryCombat      ToolCategory = "combat"
	CategoryExploration ToolCategory = "exploration"
	CategoryQuest       ToolCategory = "quest"
	CategoryProgression ToolCategory = "progression"
)

// ToolMeta describes a tool's identity, category, and availability.
type ToolMeta struct {
	Name       string
	Category   ToolCategory
	RulesModes []string // empty = all modes; non-empty = only these modes
	Definition llm.Tool
}
