package memory

import (
	"context"
	"fmt"
	"time"

	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/memory/assembler"
	"go-rich-buddy-platform/internal/memory/extractor"
	"go-rich-buddy-platform/internal/memory/privacy"
	"go-rich-buddy-platform/internal/memory/store"
	"go-rich-buddy-platform/internal/memory/summarizer"
	"go-rich-buddy-platform/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const maxMemoryJobRetries = 3

// ServiceImpl implements Service — coordinates all memory read/write operations.
type ServiceImpl struct {
	dbConnection *gorm.DB
	messageRepo  store.MessageRepository
	memoryRepo   store.MemoryRepository
	assembler    assembler.Assembler
	summarizer   *summarizer.Summarizer
	extractor    *extractor.Extractor
}

// NewService creates a new memory ServiceImpl and wires all sub-components.
func NewService(
	dbConnection *gorm.DB,
	agentClient client.AgentClient,
	extractorModel string,
	summarizerModel string,
) Service {
	messageRepo := store.NewMessageRepository()
	memoryRepo := store.NewMemoryRepository()
	contextAssembler := assembler.NewAssembler(messageRepo, memoryRepo)
	rollingSummarizer := summarizer.NewSummarizer(agentClient, messageRepo, summarizerModel)
	memoryExtractor := extractor.NewExtractor(agentClient, memoryRepo, messageRepo, extractorModel)

	return &ServiceImpl{
		dbConnection: dbConnection,
		messageRepo:  messageRepo,
		memoryRepo:   memoryRepo,
		assembler:    contextAssembler,
		summarizer:   rollingSummarizer,
		extractor:    memoryExtractor,
	}
}

// SaveIncomingMessage redacts PII, deduplicates by WA message ID, and persists the message.
func (memoryService *ServiceImpl) SaveIncomingMessage(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	waMessageID, content, role string,
) (*entity.ChatMessage, *entity.ChatSession, error) {
	// Deduplication: check wa_message_id uniqueness.
	if waMessageID != "" {
		existing, _ := memoryService.messageRepo.FindMessageByWaID(gormTransaction, waMessageID)
		if existing != nil {
			logrus.WithField("wa_message_id", waMessageID).Debug("duplicate message skipped")
			session, _ := memoryService.messageRepo.FindOrCreateActiveSession(gormTransaction, userID)
			return existing, session, nil
		}
	}

	// Redact PII before storing.
	redactedContent := privacy.RedactSensitiveData(content)
	wasRedacted := redactedContent != content

	// Find or create the active session.
	chatSession, err := memoryService.messageRepo.FindOrCreateActiveSession(gormTransaction, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find or create chat session: %w", err)
	}

	var waID *string
	if waMessageID != "" {
		waID = &waMessageID
	}

	chatMessage := &entity.ChatMessage{
		SessionID:   chatSession.ID,
		UserID:      userID,
		WaMessageID: waID,
		Role:        role,
		Content:     redactedContent,
		ContentType: "text",
		Tokens:      estimateTokens(redactedContent),
		Redacted:    wasRedacted,
		CreatedAt:   time.Now(),
	}

	if saveErr := memoryService.messageRepo.SaveMessage(gormTransaction, chatMessage); saveErr != nil {
		return nil, nil, fmt.Errorf("could not save chat message: %w", saveErr)
	}

	// Refresh session last_active_at.
	chatSession.LastActiveAt = time.Now()
	_ = memoryService.messageRepo.UpdateSessionSummary(gormTransaction, chatSession)

	return chatMessage, chatSession, nil
}

// BuildContext assembles the full LLM context from DB sources.
func (memoryService *ServiceImpl) BuildContext(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	incoming model.ChatMessage,
	budget assembler.Budget,
) (assembler.Assembled, error) {
	return memoryService.assembler.Build(ctx, gormTransaction, userID, incoming, budget)
}

// SaveAssistantMessage persists the LLM's reply to the chat history.
func (memoryService *ServiceImpl) SaveAssistantMessage(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	sessionID uint64,
	content string,
) error {
	chatMessage := &entity.ChatMessage{
		SessionID:   sessionID,
		UserID:      userID,
		Role:        "assistant",
		Content:     content,
		ContentType: "text",
		Tokens:      estimateTokens(content),
		CreatedAt:   time.Now(),
	}
	return memoryService.messageRepo.SaveMessage(gormTransaction, chatMessage)
}

// TriggerAsyncJobs fires off summarization and memory extraction as background goroutines.
// Per PRD §2 (tulis asinkron) and §6.2/§6.3, these must not block the reply path.
func (memoryService *ServiceImpl) TriggerAsyncJobs(
	ctx context.Context,
	userID uint64,
	session *entity.ChatSession,
	recentMessages []*entity.ChatMessage,
) {
	go func() {
		asyncCtx := context.Background()
		memoryService.dbConnection.WithContext(asyncCtx).Transaction(func(asyncTx *gorm.DB) error {
			memoryService.summarizer.MaybeSummarize(asyncCtx, asyncTx, session)
			return nil
		})
	}()

	go func() {
		asyncCtx := context.Background()
		memoryService.dbConnection.WithContext(asyncCtx).Transaction(func(asyncTx *gorm.DB) error {
			memoryService.extractor.ExtractAndSave(asyncCtx, asyncTx, userID, recentMessages)
			return nil
		})
	}()
}

// GetUserMemoryItems returns all active + pending_confirmation memory items for a user.
func (memoryService *ServiceImpl) GetUserMemoryItems(ctx context.Context, gormTransaction *gorm.DB, userID uint64) ([]*entity.UserMemoryItem, error) {
	return memoryService.memoryRepo.FindActiveMemoryItems(gormTransaction, userID)
}

// ForgetMemoryByKey finds and soft-deletes a memory item by its key.
func (memoryService *ServiceImpl) ForgetMemoryByKey(ctx context.Context, gormTransaction *gorm.DB, userID uint64, key string) error {
	item, err := memoryService.memoryRepo.FindMemoryItemByKey(gormTransaction, userID, key)
	if err != nil {
		return err
	}
	if item == nil {
		return nil // Already gone; idempotent.
	}

	if deleteErr := memoryService.memoryRepo.SoftDeleteMemoryItem(gormTransaction, item.ID, userID, "user"); deleteErr != nil {
		return deleteErr
	}

	beforeJSON := fmt.Sprintf(`{"key":%q,"value":%q}`, item.Key, item.Value)
	return memoryService.memoryRepo.AppendAuditLog(gormTransaction, &entity.MemoryAuditLog{
		UserID:     userID,
		Action:     "delete",
		ItemID:     &item.ID,
		Actor:      "user",
		BeforeJSON: &beforeJSON,
	})
}

// DeleteAllMemory purges all memory items, learning progress, and interaction events for a user.
func (memoryService *ServiceImpl) DeleteAllMemory(ctx context.Context, gormTransaction *gorm.DB, userID uint64) error {
	if err := memoryService.memoryRepo.PurgeAllMemoryItems(gormTransaction, userID, "user"); err != nil {
		return err
	}

	if err := gormTransaction.Exec("DELETE FROM user_learning_progress WHERE user_id = ?", userID).Error; err != nil {
		return err
	}

	if err := gormTransaction.Exec("DELETE FROM user_interaction_event WHERE user_id = ?", userID).Error; err != nil {
		return err
	}

	return memoryService.memoryRepo.AppendAuditLog(gormTransaction, &entity.MemoryAuditLog{
		UserID: userID,
		Action: "purge",
		Actor:  "user",
	})
}

// DeleteChatHistory deletes all raw chat messages and clears session summaries for a user.
func (memoryService *ServiceImpl) DeleteChatHistory(ctx context.Context, gormTransaction *gorm.DB, userID uint64) error {
	if err := gormTransaction.Exec("DELETE FROM chat_message WHERE user_id = ?", userID).Error; err != nil {
		return err
	}

	return gormTransaction.Exec(
		"UPDATE chat_session SET summary_text = NULL, summary_tokens = 0, summary_upto_message_id = NULL WHERE user_id = ?",
		userID,
	).Error
}

// UpdateLearningProgress increments the explained_count for a glossary term and marks it as
// understood if it has been explained ≥ 2 times without a follow-up question.
func (memoryService *ServiceImpl) UpdateLearningProgress(ctx context.Context, gormTransaction *gorm.DB, userID uint64, term string) error {
	progress, err := memoryService.memoryRepo.FindLearningProgress(gormTransaction, userID, term)
	if err != nil {
		return err
	}

	now := time.Now()
	if progress == nil {
		progress = &entity.UserLearningProgress{
			UserID:          userID,
			Term:            term,
			ExplainedCount:  1,
			LastExplainedAt: &now,
			Understood:      false,
		}
	} else {
		progress.ExplainedCount++
		progress.LastExplainedAt = &now
		if progress.ExplainedCount >= 2 {
			progress.Understood = true
		}
	}

	return memoryService.memoryRepo.UpsertLearningProgress(gormTransaction, progress)
}

// RecordInteractionEvent persists a user interaction event for analytics and personalization.
func (memoryService *ServiceImpl) RecordInteractionEvent(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	eventType, symbol, subSector, radarDate string,
) error {
	var radarDatePtr *string
	if radarDate != "" {
		radarDatePtr = &radarDate
	}
	event := &entity.UserInteractionEvent{
		UserID:    userID,
		Type:      eventType,
		Symbol:    symbol,
		SubSector: subSector,
		RadarDate: radarDatePtr,
	}
	return memoryService.memoryRepo.RecordInteractionEvent(gormTransaction, event)
}

// GetMessageRepository exposes the underlying message repository.
func (memoryService *ServiceImpl) GetMessageRepository() store.MessageRepository {
	return memoryService.messageRepo
}

// estimateTokens provides a rough token estimate: ~4 characters per token.
func estimateTokens(text string) int {
	return (len(text) + 3) / 4
}
