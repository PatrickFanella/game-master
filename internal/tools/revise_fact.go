package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const reviseFactToolName = "revise_fact"

// ReviseFactStore persists fact supersession.
type ReviseFactStore interface {
	GetFactByID(ctx context.Context, id pgtype.UUID) (statedb.WorldFact, error)
	SupersedeFact(ctx context.Context, arg statedb.SupersedeFactParams) (statedb.WorldFact, error)
}

// ReviseFactTool returns the revise_fact tool definition and JSON schema.
func ReviseFactTool() llm.Tool {
	return llm.Tool{
		Name:        reviseFactToolName,
		Description: "Revise a canonical world fact by creating a new fact that supersedes the old one. The old fact is marked as superseded and excluded from active queries.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fact_id": map[string]any{
					"type":        "string",
					"description": "UUID of the existing world fact to supersede.",
				},
				"new_fact": map[string]any{
					"type":        "string",
					"description": "The revised fact text that will replace the old one.",
				},
			},
			"required":             []string{"fact_id", "new_fact"},
			"additionalProperties": false,
		},
	}
}

// RegisterReviseFact registers the revise_fact tool in the registry.
func RegisterReviseFact(reg *Registry, factStore ReviseFactStore, memoryStore MemoryStore, embedder Embedder) error {
	if factStore == nil {
		return errors.New("revise_fact fact store is required")
	}
	tool := ReviseFactTool()
	handler := NewReviseFactHandler(factStore, memoryStore, embedder)
	return reg.Register(tool, func(ctx context.Context, args map[string]any) (*ToolResult, error) {
		return handler.Handle(ctx, args)
	})
}

// ReviseFactHandler handles the revise_fact tool call.
type ReviseFactHandler struct {
	factStore   ReviseFactStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewReviseFactHandler creates a new ReviseFactHandler.
func NewReviseFactHandler(factStore ReviseFactStore, memoryStore MemoryStore, embedder Embedder) *ReviseFactHandler {
	return &ReviseFactHandler{
		factStore:   factStore,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the revise_fact tool.
func (h *ReviseFactHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("revise_fact handler is nil")
	}
	if h.factStore == nil {
		return nil, errors.New("revise_fact fact store is required")
	}

	factID, err := parseUUIDArg(args, "fact_id")
	if err != nil {
		return nil, err
	}
	newFactText, err := parseStringArg(args, "new_fact")
	if err != nil {
		return nil, err
	}

	oldFact, err := h.factStore.GetFactByID(ctx, dbutil.ToPgtype(factID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("fact_id does not reference an existing world fact")
		}
		return nil, fmt.Errorf("get existing fact: %w", err)
	}
	if oldFact.SupersededBy.Valid {
		return nil, fmt.Errorf("fact %s is already superseded and cannot be revised", factID)
	}

	newFact, err := h.factStore.SupersedeFact(ctx, statedb.SupersedeFactParams{
		OldFactID: dbutil.ToPgtype(factID),
		Fact:      newFactText,
		Category:  oldFact.Category,
		Source:    reviseFactToolName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("fact %s was superseded concurrently; fetch the current fact and retry", factID)
		}
		return nil, fmt.Errorf("supersede fact: %w", err)
	}

	newFactID := dbutil.FromPgtype(newFact.ID)
	campaignID := dbutil.FromPgtype(newFact.CampaignID)

	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedRevisedFactMemory(ctx, campaignID, factID, newFactID, newFactText, oldFact.Category); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"new_fact_id":    newFactID.String(),
			"old_fact_id":    factID.String(),
			"campaign_id":    campaignID.String(),
			"new_fact":       newFactText,
			"category":       oldFact.Category,
			"source":         reviseFactToolName,
			"supersedes":     factID.String(),
		},
		Narrative: fmt.Sprintf("World fact revised: %q supersedes fact %s.", newFactText, factID),
	}, nil
}

func (h *ReviseFactHandler) embedRevisedFactMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	oldFactID uuid.UUID,
	newFactID uuid.UUID,
	newFactText string,
	category string,
) error {
	memoryContent := fmt.Sprintf("World fact revised. Category: %s. New fact: %s (supersedes fact %s)", category, newFactText, oldFactID)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed revised fact memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"fact_id":      newFactID.String(),
		"old_fact_id":  oldFactID.String(),
		"category":     category,
		"source":       reviseFactToolName,
		"supersedes":   oldFactID.String(),
	})
	if err != nil {
		return fmt.Errorf("marshal revised fact memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create revised fact memory: %w", err)
	}
	return nil
}
