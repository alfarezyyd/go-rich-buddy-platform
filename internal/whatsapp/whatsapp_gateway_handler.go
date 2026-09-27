package whatsapp_gateway

import (
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/helper"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GatewayHandler struct {
	gatewayService Service
}

func NewGatewayHandler(gatewayService Service) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: gatewayService,
	}
}

func (gatewayHandler *GatewayHandler) Webhook(ginContext *gin.Context) {
	var messagePayload model.TextMessage
	if err := ginContext.ShouldBindJSON(&messagePayload); err != nil {
		helper.CheckErrorOperation(
			err,
			exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest),
		)
		return
	}
	err := gatewayHandler.gatewayService.HandleIncoming(ginContext, messagePayload)
	if err != nil {
		panic(exception.NewApplicationError(http.StatusInternalServerError, err.Error()))
	}
}
