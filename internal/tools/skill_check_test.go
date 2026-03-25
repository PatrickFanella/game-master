package tools

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type stubModifierResolver struct {
	modifier int
	err      error
}

func (s stubModifierResolver) GetStatModifier(_ context.Context, _ uuid.UUID, _ string) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.modifier, nil
}

type stubRoller struct {
	rolls []int
	idx   int
}

func (s *stubRoller) Intn(_ int) int {
	v := s.rolls[s.idx]
	s.idx++
	return v
}

func TestRegisterSkillCheck(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterSkillCheck(reg, stubModifierResolver{modifier: 2}, &stubRoller{rolls: []int{9}}); err != nil {
		t.Fatalf("register skill_check: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != skillCheckToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, skillCheckToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	if len(required) != 3 {
		t.Fatalf("required schema length = %d, want 3: %#v", len(required), required)
	}
	for _, field := range []string{"character_id", "skill", "difficulty"} {
		found := false
		for _, got := range required {
			if got == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("required schema = %#v, missing field %q", required, field)
		}
	}
}

func TestSkillCheckNormal(t *testing.T) {
	h := NewSkillCheckHandler(stubModifierResolver{modifier: 4}, &stubRoller{rolls: []int{11}})
	id := uuid.New()

	got, err := h.Handle(context.Background(), map[string]any{
		"character_id": id.String(),
		"skill":        "athletics",
		"difficulty":   15.0,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got.Data["roll"] != 12 {
		t.Fatalf("roll = %v, want 12", got.Data["roll"])
	}
	if got.Data["modifier"] != 4 {
		t.Fatalf("modifier = %v, want 4", got.Data["modifier"])
	}
	if got.Data["total"] != 16 {
		t.Fatalf("total = %v, want 16", got.Data["total"])
	}
	if got.Data["dc"] != 15 {
		t.Fatalf("dc = %v, want 15", got.Data["dc"])
	}
	if got.Data["success"] != true {
		t.Fatalf("success = %v, want true", got.Data["success"])
	}
	if got.Data["margin"] != 1 {
		t.Fatalf("margin = %v, want 1", got.Data["margin"])
	}
}

func TestSkillCheckAdvantage(t *testing.T) {
	h := NewSkillCheckHandler(stubModifierResolver{modifier: 2}, &stubRoller{rolls: []int{2, 14}})
	id := uuid.New()

	got, err := h.Handle(context.Background(), map[string]any{
		"character_id": id.String(),
		"skill":        "dexterity",
		"difficulty":   16,
		"advantage":    true,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got.Data["roll"] != 15 {
		t.Fatalf("roll = %v, want 15", got.Data["roll"])
	}
	if got.Data["total"] != 17 {
		t.Fatalf("total = %v, want 17", got.Data["total"])
	}
	if got.Data["success"] != true {
		t.Fatalf("success = %v, want true", got.Data["success"])
	}
}

func TestSkillCheckDisadvantage(t *testing.T) {
	h := NewSkillCheckHandler(stubModifierResolver{modifier: 5}, &stubRoller{rolls: []int{16, 3}})
	id := uuid.New()

	got, err := h.Handle(context.Background(), map[string]any{
		"character_id": id.String(),
		"skill":        "stealth",
		"difficulty":   11,
		"disadvantage": true,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got.Data["roll"] != 4 {
		t.Fatalf("roll = %v, want 4", got.Data["roll"])
	}
	if got.Data["total"] != 9 {
		t.Fatalf("total = %v, want 9", got.Data["total"])
	}
	if got.Data["success"] != false {
		t.Fatalf("success = %v, want false", got.Data["success"])
	}
}

func TestSkillCheckCriticals(t *testing.T) {
	id := uuid.New()

	t.Run("critical success on natural 20", func(t *testing.T) {
		// stubRoller returns Intn(20) values (0-19), and rollD20 adds +1.
		h := NewSkillCheckHandler(stubModifierResolver{modifier: -5}, &stubRoller{rolls: []int{19}})
		got, err := h.Handle(context.Background(), map[string]any{
			"character_id": id.String(),
			"skill":        "arcana",
			"difficulty":   30,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if got.Data["critical_success"] != true {
			t.Fatalf("critical_success = %v, want true", got.Data["critical_success"])
		}
		if got.Data["success"] != true {
			t.Fatalf("success = %v, want true", got.Data["success"])
		}
	})

	t.Run("critical failure on natural 1", func(t *testing.T) {
		// stubRoller returns Intn(20) values (0-19), and rollD20 adds +1.
		h := NewSkillCheckHandler(stubModifierResolver{modifier: 20}, &stubRoller{rolls: []int{0}})
		got, err := h.Handle(context.Background(), map[string]any{
			"character_id": id.String(),
			"skill":        "wisdom",
			"difficulty":   5,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if got.Data["critical_failure"] != true {
			t.Fatalf("critical_failure = %v, want true", got.Data["critical_failure"])
		}
		if got.Data["success"] != false {
			t.Fatalf("success = %v, want false", got.Data["success"])
		}
	})
}

func TestSkillCheckArgumentValidation(t *testing.T) {
	h := NewSkillCheckHandler(stubModifierResolver{modifier: 0}, &stubRoller{rolls: []int{10, 10}})
	id := uuid.New()

	_, err := h.Handle(context.Background(), map[string]any{
		"character_id": id.String(),
		"skill":        "athletics",
		"difficulty":   10,
		"advantage":    true,
		"disadvantage": true,
	})
	if err == nil {
		t.Fatal("expected error when both advantage and disadvantage are true")
	}
}

func TestSkillCheckResolverError(t *testing.T) {
	h := NewSkillCheckHandler(stubModifierResolver{err: errors.New("boom")}, &stubRoller{rolls: []int{10}})
	id := uuid.New()

	_, err := h.Handle(context.Background(), map[string]any{
		"character_id": id.String(),
		"skill":        "athletics",
		"difficulty":   10,
	})
	if err == nil {
		t.Fatal("expected resolver error")
	}
}

func TestRegisterSkillCheckNilResolver(t *testing.T) {
	reg := NewRegistry()
	err := RegisterSkillCheck(reg, nil, &stubRoller{rolls: []int{0}})
	if err == nil {
		t.Fatal("expected error for nil resolver")
	}
	if !strings.Contains(err.Error(), "resolver is required") {
		t.Fatalf("error = %v, want resolver-required message", err)
	}
}

func TestParseIntArgOutOfRangeOrNonFinite(t *testing.T) {
	t.Run("float too large", func(t *testing.T) {
		_, err := parseIntArg(map[string]any{"difficulty": float64(math.MaxInt) * 2}, "difficulty")
		if err == nil {
			t.Fatal("expected out-of-range error")
		}
		if !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("error = %v, want out-of-range message", err)
		}
	})

	t.Run("float nan", func(t *testing.T) {
		_, err := parseIntArg(map[string]any{"difficulty": math.NaN()}, "difficulty")
		if err == nil {
			t.Fatal("expected non-finite error")
		}
		if !strings.Contains(err.Error(), "finite integer") {
			t.Fatalf("error = %v, want finite-integer message", err)
		}
	})

	t.Run("float inf", func(t *testing.T) {
		_, err := parseIntArg(map[string]any{"difficulty": math.Inf(1)}, "difficulty")
		if err == nil {
			t.Fatal("expected non-finite error")
		}
		if !strings.Contains(err.Error(), "finite integer") {
			t.Fatalf("error = %v, want finite-integer message", err)
		}
	})
}
