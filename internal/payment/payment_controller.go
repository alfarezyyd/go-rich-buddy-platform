package payment

import (
	"encoding/json"
	"net/http"
	"time"

	"go-rich-buddy-platform/internal/midtrans"
	"go-rich-buddy-platform/internal/order"
	"go-rich-buddy-platform/internal/user"
	whatsappGateway "go-rich-buddy-platform/internal/whatsapp"
	pkgi18n "go-rich-buddy-platform/pkg/i18n"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Controller struct {
	dbConnection    *gorm.DB
	orderRepo       order.Repository
	userRepo        user.Repository
	midtransService midtrans.Service
	whatsappService whatsappGateway.Service
	localizer       *pkgi18n.Localizer
}

func NewController(
	dbConnection *gorm.DB,
	orderRepo order.Repository,
	userRepo user.Repository,
	midtransService midtrans.Service,
	whatsappService whatsappGateway.Service,
	i18nBundle *pkgi18n.Bundle,
) *Controller {
	return &Controller{
		dbConnection:    dbConnection,
		orderRepo:       orderRepo,
		userRepo:        userRepo,
		midtransService: midtransService,
		whatsappService: whatsappService,
		localizer:       i18nBundle.NewLocalizer("id"),
	}
}

func (paymentController *Controller) MidtransWebhook(ginContext *gin.Context) {
	var midtransOrderPayload midtrans.WebhookNotificationPayload
	if err := ginContext.ShouldBindJSON(&midtransOrderPayload); err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "invalid payload"})
		return
	}

	if !paymentController.midtransService.VerifySignature(midtransOrderPayload) {
		logrus.Warnf("Invalid midtrans webhook signature for order %s", midtransOrderPayload.OrderID)
		ginContext.JSON(http.StatusForbidden, gin.H{"message": "invalid signature"})
		return
	}

	logrus.Infof("Received midtrans webhook for order %s status %s", midtransOrderPayload.OrderID, midtransOrderPayload.TransactionStatus)

	tx := paymentController.dbConnection.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	ord, err := paymentController.orderRepo.FindOrderByOrderId(tx, midtransOrderPayload.OrderID)
	if err != nil {
		tx.Rollback()
		logrus.Errorf("Order not found for midtrans notification: %s", midtransOrderPayload.OrderID)
		ginContext.JSON(http.StatusOK, gin.H{"message": "order not found ignored"})
		return
	}

	rawResp, _ := json.Marshal(midtransOrderPayload)
	ord.MidtransResponse = string(rawResp)
	ord.TransactionStatus = midtransOrderPayload.TransactionStatus

	if midtransOrderPayload.TransactionStatus == "settlement" || midtransOrderPayload.TransactionStatus == "capture" {
		now := time.Now()
		ord.PaidAt = &now

		if ord.User != nil && ord.Package != nil {
			ord.User.Tier = ord.Package.Tier
			ord.User.CreditBalance += ord.Package.Credits
			if err := paymentController.userRepo.Update(tx, ord.User); err != nil {
				tx.Rollback()
				logrus.Errorf("Failed to update user credit balance: %v", err)
				ginContext.JSON(http.StatusInternalServerError, gin.H{"message": "failed to update user"})
				return
			}

			// Send WhatsApp confirmation & return to main menu
			go func(phone, tierName string, credits int) {
				msg := paymentController.localizer.TData(pkgi18n.MsgOrderSuccess, map[string]any{
					"Tier":    tierName,
					"Credits": credits,
				})
				_ = paymentController.whatsappService.SendDirectMessage(phone, msg)
				_ = paymentController.whatsappService.SendMainMenu(phone)
			}(ord.User.Phone, ord.Package.Tier, ord.User.CreditBalance)
		}
	}

	if err := paymentController.orderRepo.UpdateOrder(tx, ord); err != nil {
		tx.Rollback()
		logrus.Errorf("Failed to update order: %v", err)
		ginContext.JSON(http.StatusInternalServerError, gin.H{"message": "failed to update order"})
		return
	}

	tx.Commit()
	ginContext.JSON(http.StatusOK, gin.H{"message": "success"})
}
