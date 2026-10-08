package entity

import "time"

type Package struct {
	Id          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Code        string    `gorm:"column:code;unique;not null"`
	Name        string    `gorm:"column:name;not null"`
	Tier        string    `gorm:"column:tier;not null"`
	Credits     int       `gorm:"column:credits;not null"`
	Price       int64     `gorm:"column:price;not null"`
	Description string    `gorm:"column:description"`
	Active      bool      `gorm:"column:active;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Package) TableName() string {
	return "packages"
}
