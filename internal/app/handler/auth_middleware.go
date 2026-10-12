package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"heat-backend/internal/app/auth"
)

// Ключ контекста gin, под которым middleware кладёт проверенные claims.
const claimsKey = "jwt_claims"

// AuthMiddleware проверяет JWT из заголовка Authorization: Bearer <токен>.
//
// Два режима:
//   - RequireAuth — методы создателя: без действительного токена 401;
//   - OptionalAuth — чтение: гостю отвечает как обычно, а для вошедшего
//     пользователя дополнительно считает is_liked / is_mine. Неверный или
//     отозванный токен здесь не ошибка — запрос выполняется как от гостя.
type AuthMiddleware struct {
	tokens    *auth.TokenManager
	blacklist *auth.Blacklist
	logger    *logrus.Logger
}

func NewAuthMiddleware(tokens *auth.TokenManager, blacklist *auth.Blacklist, logger *logrus.Logger) *AuthMiddleware {
	return &AuthMiddleware{tokens: tokens, blacklist: blacklist, logger: logger}
}

// bearerToken достаёт токен из заголовка. Префикс Bearer по стандарту обязателен,
// но в Swagger UI его часто забывают — голый токен тоже принимается.
func bearerToken(ctx *gin.Context) string {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

// authenticate: подпись и срок → чёрный список Redis.
func (m *AuthMiddleware) authenticate(ctx *gin.Context) (*auth.Claims, int, string) {
	token := bearerToken(ctx)
	if token == "" {
		return nil, http.StatusUnauthorized,
			"Требуется аутентификация: передайте заголовок Authorization: Bearer <токен> из POST /api/users/login"
	}

	claims, err := m.tokens.Parse(token)
	if err != nil {
		return nil, http.StatusUnauthorized, "Токен недействителен или просрочен — войдите заново"
	}

	revoked, err := m.blacklist.Contains(ctx.Request.Context(), claims.ID)
	if err != nil {
		m.logger.Errorf("redis: проверка чёрного списка: %v", err)
		return nil, http.StatusInternalServerError, "Не удалось проверить токен"
	}
	if revoked {
		return nil, http.StatusUnauthorized, "Токен отозван: пользователь вышел из системы — войдите заново"
	}
	return claims, 0, ""
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		claims, status, message := m.authenticate(ctx)
		if claims == nil {
			if status == http.StatusUnauthorized {
				ctx.Header("WWW-Authenticate", `Bearer realm="heat-fuels"`)
			}
			ctx.AbortWithStatusJSON(status, gin.H{"error": message})
			return
		}
		ctx.Set(claimsKey, claims)
		ctx.Next()
	}
}

func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if claims, _, _ := m.authenticate(ctx); claims != nil {
			ctx.Set(claimsKey, claims)
		}
		ctx.Next()
	}
}

// currentClaims — claims пользователя запроса; nil у гостя.
func currentClaims(ctx *gin.Context) *auth.Claims {
	if value, ok := ctx.Get(claimsKey); ok {
		if claims, ok := value.(*auth.Claims); ok {
			return claims
		}
	}
	return nil
}

// currentUserID — ид пользователя из JWT; 0 у гостя.
func currentUserID(ctx *gin.Context) uint {
	if claims := currentClaims(ctx); claims != nil {
		return claims.UserID
	}
	return 0
}
