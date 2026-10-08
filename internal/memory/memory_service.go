package memory

import (
	"context"

	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/memory/assembler"
	"go-rich-buddy-platform/internal/memory/store"
	"go-rich-buddy-platform/internal/model"

	"gorm.io/gorm"
)

// Service orchestrates all memory read and write operations for a single conversation turn.
type Service interface {
	// SaveIncomingMessage redacts PII, deduplicates, and persists the user message.
	// Returns the saved ChatMessage and the active session.
	SaveIncomingMessage(ctx context.Context, gormTransaction *gorm.DB, userID uint64, waMessageID, content, role string) (*entity.ChatMessage, *entity.ChatSession, error)

	// BuildContext assembles the LLM context for the current turn (reads from DB).
	BuildContext(ctx context.Context, gormTransaction *gorm.DB, userID uint64, incoming model.ChatMessage, budget assembler.Budget) (assembler.Assembled, error)

	// SaveAssistantMessage persists the assistant's reply.
	SaveAssistantMessage(ctx context.Context, gormTransaction *gorm.DB, userID uint64, sessionID uint64, content string) error

	// TriggerAsyncJobs launches summarization and memory extraction as background goroutines.
	// It must be called after the synchronous reply has been sent.
	TriggerAsyncJobs(ctx context.Context, userID uint64, session *entity.ChatSession, recentMessages []*entity.ChatMessage)

	// GetUserMemoryItems returns all active memory items for display ("memori saya" command).
	GetUserMemoryItems(ctx context.Context, gormTransaction *gorm.DB, userID uint64) ([]*entity.UserMemoryItem, error)

	// ForgetMemoryByKey soft-deletes a memory item by key.
	ForgetMemoryByKey(ctx context.Context, gormTransaction *gorm.DB, userID uint64, key string) error

	// DeleteAllMemory purges all memory items, learning progress, and interaction events.
	DeleteAllMemory(ctx context.Context, gormTransaction *gorm.DB, userID uint64) error

	// DeleteChatHistory deletes all chat messages and session summaries for a user.
	DeleteChatHistory(ctx context.Context, gormTransaction *gorm.DB, userID uint64) error

	// UpdateLearningProgress increments the explained_count for a glossary term.
	UpdateLearningProgress(ctx context.Context, gormTransaction *gorm.DB, userID uint64, term string) error

	// RecordInteractionEvent persists a user interaction event (radar view, drilldown, etc.).
	RecordInteractionEvent(ctx context.Context, gormTransaction *gorm.DB, userID uint64, eventType, symbol, subSector, radarDate string) error

	// GetMessageRepository exposes the message repository for use in context assembly.
	GetMessageRepository() store.MessageRepository
}
