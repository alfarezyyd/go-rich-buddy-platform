package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// radarDisclaimer is appended to every Radar output message as required by the PRD.
	// Note: The user-facing text is intentionally kept in Indonesian as the product targets Indonesian users.
	radarDisclaimer = "⚠️ Radar ini adalah alat bantu belajar, bukan rekomendasi beli/jual. Data bersifat historis (closing hari sebelumnya) dan tidak menjamin pergerakan harga ke depan."

	// maxWatchlistPerUser enforces the PRD §5.4 limit of 10 tickers per user to control API credit costs.
	maxWatchlistPerUser = 10
)

type ServiceImpl struct {
	dbConnection    *gorm.DB
	radarRepository Repository
	sectorsClient   SectorsClient
	agentClient     client.AgentClient
}

func NewService(
	dbConnection *gorm.DB,
	radarRepository Repository,
	sectorsClient SectorsClient,
	agentClient client.AgentClient,
) Service {
	return &ServiceImpl{
		dbConnection:    dbConnection,
		radarRepository: radarRepository,
		sectorsClient:   sectorsClient,
		agentClient:     agentClient,
	}
}

// getEffectiveDate returns the target date for pipeline operations.
// If no date is provided, it checks data freshness from the Sectors API, falling back to yesterday.
func (radarServiceImpl *ServiceImpl) getEffectiveDate(ctx context.Context, requestedDate string) string {
	if requestedDate != "" {
		return requestedDate
	}
	freshnessDate, err := radarServiceImpl.sectorsClient.CheckDataFreshness(ctx)
	if err == nil && freshnessDate != "" {
		return freshnessDate
	}
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

// RunTier1 executes the universe scan pipeline (PRD §6.1).
// It fetches cheap signals for all eligible tickers and saves partial composite scores.
// Estimated credit cost: ~35–50 credits/day.
func (radarServiceImpl *ServiceImpl) RunTier1(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error) {
	date := radarServiceImpl.getEffectiveDate(ctx, targetDate)
	logrus.Infof("Executing Radar Tier 1 (Universe Scan) for date %s", date)

	// Fetch eligible universe with market cap, listing board, listing date, and suspension filters (PRD §5.1)
	universe, err := radarServiceImpl.sectorsClient.GetEligibleUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch eligible universe: %w", err)
	}

	foreignFlowMap, _ := radarServiceImpl.sectorsClient.GetForeignFlow(ctx)
	mostTradedMap, _ := radarServiceImpl.sectorsClient.GetMostTraded(ctx)
	topChangesMap, _ := radarServiceImpl.sectorsClient.GetTopChanges(ctx)

	startDate := time.Now().Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	corpActionsMap, _ := radarServiceImpl.sectorsClient.GetCorporateActions(ctx, startDate, endDate)

	// Look back 2 days for new quarterly reports (PRD §5.2 Signal 6)
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	quarterlyDatesMap, _ := radarServiceImpl.sectorsClient.GetQuarterlyFinancialDates(ctx, twoDaysAgo)

	// Build cross-sectional percentile ranking for foreign flow (PRD §5.2 Signal 1)
	type flowEntry struct {
		symbol string
		flow   float64
	}
	var flows []flowEntry
	for _, comp := range universe {
		flowVal := foreignFlowMap[comp.Symbol]
		flows = append(flows, flowEntry{symbol: comp.Symbol, flow: flowVal})
	}
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].flow < flows[j].flow
	})

	foreignScoreMap := make(map[string]int)
	n := len(flows)
	for i, entry := range flows {
		if n <= 1 {
			foreignScoreMap[entry.symbol] = 50
		} else {
			pct := int(math.Round(float64(i) / float64(n-1) * 100.0))
			foreignScoreMap[entry.symbol] = pct
		}
	}

	var signals []entity.SignalDaily
	for _, comp := range universe {
		sym := comp.Symbol
		foreignFlowScore := foreignScoreMap[sym]

		// Volume score: most-traded ranking → score 20–100 (PRD §5.2 Signal 3)
		volumeScore := 10
		if rank, ok := mostTradedMap[sym]; ok && rank > 0 {
			vScore := 100 - (rank-1)*2
			if vScore < 20 {
				vScore = 20
			}
			volumeScore = vScore
		}

		// Momentum score: top-gainers/losers ranking → score 20–100 (PRD §5.2 Signal 4)
		momentumScore := 15
		if rank, ok := topChangesMap[sym]; ok && rank > 0 {
			mScore := 100 - (rank-1)*2
			if mScore < 20 {
				mScore = 20
			}
			momentumScore = mScore
		}

		// Corporate action bonus: binary 0 or 100 (PRD §5.2 Signal 5)
		bonusCorporateAction := 0
		if corpActionsMap[sym] {
			bonusCorporateAction = 100
		}

		// Quarterly report bonus: binary 0 or 100 (PRD §5.2 Signal 6)
		bonusQuarterlyReport := 0
		if quarterlyDatesMap[sym] {
			bonusQuarterlyReport = 100
		}

		// Tier 1 partial composite score — broker (Signal 2) and insider (Signal 7) are not
		// available at this stage (too expensive to compute for all universe tickers).
		// Weights are redistributed proportionally from the PRD §5.3 formula:
		//   0.25 asing + 0.15 volume + 0.10 momentum + 0.10 corp + 0.10 quarterly  → sum=0.70
		//   Scaled: 0.35/0.25/0.15/0.15/0.10 to use full 1.0 range.
		partialCompositeScore := int(math.Round(
			0.35*float64(foreignFlowScore) +
				0.25*float64(volumeScore) +
				0.15*float64(momentumScore) +
				0.15*float64(bonusCorporateAction) +
				0.10*float64(bonusQuarterlyReport),
		))

		signals = append(signals, entity.SignalDaily{
			Date:                     date,
			Symbol:                   sym,
			SubSector:                comp.SubSector,
			ForeignFlowScore:         foreignFlowScore,
			InstitutionalBrokerScore: 0,
			VolumeScore:              volumeScore,
			MomentumScore:            momentumScore,
			BonusCorporateAction:     bonusCorporateAction,
			BonusQuarterlyReport:     bonusQuarterlyReport,
			BonusInsiderBuy:          0,
			CompositeScore:           partialCompositeScore,
			IsShortlisted:            false,
			IsEnriched:               false,
		})
	}

	// Mark top 100 as shortlisted (PRD §6.2)
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].CompositeScore > signals[j].CompositeScore
	})

	shortlistLimit := 100
	if len(signals) < shortlistLimit {
		shortlistLimit = len(signals)
	}
	for i := 0; i < shortlistLimit; i++ {
		signals[i].IsShortlisted = true
	}

	err = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		return radarServiceImpl.radarRepository.SaveSignals(tx, signals)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save Tier 1 signals: %w", err)
	}

	return &model.RunPipelineResponse{
		Date:             date,
		Tier:             "Tier 1",
		ScannedCount:     len(signals),
		ShortlistedCount: shortlistLimit,
		Message:          fmt.Sprintf("Tier 1 completed: scanned %d companies, shortlisted %d", len(signals), shortlistLimit),
	}, nil
}

// RunTier2 executes the deep enrichment pipeline for shortlisted tickers (PRD §6.3).
// It computes institutional broker scores, insider buy bonuses, fetches news,
// and stores final composite scores and drill-down explanations.
// Estimated credit cost: ~210–260 credits/day.
func (radarServiceImpl *ServiceImpl) RunTier2(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error) {
	date := radarServiceImpl.getEffectiveDate(ctx, targetDate)
	logrus.Infof("Executing Radar Tier 2 (Deep Enrichment) for date %s", date)

	var shortlistedSignals []entity.SignalDaily
	err := radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		shortlistedSignals, err = radarServiceImpl.radarRepository.GetShortlistedSignals(tx, date)
		return err
	})
	if err != nil || len(shortlistedSignals) == 0 {
		// Run Tier 1 first if data is not ready
		_, err = radarServiceImpl.RunTier1(ctx, date)
		if err != nil {
			return nil, err
		}
		_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
			shortlistedSignals, err = radarServiceImpl.radarRepository.GetShortlistedSignals(tx, date)
			return err
		})
	}

	// Union shortlisted symbols with all active watchlist symbols (PRD §6.2)
	var watchlistSymbols []string
	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		watchlistSymbols, err = radarServiceImpl.radarRepository.GetAllActiveWatchlistSymbols(tx)
		return err
	})

	targetSymbolMap := make(map[string]bool)
	for _, sig := range shortlistedSignals {
		targetSymbolMap[sig.Symbol] = true
	}
	for _, sym := range watchlistSymbols {
		targetSymbolMap[strings.ToUpper(sym)] = true
	}

	var targetSymbols []string
	for sym := range targetSymbolMap {
		targetSymbols = append(targetSymbols, sym)
	}

	// Fetch broker summary and insider filings for each target symbol (PRD §6.3)
	type brokerInfo struct {
		symbol string
		ratio  float64
		item   *BrokerSummaryItem
	}
	var brokerInfos []brokerInfo
	insiderBonusMap := make(map[string]int)
	insiderFilingsMap := make(map[string][]FilingItem)

	for _, sym := range targetSymbols {
		brokerSummary, err := radarServiceImpl.sectorsClient.GetInstitutionalBrokerSummary(ctx, sym)
		if err == nil && brokerSummary != nil {
			brokerInfos = append(brokerInfos, brokerInfo{
				symbol: sym,
				ratio:  brokerSummary.Ratio,
				item:   brokerSummary,
			})
		}

		filings, err := radarServiceImpl.sectorsClient.GetInsiderFilings(ctx, sym)
		if err == nil && len(filings) > 0 {
			insiderFilingsMap[sym] = filings
			// Insider buy threshold: ≥0.5% of outstanding shares (PRD §5.2 Signal 7)
			for _, f := range filings {
				if strings.ToLower(f.TransactionType) == "buy" && f.Percentage >= 0.5 {
					insiderBonusMap[sym] = 100
					break
				}
			}
		}
	}

	// Calculate cross-sectional percentile of institutional broker ratios (PRD §5.2 Signal 2)
	sort.Slice(brokerInfos, func(i, j int) bool {
		return brokerInfos[i].ratio < brokerInfos[j].ratio
	})
	brokerScoreMap := make(map[string]int)
	brokerSummaryMap := make(map[string]*BrokerSummaryItem)
	bn := len(brokerInfos)
	for i, info := range brokerInfos {
		if bn <= 1 {
			brokerScoreMap[info.symbol] = 50
		} else {
			pct := int(math.Round(float64(i) / float64(bn-1) * 100.0))
			brokerScoreMap[info.symbol] = pct
		}
		brokerSummaryMap[info.symbol] = info.item
	}

	newsMap, _ := radarServiceImpl.sectorsClient.GetNews(ctx, targetSymbols)

	// Fetch current signals from DB for all target symbols
	var currentSignals []entity.SignalDaily
	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		currentSignals, err = radarServiceImpl.radarRepository.GetSignalsBySymbols(tx, date, targetSymbols)
		return err
	})

	var updatedSignals []entity.SignalDaily
	var explanations []entity.TickerExplanationDaily

	for _, sig := range currentSignals {
		sym := sig.Symbol

		institutionalBrokerScore := brokerScoreMap[sym]
		if institutionalBrokerScore == 0 {
			institutionalBrokerScore = 50 // neutral default when no data available
		}
		bonusInsiderBuy := insiderBonusMap[sym]

		// Final composite score per PRD §5.3 formula:
		// 0.25*foreign_flow + 0.25*broker + 0.15*volume + 0.10*momentum
		//   + 0.10*corp_action + 0.10*quarterly_report + 0.05*insider_buy
		compositeScore := int(math.Round(
			0.25*float64(sig.ForeignFlowScore) +
				0.25*float64(institutionalBrokerScore) +
				0.15*float64(sig.VolumeScore) +
				0.10*float64(sig.MomentumScore) +
				0.10*float64(sig.BonusCorporateAction) +
				0.10*float64(sig.BonusQuarterlyReport) +
				0.05*float64(bonusInsiderBuy),
		))
		if compositeScore > 100 {
			compositeScore = 100
		}

		sig.InstitutionalBrokerScore = institutionalBrokerScore
		sig.BonusInsiderBuy = bonusInsiderBuy
		sig.CompositeScore = compositeScore
		sig.IsEnriched = true
		updatedSignals = append(updatedSignals, sig)

		// Build structured raw evidence for auditability (PRD §9 NFR)
		rawEvidence := map[string]interface{}{
			"foreign_flow_score":         sig.ForeignFlowScore,
			"institutional_broker_score": institutionalBrokerScore,
			"volume_score":               sig.VolumeScore,
			"momentum_score":             sig.MomentumScore,
			"bonus_corporate_action":     sig.BonusCorporateAction,
			"bonus_quarterly_report":     sig.BonusQuarterlyReport,
			"bonus_insider_buy":          bonusInsiderBuy,
			"composite_score":            compositeScore,
		}
		if bs, ok := brokerSummaryMap[sym]; ok && bs != nil {
			rawEvidence["broker_net_buy_idr"] = bs.InstitutionalNet
			rawEvidence["broker_ratio"] = bs.Ratio
		}
		if fl, ok := insiderFilingsMap[sym]; ok {
			rawEvidence["insider_filings"] = fl
		}

		evidenceBytes, _ := json.Marshal(rawEvidence)
		newsList := newsMap[sym]
		var modelNews []model.NewsItem
		for _, n := range newsList {
			modelNews = append(modelNews, model.NewsItem{
				Title:     n.Title,
				Source:    n.Source,
				Timestamp: n.Timestamp,
				URL:       n.URL,
			})
		}
		newsBytes, _ := json.Marshal(modelNews)

		summaryReason := radarServiceImpl.buildSummaryReason(sym, sig, brokerSummaryMap[sym], modelNews)

		explanations = append(explanations, entity.TickerExplanationDaily{
			Date:          date,
			Symbol:        sym,
			SummaryReason: summaryReason,
			EvidenceJSON:  string(evidenceBytes),
			RelatedNews:   string(newsBytes),
		})
	}

	err = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		if err := radarServiceImpl.radarRepository.SaveSignals(tx, updatedSignals); err != nil {
			return err
		}
		return radarServiceImpl.radarRepository.SaveExplanations(tx, explanations)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save Tier 2 enriched data: %w", err)
	}

	return &model.RunPipelineResponse{
		Date:             date,
		Tier:             "Tier 2",
		ScannedCount:     len(updatedSignals),
		ShortlistedCount: len(explanations),
		Message:          fmt.Sprintf("Tier 2 completed: enriched and generated explanations for %d tickers", len(explanations)),
	}, nil
}

// buildSummaryReason generates a human-readable explanation of why a ticker appeared in the Radar.
// Note: User-facing text is in Indonesian as the product targets Indonesian users.
func (radarServiceImpl *ServiceImpl) buildSummaryReason(
	symbol string,
	sig entity.SignalDaily,
	broker *BrokerSummaryItem,
	news []model.NewsItem,
) string {
	var signalDescriptions []string
	if sig.ForeignFlowScore >= 75 {
		signalDescriptions = append(signalDescriptions, fmt.Sprintf("Arus dana asing masuk signifikan (persentil ke-%d)", sig.ForeignFlowScore))
	}
	if sig.InstitutionalBrokerScore >= 75 {
		if broker != nil && broker.InstitutionalNet > 0 {
			netBillion := broker.InstitutionalNet / 1e9
			signalDescriptions = append(signalDescriptions, fmt.Sprintf("Akumulasi broker institusi kuat (net beli Rp %.1f M dalam 5 hari)", netBillion))
		} else {
			signalDescriptions = append(signalDescriptions, fmt.Sprintf("Akumulasi broker institusi kuat (skor %d/100)", sig.InstitutionalBrokerScore))
		}
	}
	if sig.VolumeScore >= 70 {
		signalDescriptions = append(signalDescriptions, "Lonjakan volume dan nilai transaksi di atas rata-rata")
	}
	if sig.MomentumScore >= 70 {
		signalDescriptions = append(signalDescriptions, "Momentum pergerakan harga positif")
	}
	if sig.BonusCorporateAction > 0 {
		signalDescriptions = append(signalDescriptions, "Terdapat jadwal aksi korporasi penting dalam 7 hari ke depan")
	}
	if sig.BonusQuarterlyReport > 0 {
		signalDescriptions = append(signalDescriptions, "Laporan keuangan kuartal baru saja dirilis")
	}
	if sig.BonusInsiderBuy > 0 {
		signalDescriptions = append(signalDescriptions, "Terdeteksi transaksi pembelian saham oleh orang dalam (insider buy)")
	}

	if len(signalDescriptions) == 0 {
		signalDescriptions = append(signalDescriptions, "Aktivitas perdagangan stabil dan dalam pemantauan rutin")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Saham %s terpilih dalam Radar Saham karena:\n", symbol))
	for _, s := range signalDescriptions {
		sb.WriteString(fmt.Sprintf("• %s\n", s))
	}
	if len(news) > 0 {
		sb.WriteString(fmt.Sprintf("• Katalis berita terkini: \"%s\"\n", news[0].Title))
	}
	return strings.TrimSpace(sb.String())
}

// ensureDataReady guarantees today's pipeline has been run before serving user queries.
// If no data exists for today it auto-runs Tier 1 + Tier 2 on-demand.
func (radarServiceImpl *ServiceImpl) ensureDataReady(ctx context.Context, gormTransaction *gorm.DB) (string, error) {
	date, err := radarServiceImpl.radarRepository.GetLatestAvailableDate(gormTransaction)
	today := time.Now().Format("2006-01-02")
	if err != nil || date == "" {
		_, err = radarServiceImpl.RunTier1(ctx, today)
		if err != nil {
			return "", err
		}
		_, err = radarServiceImpl.RunTier2(ctx, today)
		if err != nil {
			return "", err
		}
		return today, nil
	}
	return date, nil
}

// getIndicatorEmoji returns the strength emoji for a given composite score (PRD §8.1).
func getIndicatorEmoji(score int) string {
	if score >= 80 {
		return "🔥"
	}
	if score >= 60 {
		return "📈"
	}
	return "➖"
}

// GetRadarBySubSector returns the top 5 signals for a given sub-sector (PRD §5.4 Path A).
func (radarServiceImpl *ServiceImpl) GetRadarBySubSector(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	subSector string,
) (*model.RadarResult, error) {
	date, err := radarServiceImpl.ensureDataReady(ctx, gormTransaction)
	if err != nil {
		return nil, err
	}

	signals, err := radarServiceImpl.radarRepository.GetTopSignalsBySubSector(gormTransaction, date, subSector, 5)
	if err != nil {
		return nil, err
	}

	totalMonitored, _ := radarServiceImpl.radarRepository.GetSubSectorCounts(gormTransaction, date, subSector)
	if totalMonitored == 0 {
		totalMonitored = int64(len(signals))
	}

	var tickerItems []model.RadarSignalItem
	for _, s := range signals {
		tickerItems = append(tickerItems, model.RadarSignalItem{
			Symbol:                   s.Symbol,
			SubSector:                s.SubSector,
			ForeignFlowScore:         s.ForeignFlowScore,
			InstitutionalBrokerScore: s.InstitutionalBrokerScore,
			VolumeScore:              s.VolumeScore,
			MomentumScore:            s.MomentumScore,
			BonusCorporateAction:     s.BonusCorporateAction,
			BonusQuarterlyReport:     s.BonusQuarterlyReport,
			BonusInsiderBuy:          s.BonusInsiderBuy,
			CompositeScore:           s.CompositeScore,
			IndicatorEmoji:           getIndicatorEmoji(s.CompositeScore),
		})
	}

	// Note: if sub-sector has fewer than 5 eligible tickers, show all with a note (PRD §5.4)
	var summaryNote string
	if len(signals) < 5 {
		summaryNote = fmt.Sprintf("Menampilkan seluruh %d saham eligible pada subsektor ini.", len(signals))
	}

	paramsBytes, _ := json.Marshal(map[string]string{"sub_sector": subSector})
	_ = radarServiceImpl.radarRepository.LogRequest(gormTransaction, &entity.RadarRequestLog{
		UserID:              userID,
		RequestType:         "subsector",
		Params:              string(paramsBytes),
		ResponseTickerCount: len(tickerItems),
	})

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "subsector",
		SubSector:       subSector,
		TotalMonitored:  int(totalMonitored),
		Tickers:         tickerItems,
		SummaryNote:     summaryNote,
		Disclaimer:      radarDisclaimer,
	}, nil
}

// GetRadarByWatchlist returns radar results for all tickers in the user's watchlist,
// displaying up to 5 sorted by score (PRD §5.4 Path B).
func (radarServiceImpl *ServiceImpl) GetRadarByWatchlist(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
) (*model.RadarResult, error) {
	date, err := radarServiceImpl.ensureDataReady(ctx, gormTransaction)
	if err != nil {
		return nil, err
	}

	userWatchlist, err := radarServiceImpl.radarRepository.GetUserWatchlist(gormTransaction, userID)
	if err != nil || len(userWatchlist) == 0 {
		return &model.RadarResult{
			Date:            time.Now().Format("02 Jan 2006"),
			DataClosingDate: time.Now().AddDate(0, 0, -1).Format("02 Jan 2006"),
			Mode:            "watchlist",
			TotalMonitored:  0,
			Tickers:         []model.RadarSignalItem{},
			SummaryNote:     "Watchlist Anda masih kosong. Tambahkan kode saham ke watchlist Anda.",
			Disclaimer:      radarDisclaimer,
		}, nil
	}

	signals, err := radarServiceImpl.radarRepository.GetTopSignalsByWatchlist(gormTransaction, date, userID, 0)
	if err != nil {
		return nil, err
	}

	// On-demand enrichment for watchlist tickers not yet in the shortlist (PRD §5.4 Path B)
	foundMap := make(map[string]bool)
	for _, s := range signals {
		foundMap[s.Symbol] = true
	}
	var missingSymbols []string
	for _, wl := range userWatchlist {
		upper := strings.ToUpper(wl.Symbol)
		if !foundMap[upper] {
			missingSymbols = append(missingSymbols, upper)
		}
	}
	if len(missingSymbols) > 0 {
		logrus.Infof("Triggering on-demand enrichment for %d non-shortlisted watchlist tickers: %v", len(missingSymbols), missingSymbols)
		radarServiceImpl.enrichSymbolsOnDemand(ctx, date, missingSymbols)
		// Re-fetch signals after enrichment
		signals, _ = radarServiceImpl.radarRepository.GetTopSignalsByWatchlist(gormTransaction, date, userID, 0)
	}

	totalWatchlist := len(userWatchlist)
	const displayLimit = 5
	displayedSignals := signals
	if len(displayedSignals) > displayLimit {
		displayedSignals = displayedSignals[:displayLimit]
	}

	var tickerItems []model.RadarSignalItem
	for _, s := range displayedSignals {
		note := ""
		if s.CompositeScore < 60 {
			note = "tidak ada sinyal signifikan hari ini"
		}
		tickerItems = append(tickerItems, model.RadarSignalItem{
			Symbol:                   s.Symbol,
			SubSector:                s.SubSector,
			ForeignFlowScore:         s.ForeignFlowScore,
			InstitutionalBrokerScore: s.InstitutionalBrokerScore,
			VolumeScore:              s.VolumeScore,
			MomentumScore:            s.MomentumScore,
			BonusCorporateAction:     s.BonusCorporateAction,
			BonusQuarterlyReport:     s.BonusQuarterlyReport,
			BonusInsiderBuy:          s.BonusInsiderBuy,
			CompositeScore:           s.CompositeScore,
			IndicatorEmoji:           getIndicatorEmoji(s.CompositeScore),
			Note:                     note,
		})
	}

	var summaryNote string
	if totalWatchlist <= 5 {
		summaryNote = fmt.Sprintf("%d dari %d ticker (≤5, semua ditampilkan)", len(tickerItems), totalWatchlist)
	} else {
		summaryNote = fmt.Sprintf("%d ticker lain tidak menunjukkan sinyal signifikan hari ini", totalWatchlist-5)
	}

	paramsBytes, _ := json.Marshal(map[string]int{"watchlist_count": totalWatchlist})
	_ = radarServiceImpl.radarRepository.LogRequest(gormTransaction, &entity.RadarRequestLog{
		UserID:              userID,
		RequestType:         "watchlist",
		Params:              string(paramsBytes),
		ResponseTickerCount: len(tickerItems),
	})

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "watchlist",
		TotalMonitored:  totalWatchlist,
		Tickers:         tickerItems,
		SummaryNote:     summaryNote,
		Disclaimer:      radarDisclaimer,
	}, nil
}

// enrichSymbolsOnDemand fetches broker summary, insider filings, and news for symbols
// that were not part of the daily shortlist, and saves them to the cache (PRD §5.4 Path B).
func (radarServiceImpl *ServiceImpl) enrichSymbolsOnDemand(ctx context.Context, date string, symbols []string) {
	var brokerInfos []struct {
		symbol string
		ratio  float64
		item   *BrokerSummaryItem
	}
	insiderBonusMap := make(map[string]int)
	insiderFilingsMap := make(map[string][]FilingItem)

	for _, sym := range symbols {
		bs, err := radarServiceImpl.sectorsClient.GetInstitutionalBrokerSummary(ctx, sym)
		if err == nil && bs != nil {
			brokerInfos = append(brokerInfos, struct {
				symbol string
				ratio  float64
				item   *BrokerSummaryItem
			}{sym, bs.Ratio, bs})
		}
		filings, err := radarServiceImpl.sectorsClient.GetInsiderFilings(ctx, sym)
		if err == nil && len(filings) > 0 {
			insiderFilingsMap[sym] = filings
			for _, f := range filings {
				if strings.ToLower(f.TransactionType) == "buy" && f.Percentage >= 0.5 {
					insiderBonusMap[sym] = 100
					break
				}
			}
		}
	}

	brokerScoreMap := make(map[string]int)
	brokerSummaryMap := make(map[string]*BrokerSummaryItem)
	bn := len(brokerInfos)
	sort.Slice(brokerInfos, func(i, j int) bool { return brokerInfos[i].ratio < brokerInfos[j].ratio })
	for i, info := range brokerInfos {
		score := 50
		if bn > 1 {
			score = int(math.Round(float64(i) / float64(bn-1) * 100.0))
		}
		brokerScoreMap[info.symbol] = score
		brokerSummaryMap[info.symbol] = info.item
	}

	newsMap, _ := radarServiceImpl.sectorsClient.GetNews(ctx, symbols)

	var currentSignals []entity.SignalDaily
	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		currentSignals, err = radarServiceImpl.radarRepository.GetSignalsBySymbols(tx, date, symbols)
		return err
	})

	var updatedSignals []entity.SignalDaily
	var explanations []entity.TickerExplanationDaily

	for _, sig := range currentSignals {
		sym := sig.Symbol
		institutionalBrokerScore := brokerScoreMap[sym]
		if institutionalBrokerScore == 0 {
			institutionalBrokerScore = 50
		}
		bonusInsiderBuy := insiderBonusMap[sym]
		compositeScore := int(math.Round(
			0.25*float64(sig.ForeignFlowScore) +
				0.25*float64(institutionalBrokerScore) +
				0.15*float64(sig.VolumeScore) +
				0.10*float64(sig.MomentumScore) +
				0.10*float64(sig.BonusCorporateAction) +
				0.10*float64(sig.BonusQuarterlyReport) +
				0.05*float64(bonusInsiderBuy),
		))
		if compositeScore > 100 {
			compositeScore = 100
		}
		sig.InstitutionalBrokerScore = institutionalBrokerScore
		sig.BonusInsiderBuy = bonusInsiderBuy
		sig.CompositeScore = compositeScore
		sig.IsEnriched = true
		updatedSignals = append(updatedSignals, sig)

		rawEvidence := map[string]interface{}{
			"foreign_flow_score":         sig.ForeignFlowScore,
			"institutional_broker_score": institutionalBrokerScore,
			"volume_score":               sig.VolumeScore,
			"momentum_score":             sig.MomentumScore,
			"bonus_corporate_action":     sig.BonusCorporateAction,
			"bonus_quarterly_report":     sig.BonusQuarterlyReport,
			"bonus_insider_buy":          bonusInsiderBuy,
			"composite_score":            compositeScore,
		}
		if bs, ok := brokerSummaryMap[sym]; ok && bs != nil {
			rawEvidence["broker_net_buy_idr"] = bs.InstitutionalNet
			rawEvidence["broker_ratio"] = bs.Ratio
		}
		if fl, ok := insiderFilingsMap[sym]; ok {
			rawEvidence["insider_filings"] = fl
		}
		evidenceBytes, _ := json.Marshal(rawEvidence)

		newsList := newsMap[sym]
		var modelNews []model.NewsItem
		for _, n := range newsList {
			modelNews = append(modelNews, model.NewsItem{
				Title: n.Title, Source: n.Source, Timestamp: n.Timestamp, URL: n.URL,
			})
		}
		newsBytes, _ := json.Marshal(modelNews)

		summaryReason := radarServiceImpl.buildSummaryReason(sym, sig, brokerSummaryMap[sym], modelNews)
		explanations = append(explanations, entity.TickerExplanationDaily{
			Date: date, Symbol: sym,
			SummaryReason: summaryReason,
			EvidenceJSON:  string(evidenceBytes),
			RelatedNews:   string(newsBytes),
		})
	}

	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		if err := radarServiceImpl.radarRepository.SaveSignals(tx, updatedSignals); err != nil {
			return err
		}
		return radarServiceImpl.radarRepository.SaveExplanations(tx, explanations)
	})
}

// GetRadarByManualTickers returns radar results for a user-supplied list of tickers (PRD §5.4 Path B manual).
func (radarServiceImpl *ServiceImpl) GetRadarByManualTickers(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	symbols []string,
) (*model.RadarResult, error) {
	date, err := radarServiceImpl.ensureDataReady(ctx, gormTransaction)
	if err != nil {
		return nil, err
	}

	var cleanSymbols []string
	for _, s := range symbols {
		cleaned := strings.TrimSpace(strings.ToUpper(s))
		if cleaned != "" {
			cleanSymbols = append(cleanSymbols, cleaned)
		}
	}

	signals, err := radarServiceImpl.radarRepository.GetSignalsBySymbols(gormTransaction, date, cleanSymbols)
	if err != nil {
		return nil, err
	}

	// For any symbols not found in the signal table, create a placeholder entry (PRD §10 invalid/delisted handling)
	foundMap := make(map[string]bool)
	for _, s := range signals {
		foundMap[s.Symbol] = true
	}
	var notFoundSymbols []string
	for _, sym := range cleanSymbols {
		if !foundMap[sym] {
			notFoundSymbols = append(notFoundSymbols, sym)
			signals = append(signals, entity.SignalDaily{
				Date:           date,
				Symbol:         sym,
				SubSector:      "Unknown",
				CompositeScore: 0,
			})
		}
	}
	if len(notFoundSymbols) > 0 {
		logrus.Warnf("Manual ticker request: symbols not found in today's universe: %v", notFoundSymbols)
	}

	sort.Slice(signals, func(i, j int) bool {
		return signals[i].CompositeScore > signals[j].CompositeScore
	})

	totalInput := len(cleanSymbols)
	const displayLimit = 5
	displayedSignals := signals
	if len(displayedSignals) > displayLimit {
		displayedSignals = displayedSignals[:displayLimit]
	}

	var tickerItems []model.RadarSignalItem
	for _, s := range displayedSignals {
		note := ""
		if s.CompositeScore >= 80 {
			note = "ada sinyal kuat"
		} else if s.CompositeScore < 60 {
			note = "tidak ada sinyal signifikan hari ini"
		}
		tickerItems = append(tickerItems, model.RadarSignalItem{
			Symbol:                   s.Symbol,
			SubSector:                s.SubSector,
			ForeignFlowScore:         s.ForeignFlowScore,
			InstitutionalBrokerScore: s.InstitutionalBrokerScore,
			VolumeScore:              s.VolumeScore,
			MomentumScore:            s.MomentumScore,
			BonusCorporateAction:     s.BonusCorporateAction,
			BonusQuarterlyReport:     s.BonusQuarterlyReport,
			BonusInsiderBuy:          s.BonusInsiderBuy,
			CompositeScore:           s.CompositeScore,
			IndicatorEmoji:           getIndicatorEmoji(s.CompositeScore),
			Note:                     note,
		})
	}

	var summaryNote string
	if totalInput <= 5 {
		summaryNote = fmt.Sprintf("%d dari %d ticker (≤5, semua ditampilkan)", len(tickerItems), totalInput)
	} else {
		summaryNote = fmt.Sprintf("%d ticker lain tidak menunjukkan sinyal signifikan hari ini", totalInput-5)
	}

	paramsBytes, _ := json.Marshal(map[string]interface{}{"symbols": cleanSymbols})
	_ = radarServiceImpl.radarRepository.LogRequest(gormTransaction, &entity.RadarRequestLog{
		UserID:              userID,
		RequestType:         "manual",
		Params:              string(paramsBytes),
		ResponseTickerCount: len(tickerItems),
	})

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "manual",
		TotalMonitored:  totalInput,
		Tickers:         tickerItems,
		SummaryNote:     summaryNote,
		Disclaimer:      radarDisclaimer,
	}, nil
}

// GetDrillDownExplanation returns the detailed signal breakdown for a single ticker (PRD §4.4).
func (radarServiceImpl *ServiceImpl) GetDrillDownExplanation(
	ctx context.Context,
	gormTransaction *gorm.DB,
	symbol string,
) (*model.RadarDrillDownResult, error) {
	upperSymbol := strings.ToUpper(strings.TrimSpace(symbol))
	date, err := radarServiceImpl.ensureDataReady(ctx, gormTransaction)
	if err != nil {
		return nil, err
	}

	explanation, err := radarServiceImpl.radarRepository.GetExplanation(gormTransaction, date, upperSymbol)
	var signal entity.SignalDaily
	signals, _ := radarServiceImpl.radarRepository.GetSignalsBySymbols(gormTransaction, date, []string{upperSymbol})
	if len(signals) > 0 {
		signal = signals[0]
	}

	var detectedSignals []string
	var additionalContext []string
	var newsList []model.NewsItem
	rawEvidence := make(map[string]interface{})

	if explanation != nil {
		_ = json.Unmarshal([]byte(explanation.EvidenceJSON), &rawEvidence)
		_ = json.Unmarshal([]byte(explanation.RelatedNews), &newsList)
	}

	// Build detected signals list for drill-down display (PRD §4.4)
	if signal.ForeignFlowScore > 0 {
		if signal.ForeignFlowScore >= 70 {
			detectedSignals = append(detectedSignals, fmt.Sprintf("🌍 Asing net inflow signifikan (persentil ke-%d hari ini)", signal.ForeignFlowScore))
		} else {
			detectedSignals = append(detectedSignals, fmt.Sprintf("🌍 Aliran asing netral/moderat (skor %d/100)", signal.ForeignFlowScore))
		}
	}
	if signal.InstitutionalBrokerScore > 0 {
		if netBuy, ok := rawEvidence["broker_net_buy_idr"].(float64); ok && netBuy > 0 {
			detectedSignals = append(detectedSignals, fmt.Sprintf("🏦 Broker institusi net beli Rp %.1f M (5 hari terakhir)", netBuy/1e9))
		} else {
			detectedSignals = append(detectedSignals, fmt.Sprintf("🏦 Broker institusi akumulasi skor %d/100", signal.InstitutionalBrokerScore))
		}
	}
	if signal.VolumeScore >= 60 {
		detectedSignals = append(detectedSignals, "📈 Volume transaksi mengalami lonjakan di atas rata-rata")
	}
	if len(newsList) > 0 {
		detectedSignals = append(detectedSignals, fmt.Sprintf("📰 Ada berita: \"%s\"", newsList[0].Title))
	}

	if len(detectedSignals) == 0 {
		detectedSignals = append(detectedSignals, "• Belum terdeteksi sinyal anomali besar hari ini")
	}

	// Additional context (PRD §4.4)
	if signal.BonusQuarterlyReport > 0 {
		additionalContext = append(additionalContext, "• Laporan keuangan kuartal baru saja dirilis")
	} else {
		additionalContext = append(additionalContext, "• Tidak ada laporan kuartal baru dalam 2 hari terakhir")
	}
	if signal.BonusCorporateAction > 0 {
		additionalContext = append(additionalContext, "• Terdapat aksi korporasi (dividen/RUPS) dalam 7 hari ke depan")
	} else {
		additionalContext = append(additionalContext, "• Tidak ada aksi korporasi dalam 7 hari ke depan")
	}

	summaryReason := ""
	if explanation != nil && explanation.SummaryReason != "" {
		summaryReason = explanation.SummaryReason
	} else {
		summaryReason = fmt.Sprintf("Saham %s menunjukkan skor komposit %d/100 berdasarkan data closing pasar.", upperSymbol, signal.CompositeScore)
	}

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarDrillDownResult{
		Symbol:            upperSymbol,
		Date:              time.Now().Format("02 Jan 2006"),
		DataClosingDate:   closingDate,
		CompositeScore:    signal.CompositeScore,
		Signals:           detectedSignals,
		AdditionalContext: additionalContext,
		SummaryReason:     summaryReason,
		News:              newsList,
		RawEvidence:       rawEvidence,
		Disclaimer:        radarDisclaimer,
	}, nil
}

// SetUserPreference saves the user's broadcast preference (PRD §4.5).
func (radarServiceImpl *ServiceImpl) SetUserPreference(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	req model.SetRadarPreferenceRequest,
) (*model.RadarPreferenceResponse, error) {
	broadcastEnabled := false
	if req.BroadcastEnabled != nil {
		broadcastEnabled = *req.BroadcastEnabled
	}

	broadcastTime := req.BroadcastTime
	if broadcastTime == "" {
		broadcastTime = "08:00"
	}

	pref := &entity.UserRadarPreference{
		UserID:             userID,
		Mode:               req.Mode,
		PreferredSubSector: req.PreferredSubSector,
		BroadcastEnabled:   broadcastEnabled,
		BroadcastTime:      broadcastTime,
	}

	err := radarServiceImpl.radarRepository.SaveUserPreference(gormTransaction, pref)
	if err != nil {
		return nil, err
	}

	return &model.RadarPreferenceResponse{
		UserID:             userID,
		Mode:               pref.Mode,
		PreferredSubSector: pref.PreferredSubSector,
		BroadcastEnabled:   pref.BroadcastEnabled,
		BroadcastTime:      pref.BroadcastTime,
	}, nil
}

// GetUserPreference retrieves the user's radar preference, returning sensible defaults if none set.
func (radarServiceImpl *ServiceImpl) GetUserPreference(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
) (*model.RadarPreferenceResponse, error) {
	pref, err := radarServiceImpl.radarRepository.GetUserPreference(gormTransaction, userID)
	if err != nil {
		return &model.RadarPreferenceResponse{
			UserID:             userID,
			Mode:               "subsector",
			PreferredSubSector: "Banks",
			BroadcastEnabled:   false,
			BroadcastTime:      "08:00",
		}, nil
	}
	return &model.RadarPreferenceResponse{
		UserID:             pref.UserID,
		Mode:               pref.Mode,
		PreferredSubSector: pref.PreferredSubSector,
		BroadcastEnabled:   pref.BroadcastEnabled,
		BroadcastTime:      pref.BroadcastTime,
	}, nil
}

// GetUserWatchlist returns all symbols in the user's watchlist.
func (radarServiceImpl *ServiceImpl) GetUserWatchlist(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
) (*model.WatchlistResponse, error) {
	items, err := radarServiceImpl.radarRepository.GetUserWatchlist(gormTransaction, userID)
	if err != nil {
		return nil, err
	}
	var symbols []string
	for _, item := range items {
		symbols = append(symbols, item.Symbol)
	}
	return &model.WatchlistResponse{
		UserID:  userID,
		Symbols: symbols,
	}, nil
}

// AddToWatchlist adds symbols to the user's watchlist, enforcing the PRD §5.4 max-10-ticker cap.
func (radarServiceImpl *ServiceImpl) AddToWatchlist(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	symbols []string,
) (*model.WatchlistResponse, error) {
	// Enforce watchlist cap: max 10 tickers per user (PRD §5.4)
	existing, err := radarServiceImpl.radarRepository.GetUserWatchlist(gormTransaction, userID)
	if err == nil {
		remaining := maxWatchlistPerUser - len(existing)
		if remaining <= 0 {
			return nil, fmt.Errorf("watchlist is full: maximum %d tickers allowed per user (PRD §5.4)", maxWatchlistPerUser)
		}
		if len(symbols) > remaining {
			symbols = symbols[:remaining]
			logrus.Warnf("Watchlist cap reached for user %d: only adding first %d symbols", userID, remaining)
		}
	}

	var items []entity.UserWatchlist
	for _, s := range symbols {
		cleaned := strings.TrimSpace(strings.ToUpper(s))
		if cleaned != "" {
			items = append(items, entity.UserWatchlist{
				UserID: userID,
				Symbol: cleaned,
			})
		}
	}
	if err := radarServiceImpl.radarRepository.AddUserWatchlist(gormTransaction, items); err != nil {
		return nil, err
	}
	return radarServiceImpl.GetUserWatchlist(ctx, gormTransaction, userID)
}

// RemoveFromWatchlist removes a single symbol from the user's watchlist.
func (radarServiceImpl *ServiceImpl) RemoveFromWatchlist(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	symbol string,
) (*model.WatchlistResponse, error) {
	if err := radarServiceImpl.radarRepository.RemoveUserWatchlist(gormTransaction, userID, strings.ToUpper(symbol)); err != nil {
		return nil, err
	}
	return radarServiceImpl.GetUserWatchlist(ctx, gormTransaction, userID)
}

// FormatRadarMessage formats a RadarResult into the WhatsApp message format (PRD §8.1).
// Note: Message text is intentionally in Indonesian as the product targets Indonesian users.
func (radarServiceImpl *ServiceImpl) FormatRadarMessage(result *model.RadarResult) string {
	var sb strings.Builder
	numberEmojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

	if result.Mode == "subsector" {
		icon := "🏦"
		if strings.EqualFold(result.SubSector, "food & beverage") {
			icon = "🍔"
		} else if strings.EqualFold(result.SubSector, "coal") || strings.EqualFold(result.SubSector, "energy") {
			icon = "⚡"
		} else if strings.EqualFold(result.SubSector, "telco") {
			icon = "📡"
		}
		sb.WriteString(fmt.Sprintf("%s *Radar Subsektor %s* — %s\n_(Data per closing %s)_\n\n", icon, result.SubSector, result.Date, result.DataClosingDate))
		sb.WriteString(fmt.Sprintf("Top %d dari %d saham yang dipantau:\n\n", len(result.Tickers), result.TotalMonitored))
	} else if result.Mode == "watchlist" {
		sb.WriteString(fmt.Sprintf("🔎 *Radar Watchlist Kamu* — %s\n\n", result.Date))
		if result.SummaryNote != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", result.SummaryNote))
		}
	} else {
		sb.WriteString(fmt.Sprintf("🔎 *Radar Saham* — %s\n\n", result.Date))
		if result.SummaryNote != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", result.SummaryNote))
		}
	}

	for i, ticker := range result.Tickers {
		emoji := numberEmojis[i%len(numberEmojis)]
		line := fmt.Sprintf("%s *%s* — Skor %d %s", emoji, ticker.Symbol, ticker.CompositeScore, ticker.IndicatorEmoji)
		if ticker.Note != "" {
			line += fmt.Sprintf(" (%s)", ticker.Note)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\nBalas angka (1-5) untuk melihat detail *\"Kenapa muncul?\"* atau tanyakan seputar saham di atas.")
	sb.WriteString("\n_(Ketik *batal* untuk kembali ke menu utama)_\n\n")
	sb.WriteString(result.Disclaimer)

	return sb.String()
}

// FormatDrillDownMessage formats a RadarDrillDownResult into the WhatsApp drill-down format (PRD §8.2).
// Note: Message text is intentionally in Indonesian as the product targets Indonesian users.
func (radarServiceImpl *ServiceImpl) FormatDrillDownMessage(result *model.RadarDrillDownResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📌 *%s* — Skor %d/100\n\n", result.Symbol, result.CompositeScore))

	sb.WriteString("*Sinyal terdeteksi:*\n")
	for _, sig := range result.Signals {
		sb.WriteString(fmt.Sprintf("• %s\n", sig))
	}

	sb.WriteString("\n*Konteks tambahan:*\n")
	for _, ctx := range result.AdditionalContext {
		sb.WriteString(fmt.Sprintf("%s\n", ctx))
	}

	sb.WriteString("\n" + result.Disclaimer)
	sb.WriteString("\n\nAnda dapat menanyakan analisis mendalam tentang saham ini, atau balas angka lain untuk cek saham berikutnya.")
	return sb.String()
}

// RunMorningBroadcast sends the daily Radar to all opted-in users (PRD §6.5, §9 NFR).
// It validates data freshness before broadcasting and applies rate limiting between messages.
func (radarServiceImpl *ServiceImpl) RunMorningBroadcast(
	ctx context.Context,
	broadcastFunc func(phone string, message string) error,
) error {
	logrus.Info("Starting Morning Radar Broadcast")

	// Staleness check: validate data is fresh before broadcasting (PRD §9 NFR)
	freshnessDate, err := radarServiceImpl.sectorsClient.CheckDataFreshness(ctx)
	if err != nil {
		logrus.Warn("Could not verify data freshness; skipping broadcast to avoid stale data")
		return fmt.Errorf("broadcast aborted: data freshness check failed: %w", err)
	}
	today := time.Now().Format("2006-01-02")
	if freshnessDate != today {
		logrus.Warnf("Data not updated for today (%s), latest available: %s. Skipping broadcast.", today, freshnessDate)
		return fmt.Errorf("broadcast aborted: data not yet updated for today (latest: %s)", freshnessDate)
	}

	var prefs []entity.UserRadarPreference
	err = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		prefs, err = radarServiceImpl.radarRepository.GetUsersWithBroadcastEnabled(tx)
		return err
	})
	if err != nil {
		return err
	}

	for _, pref := range prefs {
		var userEntity entity.User
		err := radarServiceImpl.dbConnection.Where("id = ?", pref.UserID).First(&userEntity).Error
		if err != nil || userEntity.Phone == "" {
			continue
		}

		var result *model.RadarResult
		_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
			if pref.Mode == "watchlist" {
				result, _ = radarServiceImpl.GetRadarByWatchlist(ctx, tx, pref.UserID)
			} else {
				subSector := pref.PreferredSubSector
				if subSector == "" {
					subSector = "Banks"
				}
				result, _ = radarServiceImpl.GetRadarBySubSector(ctx, tx, pref.UserID, subSector)
			}
			return nil
		})

		if result != nil {
			msg := "☀️ *Selamat Pagi! Radar Saham Harian Anda sudah siap:*\n\n" + radarServiceImpl.FormatRadarMessage(result)
			_ = broadcastFunc(userEntity.Phone, msg)
			// Rate limiting: delay between messages to avoid overwhelming the WA gateway (PRD §9 NFR)
			time.Sleep(200 * time.Millisecond)
		}
	}
	return nil
}
