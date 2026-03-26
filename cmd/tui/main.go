package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/tui"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	configPath, err := parseConfigPath(args, os.Getenv("GM_CONFIG"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse flags: %v\n", err)
		return 2
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
	})
	log.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Errorf("load config: %v", err)
		return 1
	}
	if _, err := llm.NewLLMProvider(cfg); err != nil {
		logger.Errorf("initialize llm provider: %v", err)
		return 1
	}
	logger.Infof("starting TUI (provider=%s)", cfg.LLM.Provider)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		logger.Errorf("open database: %v", err)
		return 1
	}
	defer pool.Close()

	queries := statedb.New(pool)
	provider, err := newLLMProvider(cfg)
	if err != nil {
		logger.Errorf("create llm provider: %v", err)
		return 1
	}
	gameEngine := engine.New(pool, queries, provider)

	p := tea.NewProgram(
		tui.NewLauncherWithEngine(cfg, ctx, queries, gameEngine),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(ctx),
	)

	go func() {
		<-ctx.Done()
		logger.Info("shutdown signal received")
	}()

	if _, err := p.Run(); err != nil {
		if ctx.Err() != nil && (errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled)) {
			logger.Info("TUI shutdown complete")
			return 0
		}
		logger.Errorf("tui error: %v", err)
		return 1
	}

	logger.Info("TUI stopped")
	return 0
}

func parseConfigPath(args []string, defaultPath string) (string, error) {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := defaultPath
	fs.StringVar(&configPath, "config", configPath, "Path to config file")

	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return configPath, nil
}

func newLLMProvider(cfg config.Config) (llm.Provider, error) {
	switch cfg.LLM.Provider {
	case "claude":
		return llm.NewClaudeClient("", cfg.LLM.Claude.APIKey, cfg.LLM.Claude.Model), nil
	case "ollama":
		return llm.NewOllamaClient(cfg.LLM.Ollama.Endpoint, cfg.LLM.Ollama.Model), nil
	default:
		return nil, fmt.Errorf("unknown llm provider: %q", cfg.LLM.Provider)
	}
}
