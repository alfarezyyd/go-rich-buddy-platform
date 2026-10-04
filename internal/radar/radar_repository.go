package radar

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
)

type Repository interface {
	SaveSignals(tx *gorm.DB, signals []entity.SignalDaily) error
	GetTopSignalsBySubSector(tx *gorm.DB, date string, subSector string, limit int) ([]entity.SignalDaily, error)
	GetTopSignalsByWatchlist(tx *gorm.DB, date string, userID uint64, limit int) ([]entity.SignalDaily, error)
	GetSignalsBySymbols(tx *gorm.DB, date string, symbols []string) ([]entity.SignalDaily, error)
	GetShortlistedSignals(tx *gorm.DB, date string) ([]entity.SignalDaily, error)
	GetSubSectorCounts(tx *gorm.DB, date string, subSector string) (int64, error)
	GetLatestAvailableDate(tx *gorm.DB) (string, error)

	SaveExplanations(tx *gorm.DB, explanations []entity.TickerExplanationDaily) error
	GetExplanation(tx *gorm.DB, date string, symbol string) (*entity.TickerExplanationDaily, error)

	SaveUserPreference(tx *gorm.DB, pref *entity.UserRadarPreference) error
	GetUserPreference(tx *gorm.DB, userID uint64) (*entity.UserRadarPreference, error)
	GetUsersWithBroadcastEnabled(tx *gorm.DB) ([]entity.UserRadarPreference, error)

	GetUserWatchlist(tx *gorm.DB, userID uint64) ([]entity.UserWatchlist, error)
	AddUserWatchlist(tx *gorm.DB, items []entity.UserWatchlist) error
	RemoveUserWatchlist(tx *gorm.DB, userID uint64, symbol string) error
	GetAllActiveWatchlistSymbols(tx *gorm.DB) ([]string, error)

	LogRequest(tx *gorm.DB, log *entity.RadarRequestLog) error

	// PRD v2 Methods
	SaveDiscoveryCandidates(tx *gorm.DB, candidates []entity.DiscoveryCandidateDaily) error
	SaveGateResults(tx *gorm.DB, gates []entity.GateResultDaily) error
	SavePillarScores(tx *gorm.DB, scores []entity.PillarScoreDaily) error
	GetPillarScoresByDate(tx *gorm.DB, date string) ([]entity.PillarScoreDaily, error)
	GetTopPillarScoresBySubSector(tx *gorm.DB, date string, subSector string, limit int) ([]entity.PillarScoreDaily, error)
	GetPillarScoresBySymbols(tx *gorm.DB, date string, symbols []string) ([]entity.PillarScoreDaily, error)
	SaveCaseFile(tx *gorm.DB, caseFile *entity.CaseFileDaily) error
	GetCaseFile(tx *gorm.DB, date string, symbol string) (*entity.CaseFileDaily, error)
	LogRadarPick(tx *gorm.DB, pick *entity.RadarPickLog) error
}
