package store

import (
	"errors"
	"time"

	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

// SessionInactivityGap defines the maximum gap between messages before a new session starts.
const SessionInactivityGap = 6 * time.Hour

// MessageRepositoryImpl implements MessageRepository using GORM.
type MessageRepositoryImpl struct{}

// NewMessageRepository creates a new MessageRepository.
func NewMessageRepository() MessageRepository {
	return &MessageRepositoryImpl{}
}

func (messageRepository *MessageRepositoryImpl) FindOrCreateActiveSession(gormTransaction *gorm.DB, userID uint64) (*entity.ChatSession, error) {
	var chatSession entity.ChatSession
	cutoff := time.Now().Add(-SessionInactivityGap)

	err := gormTransaction.
		Where("user_id = ? AND status = ? AND last_active_at >= ?", userID, "active", cutoff).
		Order("last_active_at DESC").
		First(&chatSession).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// No active session found — create a new one.
		chatSession = entity.ChatSession{
			UserID:    userID,
			Status:    "active",
			StartedAt: time.Now(),
		}
		if createErr := gormTransaction.Create(&chatSession).Error; createErr != nil {
			return nil, createErr
		}
	}

	return &chatSession, nil
}

func (messageRepository *MessageRepositoryImpl) UpdateSessionSummary(gormTransaction *gorm.DB, session *entity.ChatSession) error {
	return gormTransaction.Save(session).Error
}

func (messageRepository *MessageRepositoryImpl) SaveMessage(gormTransaction *gorm.DB, message *entity.ChatMessage) error {
	return gormTransaction.Create(message).Error
}

func (messageRepository *MessageRepositoryImpl) FindMessageByWaID(gormTransaction *gorm.DB, waMessageID string) (*entity.ChatMessage, error) {
	var chatMessage entity.ChatMessage
	err := gormTransaction.Where("wa_message_id = ?", waMessageID).First(&chatMessage).Error
	if err != nil {
		return nil, err
	}
	return &chatMessage, nil
}

func (messageRepository *MessageRepositoryImpl) FindRecentMessages(gormTransaction *gorm.DB, sessionID uint64, limit int) ([]*entity.ChatMessage, error) {
	var messages []*entity.ChatMessage
	err := gormTransaction.
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	// Reverse to chronological order.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (messageRepository *MessageRepositoryImpl) FindUnsummarizedMessages(gormTransaction *gorm.DB, sessionID, afterMessageID uint64) ([]*entity.ChatMessage, error) {
	var messages []*entity.ChatMessage
	query := gormTransaction.Where("session_id = ?", sessionID)
	if afterMessageID > 0 {
		query = query.Where("id > ?", afterMessageID)
	}
	err := query.Order("created_at ASC").Find(&messages).Error
	return messages, err
}

func (messageRepository *MessageRepositoryImpl) UpsertConversationState(gormTransaction *gorm.DB, state *entity.ConversationState) error {
	return gormTransaction.Save(state).Error
}

func (messageRepository *MessageRepositoryImpl) GetConversationState(gormTransaction *gorm.DB, userID uint64) (*entity.ConversationState, error) {
	var state entity.ConversationState
	err := gormTransaction.Where("user_id = ?", userID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entity.ConversationState{UserID: userID}, nil
		}
		return nil, err
	}
	return &state, nil
}

func (messageRepository *MessageRepositoryImpl) CreateMemoryJobIfNotExists(gormTransaction *gorm.DB, job *entity.MemoryJob) error {
	result := gormTransaction.
		Where(entity.MemoryJob{MessageID: job.MessageID, Kind: job.Kind}).
		FirstOrCreate(job)
	return result.Error
}

func (messageRepository *MessageRepositoryImpl) UpdateMemoryJobStatus(gormTransaction *gorm.DB, messageID uint64, status string) error {
	return gormTransaction.
		Model(&entity.MemoryJob{}).
		Where("message_id = ?", messageID).
		Update("status", status).Error
}

func (messageRepository *MessageRepositoryImpl) IncrementMemoryJobAttempts(gormTransaction *gorm.DB, messageID uint64) error {
	return gormTransaction.
		Model(&entity.MemoryJob{}).
		Where("message_id = ?", messageID).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
}
