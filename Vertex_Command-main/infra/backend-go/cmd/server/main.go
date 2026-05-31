package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"vertex-command/internal/config"
	"vertex-command/internal/followerws"
	"vertex-command/internal/pubsub"
	"vertex-command/internal/webhook"
	"vertex-command/internal/wsmanager"
)

var version string

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Printf("[Main] Starting Vertex Command Go Routing Engine (version: %s)...", version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Main] Failed to load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("[Main] Failed to connect to Redis at %s: %v", cfg.RedisAddr, err)
	}
	log.Printf("[Main] Connected to Redis at %s", cfg.RedisAddr)

	wsMgr := wsmanager.NewManager(wsmanager.ManagerConfig{
		HeartbeatInterval: cfg.WSHeartbeatInterval,
		ReconnectDelay:    cfg.WSReconnectDelay,
		MaxReconnect:      cfg.WSMaxReconnect,
		ReadBufferSize:    cfg.WSReadBufferSize,
		WriteBufferSize:   cfg.WSWriteBufferSize,
		OnMessage: func(connID string, msg []byte) {
			log.Printf("[WS] Message from %s: %s", connID, string(msg[:min(len(msg), 200)]))
		},
		OnStateChange: func(connID string, oldState, newState wsmanager.ConnectionState) {
			log.Printf("[WS] Connection %s: %s -> %s", connID, oldState, newState)
		},
	})

	if cfg.BrokerWSURL != "" {
		wsMgr.AddConnection("primary", cfg.BrokerWSURL, cfg.BrokerAPIKey, cfg.BrokerAPISecret, nil)
		go wsMgr.ConnectAll(ctx)
	}

	sub := pubsub.NewSubscriber(rdb, cfg.RedisPubSubChannel)
	sub.RegisterHandler(func(signal *pubsub.TradeSignal) error {
		log.Printf("[Signal] Processing: %s %s %.0f @ %.2f", signal.Action, signal.Ticker, signal.Contracts, signal.Price)
		return nil
	})

	if err := sub.Start(ctx); err != nil {
		log.Fatalf("[Main] Failed to start pub/sub subscriber: %v", err)
	}

	webhookHandler := webhook.NewHandler(rdb, cfg.WebhookSecret, cfg.RedisPubSubChannel, cfg.RedisSignalTTL)
	followerHandler := followerws.NewHandler(sub)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/tradingview", webhookHandler.ServeHTTP)
	mux.HandleFunc("/ws/connections", wsMgr.HandleStatusHTTP)
	mux.HandleFunc("/ws/follower", followerHandler.ServeHTTP)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		redisOk := rdb.Ping(r.Context()).Err() == nil
		status := "healthy"
		statusCode := http.StatusOK
		if !redisOk {
			status = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": status,
			"services": map[string]bool{
				"redis": redisOk,
			},
			"ws_connections": wsMgr.GetStates(),
			"pubsub":         sub.GetStats(),
			"webhook":        webhookHandler.GetStats(),
			"followers":      followerHandler.GetStats(),
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		})
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[Main] HTTP server listening on :%d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] HTTP server error: %v", err)
		}
	}()

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HealthPort),
		Handler: healthMux,
	}
	go func() {
		log.Printf("[Main] Health check server on :%d", cfg.HealthPort)
		healthServer.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[Main] Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	sub.Stop()
	wsMgr.DisconnectAll()
	server.Shutdown(shutdownCtx)
	healthServer.Shutdown(shutdownCtx)
	rdb.Close()

	log.Println("[Main] Server stopped")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
