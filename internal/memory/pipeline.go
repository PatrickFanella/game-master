package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	pgvector_go "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

// EmbedJob describes a single piece of text to be embedded and persisted as a
// memory. Jobs are submitted to a Pipeline for asynchronous processing.
type EmbedJob struct {
	Text       string
	CampaignID uuid.UUID
	MemoryType string
	LocationID *uuid.UUID
	NPCs       []uuid.UUID
	InGameTime string
	Metadata   map[string]any
}

// PipelineStore is the narrow persistence interface consumed by Pipeline.
type PipelineStore interface {
	CreateMemory(ctx context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error)
}

// Pipeline is an asynchronous embedding-and-store worker. Callers submit
// EmbedJob values via Submit; a single background goroutine embeds the text
// and writes the resulting memory to the store.
type Pipeline struct {
	embedder Embedder
	store    PipelineStore
	jobs     chan EmbedJob
	done     chan struct{}
	logger   *slog.Logger
}

const defaultBufferSize = 64

// NewPipeline creates a Pipeline and starts its background worker.
// bufferSize controls the channel capacity; values <= 0 default to 64.
// A nil logger falls back to slog.Default().
func NewPipeline(embedder Embedder, store PipelineStore, bufferSize int, logger *slog.Logger) *Pipeline {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	if logger == nil {
		logger = slog.Default()
	}
	p := &Pipeline{
		embedder: embedder,
		store:    store,
		jobs:     make(chan EmbedJob, bufferSize),
		done:     make(chan struct{}),
		logger:   logger,
	}
	go p.run()
	return p
}

// Submit enqueues a job for asynchronous processing. It returns false without
// blocking if the buffer is full or the pipeline has been shut down.
func (p *Pipeline) Submit(job EmbedJob) (ok bool) {
	// Recover from a send-on-closed-channel panic that occurs if Shutdown
	// already closed p.jobs.
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()

	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

// Shutdown signals the worker to stop by closing the jobs channel, then waits
// for it to drain remaining work. If ctx expires before the worker finishes,
// ctx.Err() is returned.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	close(p.jobs)

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("pipeline shutdown: %w", ctx.Err())
	}
}

// run is the background loop that processes jobs until the channel is closed.
func (p *Pipeline) run() {
	defer close(p.done)

	for job := range p.jobs {
		p.process(job)
	}
}

// process embeds and persists a single job. Errors are logged, never
// propagated—the pipeline must keep draining.
func (p *Pipeline) process(job EmbedJob) {
	ctx := context.Background()

	vec, err := p.embedder.Embed(ctx, job.Text)
	if err != nil {
		p.logger.Error("embedding failed", "error", err, "memory_type", job.MemoryType)
		return
	}

	var metaBytes []byte
	if job.Metadata != nil {
		metaBytes, err = json.Marshal(job.Metadata)
		if err != nil {
			p.logger.Error("metadata marshal failed", "error", err, "memory_type", job.MemoryType)
			return
		}
	}

	params := statedb.CreateMemoryParams{
		CampaignID:   dbutil.ToPgtype(job.CampaignID),
		Content:      job.Text,
		Embedding:    pgvector_go.NewVector(vec),
		MemoryType:   job.MemoryType,
		LocationID:   locationToPgtype(job.LocationID),
		NpcsInvolved: npcsToPgtype(job.NPCs),
		InGameTime:   inGameTimeToPgtype(job.InGameTime),
		Metadata:     metaBytes,
	}

	if _, err := p.store.CreateMemory(ctx, params); err != nil {
		p.logger.Error("store failed", "error", err, "memory_type", job.MemoryType)
	}
}

func locationToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return dbutil.ToPgtype(*id)
}

func npcsToPgtype(ids []uuid.UUID) []pgtype.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = dbutil.ToPgtype(id)
	}
	return out
}

func inGameTimeToPgtype(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
