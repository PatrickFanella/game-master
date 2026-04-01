package handlers

import "net/http"

// HandleWebSocket upgrades to WebSocket for streaming game events.
// TODO(#165): Implement with github.com/coder/websocket once added to go.mod.
func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "WebSocket streaming not yet implemented")
}
