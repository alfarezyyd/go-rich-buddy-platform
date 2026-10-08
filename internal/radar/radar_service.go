package radar

import (
	"context"
	"go-rich-buddy-platform/internal/model"

	"gorm.io/gorm"
)

type Service interface {
	RunTier1(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error)
	RunTier2(ctx context.Context, targetDate string) (*model.RunPipelineResponse, error)

	GetRadarBySubSector(ctx context.Context, gormTransaction *gorm.DB, userID uint64, subSector string) (*model.RadarResult, error)
	GetRadarByWatchlist(ctx context.Context, gormTransaction *gorm.DB, userID uint64) (*model.RadarResult, error)
	GetRadarByManualTickers(ctx context.Context, gormTransaction *gorm.DB, userID uint64, symbols []string) (*model.RadarResult, error)
	GetDrillDownExplanation(ctx context.Context, gormTransaction *gorm.DB, symbol string) (*model.RadarDrillDownResult, error)

	SetUserPreference(ctx context.Context, gormTransaction *gorm.DB, userID uint64, req model.SetRadarPreferenceRequest) (*model.RadarPreferenceResponse, error)
	GetUserPreference(ctx context.Context, gormTransaction *gorm.DB, userID uint64) (*model.RadarPreferenceResponse, error)

	GetUserWatchlist(ctx context.Context, gormTransaction *gorm.DB, userID uint64) (*model.WatchlistResponse, error)
	AddToWatchlist(ctx context.Context, gormTransaction *gorm.DB, userID uint64, symbols []string) (*model.WatchlistResponse, error)
	RemoveFromWatchlist(ctx context.Context, gormTransaction *gorm.DB, userID uint64, symbol string) (*model.WatchlistResponse, error)

	FormatRadarMessage(result *model.RadarResult) string
	FormatDrillDownMessage(result *model.RadarDrillDownResult) string
	RunMorningBroadcast(ctx context.Context, broadcastFunc func(phone string, message string) error) error
}
