package api

import (
	"context"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "heat-backend/docs/swagger" // сгенерированное описание API (swag init)
	"heat-backend/internal/app/auth"
	"heat-backend/internal/app/config"
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

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	blacklist, err := auth.NewBlacklist(context.Background(), cfg.Redis)
	if err != nil {
		return err
	}
	defer blacklist.Close()
	tokens := auth.NewTokenManager(cfg.JWT)

	mediaResolver := storage.NewMediaResolver()
	fuelHandler := handler.NewFuelHandler(fuelRepository, mediaResolver, logger)
	fuelAPIHandler := handler.NewFuelAPIHandler(fuelRepository, minioStorage, logger)
	userAPIHandler := handler.NewUserAPIHandler(userRepository, tokens, blacklist, logger)
	authMiddleware := handler.NewAuthMiddleware(tokens, blacklist, logger)

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

	// --- Веб-сервис для SPA (ЛР3–ЛР4), JSON --- //

	registerAPI(router.Group("/api"), fuelAPIHandler, userAPIHandler, authMiddleware)

	// Swagger UI: http://localhost:3030/swagger/index.html
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3030"
	}

	logger.Infof("сервер расчёта теплоты сгорания запущен на http://localhost:%s, swagger — /swagger/index.html", port)
	return router.Run(":" + port)
}

// registerAPI — десять методов веб-сервиса в двух доменах.
//
// Доступ (ЛР4): гость — только чтение (список, лента), регистрация и вход;
// методы создателя требуют JWT — без него 401, чужая карточка — 403.
func registerAPI(api *gin.RouterGroup, fuels handler.FuelDomain, users handler.UserDomain, auth *handler.AuthMiddleware) {
	// Домен услуги: /api/fuels
	fuelGroup := api.Group("/fuels")

	// Чтение — гостю и пользователю; с токеном дополнительно is_liked / is_mine.
	public := fuelGroup.Group("", auth.OptionalAuth())
	public.GET("", fuels.GetFuels)                  // список с фильтром ?min_heat=
	public.GET("/feed", fuels.GetFuelFeed)          // лента без ид
	public.GET("/feed/:fuel_id", fuels.GetFuelFeed) // лента по ид, ?next=true

	// Методы создателя — только с JWT; создатель берётся из токена.
	creator := fuelGroup.Group("", auth.RequireAuth())
	creator.GET("/draft", fuels.GetFuelDraft)           // черновик текущего пользователя
	creator.POST("", fuels.CreateFuel)                  // добавление + изображение и видео
	creator.PUT("/:fuel_id/publish", fuels.PublishFuel) // черновик -> опубликован
	creator.DELETE("/:fuel_id", fuels.DeleteFuel)       // soft delete
	creator.POST("/:fuel_id/like", fuels.LikeFuel)      // like: 1 / 0

	// Домен пользователя: /api/users
	userGroup := api.Group("/users")
	userGroup.POST("/register", users.Register)
	userGroup.POST("/login", users.Login)                       // выдаёт JWT
	userGroup.POST("/logout", auth.RequireAuth(), users.Logout) // JWT -> чёрный список Redis
}
