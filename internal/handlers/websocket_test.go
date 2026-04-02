package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// wsStubEngine implements engine.GameEngine with controllable ProcessTurnStream.
type wsStubEngine struct {
	streamEvents []engine.StreamEvent
	streamErr    error
}

func (s *wsStubEngine) ProcessTurn(_ context.Context, _ uuid.UUID, _ string) (*engine.TurnResult, error) {
	return nil, nil
}

func (s *wsStubEngine) GetGameState(_ context.Context, _ uuid.UUID) (*engine.GameState, error) {
	return nil, nil
}

func (s *wsStubEngine) NewCampaign(_ context.Context, _ uuid.UUID) (*domain.Campaign, error) {
	return nil, nil
}

func (s *wsStubEngine) LoadCampaign(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *wsStubEngine) ProcessTurnStream(_ context.Context, _ uuid.UUID, _ string) (<-chan engine.StreamEvent, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	ch := make(chan engine.StreamEvent, len(s.streamEvents))
	for _, e := range s.streamEvents {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func newWSRouter(h *Handlers) *chi.Mux {
	r := chi.NewRouter()
	authMW := auth.NewNoOpMiddleware(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	r.Use(authMW.Authenticate)
	r.Route("/campaigns/{id}", func(r chi.Router) {
		r.Get("/ws", h.HandleWebSocket)
	})
	return r
}

// dialWS starts a test server and dials its WebSocket endpoint.
func dialWS(t *testing.T, h *Handlers, campaignID string) (*websocket.Conn, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(newWSRouter(h))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/campaigns/" + campaignID + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	return conn, srv
}

// sendAction writes an action message to the WebSocket.
func sendAction(t *testing.T, ctx context.Context, conn *websocket.Conn, input string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"input": input})
	msg := map[string]any{
		"type":    "action",
		"payload": json.RawMessage(payload),
	}
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		t.Fatalf("write action: %v", err)
	}
}

// readEnvelope reads a single WebSocketMessageEnvelope.
func readEnvelope(t *testing.T, ctx context.Context, conn *websocket.Conn) api.WebSocketMessageEnvelope {
	t.Helper()
	var env api.WebSocketMessageEnvelope
	if err := wsjson.Read(ctx, conn, &env); err != nil {
		t.Fatalf("read envelope: %v", err)
	}
	return env
}

func TestHandleWebSocket_Success(t *testing.T) {
	eng := &wsStubEngine{
		streamEvents: []engine.StreamEvent{
			{Type: "chunk", Text: "You see a dragon."},
			{Type: "result", Result: &engine.TurnResult{Narrative: "You see a dragon."}},
		},
	}
	h := New(eng, nil, log.Default())
	campaignID := uuid.New().String()
	conn, _ := dialWS(t, h, campaignID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sendAction(t, ctx, conn, "look around")

	// Expect chunk envelope.
	env := readEnvelope(t, ctx, conn)
	if env.Type != "chunk" {
		t.Fatalf("expected type chunk, got %q", env.Type)
	}
	var chunkPayload map[string]string
	if err := json.Unmarshal(env.Payload, &chunkPayload); err != nil {
		t.Fatalf("unmarshal chunk payload: %v", err)
	}
	if chunkPayload["text"] != "You see a dragon." {
		t.Errorf("expected chunk text %q, got %q", "You see a dragon.", chunkPayload["text"])
	}

	// Expect result envelope.
	env = readEnvelope(t, ctx, conn)
	if env.Type != "result" {
		t.Fatalf("expected type result, got %q", env.Type)
	}
	var result engine.TurnResult
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if result.Narrative != "You see a dragon." {
		t.Errorf("expected narrative %q, got %q", "You see a dragon.", result.Narrative)
	}
}

func TestHandleWebSocket_InvalidCampaignID(t *testing.T) {
	h := New(&wsStubEngine{}, nil, log.Default())
	srv := httptest.NewServer(newWSRouter(h))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/campaigns/not-a-uuid/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Dial should fail because the server responds with HTTP 400 before upgrade.
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected dial to fail for invalid campaign ID")
	}
}

func TestHandleWebSocket_EmptyInput(t *testing.T) {
	eng := &wsStubEngine{}
	h := New(eng, nil, log.Default())
	campaignID := uuid.New().String()
	conn, _ := dialWS(t, h, campaignID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sendAction(t, ctx, conn, "")

	env := readEnvelope(t, ctx, conn)
	if env.Type != "error" {
		t.Fatalf("expected type error, got %q", env.Type)
	}
	var errPayload map[string]string
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if errPayload["error"] != "input is required" {
		t.Errorf("expected error %q, got %q", "input is required", errPayload["error"])
	}
}

func TestHandleWebSocket_StreamError(t *testing.T) {
	eng := &wsStubEngine{
		streamErr: errors.New("llm unavailable"),
	}
	h := New(eng, nil, log.Default())
	campaignID := uuid.New().String()
	conn, _ := dialWS(t, h, campaignID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sendAction(t, ctx, conn, "attack")

	env := readEnvelope(t, ctx, conn)
	if env.Type != "error" {
		t.Fatalf("expected type error, got %q", env.Type)
	}
	var errPayload map[string]string
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if errPayload["error"] != "failed to process turn" {
		t.Errorf("expected error %q, got %q", "failed to process turn", errPayload["error"])
	}
}

func TestHandleWebSocket_GracefulClose(t *testing.T) {
	eng := &wsStubEngine{}
	h := New(eng, nil, log.Default())
	campaignID := uuid.New().String()
	conn, _ := dialWS(t, h, campaignID)

	// Close normally — handler should not panic or log an error.
	err := conn.Close(websocket.StatusNormalClosure, "bye")
	if err != nil {
		t.Fatalf("close websocket: %v", err)
	}

}


func TestHandleWebSocket_StreamErrorEvent(t *testing.T) {
	t.Run("NilErr", func(t *testing.T) {
		eng := &wsStubEngine{
			streamEvents: []engine.StreamEvent{
				{Type: "error", Err: nil},
			},
		}
		h := New(eng, nil, log.Default())
		campaignID := uuid.New().String()
		conn, _ := dialWS(t, h, campaignID)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sendAction(t, ctx, conn, "look")

		env := readEnvelope(t, ctx, conn)
		if env.Type != "error" {
			t.Fatalf("expected type error, got %q", env.Type)
		}
		var errPayload map[string]string
		if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
			t.Fatalf("unmarshal error payload: %v", err)
		}
		if errPayload["error"] != "an internal error occurred" {
			t.Errorf("expected generic error, got %q", errPayload["error"])
		}
	})

	t.Run("WithErr", func(t *testing.T) {
		eng := &wsStubEngine{
			streamEvents: []engine.StreamEvent{
				{Type: "error", Err: errors.New("db timeout")},
			},
		}
		h := New(eng, nil, log.Default())
		campaignID := uuid.New().String()
		conn, _ := dialWS(t, h, campaignID)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sendAction(t, ctx, conn, "look")

		env := readEnvelope(t, ctx, conn)
		if env.Type != "error" {
			t.Fatalf("expected type error, got %q", env.Type)
		}
		var errPayload map[string]string
		if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
			t.Fatalf("unmarshal error payload: %v", err)
		}
		if errPayload["error"] != "an internal error occurred" {
			t.Errorf("expected generic error, got %q", errPayload["error"])
		}
		if strings.Contains(errPayload["error"], "db timeout") {
			t.Errorf("error leaked internal details: %q", errPayload["error"])
		}
	})
}