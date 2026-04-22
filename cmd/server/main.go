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
	"github.com/PatrickFanella/game-master/internal/export"
	"github.com/PatrickFanella/game-master/internal/handlers"
	"github.com/PatrickFanella/game-master/internal/journal"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/logging"
	"github.com/PatrickFanella/game-master/internal/memory"
	"github.com/PatrickFanella/game-master/internal/saves"
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

	saveStore := saves.NewStore(pool)

	engineOpts := []engine.Option{
		engine.WithLogger(slog.Default().WithGroup("engine")),
		engine.WithSaveStore(saveStore),
	}
	if cfg.LLM.Provider == "ollama" {
		embedEndpoint := cfg.LLM.Ollama.EmbeddingEndpoint
		if embedEndpoint == "" {
			embedEndpoint = cfg.LLM.Ollama.Endpoint
		}
		embedder := memory.NewOllamaEmbedder(
			embedEndpoint, cfg.LLM.Ollama.EmbeddingModel,
			memory.WithOllamaEmbedderTimeout(cfg.LLM.Ollama.RequestTimeout()),
		)
		searcher := memory.NewSearcher(embedder, queries)
		tier3 := assembly.NewTier3Retriever(searcher, 5, slog.Default().WithGroup("tier3"))
		engineOpts = append(engineOpts,
			engine.WithTier3Retriever(tier3),
			engine.WithEmbedder(embedder),
			engine.WithSearcher(searcher),
		)
	}

	gameEngine, err := engine.New(pool, provider, cfg.LLM, engineOpts...)
	if err != nil {
		logger.Errorf("create engine: %v", err)
		return 1
	}
	router := newRouterWithProvider(logger, gameEngine, queries, provider, pool, defaultUserID, cfg, saveStore)

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

func newRouterWithProvider(logger *log.Logger, gameEngine engine.GameEngine, queries statedb.Querier, provider llm.Provider, pool *pgxpool.Pool, defaultUserID uuid.UUID, cfg config.Config, saveStore *saves.Store) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*", "https://gm.subcult.tv", "http://gm.subcult.tv", "https://edda.subcult.tv"},
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

	campaignH := &handlers.CampaignHandlers{Engine: gameEngine, Queries: queries, Logger: logger, Pool: pool}
	charH := &handlers.CharacterHandlers{Queries: queries, Logger: logger, Pool: pool}
	worldH := &handlers.WorldHandlers{Queries: queries, Logger: logger}
	actionH := &handlers.ActionHandlers{Engine: gameEngine, Queries: queries, Provider: provider, Logger: logger}
	startupH := handlers.NewStartupHandlers(provider, queries, logger, pool)
	registerAPIRoutes(logger, r, campaignH, charH, worldH, actionH, startupH, pool, defaultUserID, cfg, saveStore, provider)
	return r
}

func registerAPIRoutes(logger *log.Logger, r chi.Router, campaignH *handlers.CampaignHandlers, charH *handlers.CharacterHandlers, worldH *handlers.WorldHandlers, actionH *handlers.ActionHandlers, startupH *handlers.StartupHandlers, pool *pgxpool.Pool, defaultUserID uuid.UUID, cfg config.Config, saveStore *saves.Store, provider llm.Provider) {
	// Choose auth middleware: JWT if a secret is configured, NoOp otherwise (TUI).
	var authMW auth.AuthMiddleware
	if cfg.Server.JWTSecret != "" {
		authMW = auth.NewJWTMiddleware(cfg.Server.JWTSecret)
	} else {
		authMW = auth.NewNoOpMiddleware(defaultUserID)
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(disableCacheMiddleware)

		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(logger, w, http.StatusOK, map[string]any{
				"status":       "ok",
				"engine_ready": campaignH.Engine != nil,
			})
		})

		r.Route("/v1", func(r chi.Router) {
			// Public auth routes (no token required).
			if cfg.Server.JWTSecret != "" {
				r.Route("/auth", func(r chi.Router) {
					authH := auth.NewAuthHandlers(auth.NewDBAuthQuerier(pool), cfg.Server.JWTSecret)
					r.Post("/register", authH.Register)
					r.Post("/login", authH.Login)
					r.Post("/logout", func(w http.ResponseWriter, req *http.Request) {
						auth.ClearSessionCookie(w, req)
						w.WriteHeader(http.StatusNoContent)
					})
				})
			}

			// All remaining routes require authentication.
			r.Group(func(r chi.Router) {
				r.Use(authMW.Authenticate)

				// Authenticated auth routes.
				if cfg.Server.JWTSecret != "" {
					authH := auth.NewAuthHandlers(auth.NewDBAuthQuerier(pool), cfg.Server.JWTSecret)
					r.Get("/auth/me", authH.Me)
				}

				r.Route("/campaigns", func(r chi.Router) {
					r.Get("/", campaignH.ListCampaigns)
					r.Post("/", campaignH.CreateCampaign)
					r.Route("/start", func(r chi.Router) {
						r.Post("/campaign-interview", startupH.StartCampaignInterview)
						r.Post("/campaign-interview/{sessionID}", startupH.StepCampaignInterview)
						r.Post("/proposals", startupH.GenerateCampaignProposals)
						r.Post("/name", startupH.GenerateCampaignName)
						r.Post("/character-interview", startupH.StartCharacterInterview)
						r.Post("/character-interview/{sessionID}", startupH.StepCharacterInterview)
						r.Post("/world", startupH.BuildWorld)
					})
					r.Route("/{id}", func(r chi.Router) {
						r.Get("/", campaignH.GetCampaign)
						r.Put("/", campaignH.UpdateCampaign)
						r.Delete("/", campaignH.DeleteCampaign)
						r.Get("/history", campaignH.GetSessionHistory)

						r.Get("/character", charH.GetCharacter)
						r.Get("/character/inventory", charH.GetCharacterInventory)
						r.Get("/character/abilities", charH.GetCharacterAbilities)
						r.Get("/character/feats", charH.GetCharacterFeats)
						r.Get("/character/skills", charH.GetCharacterSkills)

						r.Get("/locations", worldH.ListLocations)
						r.Get("/locations/{lid}", worldH.GetLocation)

						r.Get("/npcs/encountered", worldH.ListEncounteredNPCs)
						r.Get("/npcs", worldH.ListNPCs)
						r.Get("/npcs/{nid}/dialogue", worldH.GetNPCDialogue)
						r.Get("/npcs/{nid}", worldH.GetNPC)

						r.Get("/quests", worldH.ListQuests)
						r.Get("/quests/{qid}", worldH.GetQuest)
						r.Get("/quests/{qid}/notes", worldH.ListQuestNotes)
						r.Post("/quests/{qid}/notes", worldH.CreateQuestNote)
						r.Delete("/quests/{qid}/notes/{noteID}", worldH.DeleteQuestNote)
						r.Get("/quests/{qid}/history", worldH.ListQuestHistory)

						r.Get("/facts", worldH.ListKnownFacts)
						r.Get("/relationships", worldH.ListAwareRelationships)
						r.Get("/codex/languages", worldH.ListKnownLanguages)
						r.Get("/codex/cultures", worldH.ListKnownCultures)
						r.Get("/codex/beliefs", worldH.ListKnownBeliefSystems)
						r.Get("/codex/economies", worldH.ListKnownEconomicSystems)
						r.Get("/map", worldH.GetMapData)

						r.Post("/action", actionH.ProcessAction)
						r.Get("/ws", actionH.HandleWebSocket)

						savesH := saves.NewHandlers(saveStore)
						r.Post("/saves", savesH.ManualSave)
						r.Get("/saves", savesH.ListSaves)
						r.Post("/start-over", savesH.StartOver)
						r.Get("/time", savesH.GetTime)

						exportH := export.NewHandlers(export.NewStore(pool))
						r.Get("/export/json", exportH.ExportJSON)
						r.Get("/export/transcript", exportH.ExportTranscript)
						r.Get("/export/character", exportH.ExportCharacterSheet)

						journalStore := journal.NewStore(pool)
						journalH := journal.NewHandlersWithSummarizer(journalStore, journal.NewSummarizer(provider, journalStore))
						r.Route("/journal", func(r chi.Router) {
							r.Get("/summaries", journalH.ListSummaries)
							r.Get("/entries", journalH.ListEntries)
							r.Post("/entries", journalH.CreateEntry)
							r.Delete("/entries/{eid}", journalH.DeleteEntry)
							r.Post("/summarize", journalH.Summarize)
						})
					})
				})
			})
		})
	})
}

func disableCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, s-maxage=0")
		headers.Set("Pragma", "no-cache")
		headers.Set("Expires", "0")
		headers.Set("Surrogate-Control", "no-store")

		next.ServeHTTP(w, r)
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
