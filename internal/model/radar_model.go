package model

type RadarSignalItem struct {
	Symbol                   string  `json:"symbol"`
	SubSector                string  `json:"sub_sector"`
	ForeignFlowScore         int     `json:"foreign_flow_score"`
	InstitutionalBrokerScore int     `json:"institutional_broker_score"`
	VolumeScore              int     `json:"volume_score"`
	MomentumScore            int     `json:"momentum_score"`
	BonusCorporateAction     int     `json:"bonus_corporate_action"`
	BonusQuarterlyReport     int     `json:"bonus_quarterly_report"`
	BonusInsiderBuy          int     `json:"bonus_insider_buy"`
	CompositeScore           int     `json:"composite_score"`
	IndicatorEmoji           string  `json:"indicator_emoji"`
	Note                     string  `json:"note,omitempty"`

	// PRD v2 fields
	Archetype          string  `json:"archetype,omitempty"`
	ConfidenceLabel    string  `json:"confidence_label,omitempty"`
	HGS                float64 `json:"hgs,omitempty"`
	V                  float64 `json:"v,omitempty"`
	Q                  float64 `json:"q,omitempty"`
	I                  float64 `json:"i,omitempty"`
	H                  float64 `json:"h,omitempty"`
	S                  float64 `json:"s,omitempty"`
	T                  float64 `json:"t,omitempty"`
	DataCompleteness   float64 `json:"data_completeness,omitempty"`
}

type RadarResult struct {
	Date            string            `json:"date"`
	DataClosingDate string            `json:"data_closing_date"`
	Mode            string            `json:"mode"`
	SubSector       string            `json:"sub_sector,omitempty"`
	TotalMonitored  int               `json:"total_monitored"`
	Tickers         []RadarSignalItem `json:"tickers"`
	SummaryNote     string            `json:"summary_note,omitempty"`
}

type NewsItem struct {
	Title     string `json:"title"`
	Source    string `json:"source,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	URL       string `json:"url,omitempty"`
}

// CaseFileExplanation is the validated LLM JSON output per §9.1.
type CaseFileExplanation struct {
	PrimaryThesis        string   `json:"primary_thesis"`
	WhyUnderFollowed     string   `json:"why_under_followed"`
	PotentialCatalyst    string   `json:"potential_catalyst"`
	SpecificRisks        []string `json:"specific_risks"`
	CriticalQuestions    []string `json:"critical_questions"`
	NotInvestmentAdvice  string   `json:"not_investment_advice"`
}

type RadarDrillDownResult struct {
	Symbol            string                 `json:"symbol"`
	Date              string                 `json:"date"`
	DataClosingDate   string                 `json:"data_closing_date"`
	CompositeScore    int                    `json:"composite_score"`
	Signals           []string               `json:"signals"`
	AdditionalContext []string               `json:"additional_context"`
	SummaryReason     string                 `json:"summary_reason"`
	News              []NewsItem             `json:"news,omitempty"`
	RawEvidence       map[string]interface{} `json:"raw_evidence,omitempty"`

	// PRD v2 Case File Explanation
	Archetype          string               `json:"archetype,omitempty"`
	ConfidenceLabel    string               `json:"confidence_label,omitempty"`
	HGS                float64              `json:"hgs,omitempty"`
	ExplanationDetails *CaseFileExplanation `json:"explanation_details,omitempty"`
}

type SetRadarPreferenceRequest struct {
	Mode               string `json:"mode" validate:"required,oneof=subsector watchlist"`
	PreferredSubSector string `json:"preferred_sub_sector"`
	BroadcastEnabled   *bool  `json:"broadcast_enabled" validate:"required"`
	BroadcastTime      string `json:"broadcast_time"`
}

type RadarPreferenceResponse struct {
	UserID             uint64 `json:"user_id"`
	Mode               string `json:"mode"`
	PreferredSubSector string `json:"preferred_sub_sector"`
	BroadcastEnabled   bool   `json:"broadcast_enabled"`
	BroadcastTime      string `json:"broadcast_time"`
}

type AddWatchlistRequest struct {
	Symbols []string `json:"symbols" validate:"required,min=1"`
}

type WatchlistResponse struct {
	UserID  uint64   `json:"user_id"`
	Symbols []string `json:"symbols"`
}

type ManualTickersRequest struct {
	Symbols []string `json:"symbols" validate:"required,min=1"`
}

type RunPipelineRequest struct {
	Date  string `json:"date"`
	Force bool   `json:"force"`
}

type RunPipelineResponse struct {
	Date             string `json:"date"`
	Tier             string `json:"tier"`
	ScannedCount     int    `json:"scanned_count"`
	ShortlistedCount int    `json:"shortlisted_count"`
	Message          string `json:"message"`
}
