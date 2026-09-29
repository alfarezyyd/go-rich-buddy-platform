package radar

import (
	"go-rich-buddy-platform/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RepositoryImpl struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (radarRepositoryImpl *RepositoryImpl) SaveSignals(gormTransaction *gorm.DB, signals []entity.SignalDaily) error {
	if len(signals) == 0 {
		return nil
	}
	return gormTransaction.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sub_sector", "foreign_flow_score", "institutional_broker_score", "volume_score",
			"momentum_score", "bonus_corporate_action", "bonus_quarterly_report",
			"bonus_insider_buy", "composite_score", "is_shortlisted", "is_enriched",
		}),
	}).CreateInBatches(signals, 100).Error
}

func (radarRepositoryImpl *RepositoryImpl) GetTopSignalsBySubSector(gormTransaction *gorm.DB, date string, subSector string, limit int) ([]entity.SignalDaily, error) {
	var signalEntities []entity.SignalDaily
	query := gormTransaction.Where("date = ?", date)
	if subSector != "" && subSector != "all" {
		query = query.Where("LOWER(sub_sector) = LOWER(?)", subSector)
	}
	err := query.Order("composite_score DESC").
		Limit(limit).
		Find(&signalEntities).Error
	return signalEntities, err
}

func (radarRepositoryImpl *RepositoryImpl) GetTopSignalsByWatchlist(gormTransaction *gorm.DB, date string, userID uint64, limit int) ([]entity.SignalDaily, error) {
	var signalEntities []entity.SignalDaily
	query := gormTransaction.Table("signal_daily").
		Joins("JOIN user_watchlist ON UPPER(user_watchlist.symbol) = UPPER(signal_daily.symbol)").
		Where("signal_daily.date = ? AND user_watchlist.user_id = ?", date, userID).
		Order("signal_daily.composite_score DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&signalEntities).Error
	return signalEntities, err
}

func (radarRepositoryImpl *RepositoryImpl) GetSignalsBySymbols(gormTransaction *gorm.DB, date string, symbols []string) ([]entity.SignalDaily, error) {
	var signalEntities []entity.SignalDaily
	if len(symbols) == 0 {
		return signalEntities, nil
	}
	err := gormTransaction.Where("date = ? AND UPPER(symbol) IN (?)", date, symbols).
		Order("composite_score DESC").
		Find(&signalEntities).Error
	return signalEntities, err
}

func (radarRepositoryImpl *RepositoryImpl) GetShortlistedSignals(gormTransaction *gorm.DB, date string) ([]entity.SignalDaily, error) {
	var signalEntities []entity.SignalDaily
	err := gormTransaction.Where("date = ? AND is_shortlisted = ?", date, true).
		Order("composite_score DESC").
		Find(&signalEntities).Error
	return signalEntities, err
}

func (radarRepositoryImpl *RepositoryImpl) GetSubSectorCounts(gormTransaction *gorm.DB, date string, subSector string) (int64, error) {
	var count int64
	query := gormTransaction.Model(&entity.SignalDaily{}).Where("date = ?", date)
	if subSector != "" && subSector != "all" {
		query = query.Where("LOWER(sub_sector) = LOWER(?)", subSector)
	}
	err := query.Count(&count).Error
	return count, err
}

func (radarRepositoryImpl *RepositoryImpl) GetLatestAvailableDate(gormTransaction *gorm.DB) (string, error) {
	var signal entity.SignalDaily
	err := gormTransaction.Order("date DESC").First(&signal).Error
	if err != nil {
		return "", err
	}
	return signal.Date, nil
}

func (radarRepositoryImpl *RepositoryImpl) SaveExplanations(gormTransaction *gorm.DB, explanations []entity.TickerExplanationDaily) error {
	if len(explanations) == 0 {
		return nil
	}
	return gormTransaction.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}, {Name: "symbol"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"summary_reason", "evidence_json", "related_news",
		}),
	}).CreateInBatches(explanations, 100).Error
}

func (radarRepositoryImpl *RepositoryImpl) GetExplanation(gormTransaction *gorm.DB, date string, symbol string) (*entity.TickerExplanationDaily, error) {
	var explanation entity.TickerExplanationDaily
	err := gormTransaction.Where("date = ? AND UPPER(symbol) = UPPER(?)", date, symbol).First(&explanation).Error
	if err != nil {
		return nil, err
	}
	return &explanation, nil
}

func (radarRepositoryImpl *RepositoryImpl) SaveUserPreference(gormTransaction *gorm.DB, pref *entity.UserRadarPreference) error {
	return gormTransaction.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"mode", "preferred_sub_sector", "broadcast_enabled", "broadcast_time", "updated_at"}),
	}).Save(pref).Error
}

func (radarRepositoryImpl *RepositoryImpl) GetUserPreference(gormTransaction *gorm.DB, userID uint64) (*entity.UserRadarPreference, error) {
	var pref entity.UserRadarPreference
	err := gormTransaction.Where("user_id = ?", userID).First(&pref).Error
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

func (radarRepositoryImpl *RepositoryImpl) GetUsersWithBroadcastEnabled(gormTransaction *gorm.DB) ([]entity.UserRadarPreference, error) {
	var prefs []entity.UserRadarPreference
	err := gormTransaction.Where("broadcast_enabled = ?", true).Find(&prefs).Error
	return prefs, err
}

func (radarRepositoryImpl *RepositoryImpl) GetUserWatchlist(gormTransaction *gorm.DB, userID uint64) ([]entity.UserWatchlist, error) {
	var watchlist []entity.UserWatchlist
	err := gormTransaction.Where("user_id = ?", userID).Order("added_at ASC").Find(&watchlist).Error
	return watchlist, err
}

func (radarRepositoryImpl *RepositoryImpl) AddUserWatchlist(gormTransaction *gorm.DB, items []entity.UserWatchlist) error {
	if len(items) == 0 {
		return nil
	}
	return gormTransaction.Clauses(clause.OnConflict{
		DoNothing: true,
	}).CreateInBatches(items, 50).Error
}

func (radarRepositoryImpl *RepositoryImpl) RemoveUserWatchlist(gormTransaction *gorm.DB, userID uint64, symbol string) error {
	return gormTransaction.Where("user_id = ? AND UPPER(symbol) = UPPER(?)", userID, symbol).
		Delete(&entity.UserWatchlist{}).Error
}

func (radarRepositoryImpl *RepositoryImpl) GetAllActiveWatchlistSymbols(gormTransaction *gorm.DB) ([]string, error) {
	var symbols []string
	err := gormTransaction.Model(&entity.UserWatchlist{}).
		Distinct("symbol").
		Pluck("symbol", &symbols).Error
	return symbols, err
}

func (radarRepositoryImpl *RepositoryImpl) LogRequest(gormTransaction *gorm.DB, log *entity.RadarRequestLog) error {
	return gormTransaction.Create(log).Error
}
