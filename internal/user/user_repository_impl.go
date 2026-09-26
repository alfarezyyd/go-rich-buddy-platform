package user

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type RepositoryImpl struct{}

func NewRepository() *RepositoryImpl {
	return &RepositoryImpl{}
}

func (userRepositoryImpl *RepositoryImpl) FindAll(gormTransaction *gorm.DB) ([]*entity.User, error) {
	var userEntities []*entity.User
	err := gormTransaction.Find(&userEntities).Error
	return userEntities, err
}

func (userRepositoryImpl *RepositoryImpl) FindAllPagination(
	gormTransaction *gorm.DB,
	orderClause string,
	offsetVal, limitPage int,
	searchQuery string,
) ([]*entity.User, int64, error) {

	var userEntities []*entity.User
	var totalItems int64

	rawQuery := gormTransaction.Model(&entity.User{})

	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		rawQuery = rawQuery.Where("username ILIKE ? OR name ILIKE ? OR email ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	if err := rawQuery.Count(&totalItems).Error; err != nil {
		return nil, 0, err
	}

	if err := rawQuery.
		Order(orderClause).
		Offset(offsetVal).
		Limit(limitPage).
		Where("deleted_at IS NULL").
		Find(&userEntities).Error; err != nil {
		return nil, 0, err
	}

	return userEntities, totalItems, nil
}

func (userRepositoryImpl *RepositoryImpl) FindById(gormTransaction *gorm.DB, userId uint64) (*entity.User, error) {
	var userEntity entity.User
	err := gormTransaction.Model(&entity.User{}).
		Preload("AuditLog", func(gormTransaction *gorm.DB) *gorm.DB {
			return gormTransaction.Where("user_id = ?", userId).Limit(10)
		}).
		Where("id = ?", userId).Find(&userEntity).Error

	return &userEntity, err
}

func (userRepositoryImpl *RepositoryImpl) FindByIdentifier(gormTransaction *gorm.DB, userIdentifier string) (*entity.User, error) {
	var userEntity entity.User
	err := gormTransaction.
		Where("email = ?", userIdentifier).
		Or("username = ?", userIdentifier).
		First(&userEntity).Error
	return &userEntity, err
}

func (userRepositoryImpl *RepositoryImpl) FindByName(currencyName string) *entity.User {
	//TODO implement me
	panic("implement me")
}

func (userRepositoryImpl *RepositoryImpl) Create(gormTransaction *gorm.DB, currencyEntity *entity.User) error {
	return gormTransaction.Model(currencyEntity).Create(currencyEntity).Error

}

func (userRepositoryImpl *RepositoryImpl) Update(gormTransaction *gorm.DB, userEntity *entity.User) error {
	return gormTransaction.Model(userEntity).Save(userEntity).Error
}

func (userRepositoryImpl *RepositoryImpl) Delete(gormTransaction *gorm.DB, id uint64) error {
	return gormTransaction.Model(entity.User{}).Where("id = ?", id).Delete(&entity.User{}).Error
}
