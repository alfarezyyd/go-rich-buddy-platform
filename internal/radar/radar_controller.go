package radar

import "github.com/gin-gonic/gin"

type Controller interface {
	GetSubSectorRadar(ginContext *gin.Context)
	GetWatchlistRadar(ginContext *gin.Context)
	GetManualTickersRadar(ginContext *gin.Context)
	GetDrillDown(ginContext *gin.Context)

	SetPreference(ginContext *gin.Context)
	GetPreference(ginContext *gin.Context)

	GetUserWatchlist(ginContext *gin.Context)
	AddToWatchlist(ginContext *gin.Context)
	RemoveFromWatchlist(ginContext *gin.Context)

	TriggerTier1(ginContext *gin.Context)
	TriggerTier2(ginContext *gin.Context)
}
