package main

import (
	"github.com/sirupsen/logrus"

	"heat-backend/internal/api"
)

// Общее описание веб-сервиса для Swagger. Документация генерируется из
// аннотаций командой (из корня репозитория):
//
//	swag init -g cmd/app/main.go -o docs/swagger
//
// @title        Расчёт теплоты сгорания — веб-сервис
// @version      4.0
// @description  JSON API справочника видов топлива для SPA (ЛР4).
// @description  Гость: только чтение — список, лента. Создатель (после входа): черновик, добавление, публикация, удаление, лайк, выход.
// @description  Вход: POST /users/login → скопировать access_token → кнопка Authorize → «Bearer <токен>».
//
// @host      localhost:3030
// @BasePath  /api
// @schemes   http
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 JWT из ответа POST /api/users/login в виде «Bearer <access_token>».
func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if err := api.StartServer(logger); err != nil {
		logger.Fatalf("не удалось запустить сервер: %v", err)
	}
}
