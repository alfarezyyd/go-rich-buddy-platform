package cache

import "fmt"

// Key builders for all cacheable data kinds.
// All keys are versioned (by date, hash, or version tag) to avoid manual invalidation.

// RadarRenderedKey builds the key for a rendered Radar message for a specific date and sub-sector.
func RadarRenderedKey(dataDate, subSector string) string {
	return fmt.Sprintf("radar:%s:%s", dataDate, subSector)
}

// TickerExplanationKey builds the key for a ticker explanation, uniquely identified by date,
// symbol, prompt version, and evidence hash. This makes it permanently cacheable per combination.
func TickerExplanationKey(dataDate, symbol, promptVersion, evidenceHash string) string {
	return fmt.Sprintf("explanation:%s:%s:%s:%s", dataDate, symbol, promptVersion, evidenceHash)
}

// NewsExtractKey builds the key for an extracted news article identified by its content hash.
func NewsExtractKey(articleHash string) string {
	return fmt.Sprintf("news_extract:%s", articleHash)
}

// SubsectorListKey builds the versioned key for the list of sub-sectors.
func SubsectorListKey() string {
	return "subsectors:v1"
}

// UniverseDailyKey builds the key for the universe of tickers for a specific date.
func UniverseDailyKey(dataDate string) string {
	return fmt.Sprintf("universe:%s", dataDate)
}

// PriceHistoricalKey builds the key for historical price data for a symbol on a specific date.
func PriceHistoricalKey(symbol, date string) string {
	return fmt.Sprintf("daily:%s:%s", symbol, date)
}

// FinancialsQuarterlyKey builds the key for quarterly financial data for a symbol and report date.
func FinancialsQuarterlyKey(symbol, reportDate string) string {
	return fmt.Sprintf("fin_q:%s:%s", symbol, reportDate)
}

// CompanyReportKey builds the key for a specific section of a company report.
func CompanyReportKey(symbol, section string) string {
	return fmt.Sprintf("report:%s:%s", symbol, section)
}

// SubsectorReportKey builds the key for a specific section of a sub-sector report.
func SubsectorReportKey(subSector, section string) string {
	return fmt.Sprintf("subreport:%s:%s", subSector, section)
}

// CorporateActionsKey builds the key for corporate actions within a date range and type.
func CorporateActionsKey(start, end, actionType string) string {
	return fmt.Sprintf("ca:%s:%s:%s", start, end, actionType)
}

// NewsOnDemandKey builds the key for on-demand news for a set of symbols and a time window.
func NewsOnDemandKey(symbols, window string) string {
	return fmt.Sprintf("news:%s:%s", symbols, window)
}

// CommodityPriceKey builds the key for commodity price data over a year range.
func CommodityPriceKey(name, yearRange string) string {
	return fmt.Sprintf("commodity:%s:%s", name, yearRange)
}

// NegativeCacheKey builds the key for a cached 404 response to avoid re-billing for missing data.
func NegativeCacheKey(endpoint, key string) string {
	return fmt.Sprintf("notfound:%s:%s", endpoint, key)
}

// UserMemoryKey builds the short-lived L1 cache key for a user's active memory items.
func UserMemoryKey(userID uint64) string {
	return fmt.Sprintf("usermem:%d", userID)
}

// GlossaryDefinitionKey builds the exact-match key for a glossary term definition.
func GlossaryDefinitionKey(term string) string {
	return fmt.Sprintf("glossary:%s", term)
}
