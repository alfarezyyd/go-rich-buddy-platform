package routes

import (
	"go-rich-buddy-platform/internal/radar"

	"github.com/gin-gonic/gin"
)

type RadarRoutes struct {
	radarController radar.Controller
}

func NewRadarRoutes(radarController radar.Controller) *RadarRoutes {
	return &RadarRoutes{
		radarController: radarController,
	}
}

func (radarRoutes *RadarRoutes) Setup(routerGroup *gin.RouterGroup) {
	radarRouteGroup := routerGroup.Group("/radar")
	{
		radarRouteGroup.GET("/subsector", radarRoutes.radarController.GetSubSectorRadar)
		radarRouteGroup.GET("/watchlist", radarRoutes.radarController.GetWatchlistRadar)
		radarRouteGroup.POST("/manual", radarRoutes.radarController.GetManualTickersRadar)
		radarRouteGroup.GET("/manual", radarRoutes.radarController.GetManualTickersRadar)
		radarRouteGroup.GET("/drilldown/:symbol", radarRoutes.radarController.GetDrillDown)
		radarRouteGroup.GET("/drilldown", radarRoutes.radarController.GetDrillDown)

		radarRouteGroup.GET("/preferences", radarRoutes.radarController.GetPreference)
		radarRouteGroup.POST("/preferences", radarRoutes.radarController.SetPreference)

		radarRouteGroup.GET("/user-watchlist", radarRoutes.radarController.GetUserWatchlist)
		radarRouteGroup.POST("/user-watchlist", radarRoutes.radarController.AddToWatchlist)
		radarRouteGroup.DELETE("/user-watchlist/:symbol", radarRoutes.radarController.RemoveFromWatchlist)

		radarRouteGroup.POST("/pipeline/tier1", radarRoutes.radarController.TriggerTier1)
		radarRouteGroup.POST("/pipeline/tier2", radarRoutes.radarController.TriggerTier2)
	}
}
