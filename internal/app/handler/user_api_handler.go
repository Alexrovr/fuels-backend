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

	"heat-backend/internal/app/models"
	"heat-backend/internal/app/repository"
	"heat-backend/internal/app/serializers"
)

// UserDomain — домен пользователя, url /api/users.
type UserDomain interface {
	Register(ctx *gin.Context) // POST /api/users/register
	Login(ctx *gin.Context)    // POST /api/users/login  — заглушка до ЛР4
	Logout(ctx *gin.Context)   // POST /api/users/logout — заглушка до ЛР4
}

var _ UserDomain = (*UserAPIHandler)(nil)

type UserAPIHandler struct {
	repository *repository.UserRepository
	logger     *logrus.Logger
}

func NewUserAPIHandler(userRepository *repository.UserRepository, logger *logrus.Logger) *UserAPIHandler {
	return &UserAPIHandler{repository: userRepository, logger: logger}
}

var loginPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

// Register — POST /api/users/register
//
// Регистрация: логин латиницей, ФИО и пароль. В базу пишется bcrypt-хэш
// пароля, в ответе пароля нет.
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
// Заглушка до ЛР4: проверяет логин и пароль, но сессию и токен не создаёт —
// все методы по-прежнему работают от имени пользователя из singleton.
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

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Заглушка: учётные данные верны, но сессия не создаётся — авторизация появится в ЛР4",
		"user":    serializers.NewUserShort(user),
	})
}

// Logout — POST /api/users/logout
//
// Заглушка до ЛР4: сессий ещё нет, закрывать нечего.
func (h *UserAPIHandler) Logout(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Заглушка: деавторизация появится в ЛР4 вместе с сессиями",
	})
}
