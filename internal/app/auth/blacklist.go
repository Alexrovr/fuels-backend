package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"heat-backend/internal/app/config"
)

// Ключи Redis: префикс сервиса и префикс назначения через точку, чтобы
// не пересекаться с ключами других приложений на том же сервере.
const (
	servicePrefix = "heat_fuels."
	jwtPrefix     = "jwt."
)

// Blacklist — чёрный список отозванных JWT в Redis.
//
// JWT не хранится на сервере, поэтому «выйти» можно только запретив токен
// до истечения его срока. Ключ — heat_fuels.jwt.<jti>, значение — хэш с
// пользователем (user_id, login) и временем выхода, TTL = остаток срока жизни
// токена: после этого токен и так недействителен, и запись удаляется сама.
type Blacklist struct {
	client *redis.Client
}

func NewBlacklist(ctx context.Context, cfg config.RedisConfig) (*Blacklist, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        cfg.Host + ":" + cfg.Port,
		Password:    cfg.Password,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("подключение к redis %s:%s: %w", cfg.Host, cfg.Port, err)
	}
	return &Blacklist{client: client}, nil
}

func blacklistKey(tokenID string) string {
	return servicePrefix + jwtPrefix + tokenID
}

// Add запрещает токен до конца его срока действия.
func (b *Blacklist) Add(ctx context.Context, claims *Claims) error {
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // токен уже просрочен
	}

	key := blacklistKey(claims.ID)
	_, err := b.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key,
			"user_id", strconv.FormatUint(uint64(claims.UserID), 10),
			"login", claims.Login,
			"logout_at", time.Now().Format(time.RFC3339),
		)
		pipe.Expire(ctx, key, ttl)
		return nil
	})
	return err
}

// Contains — токен отозван (пользователь из него вышел).
func (b *Blacklist) Contains(ctx context.Context, tokenID string) (bool, error) {
	count, err := b.client.Exists(ctx, blacklistKey(tokenID)).Result()
	return count > 0, err
}

func (b *Blacklist) Close() error {
	return b.client.Close()
}
