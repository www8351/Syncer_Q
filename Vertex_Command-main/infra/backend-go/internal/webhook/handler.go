package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type TradingViewAlert struct {
	Ticker    string  `json:"ticker"`
	Action    string  `json:"action"`
	Contracts float64 `json:"contracts"`
	Price     float64 `json:"price"`
	Time      string  `json:"time"`
	Exchange  string  `json:"exchange,omitempty"`
	OrderType string  `json:"order_type,omitempty"`
	GroupID   string  `json:"group_id,omitempty"`
	Strategy  string  `json:"strategy,omitempty"`
	Comment   string  `json:"comment,omitempty"`
}

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
	PublishedAt time.Time `json:"published_at,omitempty"`
}

type Handler struct {
	rdb            *redis.Client
	webhookSecret  string
	pubsubChannel  string
	signalTTL      time.Duration
	signalsHandled int64
}

func NewHandler(rdb *redis.Client, webhookSecret, pubsubChannel string, signalTTL time.Duration) *Handler {
	return &Handler{
		rdb:           rdb,
		webhookSecret: webhookSecret,
		pubsubChannel: pubsubChannel,
		signalTTL:     signalTTL,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		log.Printf("[Webhook] Failed to read body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if h.webhookSecret != "" {
		sig := r.Header.Get("X-Webhook-Signature")
		if sig == "" {
			sig = r.Header.Get("X-Signature")
		}
		if !h.verifySignature(body, sig) {
			log.Printf("[Webhook] Invalid signature from %s", r.RemoteAddr)
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	receiveTime := time.Now()

	var alert TradingViewAlert
	if err := json.Unmarshal(body, &alert); err != nil {
		log.Printf("[Webhook] Invalid JSON payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if alert.Ticker == "" || alert.Action == "" {
		http.Error(w, "Missing required fields: ticker, action", http.StatusBadRequest)
		return
	}

	if alert.Action != "buy" && alert.Action != "sell" && alert.Action != "Buy" && alert.Action != "Sell" {
		http.Error(w, "Invalid action: must be buy or sell", http.StatusBadRequest)
		return
	}

	signal := TradeSignal{
		ID:         fmt.Sprintf("sig_%d", time.Now().UnixNano()),
		Source:     "tradingview",
		Ticker:     alert.Ticker,
		Action:     alert.Action,
		Contracts:  alert.Contracts,
		Price:      alert.Price,
		OrderType:  alert.OrderType,
		GroupID:    alert.GroupID,
		Strategy:   alert.Strategy,
		Comment:    alert.Comment,
		ReceivedAt: receiveTime,
	}

	if signal.OrderType == "" {
		signal.OrderType = "Market"
	}
	if signal.Contracts == 0 {
		signal.Contracts = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.publishSignal(ctx, &signal); err != nil {
		log.Printf("[Webhook] Failed to publish signal: %v", err)
		http.Error(w, "Failed to process signal", http.StatusInternalServerError)
		return
	}

	h.signalsHandled++
	latencyMs := time.Since(receiveTime).Milliseconds()
	log.Printf("[Webhook] Signal published: %s %s %.0f %s (latency: %dms)",
		signal.Action, signal.Ticker, signal.Contracts, signal.OrderType, latencyMs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"signal_id":  signal.ID,
		"latency_ms": latencyMs,
	})
}

func (h *Handler) publishSignal(ctx context.Context, signal *TradeSignal) error {
	signal.PublishedAt = time.Now()

	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("marshal signal: %w", err)
	}

	pipe := h.rdb.Pipeline()
	pipe.Publish(ctx, h.pubsubChannel, data)
	pipe.Set(ctx, fmt.Sprintf("signal:%s", signal.ID), data, h.signalTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis publish: %w", err)
	}

	return nil
}

func (h *Handler) verifySignature(body []byte, signature string) bool {
	if h.webhookSecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *Handler) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"signals_handled": h.signalsHandled,
	}
}
