package user

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type Repository interface {
	FindAll(gormTransaction *gorm.DB) ([]*entity.User, error)
	FindAllPagination(gormTransaction *gorm.DB, orderClause string, offsetVal, limitPage int, searchQuery string) ([]*entity.User, int64, error)
	FindById(gormTransaction *gorm.DB, userId uint64) (*entity.User, error)
	FindByIdentifier(gormTransaction *gorm.DB, userIdentifier string) (*entity.User, error)
	FindByName(userName string) *entity.User
	Create(gormTransaction *gorm.DB, userEntity *entity.User) error
	Update(gormTransaction *gorm.DB, userEntity *entity.User) error
	Delete(gormTransaction *gorm.DB, userId uint64) error
}
