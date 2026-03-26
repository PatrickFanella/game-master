package prompt

import (
	"strings"
	"testing"
)

func TestGameMasterPromptNonEmpty(t *testing.T) {
	if strings.TrimSpace(GameMaster) == "" {
		t.Fatal("GameMaster prompt must not be empty")
	}
}

func TestGameMasterPromptSections(t *testing.T) {
	requiredSections := []struct {
		name    string
		heading string
	}{
		{"narrative voice", "NARRATIVE VOICE AND TONE"},
		{"tool usage", "TOOL USAGE GUIDELINES"},
		{"choice presentation", "CHOICE PRESENTATION"},
		{"combat guidelines", "COMBAT GUIDELINES"},
		{"pacing guidelines", "PACING GUIDELINES"},
		{"consistency rules", "CONSISTENCY RULES"},
		{"dice rolling", "DICE ROLLING AND SKILL CHECKS"},
		{"new content", "INTRODUCING NEW CONTENT"},
	}

	for _, s := range requiredSections {
		t.Run(s.name, func(t *testing.T) {
			if !strings.Contains(GameMaster, s.heading) {
				t.Fatalf("GameMaster prompt missing section %q", s.heading)
			}
		})
	}
}

func TestGameMasterPromptToolReferences(t *testing.T) {
	tools := []string{"skill_check", "create_language", "roll_dice"}
	for _, tool := range tools {
		if !strings.Contains(GameMaster, tool) {
			t.Fatalf("GameMaster prompt must reference tool %q", tool)
		}
	}
}

func TestGameMasterPromptNarrativeVoice(t *testing.T) {
	if !strings.Contains(GameMaster, "second person") {
		t.Fatal("GameMaster prompt must mention second person narrative voice")
	}
	if !strings.Contains(GameMaster, "third person") {
		t.Fatal("GameMaster prompt must mention third person narrative voice")
	}
}

func TestGameMasterPromptChoiceGuidelines(t *testing.T) {
	if !strings.Contains(GameMaster, "3 and 5") {
		t.Fatal("GameMaster prompt must specify offering 3 to 5 choices")
	}
	if !strings.Contains(GameMaster, "never limited to the options above") {
		t.Fatal("GameMaster prompt must signal that free input is welcome")
	}
}

func TestGameMasterPromptDifficultyClasses(t *testing.T) {
	dcs := []string{"DC 10", "DC 15", "DC 20", "DC 25"}
	for _, dc := range dcs {
		if !strings.Contains(GameMaster, dc) {
			t.Fatalf("GameMaster prompt must reference %s", dc)
		}
	}
}

func TestGameMasterPromptConsistency(t *testing.T) {
	keywords := []string{
		"established facts",
		"NPC states",
		"player choices",
	}
	for _, kw := range keywords {
		if !strings.Contains(GameMaster, kw) {
			t.Fatalf("GameMaster prompt must reference %q for consistency rules", kw)
		}
	}
}

func TestGameMasterPromptPacingPillars(t *testing.T) {
	pillars := []string{"Exposition", "Action", "Dialogue"}
	for _, p := range pillars {
		if !strings.Contains(GameMaster, p) {
			t.Fatalf("GameMaster prompt must reference pacing pillar %q", p)
		}
	}
}
