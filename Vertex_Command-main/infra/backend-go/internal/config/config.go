package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            int
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	WebhookSecret   string
	BrokerWSURL     string
	BrokerAPIKey    string
	BrokerAPISecret string
	HealthPort      int

	WSHeartbeatInterval time.Duration
	WSReconnectDelay    time.Duration
	WSMaxReconnect      int
	WSReadBufferSize    int
	WSWriteBufferSize   int

	RedisPubSubChannel string
	RedisSignalTTL     time.Duration
}

func Load() (*Config, error) {
	port := envInt("PORT", 8080)
	healthPort := envInt("HEALTH_PORT", 8081)
	redisAddr := envStr("REDIS_ADDR", "redis:6379")
	redisPassword := envStr("REDIS_PASSWORD", "")
	redisDB := envInt("REDIS_DB", 0)
	webhookSecret := envStr("WEBHOOK_SECRET", "")
	brokerWSURL := envStr("BROKER_WS_URL", "")
	brokerAPIKey := envStr("BROKER_API_KEY", "")
	brokerAPISecret := envStr("BROKER_API_SECRET", "")

	if webhookSecret == "" {
		return nil, fmt.Errorf("WEBHOOK_SECRET environment variable is required")
	}

	return &Config{
		Port:            port,
		RedisAddr:       redisAddr,
		RedisPassword:   redisPassword,
		RedisDB:         redisDB,
		WebhookSecret:   webhookSecret,
		BrokerWSURL:     brokerWSURL,
		BrokerAPIKey:    brokerAPIKey,
		BrokerAPISecret: brokerAPISecret,
		HealthPort:      healthPort,

		WSHeartbeatInterval: time.Duration(envInt("WS_HEARTBEAT_MS", 2500)) * time.Millisecond,
		WSReconnectDelay:    time.Duration(envInt("WS_RECONNECT_DELAY_MS", 1000)) * time.Millisecond,
		WSMaxReconnect:      envInt("WS_MAX_RECONNECT", 20),
		WSReadBufferSize:    envInt("WS_READ_BUFFER", 4096),
		WSWriteBufferSize:   envInt("WS_WRITE_BUFFER", 4096),

		RedisPubSubChannel: envStr("REDIS_PUBSUB_CHANNEL", "vertex:trade_signals"),
		RedisSignalTTL:     time.Duration(envInt("REDIS_SIGNAL_TTL_SEC", 300)) * time.Second,
	}, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
