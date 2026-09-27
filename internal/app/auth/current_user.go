package auth

import "sync"

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
