package serializers

import (
	"time"

	"heat-backend/internal/app/models"
)

type PublishFuelRequest struct {
	CombustionNote     *string `json:"combustion_note" example:"Этилен сгорает коптящим светящимся пламенем."`
	HeatOfCombustionKJ *int    `json:"heat_of_combustion_kj" example:"59000"`
	IgnitionTempC      *int    `json:"ignition_temp_c" example:"450"`
}

type LikeRequest struct {
	Like *int `json:"like" enums:"0,1" example:"1"`
}

type FuelListItem struct {
	FuelID             uint   `json:"fuel_id"`
	FuelName           string `json:"fuel_name"`
	HeatOfCombustionKJ int    `json:"heat_of_combustion_kj"`
	IgnitionTempC      int    `json:"ignition_temp_c"`
	ImageURL           string `json:"image_url"`
	LikesCount         int    `json:"likes_count"`
	// IsMine — карточку создал текущий пользователь: в плитке по нему
	// показывается кнопка удаления
	IsMine bool `json:"is_mine"`
}

type FuelDetail struct {
	FuelID         uint              `json:"fuel_id"`
	FuelName       string            `json:"fuel_name"`
	CombustionNote string            `json:"combustion_note"`
	FuelStatus     models.FuelStatus `json:"fuel_status"`

	HeatOfCombustionKJ int `json:"heat_of_combustion_kj"`
	IgnitionTempC      int `json:"ignition_temp_c"`

	ImageURL string `json:"image_url"`
	VideoURL string `json:"video_url"`

	CreatedAt time.Time  `json:"created_at"`
	FormedAt  *time.Time `json:"formed_at"`
	// CreatorLogin — логин автора плоским полем: вложенный объект пользователя
	// клиенту не нужен
	CreatorLogin string `json:"creator_login"`

	LikesCount int `json:"likes_count"`
	// IsLiked — текущий пользователь лайкнул карточку: в ленте по нему
	// закрашивается сердечко
	IsLiked bool `json:"is_liked"`
}

// LikeResponse — состояние лайка после POST /api/fuels/:fuel_id/like.
type LikeResponse struct {
	FuelID     uint `json:"fuel_id"`
	IsLiked    bool `json:"is_liked"`
	LikesCount int  `json:"likes_count"`
}

// MediaURL приводит url медиа к виду, пригодному для клиента.
type MediaURL func(raw string) string

func NewFuelListItem(fuel models.Fuel, likesCount int, isMine bool, media MediaURL) FuelListItem {
	return FuelListItem{
		FuelID:             fuel.FuelID,
		FuelName:           fuel.FuelName,
		HeatOfCombustionKJ: fuel.HeatOfCombustionKJ,
		IgnitionTempC:      fuel.IgnitionTempC,
		ImageURL:           media(fuel.ImageURL),
		LikesCount:         likesCount,
		IsMine:             isMine,
	}
}

// NewFuelDetail собирает полную карточку. fuel.Creator должен быть загружен (Preload).
func NewFuelDetail(fuel models.Fuel, likesCount int, liked bool, media MediaURL) FuelDetail {
	return FuelDetail{
		FuelID:             fuel.FuelID,
		FuelName:           fuel.FuelName,
		CombustionNote:     fuel.CombustionNote,
		FuelStatus:         fuel.FuelStatus,
		HeatOfCombustionKJ: fuel.HeatOfCombustionKJ,
		IgnitionTempC:      fuel.IgnitionTempC,
		ImageURL:           media(fuel.ImageURL),
		VideoURL:           media(fuel.VideoURL),
		CreatedAt:          fuel.CreatedAt,
		FormedAt:           fuel.FormedAt,
		CreatorLogin:       fuel.Creator.Login,
		LikesCount:         likesCount,
		IsLiked:            liked,
	}
}

// DeleteFuelResponse — ответ DELETE /api/fuels/:fuel_id: удалённая карточка
// клиенту не отдаётся, только её ид.
type DeleteFuelResponse struct {
	FuelID  uint   `json:"fuel_id" example:"7"`
	Message string `json:"message" example:"Карточка переведена в статус «удален»"`
}

// ErrorResponse — единый формат ошибки веб-сервиса.
type ErrorResponse struct {
	Error string `json:"error" example:"Текст ошибки"`
}
