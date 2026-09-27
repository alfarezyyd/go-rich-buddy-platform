package whatsapp_gateway

import (
	"github.com/gin-gonic/gin"
)

type GatewayController interface {
	Webhook(ginContext *gin.Context)
}
