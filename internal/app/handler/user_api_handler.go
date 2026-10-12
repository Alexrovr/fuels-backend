package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"heat-backend/internal/app/auth"
	"heat-backend/internal/app/models"
	"heat-backend/internal/app/repository"
	"heat-backend/internal/app/serializers"
)

// UserDomain — домен пользователя, url /api/users.
type UserDomain interface {
	Register(ctx *gin.Context) // POST /api/users/register
	Login(ctx *gin.Context)    // POST /api/users/login  — выдаёт JWT
	Logout(ctx *gin.Context)   // POST /api/users/logout — JWT в чёрный список Redis
}

var _ UserDomain = (*UserAPIHandler)(nil)

type UserAPIHandler struct {
	repository *repository.UserRepository
	tokens     *auth.TokenManager
	blacklist  *auth.Blacklist
	logger     *logrus.Logger
}

func NewUserAPIHandler(
	userRepository *repository.UserRepository,
	tokens *auth.TokenManager,
	blacklist *auth.Blacklist,
	logger *logrus.Logger,
) *UserAPIHandler {
	return &UserAPIHandler{repository: userRepository, tokens: tokens, blacklist: blacklist, logger: logger}
}

var loginPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

// Register — POST /api/users/register
//
// Регистрация: логин латиницей, ФИО и пароль. В базу пишется bcrypt-хэш
// пароля, в ответе пароля нет.
//
// @Summary      Регистрация
// @Description  Логин латиницей 3–64 символа, ФИО, пароль 6–72 символа. В БД — bcrypt-хэш пароля.
// @Tags         Пользователи
// @Accept       json
// @Produce      json
// @Param        body  body      serializers.RegisterRequest  true  "Данные пользователя"
// @Success      201   {object}  serializers.UserShort
// @Failure      400   {object}  serializers.ErrorResponse
// @Failure      409   {object}  serializers.ErrorResponse
// @Router       /users/register [post]
func (h *UserAPIHandler) Register(ctx *gin.Context) {
	var request serializers.RegisterRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	login := strings.TrimSpace(request.Login)
	fullName := strings.TrimSpace(request.FullName)

	switch {
	case !loginPattern.MatchString(login):
		respondError(ctx, http.StatusBadRequest, "Логин — от 3 до 64 символов: латиница, цифры, _ . -")
		return
	case fullName == "" || utf8.RuneCountInString(fullName) > 128:
		respondError(ctx, http.StatusBadRequest, "Укажите ФИО (full_name), не длиннее 128 символов")
		return
	case len(request.Password) < 6 || len(request.Password) > 72:
		respondError(ctx, http.StatusBadRequest, "Пароль — от 6 до 72 символов")
		return
	}

	if _, err := h.repository.GetUserByLogin(login); err == nil {
		respondError(ctx, http.StatusConflict, "Логин "+login+" уже занят")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Errorf("api: хэш пароля: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось зарегистрировать пользователя")
		return
	}

	user := models.User{Login: login, FullName: fullName, Password: string(passwordHash)}
	err = h.repository.CreateUser(&user)
	switch {
	case errors.Is(err, repository.ErrLoginTaken):
		respondError(ctx, http.StatusConflict, "Логин "+login+" уже занят")
		return
	case err != nil:
		h.logger.Errorf("api: регистрация %s: %v", login, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось зарегистрировать пользователя")
		return
	}

	h.logger.Infof("api: зарегистрирован пользователь %d (%s)", user.UserID, user.Login)
	ctx.JSON(http.StatusCreated, serializers.NewUserShort(user))
}

// Login — POST /api/users/login
//
// Аутентификация: проверяет логин и bcrypt-хэш пароля и выдаёт JWT.
// На сервере токен не хранится — всё нужное (user_id, login, срок) лежит в нём
// самом и защищено подписью HS256.
//
// @Summary      Аутентификация (вход)
// @Description  Возвращает JWT. Его нужно передавать в заголовке Authorization: Bearer <access_token> (в Swagger — кнопка Authorize).
// @Tags         Пользователи
// @Accept       json
// @Produce      json
// @Param        body  body      serializers.LoginRequest  true  "Логин и пароль"
// @Success      200   {object}  serializers.LoginResponse
// @Failure      400   {object}  serializers.ErrorResponse
// @Failure      401   {object}  serializers.ErrorResponse
// @Router       /users/login [post]
func (h *UserAPIHandler) Login(ctx *gin.Context) {
	var request serializers.LoginRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.repository.GetUserByLogin(strings.TrimSpace(request.Login))
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		h.logger.Errorf("api: аутентификация: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось проверить учётные данные")
		return
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)) != nil {
		respondError(ctx, http.StatusUnauthorized, "Неверный логин или пароль")
		return
	}

	token, _, err := h.tokens.Issue(user)
	if err != nil {
		h.logger.Errorf("api: выпуск JWT для %s: %v", user.Login, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось выпустить токен")
		return
	}

	h.logger.Infof("api: вход пользователя %d (%s)", user.UserID, user.Login)
	ctx.JSON(http.StatusOK, serializers.LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.tokens.ExpiresIn().Seconds()),
		User:        serializers.NewUserShort(user),
	})
}

// Logout — POST /api/users/logout
//
// Деавторизация: JWT нельзя «удалить» у клиента, поэтому его ид (jti) кладётся
// в чёрный список Redis до истечения срока. Middleware отклоняет такой токен (401).
//
// @Summary      Деавторизация (выход)
// @Description  Текущий JWT попадает в чёрный список Redis (ключ heat_fuels.jwt.<jti>) до истечения срока; дальше с ним 401.
// @Tags         Пользователи
// @Produce      json
// @Success      200  {object}  serializers.MessageResponse
// @Failure      401  {object}  serializers.ErrorResponse
// @Security     BearerAuth
// @Router       /users/logout [post]
func (h *UserAPIHandler) Logout(ctx *gin.Context) {
	claims := currentClaims(ctx)
	if err := h.blacklist.Add(ctx.Request.Context(), claims); err != nil {
		h.logger.Errorf("redis: чёрный список, пользователь %d: %v", claims.UserID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось завершить сеанс")
		return
	}

	h.logger.Infof("api: выход пользователя %d (%s), токен %s отозван", claims.UserID, claims.Login, claims.ID)
	ctx.JSON(http.StatusOK, serializers.MessageResponse{
		Message: "Выход выполнен: токен пользователя " + claims.Login + " добавлен в чёрный список",
	})
}
