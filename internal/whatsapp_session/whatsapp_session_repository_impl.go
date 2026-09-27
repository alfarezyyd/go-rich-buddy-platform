package whatsapp_session

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RepositoryImpl struct {
}

func NewRepository() *RepositoryImpl {
	return &RepositoryImpl{}
}

func (sessionRepository *RepositoryImpl) FindByPhone(gormTransaction *gorm.DB, phone string) (*entity.WhatsappSession, error) {
	var session entity.WhatsappSession
	err := gormTransaction.Where("phone = ?", phone).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (sessionRepository *RepositoryImpl) Upsert(gormTransaction *gorm.DB, sessionEntity *entity.WhatsappSession) error {
	return gormTransaction.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "phone"}},
		DoUpdates: clause.AssignmentColumns([]string{"current_state", "retry_count", "updated_at", "updated_by"}),
	}).Create(sessionEntity).Error
}

func (sessionRepository *RepositoryImpl) Delete(gormTransaction *gorm.DB, phoneNumber string) error {
	return gormTransaction.Where("phone = ?", phoneNumber).Delete(&entity.WhatsappSession{}).Error
}
