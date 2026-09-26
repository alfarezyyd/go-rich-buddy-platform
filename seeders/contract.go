package seeders

import "gorm.io/gorm"

type Seeder interface {
	Run(db *gorm.DB) error
}
