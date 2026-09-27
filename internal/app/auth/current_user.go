package auth

import "sync"

// creatorUserID — пользователь-создатель, от имени которого выполняются все
// методы. Авторизация появится в ЛР4, до тех пор он зафиксирован константой
// (ivanov из стартовых данных).
const creatorUserID uint = 1

// CurrentUser — данные о пользователе текущего запроса.
type CurrentUser struct {
	UserID uint
}

var (
	currentUserOnce sync.Once
	currentUser     *CurrentUser
)

// GetCurrentUser — функция-singleton: объект пользователя создаётся один раз
// при первом вызове, дальше все методы получают один и тот же экземпляр.
func GetCurrentUser() *CurrentUser {
	currentUserOnce.Do(func() {
		currentUser = &CurrentUser{UserID: creatorUserID}
	})
	return currentUser
}

// CurrentUserID — сокращение для обработчиков.
func CurrentUserID() uint {
	return GetCurrentUser().UserID
}
