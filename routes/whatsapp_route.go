package routes

import (
	whatsappGateway "go-rich-buddy-platform/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

type WhatsappRoutes struct {
	whatsappGatewayHandler *whatsappGateway.GatewayHandler
}

func NewWhatsappRoutes(whatsappGatewayHandler *whatsappGateway.GatewayHandler) *WhatsappRoutes {
	return &WhatsappRoutes{
		whatsappGatewayHandler: whatsappGatewayHandler,
	}
}

func (whatsappRoutes *WhatsappRoutes) Setup(routerGroup *gin.RouterGroup) {
	whatsappRouteGroup := routerGroup.Group("/whatsapp")
	{
		whatsappRouteGroup.POST("/webhook", whatsappRoutes.whatsappGatewayHandler.Webhook)
	}
}
