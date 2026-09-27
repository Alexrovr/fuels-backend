package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"heat-backend/internal/app/models"
)

var ErrDraftAlreadyExists = errors.New("у пользователя уже есть черновик карточки")

// Методы веб-сервиса (ЛР3). Все обращения к базе — через ORM.

// GetActiveFuel ищет карточку по идентификатору среди неудалённых:
// удалённые записи на клиент не передаются, для API их не существует.
func (r *FuelRepository) GetActiveFuel(fuelID uint) (models.Fuel, error) {
	var fuel models.Fuel

	err := r.db.
		Preload("Creator").
		Where("fuel_id = ?", fuelID).
		Where("fuel_status <> ?", models.FuelStatusDeleted).
		First(&fuel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Fuel{}, ErrFuelNotFound
	}
	return fuel, err
}

// CreateFuel сохраняет новую карточку. Второй черновик того же пользователя
// база не пропустит (частичный уникальный индекс idx_fuels_single_draft).
func (r *FuelRepository) CreateFuel(fuel *models.Fuel) error {
	err := r.db.Create(fuel).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDraftAlreadyExists
	}
	return err
}

// MarkFuelDeleted — логическое удаление через ORM: меняется только статус,
// строка и лайки к ней остаются в базе.
func (r *FuelRepository) MarkFuelDeleted(fuelID uint) error {
	result := r.db.Model(&models.Fuel{}).
		Where("fuel_id = ?", fuelID).
		Where("fuel_status IN ?", []models.FuelStatus{models.FuelStatusDraft, models.FuelStatusPublished}).
		Update("fuel_status", models.FuelStatusDeleted)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrFuelNotFound
	}
	return nil
}

// --- Лайки ---------------------------------------------------------------- //

// AddLike ставит лайк. Повторный лайк не создаёт вторую строку:
// пара (user_id, fuel_id) уникальна, конфликт игнорируется.
func (r *FuelRepository) AddLike(userID, fuelID uint) error {
	return r.db.
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.FuelLike{UserID: userID, FuelID: fuelID}).Error
}

func (r *FuelRepository) RemoveLike(userID, fuelID uint) error {
	return r.db.
		Where("user_id = ? AND fuel_id = ?", userID, fuelID).
		Delete(&models.FuelLike{}).Error
}

func (r *FuelRepository) IsLikedBy(userID, fuelID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.FuelLike{}).
		Where("user_id = ? AND fuel_id = ?", userID, fuelID).
		Count(&count).Error
	return count > 0, err
}

// LikedFuelIDs возвращает, какие из карточек списка лайкнул пользователь.
func (r *FuelRepository) LikedFuelIDs(userID uint, fuelIDs []uint) (map[uint]bool, error) {
	liked := make(map[uint]bool, len(fuelIDs))
	if len(fuelIDs) == 0 {
		return liked, nil
	}

	var ids []uint
	err := r.db.Model(&models.FuelLike{}).
		Where("user_id = ? AND fuel_id IN ?", userID, fuelIDs).
		Pluck("fuel_id", &ids).Error
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		liked[id] = true
	}
	return liked, nil
}
