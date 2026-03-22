package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsWhenFileIsMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://game_master:game_master@localhost:5432/game_master?sslmode=disable" {
		t.Fatalf("unexpected default db url: %q", cfg.DB.URL)
	}
	if cfg.LLM.Provider != "ollama" {
		t.Fatalf("unexpected default provider: %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Ollama.Endpoint != "http://localhost:11434" {
		t.Fatalf("unexpected default ollama endpoint: %q", cfg.LLM.Ollama.Endpoint)
	}
}

func TestLoadMergesFileAndEnvironment(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")

	const fileConfig = `db:
  url: postgres://from-file/filedb?sslmode=disable
llm:
  provider: openai
  ollama:
    endpoint: http://from-file:11434
    model: file-model
`

	if err := os.WriteFile(configPath, []byte(fileConfig), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("GM_LLM_PROVIDER", "ollama")
	t.Setenv("GM_LLM_OLLAMA_ENDPOINT", "http://from-env:11434")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://from-file/filedb?sslmode=disable" {
		t.Fatalf("expected db url from file, got %q", cfg.DB.URL)
	}
	if cfg.LLM.Provider != "ollama" {
		t.Fatalf("expected env to override provider, got %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Ollama.Endpoint != "http://from-env:11434" {
		t.Fatalf("expected env to override ollama endpoint, got %q", cfg.LLM.Ollama.Endpoint)
	}
	if cfg.LLM.Ollama.Model != "file-model" {
		t.Fatalf("expected model from file, got %q", cfg.LLM.Ollama.Model)
	}
}
