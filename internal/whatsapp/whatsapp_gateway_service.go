package whatsapp_gateway

import (
	"go-rich-buddy-platform/internal/model"

	"github.com/gin-gonic/gin"
)

type Service interface {
	HandleIncoming(ginContext *gin.Context, textMessage model.TextMessage) error
	SendDirectMessage(phone, message string) error
	SendMainMenu(phone string) error
}
