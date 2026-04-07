package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const grantFeatToolName = "grant_feat"

// GrantFeatDB is the minimal database interface for the grant_feat tool.
type GrantFeatDB interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// GrantFeatTool returns the grant_feat tool definition and JSON schema.
func GrantFeatTool() llm.Tool {
	return llm.Tool{
		Name:        grantFeatToolName,
		Description: "Grant a feat to a character. Looks up the feat definition by name in the campaign and records it on the character.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"feat_name": map[string]any{
					"type":        "string",
					"description": "Name of the feat to grant (must match a feat_definition in the campaign).",
				},
				"character_id": map[string]any{
					"type":        "string",
					"description": "UUID of the character to grant the feat to. Defaults to the current player character if omitted.",
				},
			},
			"required":             []string{"feat_name"},
			"additionalProperties": false,
		},
	}
}

// RegisterGrantFeat registers the grant_feat tool and handler.
func RegisterGrantFeat(reg *Registry, db GrantFeatDB) error {
	if db == nil {
		return errors.New("grant_feat database is required")
	}
	handler := &GrantFeatHandler{db: db}
	return reg.Register(GrantFeatTool(), handler.Handle)
}

// GrantFeatHandler executes grant_feat tool calls.
type GrantFeatHandler struct {
	db GrantFeatDB
}

// Handle executes the grant_feat tool.
func (h *GrantFeatHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("grant_feat handler is not configured")
	}

	featName, err := parseStringArg(args, "feat_name")
	if err != nil {
		return nil, err
	}

	campaignID, ok := CurrentCampaignIDFromContext(ctx)
	if !ok {
		return nil, errors.New("grant_feat requires a current campaign in context")
	}

	var characterID uuid.UUID
	charIDStr, _ := args["character_id"].(string)
	if charIDStr != "" {
		characterID, err = uuid.Parse(charIDStr)
		if err != nil {
			return nil, fmt.Errorf("character_id must be a valid UUID")
		}
	} else {
		characterID, ok = CurrentPlayerCharacterIDFromContext(ctx)
		if !ok {
			return nil, errors.New("grant_feat requires a character_id or current player character in context")
		}
	}

	// Look up feat definition by name in the campaign.
	var featID uuid.UUID
	var featDesc string
	err = h.db.QueryRow(ctx,
		`SELECT id, description FROM feat_definitions WHERE campaign_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1`,
		campaignID, strings.TrimSpace(featName),
	).Scan(&featID, &featDesc)
	if err != nil {
		return nil, fmt.Errorf("feat %q not found in this campaign", featName)
	}

	// Insert into character_feats.
	_, err = h.db.Exec(ctx,
		`INSERT INTO character_feats (character_id, feat_id) VALUES ($1, $2) ON CONFLICT (character_id, feat_id) DO NOTHING`,
		characterID, featID,
	)
	if err != nil {
		return nil, fmt.Errorf("grant feat: %w", err)
	}

	narrative := fmt.Sprintf("You have gained the feat: %s. %s", featName, featDesc)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"feat_name":    featName,
			"feat_id":      featID.String(),
			"character_id": characterID.String(),
		},
		Narrative: narrative,
	}, nil
}
