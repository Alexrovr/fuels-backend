package auth

import "sync"

// Singleton пользователя из ЛР3. С ЛР4 веб-сервис /api берёт пользователя
// из JWT (см. handler.AuthMiddleware), а этот объект остался только для
// SSR-страниц ЛР2 — у них авторизации нет.
const creatorUserID uint = 1

type CurrentUser struct {
	UserID uint
}

var (
	currentUserOnce sync.Once
	currentUser     *CurrentUser
)

func GetCurrentUser() *CurrentUser {
	currentUserOnce.Do(func() {
		currentUser = &CurrentUser{UserID: creatorUserID}
	})
	return currentUser
}

func CurrentUserID() uint {
	return GetCurrentUser().UserID
}
