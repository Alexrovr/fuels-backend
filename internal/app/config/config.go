package config

import (
	"fmt"
	"os"
	"time"
)

// Config — настройки авторизации (ЛР4). Подключение к PostgreSQL собирает
// пакет dsn, к Minio — storage; здесь только JWT и Redis.
type Config struct {
	JWT   JWTConfig
	Redis RedisConfig
}

type JWTConfig struct {
	Secret    []byte        // ключ подписи HS256
	ExpiresIn time.Duration // срок жизни токена
	Issuer    string
}

type RedisConfig struct {
	Host        string
	Port        string
	Password    string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// FromEnv читает JWT_SECRET, JWT_TTL, REDIS_HOST, REDIS_PORT, REDIS_PASSWORD.
// Значения по умолчанию совпадают с docker-compose.yml.
func FromEnv() (*Config, error) {
	ttl, err := time.ParseDuration(envOrDefault("JWT_TTL", "1h"))
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("JWT_TTL — длительность Go, например 1h или 30m")
	}

	return &Config{
		JWT: JWTConfig{
			Secret:    []byte(envOrDefault("JWT_SECRET", "heat-fuels-lab4-dev-secret")),
			ExpiresIn: ttl,
			Issuer:    "heat-fuels",
		},
		Redis: RedisConfig{
			Host:        envOrDefault("REDIS_HOST", "localhost"),
			Port:        envOrDefault("REDIS_PORT", "6379"),
			Password:    envOrDefault("REDIS_PASSWORD", "heat"),
			DialTimeout: 10 * time.Second,
			ReadTimeout: 10 * time.Second,
		},
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
