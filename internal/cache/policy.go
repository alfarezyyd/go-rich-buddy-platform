package cache

import "time"

// DataKind identifies what type of data is being cached for policy lookup.
type DataKind string

const (
	KindRadarRendered      DataKind = "radar_rendered"
	KindTickerExplanation  DataKind = "ticker_explanation"
	KindNewsExtract        DataKind = "news_extract"
	KindSubsectorList      DataKind = "subsector_list"
	KindUniverseDaily      DataKind = "universe_daily"
	KindPriceHistorical    DataKind = "price_historical"
	KindFinancialsQuarter  DataKind = "financials_quarterly"
	KindCompanyReport      DataKind = "company_report"
	KindSubsectorReport    DataKind = "subsector_report"
	KindCorporateActions   DataKind = "corporate_actions"
	KindNewsOnDemand       DataKind = "news_on_demand"
	KindCommodityPrice     DataKind = "commodity_price"
	KindNegativeCache      DataKind = "not_found"
	KindUserMemory         DataKind = "user_memory"
	KindGlossaryDefinition DataKind = "glossary_definition"
)

// TTLPolicy returns the recommended TTL for each cache data kind.
// A zero duration means the item should be stored permanently (relies on key versioning).
func TTLPolicy(kind DataKind) time.Duration {
	switch kind {
	case KindRadarRendered:
		// Valid until a new data_date is published — key is date-versioned so no manual invalidation needed.
		return 24 * time.Hour
	case KindTickerExplanation:
		// Permanent per (data_date, symbol, prompt_version, evidence_hash); retain 90 days via retention job.
		return 90 * 24 * time.Hour
	case KindNewsExtract:
		// Processed once per article_hash — effectively permanent.
		return 365 * 24 * time.Hour
	case KindSubsectorList:
		return 7 * 24 * time.Hour
	case KindUniverseDaily:
		return 24 * time.Hour
	case KindPriceHistorical:
		// Dates in the past are generally immutable; 7-day TTL guards against corporate actions.
		return 7 * 24 * time.Hour
	case KindFinancialsQuarter:
		// Permanent per report_date; reload triggered by quarterly date event.
		return 90 * 24 * time.Hour
	case KindCompanyReport:
		return 24 * time.Hour
	case KindSubsectorReport:
		return 24 * time.Hour
	case KindCorporateActions:
		return 24 * time.Hour
	case KindNewsOnDemand:
		return 20 * time.Minute
	case KindCommodityPrice:
		return 18 * time.Hour
	case KindNegativeCache:
		return 24 * time.Hour
	case KindUserMemory:
		// Short L1 TTL; invalidated on write.
		return 60 * time.Second
	case KindGlossaryDefinition:
		// Exact-match cache; glossary rarely changes.
		return 7 * 24 * time.Hour
	default:
		return 15 * time.Minute
	}
}
