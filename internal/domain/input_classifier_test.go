package domain

import "testing"

func TestClassify_MetaCommands(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "inventory", input: "inventory"},
		{name: "character", input: "character"},
		{name: "quest", input: "quest"},
		{name: "save", input: "save"},
		{name: "quit", input: "quit"},
		{name: "help", input: "help"},
		{name: "mixed case", input: "HeLp"},
		{name: "surrounded by punctuation", input: "show inventory!"},
		{name: "extra whitespace", input: "   character   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.input); got != MetaAction {
				t.Fatalf("Classify(%q) = %q, want %q", tt.input, got, MetaAction)
			}
		})
	}
}

func TestClassify_DefaultsToGameAction(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "explore action", input: "look around"},
		{name: "attack action", input: "attack the goblin"},
		{name: "roleplay input", input: "I say, hello there"},
		{name: "empty string", input: ""},
		{name: "whitespace only", input: "   \t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.input); got != GameAction {
				t.Fatalf("Classify(%q) = %q, want %q", tt.input, got, GameAction)
			}
		})
	}
}

func TestInputClassifier_Classify(t *testing.T) {
	classifier := InputClassifier{}
	if got := classifier.Classify("help"); got != MetaAction {
		t.Fatalf("classifier.Classify(help) = %q, want %q", got, MetaAction)
	}
}
