package order

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type Repository interface {
	FindAllActivePackages(db *gorm.DB) ([]*entity.Package, error)
	FindPackageById(db *gorm.DB, packageId uint64) (*entity.Package, error)
	FindPackageByCode(db *gorm.DB, code string) (*entity.Package, error)

	CreateOrder(db *gorm.DB, order *entity.Order) error
	UpdateOrder(db *gorm.DB, order *entity.Order) error
	FindOrderByOrderId(db *gorm.DB, orderId string) (*entity.Order, error)
	FindLatestPendingOrderByUserId(db *gorm.DB, userId uint64) (*entity.Order, error)
}
