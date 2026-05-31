package pubsub

import (
        "context"
        "encoding/json"
        "fmt"
        "log"
        "sync"
        "time"

        "github.com/redis/go-redis/v9"
)

type TradeSignal struct {
        ID          string    `json:"id"`
        Source      string    `json:"source"`
        Ticker      string    `json:"ticker"`
        Action      string    `json:"action"`
        Contracts   float64   `json:"contracts"`
        Price       float64   `json:"price"`
        OrderType   string    `json:"order_type"`
        GroupID     string    `json:"group_id,omitempty"`
        Strategy    string    `json:"strategy,omitempty"`
        Comment     string    `json:"comment,omitempty"`
        ReceivedAt  time.Time `json:"received_at"`
        PublishedAt time.Time `json:"published_at"`
}

type FollowerSession struct {
        ID         string
        AccountIDs []string
        GroupID    string
        SendFn     func(data []byte) error
}

type SignalHandler func(signal *TradeSignal) error

type Subscriber struct {
        rdb           *redis.Client
        channel       string
        followers     map[string]*FollowerSession
        mu            sync.RWMutex
        handlers      []SignalHandler
        signalsRouted int64
        running       bool
        cancel        context.CancelFunc
}

func NewSubscriber(rdb *redis.Client, channel string) *Subscriber {
        return &Subscriber{
                rdb:       rdb,
                channel:   channel,
                followers: make(map[string]*FollowerSession),
        }
}

func (s *Subscriber) RegisterHandler(handler SignalHandler) {
        s.mu.Lock()
        defer s.mu.Unlock()
        s.handlers = append(s.handlers, handler)
}

func (s *Subscriber) AddFollower(session *FollowerSession) {
        s.mu.Lock()
        defer s.mu.Unlock()
        s.followers[session.ID] = session
        log.Printf("[PubSub] Added follower session %s (group: %s, accounts: %v)", session.ID, session.GroupID, session.AccountIDs)
}

func (s *Subscriber) RemoveFollower(id string) {
        s.mu.Lock()
        defer s.mu.Unlock()
        delete(s.followers, id)
        log.Printf("[PubSub] Removed follower session %s", id)
}

func (s *Subscriber) GetFollowerCount() int {
        s.mu.RLock()
        defer s.mu.RUnlock()
        return len(s.followers)
}

func (s *Subscriber) Start(ctx context.Context) error {
        s.mu.Lock()
        if s.running {
                s.mu.Unlock()
                return fmt.Errorf("subscriber already running")
        }
        s.running = true
        s.mu.Unlock()

        subCtx, cancel := context.WithCancel(ctx)
        s.cancel = cancel

        go s.subscriptionLoop(subCtx)

        log.Printf("[PubSub] Subscriber started on channel: %s", s.channel)
        return nil
}

func (s *Subscriber) Stop() {
        s.mu.Lock()
        defer s.mu.Unlock()

        if s.cancel != nil {
                s.cancel()
        }
        s.running = false
        log.Printf("[PubSub] Subscriber stopped (routed %d signals)", s.signalsRouted)
}

func (s *Subscriber) subscriptionLoop(ctx context.Context) {
        for {
                select {
                case <-ctx.Done():
                        return
                default:
                }

                pubsub := s.rdb.Subscribe(ctx, s.channel)

                ch := pubsub.Channel()
                log.Printf("[PubSub] Subscribed to Redis channel: %s", s.channel)

                func() {
                        defer pubsub.Close()
                        for {
                                select {
                                case <-ctx.Done():
                                        return
                                case msg, ok := <-ch:
                                        if !ok {
                                                log.Printf("[PubSub] Channel closed, resubscribing...")
                                                time.Sleep(1 * time.Second)
                                                return
                                        }
                                        s.handleMessage(msg)
                                }
                        }
                }()
        }
}

func (s *Subscriber) handleMessage(msg *redis.Message) {
        receiveTime := time.Now()

        var signal TradeSignal
        if err := json.Unmarshal([]byte(msg.Payload), &signal); err != nil {
                log.Printf("[PubSub] Failed to unmarshal signal: %v", err)
                return
        }

        s.mu.RLock()
        handlers := make([]SignalHandler, len(s.handlers))
        copy(handlers, s.handlers)
        s.mu.RUnlock()

        for _, handler := range handlers {
                if err := handler(&signal); err != nil {
                        log.Printf("[PubSub] Handler error for signal %s: %v", signal.ID, err)
                }
        }

        s.fanOutToFollowers(&signal)

        s.mu.Lock()
        s.signalsRouted++
        s.mu.Unlock()

        latencyMs := time.Since(receiveTime).Milliseconds()
        log.Printf("[PubSub] Signal %s routed to followers (latency: %dms)", signal.ID, latencyMs)
}

func (s *Subscriber) fanOutToFollowers(signal *TradeSignal) {
        s.mu.RLock()
        defer s.mu.RUnlock()

        data, err := json.Marshal(map[string]interface{}{
                "type":   "trade_signal",
                "signal": signal,
        })
        if err != nil {
                log.Printf("[PubSub] Failed to marshal fan-out message: %v", err)
                return
        }

        for id, follower := range s.followers {
                if signal.GroupID != "" && follower.GroupID != "" && signal.GroupID != follower.GroupID {
                        continue
                }

                if err := follower.SendFn(data); err != nil {
                        log.Printf("[PubSub] Failed to send to follower %s: %v", id, err)
                }
        }
}

func (s *Subscriber) GetStats() map[string]interface{} {
        s.mu.RLock()
        defer s.mu.RUnlock()

        return map[string]interface{}{
                "channel":        s.channel,
                "running":        s.running,
                "follower_count": len(s.followers),
                "signals_routed": s.signalsRouted,
        }
}
