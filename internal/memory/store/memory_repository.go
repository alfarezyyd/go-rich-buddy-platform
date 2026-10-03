package store

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

// MemoryRepository provides data-access methods for user memory items, learning progress,
// interaction events, and audit log entries.
type MemoryRepository interface {
	// Memory items
	FindActiveMemoryItems(gormTransaction *gorm.DB, userID uint64) ([]*entity.UserMemoryItem, error)
	FindMemoryItemByKey(gormTransaction *gorm.DB, userID uint64, key string) (*entity.UserMemoryItem, error)
	UpsertMemoryItem(gormTransaction *gorm.DB, item *entity.UserMemoryItem) error
	SoftDeleteMemoryItem(gormTransaction *gorm.DB, itemID, userID uint64, actor string) error
	PurgeAllMemoryItems(gormTransaction *gorm.DB, userID uint64, actor string) error
	CountActiveMemoryItems(gormTransaction *gorm.DB, userID uint64) (int64, error)
	EvictLowImportanceMemoryItem(gormTransaction *gorm.DB, userID uint64) error

	// Learning progress
	FindLearningProgress(gormTransaction *gorm.DB, userID uint64, term string) (*entity.UserLearningProgress, error)
	UpsertLearningProgress(gormTransaction *gorm.DB, progress *entity.UserLearningProgress) error

	// Interaction events
	RecordInteractionEvent(gormTransaction *gorm.DB, event *entity.UserInteractionEvent) error

	// Audit log
	AppendAuditLog(gormTransaction *gorm.DB, log *entity.MemoryAuditLog) error
}
