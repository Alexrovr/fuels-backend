package serializers

import "heat-backend/internal/app/models"

// RegisterRequest — тело POST /api/users/register.
type RegisterRequest struct {
	Login    string `json:"login" example:"sidorov"`
	FullName string `json:"full_name" example:"Сидоров Алексей"`
	Password string `json:"password" example:"heat54321"`
}

// LoginRequest — тело POST /api/users/login.
type LoginRequest struct {
	Login    string `json:"login" example:"ivanov"`
	Password string `json:"password" example:"heat12345"`
}

// LoginResponse — результат аутентификации: JWT для заголовка
// Authorization: Bearer <access_token> и пользователь, которому он выдан.
type LoginResponse struct {
	AccessToken string    `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string    `json:"token_type" example:"Bearer"`
	ExpiresIn   int       `json:"expires_in" example:"3600"` // секунд
	User        UserShort `json:"user"`
}

// MessageResponse — ответ без данных, например на выход из системы.
type MessageResponse struct {
	Message string `json:"message" example:"Выход выполнен: токен добавлен в чёрный список"`
}

// UserShort — пользователь в ответах. Поля password здесь нет,
// поэтому хэш пароля не попадает к клиенту ни в одном методе.
type UserShort struct {
	UserID   uint   `json:"user_id" example:"1"`
	Login    string `json:"login" example:"ivanov"`
	FullName string `json:"full_name" example:"Иванов Иван"`
}

func NewUserShort(user models.User) UserShort {
	return UserShort{
		UserID:   user.UserID,
		Login:    user.Login,
		FullName: user.FullName,
	}
}
