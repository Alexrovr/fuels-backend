package repository

import (
	"errors"

	"gorm.io/gorm"

	"heat-backend/internal/app/models"
)

var ErrUserNotFound = errors.New("пользователь не найден")
var ErrLoginTaken = errors.New("логин уже занят")

// UserRepository — доступ к таблице users. Использует то же подключение, что и FuelRepository.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(user *models.User) error {
	err := r.db.Create(user).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrLoginTaken
	}
	return err
}

func (r *UserRepository) GetUserByLogin(login string) (models.User, error) {
	var user models.User

	err := r.db.Where("login = ?", login).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, ErrUserNotFound
	}
	return user, err
}
