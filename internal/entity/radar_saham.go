package entity

import (
	"time"
)

type SignalDaily struct {
	ID                       uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Date                     string    `gorm:"column:date;type:date;index"`
	Symbol                   string    `gorm:"column:symbol;type:varchar(10);index"`
	SubSector                string    `gorm:"column:sub_sector;type:varchar(100);index"`
	ForeignFlowScore         int       `gorm:"column:foreign_flow_score;default:0"`
	InstitutionalBrokerScore int       `gorm:"column:institutional_broker_score;default:0"`
	VolumeScore              int       `gorm:"column:volume_score;default:0"`
	MomentumScore            int       `gorm:"column:momentum_score;default:0"`
	BonusCorporateAction     int       `gorm:"column:bonus_corporate_action;default:0"`
	BonusQuarterlyReport     int       `gorm:"column:bonus_quarterly_report;default:0"`
	BonusInsiderBuy          int       `gorm:"column:bonus_insider_buy;default:0"`
	CompositeScore           int       `gorm:"column:composite_score;default:0"`
	IsShortlisted            bool      `gorm:"column:is_shortlisted;default:false"`
	IsEnriched               bool      `gorm:"column:is_enriched;default:false"`
	CreatedAt                time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (SignalDaily) TableName() string {
	return "signal_daily"
}

type TickerExplanationDaily struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Date         string    `gorm:"column:date;type:date;index"`
	Symbol       string    `gorm:"column:symbol;type:varchar(10);index"`
	SummaryReason string   `gorm:"column:summary_reason;type:text"`
	EvidenceJSON string    `gorm:"column:evidence_json;type:jsonb"`
	RelatedNews  string    `gorm:"column:related_news;type:jsonb"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (TickerExplanationDaily) TableName() string {
	return "ticker_explanation_daily"
}

type UserWatchlist struct {
	UserID  uint64    `gorm:"column:user_id;primaryKey"`
	Symbol  string    `gorm:"column:symbol;primaryKey;type:varchar(10)"`
	AddedAt time.Time `gorm:"column:added_at;autoCreateTime"`
}

func (UserWatchlist) TableName() string {
	return "user_watchlist"
}

type UserRadarPreference struct {
	UserID            uint64    `gorm:"column:user_id;primaryKey"`
	Mode              string    `gorm:"column:mode;type:varchar(50)"` // subsector / watchlist
	PreferredSubSector string   `gorm:"column:preferred_sub_sector;type:varchar(100)"`
	BroadcastEnabled  bool      `gorm:"column:broadcast_enabled;default:false"`
	BroadcastTime     string    `gorm:"column:broadcast_time;type:varchar(5)"` // e.g. "08:00"
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserRadarPreference) TableName() string {
	return "user_radar_preference"
}

type RadarRequestLog struct {
	ID                  uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID              uint64    `gorm:"column:user_id;index"`
	RequestType         string    `gorm:"column:request_type;type:varchar(50)"`
	Params              string    `gorm:"column:params;type:jsonb"`
	ResponseTickerCount int       `gorm:"column:response_ticker_count;default:0"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RadarRequestLog) TableName() string {
	return "radar_request_log"
}
