package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatrickFanella/game-master/internal/assembly"
	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/internal/handlers"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/logging"
	"github.com/PatrickFanella/game-master/internal/memory"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
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

	logResult, err := logging.Setup(".logs/game-master.jsonl", slog.LevelDebug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logging: %v\n", err)
		return 1
	}
	defer logResult.Cleanup()

	logger := log.NewWithOptions(logResult.BridgeWriter, log.Options{ReportTimestamp: true})
	log.SetDefault(logger)

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Errorf("load config: %v", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		logger.Errorf("open database: %v", err)
		return 1
	}
	defer pool.Close()

	queries := statedb.New(pool)

	bootResult, err := bootstrap.Run(ctx, queries)
	if err != nil {
		logger.Errorf("bootstrap: %v", err)
		return 1
	}
	defaultUserID := bootResult.User.ID.Bytes

	provider, err := llm.NewLLMProvider(cfg)
	if err != nil {
		logger.Errorf("create llm provider: %v", err)
		return 1
	}

	engineOpts := []engine.Option{
		engine.WithLogger(slog.Default().WithGroup("engine")),
	}
	if cfg.LLM.Provider == "ollama" {
		embedder := memory.NewOllamaEmbedder(
			cfg.LLM.Ollama.Endpoint, "nomic-embed-text",
			memory.WithOllamaEmbedderTimeout(cfg.LLM.Ollama.RequestTimeout()),
		)
		searcher := memory.NewSearcher(embedder, queries)
		tier3 := assembly.NewTier3Retriever(searcher, 5, slog.Default().WithGroup("tier3"))
		engineOpts = append(engineOpts, engine.WithTier3Retriever(tier3))
	}

	gameEngine, err := engine.New(pool, provider, cfg.LLM, engineOpts...)
	if err != nil {
		logger.Errorf("create engine: %v", err)
		return 1
	}
	router := newRouterWithProvider(logger, gameEngine, queries, provider, defaultUserID)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	logger.Infof("starting HTTP server on %s (provider=%s)", addr, cfg.LLM.Provider)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("server failed: %v", err)
			return 1
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("graceful shutdown failed: %v", err)
			return 1
		}

		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("server failed during shutdown: %v", err)
			return 1
		}
	}

	logger.Info("server shutdown complete")
	return 0
}

func parseConfigPath(args []string, defaultPath string) (string, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := defaultPath
	fs.StringVar(&configPath, "config", configPath, "Path to config file")

	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return configPath, nil
}

func newRouterWithProvider(logger *log.Logger, gameEngine engine.GameEngine, queries statedb.Querier, provider llm.Provider, defaultUserID uuid.UUID) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	h := handlers.New(gameEngine, queries, logger, provider)
	registerAPIRoutes(logger, r, h, defaultUserID)
	return r
}

func registerAPIRoutes(logger *log.Logger, r chi.Router, h *handlers.Handlers, defaultUserID uuid.UUID) {
	authMW := auth.NewNoOpMiddleware(defaultUserID)

	r.Route("/api", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(logger, w, http.StatusOK, map[string]any{
				"status":       "ok",
				"engine_ready": h.Engine != nil,
			})
		})

		r.Route("/v1", func(r chi.Router) {
			r.Use(authMW.Authenticate)

			r.Route("/campaigns", func(r chi.Router) {
				r.Get("/", h.ListCampaigns)
				r.Post("/", h.CreateCampaign)
				r.Route("/start", func(r chi.Router) {
					r.Post("/campaign-interview", h.StartCampaignInterview)
					r.Post("/campaign-interview/{sessionID}", h.StepCampaignInterview)
					r.Post("/proposals", h.GenerateCampaignProposals)
					r.Post("/name", h.GenerateCampaignName)
					r.Post("/character-interview", h.StartCharacterInterview)
					r.Post("/character-interview/{sessionID}", h.StepCharacterInterview)
					r.Post("/world", h.BuildWorld)
				})
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetCampaign)
					r.Put("/", h.UpdateCampaign)
					r.Delete("/", h.DeleteCampaign)
					r.Get("/history", h.GetSessionHistory)

					r.Get("/character", h.GetCharacter)
					r.Get("/character/inventory", h.GetCharacterInventory)
					r.Get("/character/abilities", h.GetCharacterAbilities)

					r.Get("/locations", h.ListLocations)
					r.Get("/locations/{lid}", h.GetLocation)

					r.Get("/npcs", h.ListNPCs)
					r.Get("/npcs/{nid}", h.GetNPC)

					r.Get("/quests", h.ListQuests)
					r.Get("/quests/{qid}", h.GetQuest)

					r.Post("/action", h.ProcessAction)
					r.Get("/ws", h.HandleWebSocket)
				})
			})
		})
	})
}

func writeJSON(logger *log.Logger, w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Errorf("encode json response: %v", err)
	}
}

func loggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			next.ServeHTTP(ww, r)

			logger.Infof(
				"%s %s status=%d bytes=%d duration=%s request_id=%s",
				r.Method,
				r.URL.Path,
				ww.Status(),
				ww.BytesWritten(),
				time.Since(start).Round(time.Millisecond),
				middleware.GetReqID(r.Context()),
			)
		})
	}
}
