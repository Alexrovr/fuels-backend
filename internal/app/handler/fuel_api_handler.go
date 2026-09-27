package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"heat-backend/internal/app/auth"
	"heat-backend/internal/app/models"
	"heat-backend/internal/app/repository"
	"heat-backend/internal/app/serializers"
	"heat-backend/internal/app/storage"
)

// FuelDomain — домен услуги, url /api/fuels. Семь методов веб-сервиса.
type FuelDomain interface {
	GetFuels(ctx *gin.Context)     // GET    /api/fuels?min_heat=
	GetFuelFeed(ctx *gin.Context)  // GET    /api/fuels/feed, /api/fuels/feed/:fuel_id?next=true
	GetFuelDraft(ctx *gin.Context) // GET    /api/fuels/draft
	CreateFuel(ctx *gin.Context)   // POST   /api/fuels
	PublishFuel(ctx *gin.Context)  // PUT    /api/fuels/:fuel_id/publish
	DeleteFuel(ctx *gin.Context)   // DELETE /api/fuels/:fuel_id
	LikeFuel(ctx *gin.Context)     // POST   /api/fuels/:fuel_id/like
}

var _ FuelDomain = (*FuelAPIHandler)(nil)

type FuelAPIHandler struct {
	repository *repository.FuelRepository
	storage    *storage.MinioStorage
	logger     *logrus.Logger
}

func NewFuelAPIHandler(
	fuelRepository *repository.FuelRepository,
	minioStorage *storage.MinioStorage,
	logger *logrus.Logger,
) *FuelAPIHandler {
	return &FuelAPIHandler{
		repository: fuelRepository,
		storage:    minioStorage,
		logger:     logger,
	}
}

// fuelDetail добавляет к карточке число лайков и отметку «лайкнул текущий пользователь».
func (h *FuelAPIHandler) fuelDetail(ctx *gin.Context, fuel models.Fuel) (serializers.FuelDetail, error) {
	likesCount, err := h.repository.LikesCount(fuel.FuelID)
	if err != nil {
		return serializers.FuelDetail{}, err
	}
	liked, err := h.repository.IsLikedBy(auth.CurrentUserID(), fuel.FuelID)
	if err != nil {
		return serializers.FuelDetail{}, err
	}
	return serializers.NewFuelDetail(fuel, likesCount, liked, absoluteMediaURL(ctx)), nil
}

func (h *FuelAPIHandler) respondFuel(ctx *gin.Context, status int, fuel models.Fuel) {
	detail, err := h.fuelDetail(ctx, fuel)
	if err != nil {
		h.logger.Errorf("сборка карточки %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить данные карточки")
		return
	}
	ctx.JSON(status, detail)
}

// GetFuels — GET /api/fuels?min_heat=50000
//
// Список опубликованных карточек с фильтрацией на бэкенде по нижней границе
// теплоты сгорания (как слайдер в ЛР1). Черновики и удалённые не попадают.
func (h *FuelAPIHandler) GetFuels(ctx *gin.Context) {
	minHeatKJ := 0
	if query := strings.TrimSpace(ctx.Query("min_heat")); query != "" {
		parsed, err := strconv.Atoi(query)
		if err != nil || parsed < 0 {
			respondError(ctx, http.StatusBadRequest, "min_heat — целое неотрицательное число, кДж/м³")
			return
		}
		minHeatKJ = parsed
	}

	fuels, err := h.repository.GetPublishedFuels(minHeatKJ)
	if err != nil {
		h.logger.Errorf("api: список топлив: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить список топлив")
		return
	}

	fuelIDs := make([]uint, 0, len(fuels))
	for _, fuel := range fuels {
		fuelIDs = append(fuelIDs, fuel.FuelID)
	}
	likesByFuel, err := h.repository.LikesCountByFuel(fuelIDs)
	if err != nil {
		h.logger.Errorf("api: лайки списка: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось посчитать лайки")
		return
	}
	likedByMe, err := h.repository.LikedFuelIDs(auth.CurrentUserID(), fuelIDs)
	if err != nil {
		h.logger.Errorf("api: лайки пользователя: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось посчитать лайки")
		return
	}

	items := make([]serializers.FuelListItem, 0, len(fuels))
	for _, fuel := range fuels {
		items = append(items, serializers.NewFuelListItem(
			fuel, likesByFuel[fuel.FuelID], likedByMe[fuel.FuelID], absoluteMediaURL(ctx)))
	}
	ctx.JSON(http.StatusOK, items)
}

// GetFuelFeed — GET /api/fuels/feed и GET /api/fuels/feed/:fuel_id[?next=true]
//
// Лента: без ид — первая опубликованная карточка, с ид — эта карточка,
// с next=true — следующая за ней (по кругу).
func (h *FuelAPIHandler) GetFuelFeed(ctx *gin.Context) {
	var (
		fuel models.Fuel
		err  error
	)

	if ctx.Param("fuel_id") == "" {
		fuel, err = h.repository.GetFirstPublishedFuel()
	} else {
		fuelID, ok := parseFuelID(ctx)
		if !ok {
			return
		}
		if ctx.Query("next") == "true" {
			fuel, err = h.repository.GetNextPublishedFuel(fuelID)
		} else {
			fuel, err = h.repository.GetPublishedFuelByID(fuelID)
		}
	}

	switch {
	case errors.Is(err, repository.ErrFuelNotFound):
		respondError(ctx, http.StatusNotFound, "Такого вида топлива нет в справочнике")
		return
	case err != nil:
		h.logger.Errorf("api: лента: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить карточку ленты")
		return
	}

	h.respondFuel(ctx, http.StatusOK, fuel)
}

// GetFuelDraft — GET /api/fuels/draft
//
// Черновик текущего пользователя (не более одного), ид не указывается.
func (h *FuelAPIHandler) GetFuelDraft(ctx *gin.Context) {
	draft, err := h.repository.GetDraftByCreator(auth.CurrentUserID())
	switch {
	case errors.Is(err, repository.ErrDraftNotFound):
		respondError(ctx, http.StatusNotFound, "У пользователя нет черновика карточки")
		return
	case err != nil:
		h.logger.Errorf("api: черновик: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить черновик")
		return
	}

	h.respondFuel(ctx, http.StatusOK, draft)
}

// CreateFuel — POST /api/fuels (multipart/form-data: fuel_name, image, video)
//
// Создаёт карточку в статусе «черновик». Изображение и видео загружаются
// в Minio под сгенерированными латинскими именами, url объектов записываются
// в поля image_url и video_url.
func (h *FuelAPIHandler) CreateFuel(ctx *gin.Context) {
	userID := auth.CurrentUserID()

	if _, err := h.repository.GetDraftByCreator(userID); err == nil {
		respondError(ctx, http.StatusConflict,
			"У пользователя уже есть черновик: опубликуйте или удалите его (GET /api/fuels/draft)")
		return
	}

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadBody)
	if err := ctx.Request.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(ctx, http.StatusRequestEntityTooLarge, "Запрос больше 36 МБ")
			return
		}
		respondError(ctx, http.StatusBadRequest, "Ожидается multipart/form-data с полями fuel_name, image, video")
		return
	}
	form := ctx.Request.MultipartForm
	defer form.RemoveAll()

	for field := range form.Value {
		if field != "fuel_name" {
			respondError(ctx, http.StatusBadRequest, "Поле "+field+" не принимается: "+systemFieldsHint)
			return
		}
	}
	for field := range form.File {
		if field != imageRule.field && field != videoRule.field {
			respondError(ctx, http.StatusBadRequest, "Неизвестный файл "+field+": ожидаются image и video")
			return
		}
	}

	fuelName := strings.TrimSpace(ctx.Request.FormValue("fuel_name"))
	switch {
	case fuelName == "":
		respondError(ctx, http.StatusBadRequest, "Укажите название вида топлива (fuel_name)")
		return
	case utf8.RuneCountInString(fuelName) > 128:
		respondError(ctx, http.StatusBadRequest, "Название вида топлива — не длиннее 128 символов")
		return
	}

	image, err := checkMedia(form, imageRule)
	if err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	video, err := checkMedia(form, videoRule)
	if err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	imageName := storage.GenerateObjectName(imageRule.kind, image.extension)
	videoName := storage.GenerateObjectName(videoRule.kind, video.extension)

	imageURL, err := h.upload(ctx, imageName, image)
	if err != nil {
		h.logger.Errorf("api: загрузка изображения: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось сохранить изображение в Minio")
		return
	}
	videoURL, err := h.upload(ctx, videoName, video)
	if err != nil {
		h.logger.Errorf("api: загрузка видео: %v", err)
		h.removeObjects(ctx, imageName)
		respondError(ctx, http.StatusInternalServerError, "Не удалось сохранить видео в Minio")
		return
	}

	fuel := models.Fuel{
		FuelName:   fuelName,
		FuelStatus: models.FuelStatusDraft,
		ImageURL:   imageURL,
		VideoURL:   videoURL,
		CreatorID:  userID,
	}
	if err := h.repository.CreateFuel(&fuel); err != nil {
		h.removeObjects(ctx, imageName, videoName)
		if errors.Is(err, repository.ErrDraftAlreadyExists) {
			respondError(ctx, http.StatusConflict, "У пользователя уже есть черновик")
			return
		}
		h.logger.Errorf("api: создание карточки: %v", err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось создать карточку")
		return
	}

	h.logger.Infof("api: создан черновик %d (%s, %s) пользователем %d", fuel.FuelID, imageName, videoName, userID)

	created, err := h.repository.GetActiveFuel(fuel.FuelID)
	if err != nil {
		h.logger.Errorf("api: чтение созданной карточки %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Карточка создана, но не удалось её прочитать")
		return
	}
	ctx.Header("Location", "/api/fuels/draft")
	h.respondFuel(ctx, http.StatusCreated, created)
}

func (h *FuelAPIHandler) upload(ctx *gin.Context, objectName string, media checkedMedia) (string, error) {
	file, err := media.header.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	return h.storage.Upload(ctx.Request.Context(), objectName, file, media.header.Size, media.contentType)
}

func (h *FuelAPIHandler) removeObjects(ctx *gin.Context, objectNames ...string) {
	for _, name := range objectNames {
		if err := h.storage.Remove(ctx.Request.Context(), name); err != nil {
			h.logger.Warnf("api: не удалось убрать %s из minio: %v", name, err)
		}
	}
}

// ownActiveFuel находит неудалённую карточку и проверяет, что её создал
// текущий пользователь: публиковать и удалять может только создатель.
func (h *FuelAPIHandler) ownActiveFuel(ctx *gin.Context) (models.Fuel, bool) {
	fuelID, ok := parseFuelID(ctx)
	if !ok {
		return models.Fuel{}, false
	}

	fuel, err := h.repository.GetActiveFuel(fuelID)
	switch {
	case errors.Is(err, repository.ErrFuelNotFound):
		respondError(ctx, http.StatusNotFound, "Такого вида топлива нет в справочнике")
		return models.Fuel{}, false
	case err != nil:
		h.logger.Errorf("api: карточка %d: %v", fuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить карточку")
		return models.Fuel{}, false
	}

	if fuel.CreatorID != auth.CurrentUserID() {
		respondError(ctx, http.StatusForbidden, "Изменять статус карточки может только её создатель")
		return models.Fuel{}, false
	}
	return fuel, true
}

// PublishFuel — PUT /api/fuels/:fuel_id/publish
//
// Смена статуса «черновик» → «опубликован». Тело (JSON) дозаполняет описание
// и оба поля по теме; дата формирования ставится на бэкенде.
func (h *FuelAPIHandler) PublishFuel(ctx *gin.Context) {
	fuel, ok := h.ownActiveFuel(ctx)
	if !ok {
		return
	}

	if !fuel.FuelStatus.CanChangeTo(models.FuelStatusPublished) {
		respondError(ctx, http.StatusConflict,
			"Опубликовать можно только черновик, карточка уже в статусе «"+string(fuel.FuelStatus)+"»")
		return
	}

	var request serializers.PublishFuelRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}

	note := fuel.CombustionNote
	if request.CombustionNote != nil {
		note = strings.TrimSpace(*request.CombustionNote)
	}
	heat := fuel.HeatOfCombustionKJ
	if request.HeatOfCombustionKJ != nil {
		heat = *request.HeatOfCombustionKJ
	}
	ignition := fuel.IgnitionTempC
	if request.IgnitionTempC != nil {
		ignition = *request.IgnitionTempC
	}

	switch {
	case note == "":
		respondError(ctx, http.StatusBadRequest, "Заполните краткое описание реакции горения (combustion_note)")
		return
	case heat <= 0:
		respondError(ctx, http.StatusBadRequest, "Теплота сгорания (heat_of_combustion_kj) — целое число больше нуля, кДж/м³")
		return
	case ignition <= 0:
		respondError(ctx, http.StatusBadRequest, "Температура воспламенения (ignition_temp_c) — целое число больше нуля, °C")
		return
	}

	err := h.repository.PublishDraft(fuel.FuelID, note, heat, ignition)
	switch {
	case errors.Is(err, repository.ErrDraftNotFound):
		respondError(ctx, http.StatusConflict, "Карточка уже не черновик")
		return
	case err != nil:
		h.logger.Errorf("api: публикация %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось опубликовать карточку")
		return
	}

	h.logger.Infof("api: карточка %d опубликована пользователем %d", fuel.FuelID, auth.CurrentUserID())

	published, err := h.repository.GetActiveFuel(fuel.FuelID)
	if err != nil {
		h.logger.Errorf("api: чтение опубликованной карточки %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Карточка опубликована, но не удалось её прочитать")
		return
	}
	h.respondFuel(ctx, http.StatusOK, published)
}

// DeleteFuel — DELETE /api/fuels/:fuel_id
//
// Только логическое удаление (soft delete): статус меняется на «удален»
// через ORM, строка остаётся в базе. Удалённая карточка больше не
// отдаётся ни одним методом, поэтому в ответе только её ид.
func (h *FuelAPIHandler) DeleteFuel(ctx *gin.Context) {
	fuel, ok := h.ownActiveFuel(ctx)
	if !ok {
		return
	}

	if !fuel.FuelStatus.CanChangeTo(models.FuelStatusDeleted) {
		respondError(ctx, http.StatusConflict, "Карточку в статусе «"+string(fuel.FuelStatus)+"» удалить нельзя")
		return
	}

	err := h.repository.MarkFuelDeleted(fuel.FuelID)
	switch {
	case errors.Is(err, repository.ErrFuelNotFound):
		respondError(ctx, http.StatusNotFound, "Такого вида топлива нет в справочнике")
		return
	case err != nil:
		h.logger.Errorf("api: удаление %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось удалить карточку")
		return
	}

	h.logger.Infof("api: карточка %d переведена в статус «удален»", fuel.FuelID)
	ctx.JSON(http.StatusOK, gin.H{
		"fuel_id": fuel.FuelID,
		"message": "Карточка переведена в статус «удален»",
	})
}

// LikeFuel — POST /api/fuels/:fuel_id/like, тело {"like": 1} или {"like": 0}
//
// Лайк от текущего пользователя: 1 ставит, 0 отменяет. Повторный вызов
// с тем же значением ничего не меняет. Лайкнуть можно только опубликованную карточку.
func (h *FuelAPIHandler) LikeFuel(ctx *gin.Context) {
	fuelID, ok := parseFuelID(ctx)
	if !ok {
		return
	}

	var request serializers.LikeRequest
	if err := decodeStrictJSON(ctx, &request); err != nil {
		respondError(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if request.Like == nil || (*request.Like != 0 && *request.Like != 1) {
		respondError(ctx, http.StatusBadRequest, "Поле like обязательно: 1 — поставить лайк, 0 — отменить")
		return
	}

	fuel, err := h.repository.GetPublishedFuelByID(fuelID)
	switch {
	case errors.Is(err, repository.ErrFuelNotFound):
		respondError(ctx, http.StatusNotFound, "Лайк можно поставить только опубликованной карточке")
		return
	case err != nil:
		h.logger.Errorf("api: лайк %d: %v", fuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось получить карточку")
		return
	}

	userID := auth.CurrentUserID()
	if *request.Like == 1 {
		err = h.repository.AddLike(userID, fuel.FuelID)
	} else {
		err = h.repository.RemoveLike(userID, fuel.FuelID)
	}
	if err != nil {
		h.logger.Errorf("api: лайк %d пользователем %d: %v", fuel.FuelID, userID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось сохранить лайк")
		return
	}

	likesCount, err := h.repository.LikesCount(fuel.FuelID)
	if err != nil {
		h.logger.Errorf("api: подсчёт лайков %d: %v", fuel.FuelID, err)
		respondError(ctx, http.StatusInternalServerError, "Не удалось посчитать лайки")
		return
	}

	ctx.JSON(http.StatusOK, serializers.LikeResponse{
		FuelID:     fuel.FuelID,
		Liked:      *request.Like == 1,
		LikesCount: likesCount,
	})
}
