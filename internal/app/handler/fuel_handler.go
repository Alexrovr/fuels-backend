package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"heat-backend/internal/app/models"
	"heat-backend/internal/app/repository"
	"heat-backend/internal/app/storage"
)

// Авторизации в этой лабораторной ещё нет, поэтому все действия выполняются
// от имени одного пользователя справочника.
const currentUserID uint = 1

type FuelHandler struct {
	repository *repository.FuelRepository
	media      *storage.MediaResolver
	logger     *logrus.Logger
}

func NewFuelHandler(
	fuelRepository *repository.FuelRepository,
	media *storage.MediaResolver,
	logger *logrus.Logger,
) *FuelHandler {
	return &FuelHandler{
		repository: fuelRepository,
		media:      media,
		logger:     logger,
	}
}

// draftForm хранит то, что введено в форму публикации черновика: при ошибке
// валидации страница перерисовывается с уже набранными значениями.
type draftForm struct {
	FuelName string
	Note     string
	Heat     string
	Ignition string
}

// toCardView превращает запись топлива в данные для шаблона: подставляет
// медиа по умолчанию и добавляет число лайков из таблицы связей.
func (h *FuelHandler) toCardView(fuel models.Fuel, likesCount int) models.HeatCardView {
	return models.HeatCardView{
		FuelID:             fuel.FuelID,
		FuelName:           fuel.FuelName,
		CombustionNote:     fuel.CombustionNote,
		HeatOfCombustionKJ: fuel.HeatOfCombustionKJ,
		IgnitionTempC:      fuel.IgnitionTempC,
		ImageURL:           h.media.ImageURL(fuel.ImageURL),
		VideoURL:           h.media.VideoURL(fuel.VideoURL),
		FuelStatus:         fuel.FuelStatus,
		LikesCount:         likesCount,
	}
}

// GetFuelFeed — GET /fuel_feed и GET /fuel_feed/:fuel_id
//
// Лента горения в вертикальной развёртке. Параметр ?next=true открывает
// следующую карточку после указанной. Удалённые карточки и черновики
// в ленту не попадают.
func (h *FuelHandler) GetFuelFeed(ctx *gin.Context) {
	fuelIDParam := ctx.Param("fuel_id")
	nextParam := ctx.Query("next")

	var (
		fuel models.Fuel
		err  error
	)

	switch {
	case fuelIDParam == "":
		fuel, err = h.repository.GetFirstPublishedFuel()
	default:
		fuelID, convErr := strconv.ParseUint(fuelIDParam, 10, 64)
		if convErr != nil {
			h.logger.Warnf("некорректный идентификатор топлива в url: %q", fuelIDParam)
			h.renderFeedError(ctx, http.StatusBadRequest, "Некорректный идентификатор вида топлива")
			return
		}
		if nextParam == "true" {
			fuel, err = h.repository.GetNextPublishedFuel(uint(fuelID))
		} else {
			fuel, err = h.repository.GetPublishedFuelByID(uint(fuelID))
		}
	}

	if err != nil {
		h.logger.Warnf("лента горения: %v (id=%q, next=%q)", err, fuelIDParam, nextParam)
		h.renderFeedError(ctx, http.StatusNotFound, "Такого вида топлива нет в справочнике")
		return
	}

	likesCount, err := h.repository.LikesCount(fuel.FuelID)
	if err != nil {
		h.logger.Errorf("подсчёт лайков карточки %d: %v", fuel.FuelID, err)
	}

	ctx.HTML(http.StatusOK, "fuel_feed.html", gin.H{
		"PageTitle": "Лента горения",
		"ActiveTab": "feed",
		"HeatCard":  h.toCardView(fuel, likesCount),
		"HasCard":   true,
	})
}

func (h *FuelHandler) renderFeedError(ctx *gin.Context, status int, message string) {
	ctx.HTML(status, "fuel_feed.html", gin.H{
		"PageTitle":    "Лента горения",
		"ActiveTab":    "feed",
		"HasCard":      false,
		"FeedErrorMsg": message,
	})
}

// GetFuelDraft — GET /fuel_draft
//
// Страница добавления. Если у пользователя нет черновика, показывается форма
// создания с кнопкой «Далее». Если черновик есть — его поля и кнопка
// «Опубликовать».
func (h *FuelHandler) GetFuelDraft(ctx *gin.Context) {
	draft, err := h.repository.GetDraftByCreator(currentUserID)
	if err != nil {
		h.renderDraft(ctx, http.StatusOK, nil, draftForm{}, "")
		return
	}

	form := draftForm{
		FuelName: draft.FuelName,
		Note:     draft.CombustionNote,
		Heat:     positiveOrEmpty(draft.HeatOfCombustionKJ),
		Ignition: positiveOrEmpty(draft.IgnitionTempC),
	}
	h.renderDraft(ctx, http.StatusOK, &draft, form, "")
}

func (h *FuelHandler) renderDraft(
	ctx *gin.Context,
	status int,
	draft *models.Fuel,
	form draftForm,
	errorMsg string,
) {
	page := gin.H{
		"PageTitle":     "Добавление топлива",
		"ActiveTab":     "draft",
		"HasDraft":      draft != nil,
		"Form":          form,
		"DraftErrorMsg": errorMsg,
	}

	if draft != nil {
		likesCount, err := h.repository.LikesCount(draft.FuelID)
		if err != nil {
			h.logger.Errorf("подсчёт лайков черновика %d: %v", draft.FuelID, err)
		}
		page["HeatCard"] = h.toCardView(*draft, likesCount)
	} else {
		// Для новой карточки сразу показываем медиа по умолчанию.
		page["DefaultImageURL"] = storage.DefaultImagePath
		page["DefaultVideoURL"] = storage.DefaultVideoPath
	}

	ctx.HTML(status, "fuel_draft.html", page)
}

// GetFuelGrid — GET /fuel_grid
//
// Плитка карточек с фильтром по теплоте сгорания.
func (h *FuelHandler) GetFuelGrid(ctx *gin.Context) {
	minHeatQuery := ctx.Query("min_heat")

	minHeatKJ := 0
	filterErrorMsg := ""
	if minHeatQuery != "" {
		parsed, err := strconv.Atoi(minHeatQuery)
		switch {
		case err != nil:
			filterErrorMsg = "Теплота сгорания задаётся целым числом в кДж/м³"
		case parsed < 0:
			filterErrorMsg = "Теплота сгорания не может быть отрицательной"
		default:
			minHeatKJ = parsed
		}
	}

	fuels, err := h.repository.GetPublishedFuels(minHeatKJ)
	if err != nil {
		h.logger.Errorf("плитка топлив: %v", err)
		ctx.HTML(http.StatusInternalServerError, "fuel_grid.html", gin.H{
			"PageTitle":      "Виды топлива",
			"ActiveTab":      "grid",
			"MinHeatFilter":  minHeatQuery,
			"FilterErrorMsg": "Не удалось получить список топлив из базы данных",
		})
		return
	}

	fuelIDs := make([]uint, 0, len(fuels))
	for _, fuel := range fuels {
		fuelIDs = append(fuelIDs, fuel.FuelID)
	}
	likesByFuel, err := h.repository.LikesCountByFuel(fuelIDs)
	if err != nil {
		h.logger.Errorf("подсчёт лайков для плитки: %v", err)
	}

	cards := make([]models.HeatCardView, 0, len(fuels))
	for _, fuel := range fuels {
		cards = append(cards, h.toCardView(fuel, likesByFuel[fuel.FuelID]))
	}

	h.logger.Infof("плитка топлив: min_heat=%q, найдено карточек: %d", minHeatQuery, len(cards))

	ctx.HTML(http.StatusOK, "fuel_grid.html", gin.H{
		"PageTitle":      "Виды топлива",
		"ActiveTab":      "grid",
		"HeatCards":      cards,
		"MinHeatFilter":  minHeatQuery,
		"FilterErrorMsg": filterErrorMsg,
	})
}

// CreateFuelDraft — POST /fuel_draft/create
//
// Кнопка «Далее» на странице добавления: создаёт карточку в статусе
// «черновик» через ORM. Фото и видео в этой лабораторной не сохраняются,
// поэтому новая карточка получает медиа по умолчанию.
func (h *FuelHandler) CreateFuelDraft(ctx *gin.Context) {
	fuelName := strings.TrimSpace(ctx.PostForm("fuel_name"))

	if fuelName == "" {
		h.renderDraft(ctx, http.StatusBadRequest, nil, draftForm{}, "Укажите название вида топлива")
		return
	}

	if _, err := h.repository.GetDraftByCreator(currentUserID); err == nil {
		ctx.Redirect(http.StatusSeeOther, "/fuel_draft")
		return
	}

	draft, err := h.repository.CreateDraft(currentUserID, fuelName)
	if err != nil {
		h.logger.Errorf("создание черновика: %v", err)
		h.renderDraft(ctx, http.StatusInternalServerError, nil, draftForm{FuelName: fuelName},
			"Не удалось создать черновик карточки")
		return
	}

	h.logger.Infof("создан черновик карточки %d пользователем %d", draft.FuelID, currentUserID)
	ctx.Redirect(http.StatusSeeOther, "/fuel_draft")
}

// PublishFuelDraft — POST /fuel_draft/publish
//
// Кнопка «Опубликовать»: заполняет краткое описание и оба поля по теме,
// переводит карточку в статус «опубликован» через ORM.
func (h *FuelHandler) PublishFuelDraft(ctx *gin.Context) {
	draft, err := h.repository.GetDraftByCreator(currentUserID)
	if err != nil {
		ctx.Redirect(http.StatusSeeOther, "/fuel_draft")
		return
	}

	form := draftForm{
		FuelName: draft.FuelName,
		Note:     strings.TrimSpace(ctx.PostForm("combustion_note")),
		Heat:     strings.TrimSpace(ctx.PostForm("heat_of_combustion_kj")),
		Ignition: strings.TrimSpace(ctx.PostForm("ignition_temp_c")),
	}

	heatOfCombustionKJ, heatErr := strconv.Atoi(form.Heat)
	ignitionTempC, ignitionErr := strconv.Atoi(form.Ignition)

	switch {
	case form.Note == "":
		h.renderDraft(ctx, http.StatusBadRequest, &draft, form, "Заполните краткое описание реакции горения")
		return
	case heatErr != nil || heatOfCombustionKJ <= 0:
		h.renderDraft(ctx, http.StatusBadRequest, &draft, form, "Теплота сгорания — целое число больше нуля")
		return
	case ignitionErr != nil || ignitionTempC <= 0:
		h.renderDraft(ctx, http.StatusBadRequest, &draft, form, "Температура воспламенения — целое число больше нуля")
		return
	}

	if err := h.repository.PublishDraft(draft.FuelID, form.Note, heatOfCombustionKJ, ignitionTempC); err != nil {
		h.logger.Errorf("публикация черновика %d: %v", draft.FuelID, err)
		h.renderDraft(ctx, http.StatusInternalServerError, &draft, form, "Не удалось опубликовать карточку")
		return
	}

	h.logger.Infof("карточка %d опубликована пользователем %d", draft.FuelID, currentUserID)
	ctx.Redirect(http.StatusSeeOther, "/fuel_grid")
}

// DeleteFuel — POST /fuel_grid/delete/:fuel_id
//
// Логическое удаление карточки: статус меняется на «удален» запросом
// SQL UPDATE напрямую, без ORM.
func (h *FuelHandler) DeleteFuel(ctx *gin.Context) {
	fuelID, err := strconv.ParseUint(ctx.Param("fuel_id"), 10, 64)
	if err != nil {
		h.logger.Warnf("удаление карточки: некорректный идентификатор %q", ctx.Param("fuel_id"))
		ctx.Redirect(http.StatusSeeOther, "/fuel_grid")
		return
	}

	if err := h.repository.SoftDeleteFuel(uint(fuelID)); err != nil {
		h.logger.Errorf("логическое удаление карточки %d: %v", fuelID, err)
	} else {
		h.logger.Infof("карточка %d переведена в статус «удален»", fuelID)
	}

	// Фильтр поиска сохраняется после удаления.
	redirectURL := "/fuel_grid"
	if minHeat := ctx.PostForm("min_heat"); minHeat != "" {
		redirectURL += "?min_heat=" + url.QueryEscape(minHeat)
	}
	ctx.Redirect(http.StatusSeeOther, redirectURL)
}

func positiveOrEmpty(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}
