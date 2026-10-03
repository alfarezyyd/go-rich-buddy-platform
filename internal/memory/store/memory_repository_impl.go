package store

import (
	"errors"
	"time"

	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

// maxActiveMemoryItems is the per-user limit for active memory items per PRD §6.3.
const maxActiveMemoryItems = 50

// MemoryRepositoryImpl implements MemoryRepository using GORM.
type MemoryRepositoryImpl struct{}

// NewMemoryRepository creates a new MemoryRepository.
func NewMemoryRepository() MemoryRepository {
	return &MemoryRepositoryImpl{}
}

func (memoryRepository *MemoryRepositoryImpl) FindActiveMemoryItems(gormTransaction *gorm.DB, userID uint64) ([]*entity.UserMemoryItem, error) {
	var items []*entity.UserMemoryItem
	err := gormTransaction.
		Where("user_id = ? AND status IN ?", userID, []string{"active", "pending_confirmation"}).
		Order("importance * (EXTRACT(EPOCH FROM (NOW() - first_seen_at)) / 86400.0 + 1) DESC").
		Limit(12).
		Find(&items).Error
	return items, err
}

func (memoryRepository *MemoryRepositoryImpl) FindMemoryItemByKey(gormTransaction *gorm.DB, userID uint64, key string) (*entity.UserMemoryItem, error) {
	var item entity.UserMemoryItem
	err := gormTransaction.
		Where("user_id = ? AND key = ? AND status IN ?", userID, key, []string{"active", "pending_confirmation"}).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (memoryRepository *MemoryRepositoryImpl) UpsertMemoryItem(gormTransaction *gorm.DB, item *entity.UserMemoryItem) error {
	existing, err := memoryRepository.FindMemoryItemByKey(gormTransaction, item.UserID, item.Key)
	if err != nil {
		return err
	}

	if existing != nil {
		// Apply conflict resolution: stated always overwrites; stated beats inferred (PRD §6.3 rule 6).
		if existing.Source == "stated" && item.Source == "inferred" {
			// Inferred must not overwrite stated.
			return nil
		}
		existing.Value = item.Value
		existing.Source = item.Source
		existing.Status = item.Status
		existing.Confidence = item.Confidence
		existing.Importance = item.Importance
		existing.EvidenceMessageIDs = item.EvidenceMessageIDs
		now := time.Now()
		existing.LastConfirmedAt = &now
		return gormTransaction.Save(existing).Error
	}

	return gormTransaction.Create(item).Error
}

func (memoryRepository *MemoryRepositoryImpl) SoftDeleteMemoryItem(gormTransaction *gorm.DB, itemID, userID uint64, actor string) error {
	return gormTransaction.
		Model(&entity.UserMemoryItem{}).
		Where("id = ? AND user_id = ?", itemID, userID).
		Update("status", "deleted").Error
}

func (memoryRepository *MemoryRepositoryImpl) PurgeAllMemoryItems(gormTransaction *gorm.DB, userID uint64, actor string) error {
	return gormTransaction.
		Model(&entity.UserMemoryItem{}).
		Where("user_id = ? AND status != ?", userID, "deleted").
		Update("status", "deleted").Error
}

func (memoryRepository *MemoryRepositoryImpl) CountActiveMemoryItems(gormTransaction *gorm.DB, userID uint64) (int64, error) {
	var count int64
	err := gormTransaction.
		Model(&entity.UserMemoryItem{}).
		Where("user_id = ? AND status = ?", userID, "active").
		Count(&count).Error
	return count, err
}

// EvictLowImportanceMemoryItem removes the active item with the lowest importance × recency score
// to make room for a new item (PRD §6.3 rule 7 — 50-item limit).
func (memoryRepository *MemoryRepositoryImpl) EvictLowImportanceMemoryItem(gormTransaction *gorm.DB, userID uint64) error {
	var item entity.UserMemoryItem
	err := gormTransaction.
		Where("user_id = ? AND status = ?", userID, "active").
		Order("importance ASC, last_used_at ASC NULLS FIRST").
		First(&item).Error
	if err != nil {
		return err
	}
	return gormTransaction.
		Model(&entity.UserMemoryItem{}).
		Where("id = ?", item.ID).
		Update("status", "deleted").Error
}

func (memoryRepository *MemoryRepositoryImpl) FindLearningProgress(gormTransaction *gorm.DB, userID uint64, term string) (*entity.UserLearningProgress, error) {
	var progress entity.UserLearningProgress
	err := gormTransaction.
		Where("user_id = ? AND term = ?", userID, term).
		First(&progress).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &progress, nil
}

func (memoryRepository *MemoryRepositoryImpl) UpsertLearningProgress(gormTransaction *gorm.DB, progress *entity.UserLearningProgress) error {
	existing, err := memoryRepository.FindLearningProgress(gormTransaction, progress.UserID, progress.Term)
	if err != nil {
		return err
	}
	if existing != nil {
		existing.ExplainedCount = progress.ExplainedCount
		existing.LastExplainedAt = progress.LastExplainedAt
		existing.Understood = progress.Understood
		return gormTransaction.Save(existing).Error
	}
	return gormTransaction.Create(progress).Error
}

func (memoryRepository *MemoryRepositoryImpl) RecordInteractionEvent(gormTransaction *gorm.DB, event *entity.UserInteractionEvent) error {
	return gormTransaction.Create(event).Error
}

func (memoryRepository *MemoryRepositoryImpl) AppendAuditLog(gormTransaction *gorm.DB, log *entity.MemoryAuditLog) error {
	return gormTransaction.Create(log).Error
}
