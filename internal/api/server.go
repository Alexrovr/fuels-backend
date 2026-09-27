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
	userRepository := repository.NewUserRepository(fuelRepository.DB())

	minioStorage, err := storage.NewMinioStorage()
	if err != nil {
		return err
	}

	mediaResolver := storage.NewMediaResolver()
	fuelHandler := handler.NewFuelHandler(fuelRepository, mediaResolver, logger)
	fuelAPIHandler := handler.NewFuelAPIHandler(fuelRepository, minioStorage, logger)
	userAPIHandler := handler.NewUserAPIHandler(userRepository, logger)

	router := gin.Default()

	router.LoadHTMLGlob("templates/*.html")
	router.Static("/resources", "./resources")

	// --- SSR-страницы (ЛР1–ЛР2) --- //

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

	// --- Веб-сервис для SPA (ЛР3), JSON --- //

	registerAPI(router.Group("/api"), fuelAPIHandler, userAPIHandler)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3030"
	}

	logger.Infof("сервер расчёта теплоты сгорания запущен на http://localhost:%s", port)
	return router.Run(":" + port)
}

// registerAPI — десять методов веб-сервиса в двух доменах.
func registerAPI(api *gin.RouterGroup, fuels handler.FuelDomain, users handler.UserDomain) {
	// Домен услуги: /api/fuels
	fuelGroup := api.Group("/fuels")
	fuelGroup.GET("", fuels.GetFuels)                     // список с фильтром ?min_heat=
	fuelGroup.GET("/feed", fuels.GetFuelFeed)             // лента без ид
	fuelGroup.GET("/feed/:fuel_id", fuels.GetFuelFeed)    // лента по ид, ?next=true
	fuelGroup.GET("/draft", fuels.GetFuelDraft)           // черновик текущего пользователя
	fuelGroup.POST("", fuels.CreateFuel)                  // добавление + изображение и видео
	fuelGroup.PUT("/:fuel_id/publish", fuels.PublishFuel) // черновик -> опубликован
	fuelGroup.DELETE("/:fuel_id", fuels.DeleteFuel)       // soft delete
	fuelGroup.POST("/:fuel_id/like", fuels.LikeFuel)      // like: 1 / 0

	// Домен пользователя: /api/users
	userGroup := api.Group("/users")
	userGroup.POST("/register", users.Register)
	userGroup.POST("/login", users.Login)   // заглушка до ЛР4
	userGroup.POST("/logout", users.Logout) // заглушка до ЛР4
}
