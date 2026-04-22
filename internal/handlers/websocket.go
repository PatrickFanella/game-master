package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/PatrickFanella/game-master/pkg/api"
)

// wsActionMessage is the client-to-server message envelope.
type wsActionMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// wsActionPayload carries the player input within an action message.
type wsActionPayload struct {
	Input string `json:"input"`
}

var websocketAcceptOptions = &websocket.AcceptOptions{
	OriginPatterns: []string{
		"localhost:*",
		"127.0.0.1:*",
		"https://edda.subcult.tv",
	},
}

// HandleWebSocket upgrades to WebSocket and streams game events to the client.
func (h *ActionHandlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	campaignID, err := campaignIDFromURL(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid campaign id: %v", err))
		return
	}

	conn, err := websocket.Accept(w, r, websocketAcceptOptions)
	if err != nil {
		h.Logger.Errorf("websocket accept for campaign %s: %v", campaignID, err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	ctx := r.Context()

	for {
		var msg wsActionMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			closeStatus := websocket.CloseStatus(err)
			if closeStatus == websocket.StatusNormalClosure || closeStatus == websocket.StatusGoingAway {
				h.Logger.Infof("websocket closed normally for campaign %s", campaignID)
			} else if ctx.Err() != nil {
				h.Logger.Infof("websocket context done for campaign %s", campaignID)
			} else {
				h.Logger.Errorf("websocket read for campaign %s: %v", campaignID, err)
			}
			return
		}

		if msg.Type != "action" {
			sendErrorEnvelope(ctx, conn, fmt.Sprintf("unknown message type: %q", msg.Type))
			continue
		}

		var payload wsActionPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			sendErrorEnvelope(ctx, conn, fmt.Sprintf("invalid payload: %v", err))
			continue
		}

		if payload.Input == "" {
			sendErrorEnvelope(ctx, conn, "input is required")
			continue
		}

		ch, err := h.Engine.ProcessTurnStream(ctx, campaignID, payload.Input)
		if err != nil {
			h.Logger.Errorf("process turn stream for campaign %s: %v", campaignID, err)
			sendErrorEnvelope(ctx, conn, "failed to process turn")
			continue
		}

		for event := range ch {
			var envelope api.WebSocketMessageEnvelope
			envelope.Timestamp = time.Now()

			switch event.Type {
			case "chunk":
				envelope.Type = "chunk"
				envelope.Payload, _ = json.Marshal(map[string]string{"text": event.Text})
			case "result":
				envelope.Type = "result"
				if event.Result == nil {
					envelope.Type = "error"
					envelope.Payload, _ = json.Marshal(map[string]string{"error": "an internal error occurred"})
					break
				}
				resultPayload, err := json.Marshal(engineTurnResultToAPI(event.Result))
				if err != nil {
					h.Logger.Errorf("marshal turn result for campaign %s: %v", campaignID, err)
					envelope.Type = "error"
					envelope.Payload, _ = json.Marshal(map[string]string{"error": "an internal error occurred"})
					break
				}
				envelope.Payload = resultPayload
			case "status":
				envelope.Type = "status"
				if event.Status != nil {
					envelope.Payload, _ = json.Marshal(event.Status)
				} else {
					continue
				}
			case "error":
				envelope.Type = "error"
				errMsg := "an internal error occurred"
				if event.Err != nil {
					h.Logger.Errorf("stream event error for campaign %s: %v", campaignID, event.Err)
				}
				envelope.Payload, _ = json.Marshal(map[string]string{"error": errMsg})
			default:
				continue
			}

			if err := wsjson.Write(ctx, conn, envelope); err != nil {
				h.Logger.Errorf("websocket write for campaign %s: %v", campaignID, err)
				return
			}
		}
	}
}

// sendErrorEnvelope writes an error envelope to the WebSocket connection.
func sendErrorEnvelope(ctx context.Context, conn *websocket.Conn, msg string) {
	payload, _ := json.Marshal(map[string]string{"error": msg})
	envelope := api.WebSocketMessageEnvelope{
		Type:      "error",
		Payload:   payload,
		Timestamp: time.Now(),
	}
	//nolint:errcheck // Best-effort; connection may already be closing.
	wsjson.Write(ctx, conn, envelope)
}
