package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/assembly"
	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// Engine is the concrete GameEngine implementation used by the TUI.
type Engine struct {
	queries   statedb.Querier
	state     game.StateManager
	assembler *assembly.ContextAssembler
	processor *TurnProcessor
}

const recentTurnLimit = 10
const rightSingleQuote = "’"

// New creates a concrete GameEngine backed by the shared game and llm packages.
func New(db statedb.DBTX, queries statedb.Querier, provider llm.Provider) *Engine {
	registry := tools.NewRegistry()
	if err := tools.RegisterMovePlayer(registry, game.NewMovePlayerStore(queries)); err != nil {
		panic(fmt.Sprintf("failed to register move_player tool during initialization: %v", err))
	}
	if err := tools.RegisterRollDice(registry); err != nil {
		panic(fmt.Sprintf("failed to register roll_dice tool during initialization: %v", err))
	if err := tools.RegisterUpdateNPC(registry, game.NewUpdateNPCStore(queries)); err != nil {
		panic(fmt.Sprintf("failed to register update_npc tool during initialization: %v", err))
	}
	return &Engine{
		queries:   queries,
		state:     game.NewStateManager(db),
		assembler: assembly.NewContextAssembler(registry),
		processor: NewTurnProcessor(provider, registry, tools.NewValidator(registry)),
	}
}

var _ GameEngine = (*Engine)(nil)

func (e *Engine) ProcessTurn(ctx context.Context, campaignID uuid.UUID, playerInput string) (*TurnResult, error) {
	state, err := e.state.GatherState(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state: %w", err)
	}
	if state.Player.ID != uuid.Nil {
		ctx = tools.WithCurrentPlayerCharacterID(ctx, state.Player.ID)
	}
	if state.Player.CurrentLocationID != nil {
		ctx = tools.WithCurrentLocationID(ctx, *state.Player.CurrentLocationID)
	}

	recentLogs, err := e.queries.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: dbutil.ToPgtype(campaignID),
		LimitCount: recentTurnLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list recent session logs: %w", err)
	}

	recentTurns := sessionLogsToDomain(recentLogs)
	messages := e.assembler.AssembleContext(state, recentTurns, playerInput)
	narrative, applied, err := e.processor.ProcessWithRecovery(ctx, messages, e.assembler.Tools())
	if err != nil {
		return nil, fmt.Errorf("process turn: %w", err)
	}

	narrative, choices := extractChoices(narrative)

	result := &TurnResult{
		Narrative:        narrative,
		AppliedToolCalls: applied,
		Choices:          choices,
	}

	toolCallsJSON, err := marshalAppliedToolCalls(applied)
	if err != nil {
		return nil, fmt.Errorf("marshal applied tool calls: %w", err)
	}

	log := domain.SessionLog{
		CampaignID:  campaignID,
		TurnNumber:  nextTurnNumber(recentLogs),
		PlayerInput: playerInput,
		InputType:   domain.Classify(playerInput),
		LLMResponse: narrative,
		ToolCalls:   toolCallsJSON,
		LocationID:  state.Player.CurrentLocationID,
	}
	if err := e.state.SaveSessionLog(ctx, log); err != nil {
		return nil, fmt.Errorf("save session log: %w", err)
	}

	return result, nil
}

func (e *Engine) GetGameState(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	state, err := e.state.GatherState(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state: %w", err)
	}
	return GameStateFromFull(state), nil
}

func (e *Engine) NewCampaign(ctx context.Context, userID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := bootstrap.CreateCampaign(ctx, e.queries, dbutil.ToPgtype(userID), bootstrap.DefaultCampaignName)
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	return &domain.Campaign{
		ID:          dbutil.FromPgtype(campaign.ID),
		Name:        campaign.Name,
		Description: campaign.Description.String,
		Genre:       campaign.Genre.String,
		Tone:        campaign.Tone.String,
		Themes:      campaign.Themes,
		Status:      domain.CampaignStatus(campaign.Status),
		CreatedBy:   dbutil.FromPgtype(campaign.CreatedBy),
		CreatedAt:   campaign.CreatedAt.Time,
		UpdatedAt:   campaign.UpdatedAt.Time,
	}, nil
}

func (e *Engine) LoadCampaign(ctx context.Context, campaignID uuid.UUID) error {
	_, err := e.queries.GetCampaignByID(ctx, dbutil.ToPgtype(campaignID))
	if err != nil {
		return fmt.Errorf("load campaign: %w", err)
	}
	return nil
}

func nextTurnNumber(logs []statedb.SessionLog) int {
	if len(logs) == 0 {
		return 1
	}
	return int(logs[0].TurnNumber) + 1
}

func sessionLogsToDomain(logs []statedb.SessionLog) []domain.SessionLog {
	if len(logs) == 0 {
		return nil
	}

	turns := make([]domain.SessionLog, 0, len(logs))
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		turns = append(turns, domain.SessionLog{
			ID:           dbutil.FromPgtype(log.ID),
			CampaignID:   dbutil.FromPgtype(log.CampaignID),
			TurnNumber:   int(log.TurnNumber),
			PlayerInput:  log.PlayerInput,
			InputType:    domain.InputType(log.InputType),
			LLMResponse:  log.LlmResponse,
			ToolCalls:    append(json.RawMessage(nil), log.ToolCalls...),
			LocationID:   optionalUUID(log.LocationID),
			NPCsInvolved: pgUUIDsToUUIDs(log.NpcsInvolved),
			CreatedAt:    log.CreatedAt.Time,
		})
	}
	return turns
}

func optionalUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	value := dbutil.FromPgtype(id)
	return &value
}

func pgUUIDsToUUIDs(ids []pgtype.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, dbutil.FromPgtype(id))
	}
	return out
}

var numberedChoicePattern = regexp.MustCompile(`^\s*(\d+)[.)]\s+(.*\S)\s*$`)

func extractChoices(narrative string) (string, []Choice) {
	lines := strings.Split(narrative, "\n")
	if len(lines) == 0 {
		return narrative, nil
	}

	var (
		narrativeLines []string
		choices        []Choice
		inChoices      bool
	)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && !inChoices {
			narrativeLines = append(narrativeLines, line)
			continue
		}

		if matches := numberedChoicePattern.FindStringSubmatch(line); matches != nil {
			inChoices = true
			id := matches[1]
			text := strings.TrimSpace(matches[2])
			choices = append(choices, Choice{ID: id, Text: text})
			continue
		}

		if inChoices {
			lower := strings.ToLower(strings.ReplaceAll(trimmed, rightSingleQuote, "'"))
			if strings.HasPrefix(lower, "or describe what you'd like to do") {
				continue
			}
			if trimmed == "" {
				continue
			}
			narrativeLines = append(narrativeLines, line)
			inChoices = false
			continue
		}

		narrativeLines = append(narrativeLines, line)
	}

	cleaned := strings.TrimSpace(strings.Join(narrativeLines, "\n"))
	if len(choices) == 0 {
		return narrative, nil
	}
	return cleaned, choices
}

func marshalAppliedToolCalls(applied []AppliedToolCall) (json.RawMessage, error) {
	if applied == nil {
		applied = []AppliedToolCall{}
	}
	data, err := json.Marshal(applied)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
