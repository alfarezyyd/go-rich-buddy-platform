package routes

import (
	"go-rich-buddy-platform/internal/payment"

	"github.com/gin-gonic/gin"
)

type PaymentRoutes struct {
	paymentController *payment.Controller
}

func NewPaymentRoutes(paymentController *payment.Controller) *PaymentRoutes {
	return &PaymentRoutes{
		paymentController: paymentController,
	}
}

func (paymentRoute *PaymentRoutes) Setup(routerGroup *gin.RouterGroup) {
	paymentGroup := routerGroup.Group("/payments")
	{
		paymentGroup.POST("/midtrans/webhook", paymentRoute.paymentController.MidtransWebhook)
	}
}
