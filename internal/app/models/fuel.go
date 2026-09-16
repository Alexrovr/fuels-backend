package models

import "time"

type FuelStatus string

const (
	FuelStatusDraft     FuelStatus = "черновик"
	FuelStatusPublished FuelStatus = "опубликован"
	FuelStatusDeleted   FuelStatus = "удален"
)

// User — пользователь справочника: создаёт карточки топлив и ставит лайки.
type User struct {
	UserID   uint   `gorm:"primaryKey;column:user_id"`
	Login    string `gorm:"column:login;type:varchar(64);not null;uniqueIndex"`
	FullName string `gorm:"column:full_name;type:varchar(128);not null"`
}

func (User) TableName() string {
	return "users"
}

// Fuel — «услуга» предметной области: вид топлива в справочнике теплоты сгорания.
type Fuel struct {
	FuelID uint `gorm:"primaryKey;column:fuel_id"`

	FuelName       string     `gorm:"column:fuel_name;type:varchar(128);not null"`
	CombustionNote string     `gorm:"column:combustion_note;type:text"`
	FuelStatus     FuelStatus `gorm:"column:fuel_status;type:varchar(16);not null;index"`

	ImageURL string `gorm:"column:image_url;type:varchar(512)"`
	VideoURL string `gorm:"column:video_url;type:varchar(512)"`

	HeatOfCombustionKJ int `gorm:"column:heat_of_combustion_kj;index"`
	IgnitionTempC      int `gorm:"column:ignition_temp_c"`

	CreatedAt time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	FormedAt  *time.Time `gorm:"column:formed_at"`

	CreatorID uint `gorm:"column:creator_id;not null;index"`
	Creator   User `gorm:"foreignKey:CreatorID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (Fuel) TableName() string {
	return "fuels"
}

// FuelLike — связь «многие ко многим» между пользователями и видами топлива.
type FuelLike struct {
	LikeID uint `gorm:"primaryKey;column:like_id"`

	UserID uint `gorm:"column:user_id;not null;uniqueIndex:idx_fuel_likes_user_fuel"`
	FuelID uint `gorm:"column:fuel_id;not null;uniqueIndex:idx_fuel_likes_user_fuel"`

	User User `gorm:"constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	Fuel Fuel `gorm:"constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (FuelLike) TableName() string {
	return "fuel_likes"
}

// HeatCardView — данные одной карточки в том виде, в котором их ждут шаблоны.
type HeatCardView struct {
	FuelID uint

	FuelName       string
	CombustionNote string

	HeatOfCombustionKJ int
	IgnitionTempC      int

	ImageURL string
	VideoURL string

	FuelStatus FuelStatus
	LikesCount int
}
