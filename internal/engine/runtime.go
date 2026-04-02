package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/assembly"
	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// Engine is the concrete GameEngine implementation used by the TUI.
type Engine struct {
	logger    *slog.Logger
	state     game.StateManager
	assembler *assembly.ContextAssembler
	processor *TurnProcessor
	tier3     *assembly.Tier3Retriever
}

const recentTurnLimit = 10

// Option configures the Engine during construction.
type Option func(*Engine)

// WithTier3Retriever attaches a semantic memory retriever to the engine.
// When set, ProcessTurn includes relevant memories in the LLM context.
func WithTier3Retriever(t *assembly.Tier3Retriever) Option {
	return func(e *Engine) {
		e.tier3 = t
	}
}

// WithLogger sets the structured logger for the engine and its subsystems.
func WithLogger(l *slog.Logger) Option {
	return func(e *Engine) { e.logger = l }
}

// New creates a concrete GameEngine backed by the shared game and llm packages.
func New(db statedb.DBTX, provider llm.Provider, llmCfg config.LLMConfig, opts ...Option) (*Engine, error) {
	queries := statedb.New(db)
	registry := tools.NewRegistry()

	if err := registerAllTools(registry, queries); err != nil {
		return nil, fmt.Errorf("register tools: %w", err)
	}

	e := &Engine{
		state:     game.NewStateManager(db),
		assembler: assembly.NewContextAssembler(registry, assembly.WithTokenBudget(llmCfg.ContextTokenBudget())),
	}
	for _, opt := range opts {
		opt(e)
	}
	if e.logger == nil {
		e.logger = slog.Default()
	}
	e.processor = NewTurnProcessor(provider, registry, tools.NewValidator(registry), e.logger.WithGroup("turns"))
	return e, nil
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

	recentTurns, err := e.state.ListRecentSessionLogs(ctx, campaignID, recentTurnLimit)
	if err != nil {
		return nil, fmt.Errorf("list recent session logs: %w", err)
	}
	var retrievedMemories []string
	if e.tier3 != nil {
		var tier3Err error
		retrievedMemories, tier3Err = e.tier3.Retrieve(ctx, campaignID, playerInput, state)
		if tier3Err != nil {
			e.logger.Warn("tier3 memory retrieval failed", "campaign_id", campaignID, "error", tier3Err)
		}
	}

	messages := e.assembler.AssembleContext(state, recentTurns, playerInput, retrievedMemories...)
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
		TurnNumber:  nextTurnNumber(recentTurns),
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
	return e.state.CreateCampaign(ctx, game.CreateCampaignParams{
		Name:   bootstrap.DefaultCampaignName,
		UserID: userID,
	})
}

func (e *Engine) LoadCampaign(ctx context.Context, campaignID uuid.UUID) error {
	_, err := e.state.GetCampaignByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("load campaign: %w", err)
	}
	return nil
}

// ProcessTurnStream is like ProcessTurn but delivers narrative chunks over
// the returned channel. The channel is closed when processing completes.
// Callers must consume the channel fully to avoid goroutine leaks.
//
// In this initial implementation the full narrative is sent as a single
// chunk followed by the complete TurnResult.
func (e *Engine) ProcessTurnStream(ctx context.Context, campaignID uuid.UUID, playerInput string) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 2)
	go func() {
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				ch <- StreamEvent{Type: "error", Err: fmt.Errorf("process turn panic: %v", r)}
			}
		}()
		result, err := e.ProcessTurn(ctx, campaignID, playerInput)
		if err != nil {
			ch <- StreamEvent{Type: "error", Err: err}
			return
		}
		ch <- StreamEvent{Type: "chunk", Text: result.Narrative}
		ch <- StreamEvent{Type: "result", Result: result}
	}()
	return ch, nil
}

func nextTurnNumber(logs []domain.SessionLog) int {
	if len(logs) == 0 {
		return 1
	}
	return logs[len(logs)-1].TurnNumber + 1
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
