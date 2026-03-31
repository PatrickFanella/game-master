package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const (
	establishFactToolName = "establish_fact"
	establishedSource     = "established"
)

// EstablishFactStore persists world facts.
type EstablishFactStore interface {
	CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
}

// EstablishFactTool returns the establish_fact tool definition and JSON schema.
func EstablishFactTool() llm.Tool {
	return llm.Tool{
		Name:        establishFactToolName,
		Description: "Establish a canonical world truth (world fact) for the current campaign.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fact": map[string]any{
					"type":        "string",
					"description": "The canonical fact text to record as a world truth.",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Category for the fact (e.g. 'history', 'geography', 'politics').",
				},
			},
			"required":             []string{"fact", "category"},
			"additionalProperties": false,
		},
	}
}

// RegisterEstablishFact registers the establish_fact tool in the registry.
func RegisterEstablishFact(reg *Registry, factStore EstablishFactStore, memoryStore MemoryStore, embedder Embedder) error {
	if factStore == nil {
		return errors.New("establish_fact fact store is required")
	}
	tool := EstablishFactTool()
	handler := NewEstablishFactHandler(factStore, memoryStore, embedder)
	return reg.Register(tool, func(ctx context.Context, args map[string]any) (*ToolResult, error) {
		return handler.Handle(ctx, args)
	})
}

// EstablishFactHandler handles the establish_fact tool call.
type EstablishFactHandler struct {
	factStore   EstablishFactStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewEstablishFactHandler creates a new EstablishFactHandler.
func NewEstablishFactHandler(factStore EstablishFactStore, memoryStore MemoryStore, embedder Embedder) *EstablishFactHandler {
	return &EstablishFactHandler{
		factStore:   factStore,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the establish_fact tool.
func (h *EstablishFactHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("establish_fact handler is nil")
	}
	if h.factStore == nil {
		return nil, errors.New("establish_fact fact store is required")
	}

	fact, err := parseStringArg(args, "fact")
	if err != nil {
		return nil, err
	}
	category, err := parseStringArg(args, "category")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("establish_fact requires current location id in context")
	}
	currentLocation, err := h.factStore.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}

	worldFact, err := h.factStore.CreateFact(ctx, statedb.CreateFactParams{
		CampaignID: currentLocation.CampaignID,
		Fact:       fact,
		Category:   category,
		Source:     establishedSource,
	})
	if err != nil {
		return nil, fmt.Errorf("create world fact: %w", err)
	}

	factID := dbutil.FromPgtype(worldFact.ID)
	campaignID := dbutil.FromPgtype(worldFact.CampaignID)

	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedFactMemory(ctx, campaignID, factID, fact, category); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":          factID.String(),
			"campaign_id": campaignID.String(),
			"fact":        fact,
			"category":    category,
			"source":      establishedSource,
		},
		Narrative: fmt.Sprintf("World fact established: %q (category: %s).", fact, category),
	}, nil
}

func (h *EstablishFactHandler) embedFactMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	factID uuid.UUID,
	fact string,
	category string,
) error {
	memoryContent := fmt.Sprintf("World fact established. Category: %s. Fact: %s", category, fact)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed world fact memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"fact_id":  factID.String(),
		"category": category,
		"source":   establishedSource,
	})
	if err != nil {
		return fmt.Errorf("marshal world fact memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create world fact memory: %w", err)
	}
	return nil
}
