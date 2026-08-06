package live

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// wsWriteTimeout is the maximum time to wait for a WebSocket write.
const wsWriteTimeout = 10 * time.Second

// wsMessage is the JSON envelope for WebSocket messages. It mirrors the
// SSE event structure (snapshot, event, complete) using a type field.
type wsMessage struct {
	Type string         `json:"type"`
	Data jsontext.Value `json:"data"`
}

// handleWebSocket upgrades to WebSocket and streams events, mirroring
// the SSE handler. Used as a fallback for environments that block SSE.
//
//nolint:exhaustruct // optional upgrader fields
func (srv *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	defer func() { _ = conn.Close() }()

	sub := srv.hub.Subscribe()
	defer srv.hub.Unsubscribe(sub.id)

	if srv.snapshotProvider != nil {
		data, snapErr := srv.snapshotProvider(srv.hub.IsComplete())
		if snapErr == nil {
			srv.writeWS(conn, wsMessage{Type: "snapshot", Data: data})
		}
	}

	if srv.hub.IsComplete() {
		srv.sendWSComplete(conn)

		return
	}

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.done:
			srv.sendWSComplete(conn)

			return
		case evt := <-sub.ch:
			if !srv.writeWS(conn, wsMessage{Type: eventNameEvent, Data: evt.Data}) {
				return
			}
		}
	}
}

// writeWS marshals and sends a WebSocket message with a write deadline.
// Returns false if the write failed (connection is dead).
func (srv *Server) writeWS(conn *websocket.Conn, msg wsMessage) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return true
	}

	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	err = conn.WriteMessage(websocket.TextMessage, data)

	return err == nil
}

// sendWSComplete sends the final complete message over WebSocket.
func (srv *Server) sendWSComplete(conn *websocket.Conn) {
	if srv.completeProvider == nil {
		return
	}

	data, err := srv.completeProvider()
	if err != nil {
		return
	}

	srv.writeWS(conn, wsMessage{Type: "complete", Data: data})
}
