package entity

import (
	"time"
)

// DiscoveryCandidateDaily tracks tickers identified through the 8 screener lenses in Stage 1.
type DiscoveryCandidateDaily struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	DataDate    string    `gorm:"column:data_date;type:date;index;not null"`
	Symbol      string    `gorm:"column:symbol;type:varchar(10);index;not null"`
	LensHits    string    `gorm:"column:lens_hits;type:text"` // Comma-separated lens codes, e.g. "L1,L3"
	Stage1Score float64   `gorm:"column:stage1_score;default:0"`
	InPoolB     bool      `gorm:"column:in_pool_b;default:false"`
	InFinalist  bool      `gorm:"column:in_finalist;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (DiscoveryCandidateDaily) TableName() string {
	return "discovery_candidate_daily"
}

// GateResultDaily logs pass/penalty/veto decisions from Stage 2 gates (G1–G10).
type GateResultDaily struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	DataDate   string    `gorm:"column:data_date;type:date;index;not null"`
	Symbol     string    `gorm:"column:symbol;type:varchar(10);index;not null"`
	GateCode   string    `gorm:"column:gate_code;type:varchar(20);not null"` // e.g. "G1", "G4"
	Result     string    `gorm:"column:result;type:varchar(10);not null"`    // "pass", "penalty", "veto"
	Multiplier float64   `gorm:"column:multiplier;default:1.0"`
	Detail     string    `gorm:"column:detail;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (GateResultDaily) TableName() string {
	return "gate_result_daily"
}

// PillarScoreDaily stores calculated scores across the 6 pillars (V, Q, I, H, S, T) and final HGS.
type PillarScoreDaily struct {
	ID                 uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	DataDate           string    `gorm:"column:data_date;type:date;index;not null"`
	Symbol             string    `gorm:"column:symbol;type:varchar(10);index;not null"`
	SubSector          string    `gorm:"column:sub_sector;type:varchar(100);index"`
	V                  float64   `gorm:"column:v;default:0"`
	Q                  float64   `gorm:"column:q;default:0"`
	I                  float64   `gorm:"column:i;default:0"`
	H                  float64   `gorm:"column:h;default:0"`
	S                  float64   `gorm:"column:s;default:0"`
	T                  float64   `gorm:"column:t;default:0"`
	HGSRaw             float64   `gorm:"column:hgs_raw;default:0"`
	PenaltyTotal       float64   `gorm:"column:penalty_total;default:1.0"`
	HGS                float64   `gorm:"column:hgs;default:0"`
	DataCompleteness   float64   `gorm:"column:data_completeness;default:1.0"`
	Archetype          string    `gorm:"column:archetype;type:varchar(50)"`
	SecondaryArchetype string    `gorm:"column:secondary_archetype;type:varchar(50)"`
	ConfidenceLabel    string    `gorm:"column:confidence_label;type:varchar(20)"`
	IsFinalist         bool      `gorm:"column:is_finalist;default:false"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (PillarScoreDaily) TableName() string {
	return "pillar_score_daily"
}

// SubsectorContextWeekly holds weekly cached valuation/growth medians per subsector.
type SubsectorContextWeekly struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ISOWeek   string    `gorm:"column:iso_week;type:varchar(10);index;not null"` // e.g. "2026-W40"
	SubSector string    `gorm:"column:sub_sector;type:varchar(100);index;not null"`
	MedianPE  float64   `gorm:"column:median_pe;default:0"`
	MedianPB  float64   `gorm:"column:median_pb;default:0"`
	Growth    string    `gorm:"column:growth;type:jsonb"`
	Valuation string    `gorm:"column:valuation;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (SubsectorContextWeekly) TableName() string {
	return "subsector_context_weekly"
}

// CommodityTrendWeekly holds weekly cached commodity price trend data.
type CommodityTrendWeekly struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ISOWeek   string    `gorm:"column:iso_week;type:varchar(10);index;not null"`
	Commodity string    `gorm:"column:commodity;type:varchar(50);index;not null"`
	Trend3M   float64   `gorm:"column:trend_3m;default:0"`
	Trend12M  float64   `gorm:"column:trend_12m;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (CommodityTrendWeekly) TableName() string {
	return "commodity_trend_weekly"
}

// CaseFileDaily stores the structured Case File JSON per finalist ticker per day.
type CaseFileDaily struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	DataDate     string    `gorm:"column:data_date;type:date;index;not null"`
	Symbol       string    `gorm:"column:symbol;type:varchar(10);index;not null"`
	JSON         string    `gorm:"column:json;type:jsonb;not null"`
	EvidenceHash string    `gorm:"column:evidence_hash;type:varchar(64)"`
	Explanation  string    `gorm:"column:explanation;type:jsonb"` // validated LLM output
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (CaseFileDaily) TableName() string {
	return "case_file_daily"
}

// RadarPickLog records each ticker shown to a user for Jejak Radar paper tracking.
type RadarPickLog struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"column:user_id;index;not null"`
	Symbol    string    `gorm:"column:symbol;type:varchar(10);index;not null"`
	ShownDate string    `gorm:"column:shown_date;type:date;index;not null"`
	RefPrice  float64   `gorm:"column:ref_price;default:0"`
	HGS       float64   `gorm:"column:hgs;default:0"`
	Archetype string    `gorm:"column:archetype;type:varchar(50)"`
	Mode      string    `gorm:"column:mode;type:varchar(50)"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RadarPickLog) TableName() string {
	return "radar_pick_log"
}

// RadarPickOutcome records post-pick return performance against IHSG benchmark.
type RadarPickOutcome struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	PickID      uint64    `gorm:"column:pick_id;index;not null"`
	HorizonDays int       `gorm:"column:horizon_days;not null"` // 5, 20, 60
	ClosePrice  float64   `gorm:"column:close_price;default:0"`
	Ret         float64   `gorm:"column:ret;default:0"`
	IHSGRet     float64   `gorm:"column:ihsg_ret;default:0"`
	ComputedAt  time.Time `gorm:"column:computed_at;autoCreateTime"`
}

func (RadarPickOutcome) TableName() string {
	return "radar_pick_outcome"
}

// Legacy tables preserved for compatibility.
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
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Date          string    `gorm:"column:date;type:date;index"`
	Symbol        string    `gorm:"column:symbol;type:varchar(10);index"`
	SummaryReason string    `gorm:"column:summary_reason;type:text"`
	EvidenceJSON  string    `gorm:"column:evidence_json;type:jsonb"`
	RelatedNews   string    `gorm:"column:related_news;type:jsonb"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
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
	UserID             uint64    `gorm:"column:user_id;primaryKey"`
	Mode               string    `gorm:"column:mode;type:varchar(50)"` // subsector / watchlist
	PreferredSubSector string    `gorm:"column:preferred_sub_sector;type:varchar(100)"`
	BroadcastEnabled   bool      `gorm:"column:broadcast_enabled;default:false"`
	BroadcastTime      string    `gorm:"column:broadcast_time;type:varchar(5)"` // e.g. "08:00"
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`
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
