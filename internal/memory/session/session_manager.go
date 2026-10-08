package session

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/memory/store"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager handles conversation state updates after each turn.
type Manager struct {
	messageRepo store.MessageRepository
}

// NewManager creates a new session Manager.
func NewManager(messageRepo store.MessageRepository) *Manager {
	return &Manager{messageRepo: messageRepo}
}

// UpdateWorkingState persists the latest radar symbols and focus ticker to the conversation state.
// This enables reference resolution ("yang kedua tadi") per PRD §5.4.
func (sessionManager *Manager) UpdateWorkingState(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	lastRadarDate string,
	lastRadarSymbols []string,
	lastFocusSymbol string,
	pendingAction string,
) error {
	symbolsJSON, err := json.Marshal(lastRadarSymbols)
	if err != nil {
		return err
	}

	var radarDatePtr *string
	if lastRadarDate != "" {
		radarDatePtr = &lastRadarDate
	}

	symbolsStr := string(symbolsJSON)
	var symbolsPtr *string
	if symbolsStr != "null" && symbolsStr != "[]" && symbolsStr != "" {
		symbolsPtr = &symbolsStr
	}

	expiresAt := time.Now().Add(6 * time.Hour)
	state := &entity.ConversationState{
		UserID:           userID,
		LastRadarDate:    radarDatePtr,
		LastRadarSymbols: symbolsPtr,
		LastFocusSymbol:  lastFocusSymbol,
		PendingAction:    pendingAction,
		ExpiresAt:        &expiresAt,
	}

	return sessionManager.messageRepo.UpsertConversationState(gormTransaction, state)
}

// ResolveSymbolReference attempts to resolve natural language references like
// "yang kedua" (the second one) from the conversation working state.
// Returns the resolved ticker symbol or empty string if it cannot be resolved without LLM.
func (sessionManager *Manager) ResolveSymbolReference(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	inputBody string,
) string {
	state, err := sessionManager.messageRepo.GetConversationState(gormTransaction, userID)
	if err != nil || state == nil || state.LastRadarSymbols == nil || *state.LastRadarSymbols == "" {
		return ""
	}

	var symbols []string
	if unmarshalErr := json.Unmarshal([]byte(*state.LastRadarSymbols), &symbols); unmarshalErr != nil {
		return ""
	}

	normalized := strings.ToLower(strings.TrimSpace(inputBody))

	// Direct index resolution: "yang pertama", "yang kedua", "the first", "the second", etc.
	ordinalMap := map[string]int{
		"pertama": 1, "first": 1, "1": 1,
		"kedua": 2, "second": 2, "2": 2,
		"ketiga": 3, "third": 3, "3": 3,
		"keempat": 4, "fourth": 4, "4": 4,
		"kelima": 5, "fifth": 5, "5": 5,
	}

	for keyword, idx := range ordinalMap {
		if strings.Contains(normalized, "yang "+keyword) ||
			strings.Contains(normalized, "the "+keyword) ||
			strings.HasPrefix(normalized, keyword) {
			if idx > 0 && idx <= len(symbols) {
				logrus.WithFields(logrus.Fields{
					"user_id":  userID,
					"keyword":  keyword,
					"resolved": symbols[idx-1],
				}).Debug("reference resolved from conversation_state")
				return symbols[idx-1]
			}
		}
	}

	// If input is a single last_focus_symbol reference.
	if normalized == "itu" || normalized == "saham itu" || normalized == "that" {
		return state.LastFocusSymbol
	}

	return ""
}

// TouchSession updates the session's last_active_at to keep it alive.
func (sessionManager *Manager) TouchSession(gormTransaction *gorm.DB, session *entity.ChatSession) error {
	session.LastActiveAt = time.Now()
	return sessionManager.messageRepo.UpdateSessionSummary(gormTransaction, session)
}
