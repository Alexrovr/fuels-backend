package serializers

import "heat-backend/internal/app/models"

// RegisterRequest — тело POST /api/users/register.
type RegisterRequest struct {
	Login    string `json:"login"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

// LoginRequest — тело POST /api/users/login (заглушка до ЛР4).
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// UserShort — пользователь в ответах. Поля password здесь нет,
// поэтому хэш пароля не попадает к клиенту ни в одном методе.
type UserShort struct {
	UserID   uint   `json:"user_id"`
	Login    string `json:"login"`
	FullName string `json:"full_name"`
}

func NewUserShort(user models.User) UserShort {
	return UserShort{
		UserID:   user.UserID,
		Login:    user.Login,
		FullName: user.FullName,
	}
}
