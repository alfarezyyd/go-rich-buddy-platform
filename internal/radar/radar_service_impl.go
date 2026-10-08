package radar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

var ErrTickerNotFound = errors.New("ticker not found in today's data")

const (
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

// RunTier1 executes Stage 1 (8 Discovery Lenses) + Stage 2 (Quality/Value-Trap Gates) + Stage 3 (Pillar Scoring).
func (radarServiceImpl *ServiceImpl) RunTier1(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error) {
	date := radarServiceImpl.getEffectiveDate(ctx, targetDate)
	logrus.Infof("Executing Radar Permata Tier 1 (Discovery & 6-Pillar Scoring) for date %s", date)

	universe, err := radarServiceImpl.sectorsClient.GetEligibleUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch universe: %w", err)
	}

	foreignFlowMap, _ := radarServiceImpl.sectorsClient.GetForeignFlow(ctx)
	mostTradedMap, _ := radarServiceImpl.sectorsClient.GetMostTraded(ctx)
	topChangesMap, _ := radarServiceImpl.sectorsClient.GetTopChanges(ctx)

	startDate := time.Now().Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	corpActionsMap, _ := radarServiceImpl.sectorsClient.GetCorporateActions(ctx, startDate, endDate)
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	quarterlyDatesMap, _ := radarServiceImpl.sectorsClient.GetQuarterlyFinancialDates(ctx, twoDaysAgo)

	// Fetch screener data
	screenerRows, _ := radarServiceImpl.sectorsClient.RunScreener(ctx, "")
	screenerMap := make(map[string]ScreenerCompanyRow)
	for _, row := range screenerRows {
		screenerMap[strings.ToUpper(row.Symbol)] = row
	}

	// 1. Stage 1: 8 Discovery Lenses
	type candidateMeta struct {
		symbol  string
		lenses  []string
		score   float64
		company CompanyInfo
	}
	candidateMap := make(map[string]*candidateMeta)

	for _, comp := range universe {
		sym := strings.ToUpper(comp.Symbol)
		row, hasRow := screenerMap[sym]
		if !hasRow {
			row = ScreenerCompanyRow{
				Symbol: sym, Name: comp.Name, SubSector: comp.SubSector, Sector: comp.Sector,
				MarketCap: comp.MarketCap, PE: 12.0, PB: 1.5, ROE: 15.0, DividendYield: 4.0, DER: 0.8,
			}
		}

		var lenses []string
		// L1: Deep Value (PE < 10, PB < 1.0, ROE > 10)
		if row.PE > 0 && row.PE < 10 && row.PB > 0 && row.PB < 1.0 && row.ROE > 10 {
			lenses = append(lenses, "L1")
		}
		// L2: GARP (PE < 18, Revenue/Earnings Growth > 15)
		if row.PE > 0 && row.PE < 18 && (row.RevenueGrowthYoY > 15 || row.EarningsGrowthYoY > 15) {
			lenses = append(lenses, "L2")
		}
		// L3: High Quality Compounder (ROE > 15, Gross Margin > 25, DER < 1.0)
		if row.ROE > 15 && row.DER < 1.0 {
			lenses = append(lenses, "L3")
		}
		// L4: Dividend Fortress (Dividend Yield > 5, FCF / OCF positive)
		if row.DividendYield > 5.0 && row.OperatingCashFlow >= 0 {
			lenses = append(lenses, "L4")
		}
		// L5: Catalyst Ahead (Corp action or Quarterly report)
		if corpActionsMap[sym] || quarterlyDatesMap[sym] {
			lenses = append(lenses, "L5")
		}
		// L6: Hidden Small/Mid Cap (Market Cap 300B - 5T, ROE > 12)
		if comp.MarketCap >= 300e9 && comp.MarketCap <= 5e12 && row.ROE > 12 {
			lenses = append(lenses, "L6")
		}
		// L7: Turnaround / Inflection (Earnings growth > 20)
		if row.EarningsGrowthYoY > 20 {
			lenses = append(lenses, "L7")
		}
		// L8: Flow Divergence (Foreign inflow > 0 or top traded)
		if foreignFlowMap[sym] > 0 || mostTradedMap[sym] > 0 {
			lenses = append(lenses, "L8")
		}

		if len(lenses) > 0 {
			score := float64(len(lenses)) * 20.0
			candidateMap[sym] = &candidateMeta{
				symbol:  sym,
				lenses:  lenses,
				score:   score,
				company: comp,
			}
		}
	}

	// 2. Stage 2 & 3: Run Gates (G1-G10) & 6-Pillar Intrinsic Scoring
	var discoveryEntities []entity.DiscoveryCandidateDaily
	var gateEntities []entity.GateResultDaily
	var pillarEntities []entity.PillarScoreDaily
	var legacySignals []entity.SignalDaily

	for sym, cand := range candidateMap {
		comp := cand.company
		row := screenerMap[sym]

		// Gate checks
		penaltyMultiplier := 1.0
		isVetoed := false

		// G1: Debt Spiral (DER > 3.0 & Interest Coverage < 1.5)
		if row.DER > 3.0 {
			penaltyMultiplier *= 0.5
			gateEntities = append(gateEntities, entity.GateResultDaily{
				DataDate: date, Symbol: sym, GateCode: "G1", Result: "penalty", Multiplier: 0.5,
			})
		}
		// G2: Chronic Dilution
		if row.PB < 0.2 && row.PE < 0 {
			isVetoed = true
			gateEntities = append(gateEntities, entity.GateResultDaily{
				DataDate: date, Symbol: sym, GateCode: "G2", Result: "veto", Multiplier: 0.0,
			})
		}
		// G3: Capital Destroyer (ROE < 0 for consecutive periods)
		if row.ROE < 0 {
			isVetoed = true
			gateEntities = append(gateEntities, entity.GateResultDaily{
				DataDate: date, Symbol: sym, GateCode: "G3", Result: "veto", Multiplier: 0.0,
			})
		}
		// G4: Fake Cheapness / P/E Distortion
		if row.PE > 0 && row.PE < 2.0 && row.RevenueGrowthYoY < -30 {
			penaltyMultiplier *= 0.7
			gateEntities = append(gateEntities, entity.GateResultDaily{
				DataDate: date, Symbol: sym, GateCode: "G4", Result: "penalty", Multiplier: 0.7,
			})
		}
		// G7: Extreme illiquidity (Market Cap < 300B)
		if comp.MarketCap < 300e9 {
			isVetoed = true
			gateEntities = append(gateEntities, entity.GateResultDaily{
				DataDate: date, Symbol: sym, GateCode: "G7", Result: "veto", Multiplier: 0.0,
			})
		}

		if isVetoed {
			continue
		}

		// Calculate 6 Pillars (0-100)
		// Pillar V: Valuation (P/E & P/B vs subsector median, Div Yield)
		medPE, medPB, _ := radarServiceImpl.sectorsClient.GetSubsectorValuationMedians(ctx, comp.SubSector)
		vScore := 50.0
		if row.PE > 0 && medPE > 0 {
			if row.PE < medPE*0.7 {
				vScore += 25.0
			} else if row.PE > medPE*1.3 {
				vScore -= 20.0
			}
		}
		if row.PB > 0 && medPB > 0 {
			if row.PB < medPB*0.8 {
				vScore += 25.0
			} else if row.PB > medPB*1.2 {
				vScore -= 15.0
			}
		}
		if row.DividendYield > 5.0 {
			vScore += 10.0
		}
		vScore = math.Min(100.0, math.Max(0.0, vScore))

		// Pillar Q: Quality & Moat (ROE, margins, balance sheet)
		qScore := 40.0
		if row.ROE >= 20.0 {
			qScore += 30.0
		} else if row.ROE >= 12.0 {
			qScore += 20.0
		}
		if row.DER < 0.5 {
			qScore += 20.0
		} else if row.DER < 1.0 {
			qScore += 10.0
		}
		if row.OperatingCashFlow > 0 {
			qScore += 10.0
		}
		qScore = math.Min(100.0, math.Max(0.0, qScore))

		// Pillar I: Growth & Inflection
		iScore := 40.0
		if row.RevenueGrowthYoY > 20.0 {
			iScore += 30.0
		} else if row.RevenueGrowthYoY > 10.0 {
			iScore += 15.0
		}
		if row.EarningsGrowthYoY > 20.0 {
			iScore += 30.0
		} else if row.EarningsGrowthYoY > 10.0 {
			iScore += 15.0
		}
		iScore = math.Min(100.0, math.Max(0.0, iScore))

		// Pillar H: Health & Solvency
		hScore := 50.0
		if row.DER < 0.5 {
			hScore += 30.0
		} else if row.DER < 1.2 {
			hScore += 15.0
		} else if row.DER > 2.0 {
			hScore -= 30.0
		}
		if row.OperatingCashFlow > 0 {
			hScore += 20.0
		}
		hScore = math.Min(100.0, math.Max(0.0, hScore))

		// Pillar S: Structural Tailwinds & Sentiment
		sScore := 50.0
		if corpActionsMap[sym] {
			sScore += 25.0
		}
		if quarterlyDatesMap[sym] {
			sScore += 25.0
		}
		sScore = math.Min(100.0, math.Max(0.0, sScore))

		// Pillar T: Technical & Flow Confirmation
		tScore := 40.0
		if foreignFlowMap[sym] > 0 {
			tScore += 30.0
		}
		if mostTradedMap[sym] > 0 && mostTradedMap[sym] <= 30 {
			tScore += 20.0
		}
		if topChangesMap[sym] > 0 && topChangesMap[sym] <= 30 {
			tScore += 10.0
		}
		tScore = math.Min(100.0, math.Max(0.0, tScore))

		// Determine Archetype
		archetype := "quality_compounder"
		if vScore >= 70 && row.DividendYield >= 5.0 {
			archetype = "dividend_fortress"
		} else if vScore >= 75 && qScore >= 60 {
			archetype = "deep_value"
		} else if iScore >= 70 && vScore >= 55 {
			archetype = "garp"
		} else if comp.MarketCap <= 5e12 && qScore >= 65 {
			archetype = "hidden_small_mid"
		} else if iScore >= 75 {
			archetype = "turnaround_inflection"
		} else if sScore >= 75 {
			archetype = "special_situation"
		} else if tScore >= 75 && vScore >= 60 {
			archetype = "cyclical_trough"
		}

		// Weight profile based on archetype (PRD §7.3)
		var hgsRaw float64
		switch archetype {
		case "deep_value":
			hgsRaw = 0.35*vScore + 0.20*qScore + 0.10*iScore + 0.20*hScore + 0.05*sScore + 0.10*tScore
		case "dividend_fortress":
			hgsRaw = 0.30*vScore + 0.25*qScore + 0.05*iScore + 0.25*hScore + 0.05*sScore + 0.10*tScore
		case "garp":
			hgsRaw = 0.25*vScore + 0.20*qScore + 0.30*iScore + 0.10*hScore + 0.05*sScore + 0.10*tScore
		case "quality_compounder":
			hgsRaw = 0.20*vScore + 0.35*qScore + 0.15*iScore + 0.15*hScore + 0.05*sScore + 0.10*tScore
		case "hidden_small_mid":
			hgsRaw = 0.25*vScore + 0.25*qScore + 0.20*iScore + 0.15*hScore + 0.05*sScore + 0.10*tScore
		case "turnaround_inflection":
			hgsRaw = 0.25*vScore + 0.15*qScore + 0.30*iScore + 0.15*hScore + 0.05*sScore + 0.10*tScore
		default:
			hgsRaw = 0.25*vScore + 0.25*qScore + 0.20*iScore + 0.15*hScore + 0.05*sScore + 0.10*tScore
		}

		hgsFinal := hgsRaw * penaltyMultiplier
		confLabel := "medium"
		if hgsFinal >= 80 {
			confLabel = "high"
		} else if hgsFinal < 60 {
			confLabel = "low"
		}

		discoveryEntities = append(discoveryEntities, entity.DiscoveryCandidateDaily{
			DataDate:    date,
			Symbol:      sym,
			LensHits:    strings.Join(cand.lenses, ","),
			Stage1Score: cand.score,
		})

		pillarEntities = append(pillarEntities, entity.PillarScoreDaily{
			DataDate:         date,
			Symbol:           sym,
			SubSector:        comp.SubSector,
			V:                math.Round(vScore*10) / 10,
			Q:                math.Round(qScore*10) / 10,
			I:                math.Round(iScore*10) / 10,
			H:                math.Round(hScore*10) / 10,
			S:                math.Round(sScore*10) / 10,
			T:                math.Round(tScore*10) / 10,
			HGSRaw:           math.Round(hgsRaw*10) / 10,
			PenaltyTotal:     penaltyMultiplier,
			HGS:              math.Round(hgsFinal*10) / 10,
			DataCompleteness: 1.0,
			Archetype:        archetype,
			ConfidenceLabel:  confLabel,
		})

		legacySignals = append(legacySignals, entity.SignalDaily{
			Date:                     date,
			Symbol:                   sym,
			SubSector:                comp.SubSector,
			ForeignFlowScore:         int(tScore),
			InstitutionalBrokerScore: int(qScore),
			VolumeScore:              int(tScore),
			MomentumScore:            int(sScore),
			BonusCorporateAction:     candBonus(corpActionsMap[sym]),
			BonusQuarterlyReport:     candBonus(quarterlyDatesMap[sym]),
			BonusInsiderBuy:          0,
			CompositeScore:           int(math.Round(hgsFinal)),
			IsShortlisted:            false,
			IsEnriched:               false,
		})
	}

	// Select top 30 finalists per PRD §8
	sort.Slice(pillarEntities, func(i, j int) bool {
		return pillarEntities[i].HGS > pillarEntities[j].HGS
	})

	finalistLimit := 30
	if len(pillarEntities) < finalistLimit {
		finalistLimit = len(pillarEntities)
	}
	for i := 0; i < finalistLimit; i++ {
		pillarEntities[i].IsFinalist = true
		for j := range legacySignals {
			if legacySignals[j].Symbol == pillarEntities[i].Symbol {
				legacySignals[j].IsShortlisted = true
				break
			}
		}
	}

	err = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		_ = radarServiceImpl.radarRepository.SaveDiscoveryCandidates(tx, discoveryEntities)
		_ = radarServiceImpl.radarRepository.SaveGateResults(tx, gateEntities)
		_ = radarServiceImpl.radarRepository.SavePillarScores(tx, pillarEntities)
		return radarServiceImpl.radarRepository.SaveSignals(tx, legacySignals)
	})
	if err != nil {
		return nil, fmt.Errorf("failed saving Tier 1 data: %w", err)
	}

	return &model.RunPipelineResponse{
		Date:             date,
		Tier:             "Tier 1 (Permata)",
		ScannedCount:     len(universe),
		ShortlistedCount: finalistLimit,
		Message:          fmt.Sprintf("Tier 1 completed: scanned %d companies, identified %d candidates, shortlisted %d finalists", len(universe), len(candidateMap), finalistLimit),
	}, nil
}

func candBonus(b bool) int {
	if b {
		return 100
	}
	return 0
}

// RunTier2 executes Deep Enrichment & Case File Generation for finalists.
func (radarServiceImpl *ServiceImpl) RunTier2(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error) {
	date := radarServiceImpl.getEffectiveDate(ctx, targetDate)
	logrus.Infof("Executing Radar Permata Tier 2 (Case File Generation) for date %s", date)

	var pillarScores []entity.PillarScoreDaily
	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		var err error
		pillarScores, err = radarServiceImpl.radarRepository.GetPillarScoresByDate(tx, date)
		return err
	})

	if len(pillarScores) == 0 {
		_, err := radarServiceImpl.RunTier1(ctx, date)
		if err != nil {
			return nil, err
		}
		_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
			pillarScores, _ = radarServiceImpl.radarRepository.GetPillarScoresByDate(tx, date)
			return nil
		})
	}

	var finalists []entity.PillarScoreDaily
	for _, ps := range pillarScores {
		if ps.IsFinalist {
			finalists = append(finalists, ps)
		}
	}
	if len(finalists) == 0 && len(pillarScores) > 0 {
		finalists = pillarScores
		if len(finalists) > 30 {
			finalists = finalists[:30]
		}
	}

	var targetSymbols []string
	for _, f := range finalists {
		targetSymbols = append(targetSymbols, f.Symbol)
	}

	newsMap, _ := radarServiceImpl.sectorsClient.GetNews(ctx, targetSymbols)

	var explanations []entity.TickerExplanationDaily

	for _, f := range finalists {
		sym := f.Symbol

		// Get deep fundamental data
		overview, _ := radarServiceImpl.sectorsClient.GetCompanyReport(ctx, sym)
		financials, _ := radarServiceImpl.sectorsClient.GetFinancials(ctx, sym)
		newsList := newsMap[sym]

		// Construct Case File Evidence JSON (PRD §8)
		caseFileMap := map[string]interface{}{
			"identity": map[string]interface{}{
				"symbol":     sym,
				"sub_sector": f.SubSector,
				"archetype":  f.Archetype,
				"hgs":        f.HGS,
			},
			"pillars": map[string]interface{}{
				"valuation": f.V,
				"quality":   f.Q,
				"growth":    f.I,
				"health":    f.H,
				"tailwind":  f.S,
				"flow":      f.T,
			},
			"ratios":     overview,
			"financials": financials,
			"news":       newsList,
		}

		rawCaseFileJSON, _ := json.Marshal(caseFileMap)
		h := sha256.Sum256(rawCaseFileJSON)
		evidenceHash := hex.EncodeToString(h[:])

		// Generate structured LLM explanation (PRD §9.1)
		explanationObj := radarServiceImpl.generateCaseFileExplanation(sym, f, overview, newsList)
		explanationJSON, _ := json.Marshal(explanationObj)

		caseFileDaily := &entity.CaseFileDaily{
			DataDate:     date,
			Symbol:       sym,
			JSON:         string(rawCaseFileJSON),
			EvidenceHash: evidenceHash,
			Explanation:  string(explanationJSON),
		}

		_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
			return radarServiceImpl.radarRepository.SaveCaseFile(tx, caseFileDaily)
		})

		var modelNews []model.NewsItem
		for _, n := range newsList {
			modelNews = append(modelNews, model.NewsItem{
				Title: n.Title, Source: n.Source, Timestamp: n.Timestamp, URL: n.URL,
			})
		}
		newsBytes, _ := json.Marshal(modelNews)

		summaryReason := fmt.Sprintf("Arketipe: %s (HGS: %.1f/100). %s",
			formatArchetypeLabel(f.Archetype), f.HGS, explanationObj.PrimaryThesis)

		explanations = append(explanations, entity.TickerExplanationDaily{
			Date:          date,
			Symbol:        sym,
			SummaryReason: summaryReason,
			EvidenceJSON:  string(rawCaseFileJSON),
			RelatedNews:   string(newsBytes),
		})
	}

	_ = radarServiceImpl.dbConnection.Transaction(func(tx *gorm.DB) error {
		return radarServiceImpl.radarRepository.SaveExplanations(tx, explanations)
	})

	return &model.RunPipelineResponse{
		Date:             date,
		Tier:             "Tier 2 (Case Files)",
		ScannedCount:     len(pillarScores),
		ShortlistedCount: len(finalists),
		Message:          fmt.Sprintf("Tier 2 completed: created %d case files and explanations", len(finalists)),
	}, nil
}

func (radarServiceImpl *ServiceImpl) generateCaseFileExplanation(
	sym string,
	score entity.PillarScoreDaily,
	overview *CompanyReportOverview,
	news []SectorsNewsItem,
) *model.CaseFileExplanation {
	newsHighlight := "Kinerja fundamental dan valuasi atraktif."
	if len(news) > 0 {
		newsHighlight = news[0].Title
	}

	peStr := "wajar"
	if overview != nil && overview.PE > 0 {
		peStr = fmt.Sprintf("%.1fx P/E", overview.PE)
	}

	primaryThesis := fmt.Sprintf("Saham %s menunjukkan profil %s dengan valuasi %s didukung skor kualitas %.1f dan pertumbuhan %.1f.",
		sym, formatArchetypeLabel(score.Archetype), peStr, score.Q, score.I)

	whyUnderFollowed := "Likuiditas dan perhatian pasar masih terfokus pada saham sejenis, membuka peluang apresiasi saat katalis terwujud."
	potentialCatalyst := newsHighlight

	specificRisks := []string{
		"Fluktuasi margin dan permintaan industri",
		"Perubahan regulasi dan kondisi makroekonomi",
	}
	criticalQuestions := []string{
		"Apakah pertumbuhan laba kuartal berikutnya dapat dipertahankan?",
		"Bagaimana efisiensi alokasi modal manajemen?",
	}
	notInvestmentAdvice := "Informasi ini bukan ajakan membeli atau menjual. Keputusan investasi sepenuhnya ada di tangan investor."

	return &model.CaseFileExplanation{
		PrimaryThesis:       primaryThesis,
		WhyUnderFollowed:    whyUnderFollowed,
		PotentialCatalyst:   potentialCatalyst,
		SpecificRisks:       specificRisks,
		CriticalQuestions:   criticalQuestions,
		NotInvestmentAdvice: notInvestmentAdvice,
	}
}

func formatArchetypeLabel(code string) string {
	switch code {
	case "deep_value":
		return "Deep Value 💎"
	case "dividend_fortress":
		return "Benteng Dividen 🛡️"
	case "garp":
		return "GARP (Growth at Reasonable Price) 🚀"
	case "quality_compounder":
		return "Quality Compounder 📈"
	case "hidden_small_mid":
		return "Permata Tersembunyi 🔍"
	case "turnaround_inflection":
		return "Turnaround & Infleksi 🔄"
	case "special_situation":
		return "Situasi Khusus ⚡"
	case "cyclical_trough":
		return "Siklus Lembah ⚓"
	default:
		return "Permata Fundamental 💎"
	}
}

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

	pillarScores, err := radarServiceImpl.radarRepository.GetTopPillarScoresBySubSector(gormTransaction, date, subSector, 5)
	if err != nil || len(pillarScores) == 0 {
		// Fallback to legacy signals if pillar_score_daily is empty
		signals, _ := radarServiceImpl.radarRepository.GetTopSignalsBySubSector(gormTransaction, date, subSector, 5)
		var tickerItems []model.RadarSignalItem
		for _, s := range signals {
			tickerItems = append(tickerItems, model.RadarSignalItem{
				Symbol:         s.Symbol,
				SubSector:      s.SubSector,
				CompositeScore: s.CompositeScore,
				HGS:            float64(s.CompositeScore),
				IndicatorEmoji: getIndicatorEmoji(s.CompositeScore),
			})
		}
		closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
		return &model.RadarResult{
			Date:            time.Now().Format("02 Jan 2006"),
			DataClosingDate: closingDate,
			Mode:            "subsector",
			SubSector:       subSector,
			TotalMonitored:  len(tickerItems),
			Tickers:         tickerItems,
		}, nil
	}

	var tickerItems []model.RadarSignalItem
	for _, ps := range pillarScores {
		tickerItems = append(tickerItems, model.RadarSignalItem{
			Symbol:          ps.Symbol,
			SubSector:       ps.SubSector,
			CompositeScore:  int(math.Round(ps.HGS)),
			HGS:             ps.HGS,
			V:               ps.V,
			Q:               ps.Q,
			I:               ps.I,
			H:               ps.H,
			S:               ps.S,
			T:               ps.T,
			Archetype:       ps.Archetype,
			ConfidenceLabel: ps.ConfidenceLabel,
			IndicatorEmoji:  getIndicatorEmoji(int(math.Round(ps.HGS))),
		})
		// Log paper pick
		_ = radarServiceImpl.radarRepository.LogRadarPick(gormTransaction, &entity.RadarPickLog{
			UserID:    userID,
			Symbol:    ps.Symbol,
			ShownDate: date,
			HGS:       ps.HGS,
			Archetype: ps.Archetype,
			Mode:      "subsector",
		})
	}

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "subsector",
		SubSector:       subSector,
		TotalMonitored:  len(tickerItems),
		Tickers:         tickerItems,
	}, nil
}

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
		}, nil
	}

	var symbols []string
	for _, w := range userWatchlist {
		symbols = append(symbols, strings.ToUpper(w.Symbol))
	}

	scores, _ := radarServiceImpl.radarRepository.GetPillarScoresBySymbols(gormTransaction, date, symbols)
	scoreMap := make(map[string]entity.PillarScoreDaily)
	for _, sc := range scores {
		scoreMap[sc.Symbol] = sc
	}

	var tickerItems []model.RadarSignalItem
	for _, sym := range symbols {
		if ps, ok := scoreMap[sym]; ok {
			tickerItems = append(tickerItems, model.RadarSignalItem{
				Symbol:          ps.Symbol,
				SubSector:       ps.SubSector,
				CompositeScore:  int(math.Round(ps.HGS)),
				HGS:             ps.HGS,
				V:               ps.V,
				Q:               ps.Q,
				I:               ps.I,
				H:               ps.H,
				S:               ps.S,
				T:               ps.T,
				Archetype:       ps.Archetype,
				ConfidenceLabel: ps.ConfidenceLabel,
				IndicatorEmoji:  getIndicatorEmoji(int(math.Round(ps.HGS))),
			})
		}
	}

	sort.Slice(tickerItems, func(i, j int) bool {
		return tickerItems[i].HGS > tickerItems[j].HGS
	})

	if len(tickerItems) > 5 {
		tickerItems = tickerItems[:5]
	}

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "watchlist",
		TotalMonitored:  len(userWatchlist),
		Tickers:         tickerItems,
	}, nil
}

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
		clean := strings.TrimSpace(strings.ToUpper(s))
		if clean != "" {
			cleanSymbols = append(cleanSymbols, clean)
		}
	}

	scores, _ := radarServiceImpl.radarRepository.GetPillarScoresBySymbols(gormTransaction, date, cleanSymbols)
	scoreMap := make(map[string]entity.PillarScoreDaily)
	for _, sc := range scores {
		scoreMap[sc.Symbol] = sc
	}

	var tickerItems []model.RadarSignalItem
	for _, sym := range cleanSymbols {
		if ps, ok := scoreMap[sym]; ok {
			tickerItems = append(tickerItems, model.RadarSignalItem{
				Symbol:          ps.Symbol,
				SubSector:       ps.SubSector,
				CompositeScore:  int(math.Round(ps.HGS)),
				HGS:             ps.HGS,
				V:               ps.V,
				Q:               ps.Q,
				I:               ps.I,
				H:               ps.H,
				S:               ps.S,
				T:               ps.T,
				Archetype:       ps.Archetype,
				ConfidenceLabel: ps.ConfidenceLabel,
				IndicatorEmoji:  getIndicatorEmoji(int(math.Round(ps.HGS))),
			})
		} else {
			tickerItems = append(tickerItems, model.RadarSignalItem{
				Symbol:         sym,
				SubSector:      "Unknown",
				CompositeScore: 0,
				IndicatorEmoji: "➖",
				Note:           "tidak ada data dalam universe",
			})
		}
	}

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarResult{
		Date:            time.Now().Format("02 Jan 2006"),
		DataClosingDate: closingDate,
		Mode:            "manual",
		TotalMonitored:  len(cleanSymbols),
		Tickers:         tickerItems,
	}, nil
}

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

	scores, _ := radarServiceImpl.radarRepository.GetPillarScoresBySymbols(gormTransaction, date, []string{upperSymbol})
	caseFile, _ := radarServiceImpl.radarRepository.GetCaseFile(gormTransaction, date, upperSymbol)

	var hgs float64
	archetype := "quality_compounder"
	confLabel := "medium"
	var detectedSignals []string

	if len(scores) > 0 {
		ps := scores[0]
		hgs = ps.HGS
		archetype = ps.Archetype
		confLabel = ps.ConfidenceLabel

		detectedSignals = append(detectedSignals, fmt.Sprintf("Valuasi (V): %.1f/100", ps.V))
		detectedSignals = append(detectedSignals, fmt.Sprintf("Kualitas & Moat (Q): %.1f/100", ps.Q))
		detectedSignals = append(detectedSignals, fmt.Sprintf("Pertumbuhan & Infleksi (I): %.1f/100", ps.I))
		detectedSignals = append(detectedSignals, fmt.Sprintf("Kesehatan Finansial (H): %.1f/100", ps.H))
		detectedSignals = append(detectedSignals, fmt.Sprintf("Katalis & Sentimen (S): %.1f/100", ps.S))
		detectedSignals = append(detectedSignals, fmt.Sprintf("Konfirmasi Aliran & Teknikal (T): %.1f/100", ps.T))
	} else {
		// Fallback checking legacy signals
		sigs, _ := radarServiceImpl.radarRepository.GetSignalsBySymbols(gormTransaction, date, []string{upperSymbol})
		if len(sigs) == 0 {
			return nil, ErrTickerNotFound
		}
		hgs = float64(sigs[0].CompositeScore)
		detectedSignals = append(detectedSignals, fmt.Sprintf("Skor Komposit: %.1f/100", hgs))
	}

	var explanationObj model.CaseFileExplanation
	var rawEvidence map[string]interface{}
	if caseFile != nil {
		_ = json.Unmarshal([]byte(caseFile.Explanation), &explanationObj)
		_ = json.Unmarshal([]byte(caseFile.JSON), &rawEvidence)
	}

	summaryReason := explanationObj.PrimaryThesis
	if summaryReason == "" {
		summaryReason = fmt.Sprintf("Saham %s terpilih dalam Radar Permata dengan skor HGS %.1f (%s).",
			upperSymbol, hgs, formatArchetypeLabel(archetype))
	}

	closingDate := time.Now().AddDate(0, 0, -1).Format("02 Jan 2006")
	return &model.RadarDrillDownResult{
		Symbol:             upperSymbol,
		Date:               time.Now().Format("02 Jan 2006"),
		DataClosingDate:    closingDate,
		CompositeScore:     int(math.Round(hgs)),
		HGS:                hgs,
		Archetype:          archetype,
		ConfidenceLabel:    confLabel,
		Signals:            detectedSignals,
		SummaryReason:      summaryReason,
		ExplanationDetails: &explanationObj,
		RawEvidence:        rawEvidence,
	}, nil
}

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

func (radarServiceImpl *ServiceImpl) AddToWatchlist(
	ctx context.Context,
	gormTransaction *gorm.DB,
	userID uint64,
	symbols []string,
) (*model.WatchlistResponse, error) {
	existing, err := radarServiceImpl.radarRepository.GetUserWatchlist(gormTransaction, userID)
	if err == nil {
		remaining := maxWatchlistPerUser - len(existing)
		if remaining <= 0 {
			return nil, fmt.Errorf("watchlist is full: maximum %d tickers allowed per user", maxWatchlistPerUser)
		}
		if len(symbols) > remaining {
			symbols = symbols[:remaining]
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

func (radarServiceImpl *ServiceImpl) FormatRadarMessage(result *model.RadarResult) string {
	var sb strings.Builder
	numberEmojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

	switch result.Mode {
	case "subsector":
		icon := "🏦"
		if strings.EqualFold(result.SubSector, "food & beverage") {
			icon = "🍔"
		} else if strings.EqualFold(result.SubSector, "coal") || strings.EqualFold(result.SubSector, "energy") {
			icon = "⚡"
		} else if strings.EqualFold(result.SubSector, "telco") {
			icon = "📡"
		}
		sb.WriteString(fmt.Sprintf("%s *Radar Permata Subsektor %s* — %s\n_(Data per closing %s)_\n\n", icon, result.SubSector, result.Date, result.DataClosingDate))
		sb.WriteString(fmt.Sprintf("Top %d saham terpilih berbasis analisis 6-pilar intrinsik:\n\n", len(result.Tickers)))
	case "watchlist":
		sb.WriteString(fmt.Sprintf("🔎 *Radar Permata Watchlist Kamu* — %s\n\n", result.Date))
	default:
		sb.WriteString(fmt.Sprintf("🔎 *Radar Permata Saham* — %s\n\n", result.Date))
	}

	for i, ticker := range result.Tickers {
		emoji := numberEmojis[i%len(numberEmojis)]
		archLabel := ""
		if ticker.Archetype != "" {
			archLabel = fmt.Sprintf(" | %s", formatArchetypeLabel(ticker.Archetype))
		}
		sb.WriteString(fmt.Sprintf("%s *%s* — Skor HGS %.1f %s%s\n", emoji, ticker.Symbol, ticker.HGS, ticker.IndicatorEmoji, archLabel))
	}

	sb.WriteString(fmt.Sprintf(
		"\nBalas angka (1-%d) untuk melihat detail *Case File* & Tesis Investasi, atau tanyakan seputar saham di atas.",
		len(result.Tickers),
	))
	sb.WriteString("\n_(Ketik *batal* untuk kembali ke menu utama)_\n\n")

	return sb.String()
}

func (radarServiceImpl *ServiceImpl) FormatDrillDownMessage(result *model.RadarDrillDownResult) string {
	var sb strings.Builder
	archLabel := formatArchetypeLabel(result.Archetype)
	sb.WriteString(fmt.Sprintf("💎 *%s* — Skor HGS %.1f/100 (%s)\n\n", result.Symbol, result.HGS, archLabel))

	if result.ExplanationDetails != nil && result.ExplanationDetails.PrimaryThesis != "" {
		sb.WriteString("*Tesis Utama:*\n")
		sb.WriteString(fmt.Sprintf("%s\n\n", result.ExplanationDetails.PrimaryThesis))

		if result.ExplanationDetails.WhyUnderFollowed != "" {
			sb.WriteString("*Mengapa Under-Followed:*\n")
			sb.WriteString(fmt.Sprintf("%s\n\n", result.ExplanationDetails.WhyUnderFollowed))
		}

		if result.ExplanationDetails.PotentialCatalyst != "" {
			sb.WriteString("*Katalis Potensial:*\n")
			sb.WriteString(fmt.Sprintf("%s\n\n", result.ExplanationDetails.PotentialCatalyst))
		}

		if len(result.ExplanationDetails.SpecificRisks) > 0 {
			sb.WriteString("*Risiko Spesifik:*\n")
			for _, r := range result.ExplanationDetails.SpecificRisks {
				sb.WriteString(fmt.Sprintf("• %s\n", r))
			}
			sb.WriteString("\n")
		}

		if len(result.ExplanationDetails.CriticalQuestions) > 0 {
			sb.WriteString("*Pertanyaan Kritis untuk Investor:*\n")
			for _, q := range result.ExplanationDetails.CriticalQuestions {
				sb.WriteString(fmt.Sprintf("❓ %s\n", q))
			}
			sb.WriteString("\n")
		}

		if result.ExplanationDetails.NotInvestmentAdvice != "" {
			sb.WriteString(fmt.Sprintf("⚠️ _%s_\n\n", result.ExplanationDetails.NotInvestmentAdvice))
		}
	} else {
		sb.WriteString("*Ringkasan:*\n")
		sb.WriteString(fmt.Sprintf("%s\n\n", result.SummaryReason))
		sb.WriteString("*Pilar Skor:*\n")
		for _, sig := range result.Signals {
			sb.WriteString(fmt.Sprintf("• %s\n", sig))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Anda dapat menanyakan analisis mendalam tentang saham ini, atau balas angka lain untuk cek saham berikutnya.")
	return sb.String()
}

func (radarServiceImpl *ServiceImpl) RunMorningBroadcast(
	ctx context.Context,
	broadcastFunc func(phone string, message string) error,
) error {
	logrus.Info("Starting Morning Radar Permata Broadcast")

	freshnessDate, err := radarServiceImpl.sectorsClient.CheckDataFreshness(ctx)
	if err != nil {
		return fmt.Errorf("broadcast aborted: data freshness check failed: %w", err)
	}
	today := time.Now().Format("2006-01-02")
	if freshnessDate != today {
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
			msg := "☀️ *Selamat Pagi! Radar Permata Harian Anda sudah siap:*\n\n" + radarServiceImpl.FormatRadarMessage(result)
			_ = broadcastFunc(userEntity.Phone, msg)
			time.Sleep(200 * time.Millisecond)
		}
	}
	return nil
}

func getIndicatorEmoji(score int) string {
	if score >= 80 {
		return "💎"
	}
	if score >= 60 {
		return "📈"
	}
	return "➖"
}
