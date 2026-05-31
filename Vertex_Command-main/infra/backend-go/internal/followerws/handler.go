package followerws

import (
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "os"
        "strings"
        "sync"
        "sync/atomic"
        "time"

        "github.com/gorilla/websocket"
        "vertex-command/internal/pubsub"
)

var sessionCounter uint64

func buildUpgrader() websocket.Upgrader {
        allowedOrigins := os.Getenv("WS_ALLOWED_ORIGINS")
        return websocket.Upgrader{
                ReadBufferSize:  4096,
                WriteBufferSize: 4096,
                CheckOrigin: func(r *http.Request) bool {
                        if allowedOrigins == "" || allowedOrigins == "*" {
                                return true
                        }
                        origin := r.Header.Get("Origin")
                        if origin == "" {
                                return false
                        }
                        for _, allowed := range strings.Split(allowedOrigins, ",") {
                                if strings.TrimSpace(allowed) == origin {
                                        return true
                                }
                        }
                        log.Printf("[FollowerWS] Rejected origin: %s", origin)
                        return false
                },
        }
}

type followerConn struct {
        id      string
        groupID string
        conn    *websocket.Conn
        sendCh  chan []byte
        done    chan struct{}
        mu      sync.Mutex
}

type Handler struct {
        subscriber *pubsub.Subscriber
        conns      sync.Map
        upgrader   websocket.Upgrader
        authToken  string
}

func NewHandler(subscriber *pubsub.Subscriber) *Handler {
        authToken := os.Getenv("FOLLOWER_WS_AUTH_TOKEN")
        if authToken == "" {
                authToken = os.Getenv("WEBHOOK_SECRET")
        }
        return &Handler{
                subscriber: subscriber,
                upgrader:   buildUpgrader(),
                authToken:  authToken,
        }
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        if h.authToken != "" {
                token := r.URL.Query().Get("token")
                if token == "" {
                        token = r.Header.Get("Authorization")
                        if strings.HasPrefix(token, "Bearer ") {
                                token = token[7:]
                        }
                }
                if token != h.authToken {
                        log.Printf("[FollowerWS] Unauthorized connection attempt from %s", r.RemoteAddr)
                        http.Error(w, "Unauthorized", http.StatusUnauthorized)
                        return
                }
        }

        groupID := r.URL.Query().Get("group_id")
        accountIDs := r.URL.Query()["account_id"]

        conn, err := h.upgrader.Upgrade(w, r, nil)
        if err != nil {
                log.Printf("[FollowerWS] Upgrade error: %v", err)
                return
        }

        id := fmt.Sprintf("follower_%d", atomic.AddUint64(&sessionCounter, 1))

        fc := &followerConn{
                id:      id,
                groupID: groupID,
                conn:    conn,
                sendCh:  make(chan []byte, 64),
                done:    make(chan struct{}),
        }

        h.conns.Store(id, fc)

        session := &pubsub.FollowerSession{
                ID:         id,
                AccountIDs: accountIDs,
                GroupID:    groupID,
                SendFn: func(data []byte) error {
                        select {
                        case fc.sendCh <- data:
                                return nil
                        case <-fc.done:
                                return fmt.Errorf("connection closed")
                        default:
                                return fmt.Errorf("send buffer full")
                        }
                },
        }

        h.subscriber.AddFollower(session)

        log.Printf("[FollowerWS] Connected: %s (group: %s, accounts: %v)", id, groupID, accountIDs)

        welcome, _ := json.Marshal(map[string]interface{}{
                "type":       "connected",
                "session_id": id,
                "group_id":   groupID,
                "timestamp":  time.Now().UTC().Format(time.RFC3339),
        })
        conn.WriteMessage(websocket.TextMessage, welcome)

        go h.writePump(fc)
        go h.readPump(fc)
}

func (h *Handler) readPump(fc *followerConn) {
        defer func() {
                h.cleanup(fc)
        }()

        fc.conn.SetReadLimit(1 << 16)
        fc.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
        fc.conn.SetPongHandler(func(string) error {
                fc.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
                return nil
        })

        for {
                _, msg, err := fc.conn.ReadMessage()
                if err != nil {
                        if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
                                log.Printf("[FollowerWS] Read error on %s: %v", fc.id, err)
                        }
                        return
                }

                var req map[string]interface{}
                if json.Unmarshal(msg, &req) == nil {
                        if reqType, ok := req["type"].(string); ok && reqType == "ping" {
                                pong, _ := json.Marshal(map[string]string{"type": "pong"})
                                select {
                                case fc.sendCh <- pong:
                                default:
                                }
                        }
                }
        }
}

func (h *Handler) writePump(fc *followerConn) {
        ticker := time.NewTicker(30 * time.Second)
        defer func() {
                ticker.Stop()
                h.cleanup(fc)
        }()

        for {
                select {
                case <-fc.done:
                        return
                case msg, ok := <-fc.sendCh:
                        if !ok {
                                return
                        }
                        fc.mu.Lock()
                        fc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
                        err := fc.conn.WriteMessage(websocket.TextMessage, msg)
                        fc.mu.Unlock()
                        if err != nil {
                                log.Printf("[FollowerWS] Write error on %s: %v", fc.id, err)
                                return
                        }
                case <-ticker.C:
                        fc.mu.Lock()
                        fc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
                        err := fc.conn.WriteMessage(websocket.PingMessage, nil)
                        fc.mu.Unlock()
                        if err != nil {
                                return
                        }
                }
        }
}

func (h *Handler) cleanup(fc *followerConn) {
        select {
        case <-fc.done:
                return
        default:
                close(fc.done)
        }

        h.subscriber.RemoveFollower(fc.id)
        h.conns.Delete(fc.id)
        fc.conn.Close()
        log.Printf("[FollowerWS] Disconnected: %s", fc.id)
}

func (h *Handler) GetStats() map[string]interface{} {
        count := 0
        h.conns.Range(func(_, _ interface{}) bool {
                count++
                return true
        })
        return map[string]interface{}{
                "active_followers": count,
        }
}
