package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/internal/llm"
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

	logger := log.NewWithOptions(os.Stderr, log.Options{ReportTimestamp: true})
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
	provider, err := llm.NewLLMProvider(cfg)
	if err != nil {
		logger.Errorf("create llm provider: %v", err)
		return 1
	}
	gameEngine := engine.New(pool, queries, provider, cfg.LLM)
	router := newRouter(logger, gameEngine)

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

func newRouter(logger *log.Logger, gameEngine engine.GameEngine) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"},
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

	registerAPIRoutes(logger, r, gameEngine)
	return r
}

func registerAPIRoutes(logger *log.Logger, r chi.Router, gameEngine engine.GameEngine) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(logger, w, http.StatusOK, map[string]any{
				"status":       "ok",
				"engine_ready": gameEngine != nil,
			})
		})

		r.Route("/v1", func(r chi.Router) {
			r.Route("/campaigns", func(r chi.Router) { r.Get("/", notImplemented(logger, "campaigns")) })
			r.Route("/characters", func(r chi.Router) { r.Get("/", notImplemented(logger, "characters")) })
			r.Route("/locations", func(r chi.Router) { r.Get("/", notImplemented(logger, "locations")) })
			r.Route("/npcs", func(r chi.Router) { r.Get("/", notImplemented(logger, "npcs")) })
			r.Route("/quests", func(r chi.Router) { r.Get("/", notImplemented(logger, "quests")) })
			r.Route("/items", func(r chi.Router) { r.Get("/", notImplemented(logger, "items")) })
			r.Route("/turns", func(r chi.Router) { r.Post("/", notImplemented(logger, "turns")) })
		})
	})
}

func notImplemented(logger *log.Logger, resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(logger, w, http.StatusNotImplemented, map[string]string{
			"error":   "not_implemented",
			"message": fmt.Sprintf("%s endpoints are not implemented yet", resource),
		})
	}
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
