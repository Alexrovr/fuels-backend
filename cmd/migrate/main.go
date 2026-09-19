package main

import (
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"heat-backend/internal/app/dsn"
	"heat-backend/internal/app/models"
	"heat-backend/internal/app/storage"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		logger.Fatalf("подключение к postgres: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Fuel{}, &models.FuelLike{}); err != nil {
		logger.Fatalf("миграция таблиц: %v", err)
	}

	// У каждого пользователя не более одной карточки в статусе «черновик».
	err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_fuels_single_draft
		    ON fuels (creator_id)
		 WHERE fuel_status = 'черновик'
	`).Error
	if err != nil {
		logger.Fatalf("создание индекса единственного черновика: %v", err)
	}

	var fuelsCount int64
	if err := db.Model(&models.Fuel{}).Count(&fuelsCount).Error; err != nil {
		logger.Fatalf("подсчёт карточек: %v", err)
	}
	if fuelsCount > 0 {
		logger.Infof("таблицы уже наполнены (%d карточек), наполнение пропущено", fuelsCount)
		return
	}

	if err := seed(db); err != nil {
		logger.Fatalf("наполнение таблиц: %v", err)
	}
	logger.Info("таблицы созданы и наполнены данными")
}

// seedPassword — пароль всех стартовых пользователей. В базу пишется только его bcrypt-хэш.
const seedPassword = "heat12345"

func seed(db *gorm.DB) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	users := []models.User{
		{Login: "ivanov", FullName: "Иванов Иван", Password: string(passwordHash)},
		{Login: "petrova", FullName: "Петрова Анна", Password: string(passwordHash)},
		{Login: "smirnov", FullName: "Смирнов Пётр", Password: string(passwordHash)},
		{Login: "kuznecova", FullName: "Кузнецова Мария", Password: string(passwordHash)},
	}
	if err := db.Create(&users).Error; err != nil {
		return err
	}

	formedAt := time.Now().Add(-72 * time.Hour)

	fuels := []models.Fuel{
		{
			FuelName: "Метан",
			CombustionNote: "Основной компонент природного газа. Полное сгорание идёт по уравнению " +
				"CH4 + 2O2 -> CO2 + 2H2O. При нормальных условиях один кубометр метана отдаёт " +
				"35 800 кДж теплоты, поэтому метан используют как эталон при расчёте тепловой мощности " +
				"бытовых котлов и промышленных горелок.",
			FuelStatus:         models.FuelStatusPublished,
			ImageURL:           storage.MinioObjectURL("metan.jpg"),
			VideoURL:           storage.MinioObjectURL("methane.mp4"),
			HeatOfCombustionKJ: 35800,
			IgnitionTempC:      537,
			FormedAt:           &formedAt,
			CreatorID:          users[0].UserID,
		},
		{
			FuelName: "Пропан-бутан",
			CombustionNote: "Сжиженный углеводородный газ, смесь пропана и бутана в соотношении 50/50. " +
				"Реакция горения пропана: C3H8 + 5O2 -> 3CO2 + 4H2O. Кубометр смеси при н.у. выделяет " +
				"около 108 000 кДж — втрое больше метана, поэтому баллонный газ применяют там, где " +
				"нужна высокая тепловая мощность при малом объёме хранения.",
			FuelStatus:         models.FuelStatusPublished,
			ImageURL:           storage.MinioObjectURL("propan-bytan.jpg"),
			VideoURL:           storage.MinioObjectURL("propane_butane.mp4"),
			HeatOfCombustionKJ: 108000,
			IgnitionTempC:      470,
			FormedAt:           &formedAt,
			CreatorID:          users[0].UserID,
		},
		{
			FuelName: "Ацетилен",
			CombustionNote: "Топливо газовой сварки и резки металлов. Полное сгорание: " +
				"2C2H2 + 5O2 -> 4CO2 + 2H2O. Кубометр ацетилена при н.у. даёт 56 000 кДж, но " +
				"главное его достоинство не в теплоте, а в температуре пламени: в смеси с кислородом " +
				"оно достигает 3150 °C — выше, чем у любого другого промышленного газа.",
			FuelStatus:         models.FuelStatusPublished,
			ImageURL:           storage.MinioObjectURL("acetilen.png"),
			VideoURL:           storage.MinioObjectURL("acetylene.mp4"),
			HeatOfCombustionKJ: 56000,
			IgnitionTempC:      335,
			FormedAt:           &formedAt,
			CreatorID:          users[1].UserID,
		},
		{
			FuelName: "Водород",
			CombustionNote: "Единственное топливо подборки, при сгорании которого не образуется " +
				"диоксид углерода: 2H2 + O2 -> 2H2O. Объёмная теплота сгорания самая низкая — " +
				"10 800 кДж/м³ при н.у., потому что молекула водорода очень лёгкая, зато на единицу " +
				"массы водород выделяет 120 000 кДж/кг.",
			FuelStatus:         models.FuelStatusPublished,
			ImageURL:           storage.MinioObjectURL("vodorod.jpg"),
			VideoURL:           storage.MinioObjectURL("hydrogen.mp4"),
			HeatOfCombustionKJ: 10800,
			IgnitionTempC:      510,
			FormedAt:           &formedAt,
			CreatorID:          users[2].UserID,
		},
		{
			// Карточка без своих медиа: в url записаны фото и видео по умолчанию.
			FuelName:           "Метано-водородная смесь",
			FuelStatus:         models.FuelStatusDraft,
			ImageURL:           storage.DefaultImagePath,
			VideoURL:           storage.DefaultVideoPath,
			HeatOfCombustionKJ: 0,
			IgnitionTempC:      0,
			CreatorID:          users[0].UserID,
		},
		{
			FuelName: "Коксовый газ",
			CombustionNote: "Побочный продукт коксования угля. Карточка снята с публикации: " +
				"справочные значения расходятся у разных источников.",
			FuelStatus:         models.FuelStatusDeleted,
			ImageURL:           storage.MinioObjectURL("metan.jpg"),
			VideoURL:           storage.MinioObjectURL("methane.mp4"),
			HeatOfCombustionKJ: 16600,
			IgnitionTempC:      560,
			FormedAt:           &formedAt,
			CreatorID:          users[1].UserID,
		},
	}
	if err := db.Create(&fuels).Error; err != nil {
		return err
	}

	likes := []models.FuelLike{
		{UserID: users[0].UserID, FuelID: fuels[0].FuelID},
		{UserID: users[1].UserID, FuelID: fuels[0].FuelID},
		{UserID: users[2].UserID, FuelID: fuels[0].FuelID},
		{UserID: users[0].UserID, FuelID: fuels[1].FuelID},
		{UserID: users[3].UserID, FuelID: fuels[1].FuelID},
		{UserID: users[1].UserID, FuelID: fuels[2].FuelID},
		{UserID: users[2].UserID, FuelID: fuels[2].FuelID},
		{UserID: users[3].UserID, FuelID: fuels[2].FuelID},
		{UserID: users[0].UserID, FuelID: fuels[3].FuelID},
	}
	return db.Create(&likes).Error
}
