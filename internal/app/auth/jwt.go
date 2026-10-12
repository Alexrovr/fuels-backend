package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"heat-backend/internal/app/config"
	"heat-backend/internal/app/models"
)

var ErrInvalidToken = errors.New("токен недействителен или просрочен")

// Claims — полезная нагрузка JWT. Кроме стандартных полей (exp, iat, iss, sub, jti)
// в токене лежат ид и логин пользователя: по ним бэкенд знает, кто делает запрос,
// не обращаясь к базе.
type Claims struct {
	UserID uint   `json:"user_id"`
	Login  string `json:"login"`
	jwt.RegisteredClaims
}

// TokenManager выпускает и проверяет JWT, подписанные HS256.
type TokenManager struct {
	secret    []byte
	expiresIn time.Duration
	issuer    string
}

func NewTokenManager(cfg config.JWTConfig) *TokenManager {
	return &TokenManager{secret: cfg.Secret, expiresIn: cfg.ExpiresIn, issuer: cfg.Issuer}
}

func (m *TokenManager) ExpiresIn() time.Duration {
	return m.expiresIn
}

// Issue выпускает токен для пользователя. jti — уникальный ид токена: по нему
// токен попадает в чёрный список Redis при выходе.
func (m *TokenManager) Issue(user models.User) (string, *Claims, error) {
	now := time.Now()
	claims := &Claims{
		UserID: user.UserID,
		Login:  user.Login,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   fmt.Sprint(user.UserID),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiresIn)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", nil, err
	}
	return token, claims, nil
}

// Parse проверяет подпись, алгоритм, издателя и срок действия токена.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || claims.ID == "" || claims.UserID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
