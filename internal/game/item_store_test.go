package game

import "testing"

func TestToInt32Quantity(t *testing.T) {
	t.Run("within range", func(t *testing.T) {
		got, err := toInt32Quantity(42)
		if err != nil {
			t.Fatalf("toInt32Quantity(42) error = %v", err)
		}
		if got != 42 {
			t.Fatalf("toInt32Quantity(42) = %d, want 42", got)
		}
	})

	t.Run("too large", func(t *testing.T) {
		if _, err := toInt32Quantity(maxInt32 + 1); err == nil {
			t.Fatal("expected out-of-range error for maxInt32+1")
		}
	})

	t.Run("too small", func(t *testing.T) {
		if _, err := toInt32Quantity(minInt32 - 1); err == nil {
			t.Fatal("expected out-of-range error for minInt32-1")
		}
	})
}
