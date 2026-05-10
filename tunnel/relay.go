package tunnel

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	requestTimeout = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsConn) SendJSON(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func (w *wsConn) Close() error {
	return w.conn.Close()
}

func HandleRelay(hub *Hub, w http.ResponseWriter, r *http.Request) {
	tunnelID := r.PathValue("id")
	if tunnelID == "" {
		http.Error(w, "missing tunnel id", http.StatusBadRequest)
		return
	}

	t, ok := hub.Get(tunnelID)
	if !ok {
		http.Error(w, "tunnel not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("relay upgrade error: %v", err)
		return
	}

	ws := &wsConn{conn: conn}
	t.SetRelay(ws)
	defer func() {
		t.ClearRelay()
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go pingLoop(ws, conn, t)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("relay read error for %s: %v", tunnelID, err)
			}
			return
		}

		var resp RelayResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			log.Printf("relay unmarshal error for %s: %v", tunnelID, err)
			continue
		}

		if resp.Type == "response" {
			t.DeliverResponse(&resp)
		}
	}
}

func pingLoop(ws *wsConn, conn *websocket.Conn, t *Tunnel) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		<-ticker.C
		t.mu.RLock()
		active := t.relay == ws
		t.mu.RUnlock()
		if !active {
			return
		}
		ws.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		err := conn.WriteMessage(websocket.PingMessage, nil)
		ws.mu.Unlock()
		if err != nil {
			return
		}
	}
}
