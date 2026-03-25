// Package prompt provides embedded system prompts for the game-master LLM.
package prompt

import _ "embed"

// GameMaster is the system prompt that instructs the LLM how to behave as
// a tabletop RPG game master. It covers narrative voice, tool usage, choice
// presentation, combat, pacing, consistency, and dice-rolling guidelines.
//
//go:embed gamemaster.txt
var GameMaster string
