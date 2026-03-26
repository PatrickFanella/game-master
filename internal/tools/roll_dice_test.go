package tools

import (
	"context"
	"strings"
	"testing"
)

func TestRegisterRollDice(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterRollDice(reg); err != nil {
		t.Fatalf("register roll_dice: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != rollDiceToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, rollDiceToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	if len(required) != 2 || required[0] != "dice" || required[1] != "reason" {
		t.Fatalf("required schema = %#v, want [dice reason]", required)
	}
}

func TestParseDiceNotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  diceExpression
	}{
		{
			name:  "NdS",
			input: "2d6",
			want:  diceExpression{Count: 2, Sides: 6, Modifier: 0},
		},
		{
			name:  "NdS plus modifier",
			input: "1d20+5",
			want:  diceExpression{Count: 1, Sides: 20, Modifier: 5},
		},
		{
			name:  "NdS minus modifier",
			input: "3d8-2",
			want:  diceExpression{Count: 3, Sides: 8, Modifier: -2},
		},
		{
			name:  "uppercase and whitespace",
			input: " 4D10+1 ",
			want:  diceExpression{Count: 4, Sides: 10, Modifier: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDiceNotation(tt.input)
			if err != nil {
				t.Fatalf("parseDiceNotation(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parseDiceNotation(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDiceNotationInvalid(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"d6",
		"0d6",
		"2d7",
		"2d20+",
		"2d20-",
		"2d20+1.5",
		"2x6+3",
	}

	for _, dice := range invalid {
		t.Run(dice, func(t *testing.T) {
			t.Parallel()
			_, err := parseDiceNotation(dice)
			if err == nil {
				t.Fatalf("parseDiceNotation(%q): expected error", dice)
			}
		})
	}
}

func TestRollDiceHandleRangeValidation(t *testing.T) {
	h := NewRollDiceHandler()
	reason := "attack damage"

	tests := []struct {
		name string
		dice string
	}{
		{name: "d4 range", dice: "5d4"},
		{name: "d6 range", dice: "5d6"},
		{name: "d8 range", dice: "5d8"},
		{name: "d10 range", dice: "5d10"},
		{name: "d12 range", dice: "5d12"},
		{name: "d20 range", dice: "5d20"},
		{name: "d100 range", dice: "5d100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parseDiceNotation(tt.dice)
			if err != nil {
				t.Fatalf("parseDiceNotation(%q): %v", tt.dice, err)
			}

			result, err := h.Handle(context.Background(), map[string]any{
				"dice":   tt.dice,
				"reason": reason,
			})
			if err != nil {
				t.Fatalf("Handle(%q): %v", tt.dice, err)
			}

			rolls, ok := result.Data["rolls"].([]int)
			if !ok {
				t.Fatalf("rolls type = %T, want []int", result.Data["rolls"])
			}
			if len(rolls) != expr.Count {
				t.Fatalf("roll count = %d, want %d", len(rolls), expr.Count)
			}

			sum := 0
			for _, roll := range rolls {
				if roll < 1 || roll > expr.Sides {
					t.Fatalf("roll %d out of range [1,%d]", roll, expr.Sides)
				}
				sum += roll
			}

			modifier, ok := result.Data["modifier"].(int)
			if !ok {
				t.Fatalf("modifier type = %T, want int", result.Data["modifier"])
			}
			if modifier != expr.Modifier {
				t.Fatalf("modifier = %d, want %d", modifier, expr.Modifier)
			}

			total, ok := result.Data["total"].(int)
			if !ok {
				t.Fatalf("total type = %T, want int", result.Data["total"])
			}
			if total != sum+expr.Modifier {
				t.Fatalf("total = %d, want %d", total, sum+expr.Modifier)
			}

			gotReason, ok := result.Data["reason"].(string)
			if !ok {
				t.Fatalf("reason type = %T, want string", result.Data["reason"])
			}
			if gotReason != reason {
				t.Fatalf("reason = %q, want %q", gotReason, reason)
			}
		})
	}
}

func TestRollDiceHandleInvalidNotation(t *testing.T) {
	h := NewRollDiceHandler()
	_, err := h.Handle(context.Background(), map[string]any{
		"dice":   "2d7+1",
		"reason": "invalid die",
	})
	if err == nil {
		t.Fatal("expected invalid notation error")
	}
	if !strings.Contains(err.Error(), "NdS") {
		t.Fatalf("error = %v, want notation guidance", err)
	}
}
