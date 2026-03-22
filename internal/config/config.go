package config

import (
	"errors"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DB struct {
		URL string `koanf:"url"`
	} `koanf:"db"`
	LLM struct {
		Provider string `koanf:"provider"`
		Ollama   struct {
			Endpoint string `koanf:"endpoint"`
			Model    string `koanf:"model"`
		} `koanf:"ollama"`
	} `koanf:"llm"`
}

func Load(path string) (Config, error) {
	k := koanf.New(".")

	defaults := map[string]any{
		"db.url":              "postgres://game_master:game_master@localhost:5432/game_master?sslmode=disable",
		"llm.provider":        "ollama",
		"llm.ollama.endpoint": "http://localhost:11434",
		"llm.ollama.model":    "llama3.2",
	}

	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return Config{}, err
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
				return Config{}, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
	}

	if err := k.Load(env.Provider("GM_", ".", func(key string) string {
		trimmed := strings.TrimPrefix(key, "GM_")
		return strings.ToLower(strings.ReplaceAll(trimmed, "_", "."))
	}), nil); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
