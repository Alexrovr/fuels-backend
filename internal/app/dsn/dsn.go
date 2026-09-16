package dsn

import (
	"fmt"
	"os"
)

// FromEnv собирает строку подключения к PostgreSQL из переменных окружения.
func FromEnv() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		envOrDefault("DB_HOST", "localhost"),
		envOrDefault("DB_PORT", "5434"),
		envOrDefault("DB_USER", "heat"),
		envOrDefault("DB_PASSWORD", "heat"),
		envOrDefault("DB_NAME", "heat_fuels"),
	)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
