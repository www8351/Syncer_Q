package wsmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateReconnecting ConnectionState = "reconnecting"
	StateClosed       ConnectionState = "closed"
)

type BrokerConnection struct {
	ID                string
	URL               string
	APIKey            string
	APISecret         string
	AccountIDs        []string
	State             ConnectionState
	conn              *websocket.Conn
	mu                sync.RWMutex
	reconnectAttempts int
	maxReconnect      int
	reconnectDelay    time.Duration
	heartbeatInterval time.Duration
	lastHeartbeat     time.Time
	lastMessage       time.Time
	sendCh            chan []byte
	done              chan struct{}
	onMessage         func(connID string, msg []byte)
	onStateChange     func(connID string, oldState, newState ConnectionState)
	intentionallyClosed bool
	readBufferSize    int
	writeBufferSize   int
}

type Manager struct {
	connections map[string]*BrokerConnection
	mu          sync.RWMutex
	config      ManagerConfig
}

type ManagerConfig struct {
	HeartbeatInterval time.Duration
	ReconnectDelay    time.Duration
	MaxReconnect      int
	ReadBufferSize    int
	WriteBufferSize   int
	OnMessage         func(connID string, msg []byte)
	OnStateChange     func(connID string, oldState, newState ConnectionState)
}

func NewManager(cfg ManagerConfig) *Manager {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 2500 * time.Millisecond
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 1000 * time.Millisecond
	}
	if cfg.MaxReconnect == 0 {
		cfg.MaxReconnect = 20
	}
	if cfg.ReadBufferSize == 0 {
		cfg.ReadBufferSize = 4096
	}
	if cfg.WriteBufferSize == 0 {
		cfg.WriteBufferSize = 4096
	}

	return &Manager{
		connections: make(map[string]*BrokerConnection),
		config:      cfg,
	}
}

func (m *Manager) AddConnection(id, url, apiKey, apiSecret string, accountIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.connections[id]; ok {
		existing.Close()
	}

	bc := &BrokerConnection{
		ID:                id,
		URL:               url,
		APIKey:            apiKey,
		APISecret:         apiSecret,
		AccountIDs:        accountIDs,
		State:             StateDisconnected,
		maxReconnect:      m.config.MaxReconnect,
		reconnectDelay:    m.config.ReconnectDelay,
		heartbeatInterval: m.config.HeartbeatInterval,
		sendCh:            make(chan []byte, 256),
		done:              make(chan struct{}),
		onMessage:         m.config.OnMessage,
		onStateChange:     m.config.OnStateChange,
		readBufferSize:    m.config.ReadBufferSize,
		writeBufferSize:   m.config.WriteBufferSize,
	}

	m.connections[id] = bc
}

func (m *Manager) Connect(ctx context.Context, id string) error {
	m.mu.RLock()
	bc, ok := m.connections[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("connection %s not found", id)
	}

	return bc.Connect(ctx)
}

func (m *Manager) ConnectAll(ctx context.Context) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		go func(connID string) {
			if err := m.Connect(ctx, connID); err != nil {
				log.Printf("[WSManager] Failed to connect %s: %v", connID, err)
			}
		}(id)
	}
}

func (m *Manager) Disconnect(id string) {
	m.mu.RLock()
	bc, ok := m.connections[id]
	m.mu.RUnlock()

	if ok {
		bc.Close()
	}
}

func (m *Manager) DisconnectAll() {
	m.mu.RLock()
	conns := make([]*BrokerConnection, 0, len(m.connections))
	for _, bc := range m.connections {
		conns = append(conns, bc)
	}
	m.mu.RUnlock()

	for _, bc := range conns {
		bc.Close()
	}
}

func (m *Manager) Send(id string, data []byte) error {
	m.mu.RLock()
	bc, ok := m.connections[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("connection %s not found", id)
	}

	bc.mu.RLock()
	state := bc.State
	bc.mu.RUnlock()

	if state != StateConnected {
		return fmt.Errorf("connection %s not in connected state (current: %s)", id, state)
	}

	select {
	case bc.sendCh <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full for connection %s", id)
	}
}

func (m *Manager) Broadcast(data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, bc := range m.connections {
		bc.mu.RLock()
		state := bc.State
		bc.mu.RUnlock()

		if state == StateConnected {
			select {
			case bc.sendCh <- data:
			default:
				log.Printf("[WSManager] Send buffer full for connection %s, dropping message", bc.ID)
			}
		}
	}
}

func (m *Manager) GetStates() map[string]ConnectionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]ConnectionInfo, len(m.connections))
	for id, bc := range m.connections {
		bc.mu.RLock()
		states[id] = ConnectionInfo{
			ID:                id,
			URL:               bc.URL,
			State:             bc.State,
			ReconnectAttempts: bc.reconnectAttempts,
			LastHeartbeat:     bc.lastHeartbeat,
			LastMessage:       bc.lastMessage,
			AccountIDs:        bc.AccountIDs,
		}
		bc.mu.RUnlock()
	}
	return states
}

type ConnectionInfo struct {
	ID                string          `json:"id"`
	URL               string          `json:"url"`
	State             ConnectionState `json:"state"`
	ReconnectAttempts int             `json:"reconnect_attempts"`
	LastHeartbeat     time.Time       `json:"last_heartbeat"`
	LastMessage       time.Time       `json:"last_message"`
	AccountIDs        []string        `json:"account_ids"`
}

func (bc *BrokerConnection) Connect(ctx context.Context) error {
	bc.mu.Lock()
	if bc.State == StateConnected || bc.State == StateConnecting {
		bc.mu.Unlock()
		return nil
	}
	bc.intentionallyClosed = false
	bc.setState(StateConnecting)
	bc.mu.Unlock()

	dialer := websocket.Dialer{
		ReadBufferSize:  bc.readBufferSize,
		WriteBufferSize: bc.writeBufferSize,
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	if bc.APIKey != "" {
		headers.Set("Authorization", fmt.Sprintf("Bearer %s", bc.APIKey))
	}

	conn, _, err := dialer.DialContext(ctx, bc.URL, headers)
	if err != nil {
		bc.mu.Lock()
		bc.setState(StateDisconnected)
		bc.mu.Unlock()
		go bc.scheduleReconnect()
		return fmt.Errorf("dial %s: %w", bc.URL, err)
	}

	bc.mu.Lock()
	bc.conn = conn
	bc.reconnectAttempts = 0
	bc.lastHeartbeat = time.Now()
	bc.setState(StateConnected)
	bc.mu.Unlock()

	log.Printf("[WSManager] Connected to %s (conn: %s)", bc.URL, bc.ID)

	go bc.readPump()
	go bc.writePump()
	go bc.heartbeatPump()

	return nil
}

func (bc *BrokerConnection) Close() {
	bc.mu.Lock()
	bc.intentionallyClosed = true
	bc.setState(StateClosed)
	if bc.conn != nil {
		bc.conn.Close()
		bc.conn = nil
	}
	bc.mu.Unlock()

	select {
	case <-bc.done:
	default:
		close(bc.done)
	}

	log.Printf("[WSManager] Disconnected %s", bc.ID)
}

func (bc *BrokerConnection) setState(newState ConnectionState) {
	oldState := bc.State
	bc.State = newState
	if oldState != newState && bc.onStateChange != nil {
		go bc.onStateChange(bc.ID, oldState, newState)
	}
}

func (bc *BrokerConnection) readPump() {
	defer func() {
		bc.mu.Lock()
		if bc.conn != nil {
			bc.conn.Close()
			bc.conn = nil
		}
		bc.mu.Unlock()

		if !bc.intentionallyClosed {
			bc.scheduleReconnect()
		}
	}()

	bc.mu.RLock()
	conn := bc.conn
	bc.mu.RUnlock()

	if conn == nil {
		return
	}

	conn.SetReadLimit(1 << 20)
	conn.SetPongHandler(func(string) error {
		bc.mu.Lock()
		bc.lastHeartbeat = time.Now()
		bc.mu.Unlock()
		return nil
	})

	for {
		select {
		case <-bc.done:
			return
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[WSManager] Read error on %s: %v", bc.ID, err)
			}
			return
		}

		bc.mu.Lock()
		bc.lastMessage = time.Now()
		bc.mu.Unlock()

		if bc.onMessage != nil {
			bc.onMessage(bc.ID, msg)
		}
	}
}

func (bc *BrokerConnection) writePump() {
	bc.mu.RLock()
	conn := bc.conn
	bc.mu.RUnlock()

	if conn == nil {
		return
	}

	for {
		select {
		case <-bc.done:
			return
		case msg, ok := <-bc.sendCh:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[WSManager] Write error on %s: %v", bc.ID, err)
				return
			}
		}
	}
}

func (bc *BrokerConnection) heartbeatPump() {
	ticker := time.NewTicker(bc.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bc.done:
			return
		case <-ticker.C:
			bc.mu.RLock()
			conn := bc.conn
			lastHB := bc.lastHeartbeat
			bc.mu.RUnlock()

			if conn == nil {
				return
			}

			if time.Since(lastHB) > bc.heartbeatInterval*4 {
				log.Printf("[WSManager] Heartbeat timeout on %s, reconnecting...", bc.ID)
				conn.Close()
				return
			}

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[WSManager] Heartbeat write error on %s: %v", bc.ID, err)
				return
			}
		}
	}
}

func (bc *BrokerConnection) scheduleReconnect() {
	bc.mu.Lock()
	if bc.intentionallyClosed {
		bc.mu.Unlock()
		return
	}
	bc.reconnectAttempts++
	attempt := bc.reconnectAttempts
	if attempt > bc.maxReconnect {
		log.Printf("[WSManager] Max reconnect attempts reached for %s", bc.ID)
		bc.setState(StateDisconnected)
		bc.mu.Unlock()
		return
	}
	bc.setState(StateReconnecting)
	bc.mu.Unlock()

	delay := time.Duration(math.Min(
		float64(bc.reconnectDelay)*math.Pow(2, float64(attempt-1)),
		30*float64(time.Second),
	))

	log.Printf("[WSManager] Reconnecting %s in %v (attempt %d/%d)", bc.ID, delay, attempt, bc.maxReconnect)
	time.Sleep(delay)

	if bc.intentionallyClosed {
		return
	}

	bc.done = make(chan struct{})
	bc.sendCh = make(chan []byte, 256)
	bc.Connect(context.Background())
}

func (m *Manager) HandleStatusHTTP(w http.ResponseWriter, r *http.Request) {
	states := m.GetStates()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(states)
}
