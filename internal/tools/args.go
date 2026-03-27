package tools

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

const floatIntegerTolerance = 1e-9

func parseUUIDArg(args map[string]any, key string) (uuid.UUID, error) {
	raw, ok := args[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("%s is required", key)
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return uuid.Nil, fmt.Errorf("%s must be a non-empty string", key)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s must be a valid UUID", key)
	}
	return id, nil
}

func parseStringArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := raw.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return s, nil
}

func parseBoolArg(args map[string]any, key string) (bool, error) {
	raw, ok := args[key]
	if !ok {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}

func parseIntArg(args map[string]any, key string) (int, error) {
	raw, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch v := raw.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		if v < int64(math.MinInt) || v > int64(math.MaxInt) {
			return 0, fmt.Errorf("%s is out of range", key)
		}
		return int(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s must be a finite integer", key)
		}
		rounded := math.Round(v)
		if math.Abs(v-rounded) > floatIntegerTolerance {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		if rounded < float64(math.MinInt) || rounded > float64(math.MaxInt) {
			return 0, fmt.Errorf("%s is out of range", key)
		}
		return int(rounded), nil
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func parseJSONObjectArg(args map[string]any, key string) (map[string]any, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return obj, nil
}

func parseUUIDArrayArg(args map[string]any, key string) ([]uuid.UUID, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]uuid.UUID, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string UUID", key, i)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%s[%d] must be a valid UUID", key, i)
		}
		out = append(out, id)
	}
	return out, nil
}

func parseStringArrayArg(args map[string]any, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string", key, i)
		}
		out = append(out, s)
	}
	return out, nil
}
