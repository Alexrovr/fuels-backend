package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"heat-backend/internal/app/dsn"
	"heat-backend/internal/app/models"
)

var ErrFuelNotFound = errors.New("вид топлива не найден")
var ErrDraftNotFound = errors.New("черновик карточки топлива не найден")

type FuelRepository struct {
	db *gorm.DB
}

func NewFuelRepository() (*FuelRepository, error) {
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("подключение к postgres: %w", err)
	}
	return &FuelRepository{db: db}, nil
}

func (r *FuelRepository) DB() *gorm.DB {
	return r.db
}


func (r *FuelRepository) GetPublishedFuels(minHeatKJ int) ([]models.Fuel, error) {
	var fuels []models.Fuel

	err := r.db.
		Where("fuel_status = ?", models.FuelStatusPublished).
		Where("heat_of_combustion_kj >= ?", minHeatKJ).
		Order("fuel_id").
		Find(&fuels).Error
	if err != nil {
		return nil, err
	}
	return fuels, nil
}

func (r *FuelRepository) GetPublishedFuelByID(fuelID uint) (models.Fuel, error) {
	var fuel models.Fuel

	err := r.db.
		Where("fuel_id = ?", fuelID).
		Where("fuel_status = ?", models.FuelStatusPublished).
		First(&fuel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Fuel{}, ErrFuelNotFound
	}
	return fuel, err
}

// GetNextPublishedFuel отдаёт следующую опубликованную карточку по кругу.
func (r *FuelRepository) GetNextPublishedFuel(fuelID uint) (models.Fuel, error) {
	var fuel models.Fuel

	err := r.db.
		Where("fuel_status = ?", models.FuelStatusPublished).
		Where("fuel_id > ?", fuelID).
		Order("fuel_id").
		First(&fuel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.GetFirstPublishedFuel()
	}
	return fuel, err
}

func (r *FuelRepository) GetFirstPublishedFuel() (models.Fuel, error) {
	var fuel models.Fuel

	err := r.db.
		Where("fuel_status = ?", models.FuelStatusPublished).
		Order("fuel_id").
		First(&fuel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Fuel{}, ErrFuelNotFound
	}
	return fuel, err
}

// GetDraftByCreator ищет единственный черновик пользователя.
func (r *FuelRepository) GetDraftByCreator(userID uint) (models.Fuel, error) {
	var fuel models.Fuel

	err := r.db.
		Where("creator_id = ?", userID).
		Where("fuel_status = ?", models.FuelStatusDraft).
		First(&fuel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Fuel{}, ErrDraftNotFound
	}
	return fuel, err
}

// --- Лайки (таблица многие-ко-многим) ------------------------------------ //

func (r *FuelRepository) LikesCount(fuelID uint) (int, error) {
	var count int64
	err := r.db.Model(&models.FuelLike{}).Where("fuel_id = ?", fuelID).Count(&count).Error
	return int(count), err
}

// LikesCountByFuel считает лайки сразу для списка карточек одним запросом.
func (r *FuelRepository) LikesCountByFuel(fuelIDs []uint) (map[uint]int, error) {
	counts := make(map[uint]int, len(fuelIDs))
	if len(fuelIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		FuelID uint
		Total  int
	}
	err := r.db.Model(&models.FuelLike{}).
		Select("fuel_id, count(*) as total").
		Where("fuel_id IN ?", fuelIDs).
		Group("fuel_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.FuelID] = row.Total
	}
	return counts, nil
}

// --- Создание и публикация карточки через ORM ----------------------------- //

// CreateDraft создаёт карточку в статусе «черновик». Файлы в этой лабораторной
// на сервер не передаются, поэтому url медиа остаются пустыми и шаблоны
// подставляют изображение и видео по умолчанию.
func (r *FuelRepository) CreateDraft(creatorID uint, fuelName string) (models.Fuel, error) {
	fuel := models.Fuel{
		FuelName:   fuelName,
		FuelStatus: models.FuelStatusDraft,
		CreatorID:  creatorID,
	}

	if err := r.db.Create(&fuel).Error; err != nil {
		return models.Fuel{}, err
	}
	return fuel, nil
}

func (r *FuelRepository) PublishDraft(fuelID uint, note string, heatOfCombustionKJ, ignitionTempC int) error {
	formedAt := time.Now()

	result := r.db.Model(&models.Fuel{}).
		Where("fuel_id = ?", fuelID).
		Where("fuel_status = ?", models.FuelStatusDraft).
		Updates(map[string]any{
			"combustion_note":       note,
			"heat_of_combustion_kj": heatOfCombustionKJ,
			"ignition_temp_c":       ignitionTempC,
			"fuel_status":           models.FuelStatusPublished,
			"formed_at":             formedAt,
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDraftNotFound
	}
	return nil
}


func (r *FuelRepository) SoftDeleteFuel(fuelID uint) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	rows, err := sqlDB.Query(
		`UPDATE fuels
		    SET fuel_status = $1
		  WHERE fuel_id = $2
		    AND fuel_status <> $1
		RETURNING fuel_id`,
		models.FuelStatusDeleted, fuelID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var deletedID uint
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrFuelNotFound
	}
	if err := rows.Scan(&deletedID); err != nil {
		return err
	}
	return rows.Err()
}
