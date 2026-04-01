package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesDefaultsWhenFileIsMissing(t *testing.T) {
	unsetenv(t, "GM_DB_URL", "GM_LLM_PROVIDER", "GM_LLM_OLLAMA_ENDPOINT", "GM_LLM_OLLAMA_MODEL")

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
	if cfg.LLM.Ollama.ContextTokenBudget != 8000 {
		t.Fatalf("unexpected default ollama context token budget: %d", cfg.LLM.Ollama.ContextTokenBudget)
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

	unsetenv(t, "GM_DB_URL", "GM_LLM_OLLAMA_MODEL")

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

func TestLoadClaudeConfigFromEnv(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("GM_LLM_CLAUDE_APIKEY", "sk-ant-test-key")
	t.Setenv("GM_LLM_CLAUDE_MODEL", "claude-opus-4-6")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Provider != "claude" {
		t.Fatalf("expected claude provider, got %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Claude.APIKey != "sk-ant-test-key" {
		t.Fatalf("expected claude api key, got %q", cfg.LLM.Claude.APIKey)
	}
	if cfg.LLM.Claude.Model != "claude-opus-4-6" {
		t.Fatalf("expected claude model override, got %q", cfg.LLM.Claude.Model)
	}
}

func TestLoadServerPortDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadClaudeModelDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Claude.Model != "claude-sonnet-4-6" {
		t.Fatalf("expected default claude model, got %q", cfg.LLM.Claude.Model)
	}
	if cfg.LLM.Claude.ContextTokenBudget != 8000 {
		t.Fatalf("expected default claude context token budget, got %d", cfg.LLM.Claude.ContextTokenBudget)
	}
}

func TestLoadContextTokenBudgetFromEnv(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("GM_LLM_CLAUDE_APIKEY", "sk-ant-test-key")
	t.Setenv("GM_LLM_CLAUDE_CONTEXTTOKENBUDGET", "6400")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.ContextTokenBudget() != 6400 {
		t.Fatalf("expected active provider context token budget from env, got %d", cfg.LLM.ContextTokenBudget())
	}
}

func TestLoadRejectsInvalidProvider(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "invalid")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for invalid provider, got nil")
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Fatalf("expected provider-related error, got: %v", err)
	}
}

func TestLoadRejectsClaudeWithoutAPIKey(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	// Explicitly clear all API key env vars to ensure the test is deterministic.
	unsetenv(t, "GM_LLM_CLAUDE_APIKEY", "GM_CLAUDE_API_KEY", "ANTHROPIC_API_KEY")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for claude without api key, got nil")
	}
	if !strings.Contains(err.Error(), "api key") {
		t.Fatalf("expected api key error, got: %v", err)
	}
}

func TestLoadClaudeAPIKeyFromAnthropicEnv(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-anthropic-key")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Claude.APIKey != "sk-ant-anthropic-key" {
		t.Fatalf("expected api key from ANTHROPIC_API_KEY, got %q", cfg.LLM.Claude.APIKey)
	}
}

func TestLoadClaudeAPIKeyFromGMClaudeAPIKeyEnv(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("GM_CLAUDE_API_KEY", "sk-ant-gm-claude-key")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Claude.APIKey != "sk-ant-gm-claude-key" {
		t.Fatalf("expected api key from GM_CLAUDE_API_KEY, got %q", cfg.LLM.Claude.APIKey)
	}
}

func TestGMClaudeAPIKeyOverridesAnthropicAPIKey(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-lower-priority")
	t.Setenv("GM_CLAUDE_API_KEY", "sk-ant-higher-priority")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Claude.APIKey != "sk-ant-higher-priority" {
		t.Fatalf("expected GM_CLAUDE_API_KEY to override ANTHROPIC_API_KEY, got %q", cfg.LLM.Claude.APIKey)
	}
}

func TestGMLLMClaudeAPIKeyOverridesGMClaudeAPIKey(t *testing.T) {
	t.Setenv("GM_LLM_PROVIDER", "claude")
	t.Setenv("GM_CLAUDE_API_KEY", "sk-ant-lower-priority")
	t.Setenv("GM_LLM_CLAUDE_APIKEY", "sk-ant-highest-priority")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Claude.APIKey != "sk-ant-highest-priority" {
		t.Fatalf("expected GM_LLM_CLAUDE_APIKEY to have highest priority, got %q", cfg.LLM.Claude.APIKey)
	}
}

func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		key := key
		if orig, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, orig) })
		} else {
			t.Cleanup(func() { os.Unsetenv(key) })
		}
		os.Unsetenv(key)
	}
}

func TestClaudeAPIKeyPrecedenceFileVsEnv(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")

	const fileConfig = `llm:
  provider: claude
  claude:
    apikey: sk-ant-from-file
`
	if err := os.WriteFile(configPath, []byte(fileConfig), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Verify file key is used when no env vars are set.
	t.Run("file_key_used_when_no_env", func(t *testing.T) {
		unsetenv(t, "GM_LLM_CLAUDE_APIKEY", "GM_CLAUDE_API_KEY", "ANTHROPIC_API_KEY")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LLM.Claude.APIKey != "sk-ant-from-file" {
			t.Fatalf("expected file api key, got %q", cfg.LLM.Claude.APIKey)
		}
	})

	// Verify ANTHROPIC_API_KEY overrides file key.
	t.Run("anthropic_env_overrides_file", func(t *testing.T) {
		unsetenv(t, "GM_LLM_CLAUDE_APIKEY", "GM_CLAUDE_API_KEY")
		t.Setenv("ANTHROPIC_API_KEY", "sk-ant-anthropic-override")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LLM.Claude.APIKey != "sk-ant-anthropic-override" {
			t.Fatalf("expected ANTHROPIC_API_KEY to override file key, got %q", cfg.LLM.Claude.APIKey)
		}
	})

	// Verify GM_CLAUDE_API_KEY overrides ANTHROPIC_API_KEY and file key.
	t.Run("gm_claude_api_key_overrides_anthropic_and_file", func(t *testing.T) {
		unsetenv(t, "GM_LLM_CLAUDE_APIKEY")
		t.Setenv("GM_CLAUDE_API_KEY", "sk-ant-gm-override")
		t.Setenv("ANTHROPIC_API_KEY", "sk-ant-anthropic-override")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LLM.Claude.APIKey != "sk-ant-gm-override" {
			t.Fatalf("expected GM_CLAUDE_API_KEY to win, got %q", cfg.LLM.Claude.APIKey)
		}
	})

	// Verify GM_LLM_CLAUDE_APIKEY has highest priority over all others.
	t.Run("gm_llm_claude_apikey_highest_priority", func(t *testing.T) {
		t.Setenv("GM_LLM_CLAUDE_APIKEY", "sk-ant-gm-llm-highest")
		t.Setenv("GM_CLAUDE_API_KEY", "sk-ant-gm-override")
		t.Setenv("ANTHROPIC_API_KEY", "sk-ant-anthropic-override")
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LLM.Claude.APIKey != "sk-ant-gm-llm-highest" {
			t.Fatalf("expected GM_LLM_CLAUDE_APIKEY to have highest priority, got %q", cfg.LLM.Claude.APIKey)
		}
	})
}

func TestValidateAcceptsValidProviders(t *testing.T) {
	for _, provider := range []string{"ollama", "claude"} {
		t.Run(provider, func(t *testing.T) {
			cfg := Config{LLM: LLMConfig{Provider: provider}}
			if provider == "claude" {
				cfg.LLM.Claude.APIKey = "sk-ant-test"
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid provider %q rejected: %v", provider, err)
			}
		})
	}
}
