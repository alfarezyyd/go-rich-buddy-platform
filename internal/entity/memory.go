package entity

import (
	"time"
)

// ChatSession represents a conversation session split by inactivity or day boundaries.
type ChatSession struct {
	ID                    uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID                uint64     `gorm:"column:user_id;index"`
	StartedAt             time.Time  `gorm:"column:started_at;autoCreateTime"`
	LastActiveAt          time.Time  `gorm:"column:last_active_at;autoUpdateTime"`
	Status                string     `gorm:"column:status;type:varchar(20);default:'active'"`
	SummaryText           string     `gorm:"column:summary_text;type:text"`
	SummaryUptoMessageID  *uint64    `gorm:"column:summary_upto_message_id"`
	SummaryTokens         int        `gorm:"column:summary_tokens;default:0"`
}

func (ChatSession) TableName() string {
	return "chat_session"
}

// SummaryUptoMessageIDValue safely dereferences the nullable SummaryUptoMessageID pointer.
func (chatSession *ChatSession) SummaryUptoMessageIDValue() uint64 {
	if chatSession.SummaryUptoMessageID == nil {
		return 0
	}
	return *chatSession.SummaryUptoMessageID
}


// ChatMessage represents a single conversation turn stored in the database.
type ChatMessage struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	SessionID   uint64    `gorm:"column:session_id;index"`
	UserID      uint64    `gorm:"column:user_id;index"`
	WaMessageID string    `gorm:"column:wa_message_id;uniqueIndex;type:varchar(255)"`
	Role        string    `gorm:"column:role;type:varchar(20)"`
	Content     string    `gorm:"column:content;type:text"`
	ContentType string    `gorm:"column:content_type;type:varchar(50);default:'text'"`
	Tokens      int       `gorm:"column:tokens;default:0"`
	RefsJSON    string    `gorm:"column:refs_json;type:jsonb"`
	Redacted    bool      `gorm:"column:redacted;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ChatMessage) TableName() string {
	return "chat_message"
}

// ConversationState holds the working state for a user (one row per user).
type ConversationState struct {
	UserID            uint64    `gorm:"column:user_id;primaryKey"`
	LastRadarDate     string    `gorm:"column:last_radar_date;type:date"`
	LastRadarSymbols  string    `gorm:"column:last_radar_symbols;type:jsonb"`
	LastFocusSymbol   string    `gorm:"column:last_focus_symbol;type:varchar(10)"`
	PendingAction     string    `gorm:"column:pending_action;type:varchar(100)"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
	ExpiresAt         *time.Time `gorm:"column:expires_at"`
}

func (ConversationState) TableName() string {
	return "conversation_state"
}

// UserMemoryItem represents a single long-term memory entry for a user.
type UserMemoryItem struct {
	ID                 uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	UserID             uint64         `gorm:"column:user_id;index"`
	Kind               string         `gorm:"column:kind;type:varchar(20)"`
	Key                string         `gorm:"column:key;type:varchar(100)"`
	Value              string         `gorm:"column:value;type:text"`
	Source             string         `gorm:"column:source;type:varchar(20);default:'stated'"`
	Status             string         `gorm:"column:status;type:varchar(30);default:'active'"`
	Confidence         float64        `gorm:"column:confidence;type:numeric(4,3);default:1.0"`
	Importance         float64        `gorm:"column:importance;type:numeric(4,3);default:0.5"`
	EvidenceMessageIDs string         `gorm:"column:evidence_message_ids;type:jsonb"` // JSON array of message IDs
	FirstSeenAt        time.Time      `gorm:"column:first_seen_at;autoCreateTime"`
	LastConfirmedAt    *time.Time     `gorm:"column:last_confirmed_at"`
	LastUsedAt         *time.Time     `gorm:"column:last_used_at"`
	ExpiresAt          *time.Time     `gorm:"column:expires_at"`
}

func (UserMemoryItem) TableName() string {
	return "user_memory_item"
}

// UserLearningProgress tracks how many times a glossary term has been explained.
type UserLearningProgress struct {
	UserID          uint64     `gorm:"column:user_id;primaryKey"`
	Term            string     `gorm:"column:term;primaryKey;type:varchar(200)"`
	ExplainedCount  int        `gorm:"column:explained_count;default:0"`
	LastExplainedAt *time.Time `gorm:"column:last_explained_at"`
	Understood      bool       `gorm:"column:understood;default:false"`
}

func (UserLearningProgress) TableName() string {
	return "user_learning_progress"
}

// UserInteractionEvent records significant user interactions for analytics and personalization.
type UserInteractionEvent struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"column:user_id;index"`
	Type       string    `gorm:"column:type;type:varchar(50)"`
	Symbol     string    `gorm:"column:symbol;type:varchar(10)"`
	SubSector  string    `gorm:"column:sub_sector;type:varchar(100)"`
	RadarDate  string    `gorm:"column:radar_date;type:date"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserInteractionEvent) TableName() string {
	return "user_interaction_event"
}

// MemoryAuditLog is an immutable audit trail for memory operations.
// Sensitive content is never stored here.
type MemoryAuditLog struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"column:user_id;index"`
	Action     string    `gorm:"column:action;type:varchar(20)"`
	ItemID     *uint64   `gorm:"column:item_id"`
	Actor      string    `gorm:"column:actor;type:varchar(20)"`
	BeforeJSON string    `gorm:"column:before_json;type:jsonb"`
	AfterJSON  string    `gorm:"column:after_json;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (MemoryAuditLog) TableName() string {
	return "memory_audit_log"
}

// MemoryJob is an idempotency marker for async summarize/extract jobs.
type MemoryJob struct {
	MessageID uint64    `gorm:"column:message_id;primaryKey"`
	Kind      string    `gorm:"column:kind;type:varchar(20)"`
	Status    string    `gorm:"column:status;type:varchar(20);default:'pending'"`
	Attempts  int       `gorm:"column:attempts;default:0"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (MemoryJob) TableName() string {
	return "memory_job"
}
