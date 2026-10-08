package order

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type RepositoryImpl struct{}

func NewRepository() Repository {
	return &RepositoryImpl{}
}

func (r *RepositoryImpl) FindAllActivePackages(db *gorm.DB) ([]*entity.Package, error) {
	var packages []*entity.Package
	err := db.Where("active = ?", true).Order("price ASC").Find(&packages).Error
	return packages, err
}

func (r *RepositoryImpl) FindPackageById(db *gorm.DB, packageId uint64) (*entity.Package, error) {
	var pkg entity.Package
	err := db.First(&pkg, "id = ?", packageId).Error
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (r *RepositoryImpl) FindPackageByCode(db *gorm.DB, code string) (*entity.Package, error) {
	var pkg entity.Package
	err := db.First(&pkg, "code = ?", code).Error
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (r *RepositoryImpl) CreateOrder(db *gorm.DB, order *entity.Order) error {
	return db.Create(order).Error
}

func (r *RepositoryImpl) UpdateOrder(db *gorm.DB, order *entity.Order) error {
	return db.Save(order).Error
}

func (r *RepositoryImpl) FindOrderByOrderId(db *gorm.DB, orderId string) (*entity.Order, error) {
	var ord entity.Order
	err := db.Preload("User").Preload("Package").First(&ord, "order_id = ?", orderId).Error
	if err != nil {
		return nil, err
	}
	return &ord, nil
}

func (r *RepositoryImpl) FindLatestPendingOrderByUserId(db *gorm.DB, userId uint64) (*entity.Order, error) {
	var ord entity.Order
	err := db.Preload("User").Preload("Package").Where("user_id = ? AND transaction_status = ?", userId, "pending").Order("created_at DESC").First(&ord).Error
	if err != nil {
		return nil, err
	}
	return &ord, nil
}
