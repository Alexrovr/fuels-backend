package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"heat-backend/internal/app/handler"
	"heat-backend/internal/app/repository"
	"heat-backend/internal/app/storage"
)

// StartServer собирает приложение и регистрирует маршруты.
func StartServer(logger *logrus.Logger) error {
	fuelRepository, err := repository.NewFuelRepository()
	if err != nil {
		return err
	}
	mediaResolver := storage.NewMediaResolver()
	fuelHandler := handler.NewFuelHandler(fuelRepository, mediaResolver, logger)

	router := gin.Default()

	router.LoadHTMLGlob("templates/*.html")
	router.Static("/resources", "./resources")

	// Три GET-метода: лента по идентификатору, черновик, список всех карточек.
	router.GET("/fuel_feed", fuelHandler.GetFuelFeed)
	router.GET("/fuel_feed/:fuel_id", fuelHandler.GetFuelFeed)
	router.GET("/fuel_draft", fuelHandler.GetFuelDraft)
	router.GET("/fuel_grid", fuelHandler.GetFuelGrid)

	// Три POST-метода: создание черновика и публикация карточки через ORM,
	// логическое удаление — запросом SQL UPDATE без ORM.
	router.POST("/fuel_draft/create", fuelHandler.CreateFuelDraft)
	router.POST("/fuel_draft/publish", fuelHandler.PublishFuelDraft)
	router.POST("/fuel_grid/delete/:fuel_id", fuelHandler.DeleteFuel)

	router.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/fuel_grid")
	})

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3030"
	}

	logger.Infof("сервер расчёта теплоты сгорания запущен на http://localhost:%s", port)
	return router.Run(":" + port)
}
