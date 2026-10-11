package serializers

import (
	"time"

	"heat-backend/internal/app/models"
)

type PublishFuelRequest struct {
	CombustionNote     *string `json:"combustion_note"`
	HeatOfCombustionKJ *int    `json:"heat_of_combustion_kj"`
	IgnitionTempC      *int    `json:"ignition_temp_c"`
}

type LikeRequest struct {
	Like *int `json:"like"`
}

type FuelListItem struct {
	FuelID             uint   `json:"fuel_id"`
	FuelName           string `json:"fuel_name"`
	HeatOfCombustionKJ int    `json:"heat_of_combustion_kj"`
	IgnitionTempC      int    `json:"ignition_temp_c"`
	ImageURL           string `json:"image_url"`
	LikesCount         int    `json:"likes_count"`
	IsLiked            bool   `json:"is_liked"`
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
	// клиенту не нужен, а «своя ли карточка» передаёт IsMine
	CreatorLogin string `json:"creator_login"`

	LikesCount int  `json:"likes_count"`
	IsLiked    bool `json:"is_liked"`
	// IsMine — карточку создал текущий пользователь
	IsMine bool `json:"is_mine"`
}

// LikeResponse — состояние лайка после POST /api/fuels/:fuel_id/like.
type LikeResponse struct {
	FuelID     uint `json:"fuel_id"`
	IsLiked    bool `json:"is_liked"`
	LikesCount int  `json:"likes_count"`
}

// MediaURL приводит url медиа к виду, пригодному для клиента.
type MediaURL func(raw string) string

func NewFuelListItem(fuel models.Fuel, likesCount int, liked bool, media MediaURL) FuelListItem {
	return FuelListItem{
		FuelID:             fuel.FuelID,
		FuelName:           fuel.FuelName,
		HeatOfCombustionKJ: fuel.HeatOfCombustionKJ,
		IgnitionTempC:      fuel.IgnitionTempC,
		ImageURL:           media(fuel.ImageURL),
		LikesCount:         likesCount,
		IsLiked:            liked,
	}
}

// NewFuelDetail собирает полную карточку. fuel.Creator должен быть загружен (Preload).
func NewFuelDetail(fuel models.Fuel, likesCount int, liked, isMine bool, media MediaURL) FuelDetail {
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
		IsMine:             isMine,
	}
}
