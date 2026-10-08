package store

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

// MessageRepository provides data-access methods for chat messages and sessions.
type MessageRepository interface {
	// Session operations
	FindOrCreateActiveSession(gormTransaction *gorm.DB, userID uint64) (*entity.ChatSession, error)
	UpdateSessionSummary(gormTransaction *gorm.DB, session *entity.ChatSession) error

	// Message operations
	SaveMessage(gormTransaction *gorm.DB, message *entity.ChatMessage) error
	FindMessageByWaID(gormTransaction *gorm.DB, waMessageID string) (*entity.ChatMessage, error)
	FindRecentMessages(gormTransaction *gorm.DB, sessionID uint64, limit int) ([]*entity.ChatMessage, error)
	FindUnsummarizedMessages(gormTransaction *gorm.DB, sessionID, afterMessageID uint64) ([]*entity.ChatMessage, error)

	// Conversation state
	UpsertConversationState(gormTransaction *gorm.DB, state *entity.ConversationState) error
	GetConversationState(gormTransaction *gorm.DB, userID uint64) (*entity.ConversationState, error)

	// Memory job (idempotency)
	CreateMemoryJobIfNotExists(gormTransaction *gorm.DB, job *entity.MemoryJob) error
	UpdateMemoryJobStatus(gormTransaction *gorm.DB, messageID uint64, status string) error
	IncrementMemoryJobAttempts(gormTransaction *gorm.DB, messageID uint64) error
}
