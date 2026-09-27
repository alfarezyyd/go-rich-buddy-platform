package whatsapp_session

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type SessionRepository interface {
	FindByPhone(gormTransaction *gorm.DB, phone string) (*entity.WhatsappSession, error)
	Upsert(gormTransaction *gorm.DB, sessionEntity *entity.WhatsappSession) error
	Delete(gormTransaction *gorm.DB, phone string) error
}
