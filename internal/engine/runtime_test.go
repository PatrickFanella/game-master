package engine

import (
	"encoding/json"
	"testing"
)

func TestMarshalAppliedToolCallsNilEncodesArray(t *testing.T) {
	got, err := marshalAppliedToolCalls(nil)
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls(nil) error = %v", err)
	}
	if string(got) != "[]" {
		t.Fatalf("expected empty JSON array, got %s", got)
	}
}

func TestMarshalAppliedToolCallsPreservesEntries(t *testing.T) {
	got, err := marshalAppliedToolCalls([]AppliedToolCall{{
		Tool:      "skill_check",
		Arguments: json.RawMessage(`{"skill":"stealth"}`),
		Result:    json.RawMessage(`{"success":true}`),
	}})
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls() error = %v", err)
	}
	if string(got) == "[]" {
		t.Fatal("expected non-empty marshaled tool calls")
	}
}

func TestExtractChoicesParsesNumberedOptions(t *testing.T) {
	narrative := `The torchlight flickers across the chamber.

1. Inspect the altar for hidden mechanisms.
2. Call out to see who answers.
3. Retreat into the corridor.

Or describe what you'd like to do—you're never limited to the options above.`

	cleaned, choices := extractChoices(narrative)

	if cleaned != "The torchlight flickers across the chamber." {
		t.Fatalf("unexpected cleaned narrative: %q", cleaned)
	}
	if len(choices) != 3 {
		t.Fatalf("expected 3 parsed choices, got %d", len(choices))
	}
	if choices[1].ID != "2" || choices[1].Text != "Call out to see who answers." {
		t.Fatalf("unexpected parsed choice: %+v", choices[1])
	}
}

func TestExtractChoicesLeavesNarrativeUntouchedWithoutOptions(t *testing.T) {
	narrative := "A calm wind passes through the trees."

	cleaned, choices := extractChoices(narrative)

	if cleaned != narrative {
		t.Fatalf("expected narrative to remain unchanged, got %q", cleaned)
	}
	if choices != nil {
		t.Fatalf("expected no parsed choices, got %+v", choices)
	}
}
