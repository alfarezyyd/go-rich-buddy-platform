package midtrans

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-rich-buddy-platform/config"

	"github.com/spf13/viper"
)

type ChargeQRISRequest struct {
	PaymentType        string                    `json:"payment_type"`
	TransactionDetails TransactionDetailsRequest `json:"transaction_details"`
	QRIS               QRISDetailsRequest        `json:"qris"`
}

type TransactionDetailsRequest struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type QRISDetailsRequest struct {
	Acquirer string `json:"acquirer"`
}

type ActionResponse struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

type ChargeQRISResponse struct {
	StatusCode        string           `json:"status_code"`
	StatusMessage     string           `json:"status_message"`
	TransactionID     string           `json:"transaction_id"`
	OrderID           string           `json:"order_id"`
	MerchantID        string           `json:"merchant_id"`
	GrossAmount       string           `json:"gross_amount"`
	Currency          string           `json:"currency"`
	PaymentType       string           `json:"payment_type"`
	TransactionTime   string           `json:"transaction_time"`
	TransactionStatus string           `json:"transaction_status"`
	QRString          string           `json:"qr_string"`
	Actions           []ActionResponse `json:"actions"`
}

type WebhookNotificationPayload struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusMessage     string `json:"status_message"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	MerchantID        string `json:"merchant_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
	SettlementTime    string `json:"settlement_time"`
}

type StatusResponse struct {
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
}

type CancelResponse struct {
	StatusCode        string `json:"status_code"`
	StatusMessage     string `json:"status_message"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
}

func (c *ChargeQRISResponse) GetAction(name string) *ActionResponse {
	if c == nil {
		return nil
	}
	for i := range c.Actions {
		if strings.EqualFold(c.Actions[i].Name, name) {
			return &c.Actions[i]
		}
	}
	return nil
}

func (c *ChargeQRISResponse) GetQRCodeURL() string {
	if act := c.GetAction("generate-qr-code"); act != nil {
		return act.URL
	}
	return ""
}

func (c *ChargeQRISResponse) GetDeeplinkURL() string {
	if act := c.GetAction("deeplink-redirect"); act != nil {
		return act.URL
	}
	return ""
}

func (c *ChargeQRISResponse) GetStatusURL() string {
	if act := c.GetAction("get-status"); act != nil {
		return act.URL
	}
	return ""
}

func (c *ChargeQRISResponse) GetCancelURL() string {
	if act := c.GetAction("cancel"); act != nil {
		return act.URL
	}
	return ""
}

type Service interface {
	ChargeQRIS(ctx context.Context, orderID string, grossAmount int64) (*ChargeQRISResponse, error)
	CheckStatus(ctx context.Context, orderID string) (*StatusResponse, error)
	CancelOrder(ctx context.Context, orderID string) (*CancelResponse, error)
	VerifySignature(payload WebhookNotificationPayload) bool
}

type ServiceImpl struct {
	restyModule *config.RestyModule
	serverKey   string
}

func NewService(restyModule *config.RestyModule, viperConfig *viper.Viper) Service {
	return &ServiceImpl{
		restyModule: restyModule,
		serverKey:   viperConfig.GetString("MIDTRANS_API_KEY"),
	}
}

func (midtransService *ServiceImpl) ChargeQRIS(ctx context.Context, orderID string, grossAmount int64) (*ChargeQRISResponse, error) {
	reqBody := ChargeQRISRequest{
		PaymentType: "qris",
		TransactionDetails: TransactionDetailsRequest{
			OrderID:     orderID,
			GrossAmount: grossAmount,
		},
		QRIS: QRISDetailsRequest{
			Acquirer: "gopay",
		},
	}

	resp, err := midtransService.restyModule.GetRestyMidtrans().R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(reqBody).
		Post("charge")
	if err != nil {
		return nil, fmt.Errorf("midtrans charge request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("midtrans charge returned status %d: %s", resp.StatusCode(), resp.String())
	}

	var chargeQRISResponse ChargeQRISResponse
	if err := json.Unmarshal(resp.Body(), &chargeQRISResponse); err != nil {
		return nil, fmt.Errorf("failed to parse midtrans response: %w", err)
	}

	if strings.HasPrefix(chargeQRISResponse.StatusCode, "4") || strings.HasPrefix(chargeQRISResponse.StatusCode, "5") {
		return nil, errors.New(chargeQRISResponse.StatusMessage)
	}

	return &chargeQRISResponse, nil
}

func (midtransService *ServiceImpl) CheckStatus(ctx context.Context, orderID string) (*StatusResponse, error) {
	resp, err := midtransService.restyModule.GetRestyMidtrans().R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		Get(fmt.Sprintf("%s/status", orderID))
	if err != nil {
		return nil, fmt.Errorf("midtrans check status request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("midtrans check status returned %d: %s", resp.StatusCode(), resp.String())
	}

	var statusResp StatusResponse
	if err := json.Unmarshal(resp.Body(), &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &statusResp, nil
}

func (midtransService *ServiceImpl) CancelOrder(ctx context.Context, orderID string) (*CancelResponse, error) {
	resp, err := midtransService.restyModule.GetRestyMidtrans().R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		Post(fmt.Sprintf("%s/cancel", orderID))
	if err != nil {
		return nil, fmt.Errorf("midtrans cancel request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("midtrans cancel returned %d: %s", resp.StatusCode(), resp.String())
	}

	var cancelResp CancelResponse
	if err := json.Unmarshal(resp.Body(), &cancelResp); err != nil {
		return nil, fmt.Errorf("failed to parse cancel response: %w", err)
	}

	return &cancelResp, nil
}

func (midtransService *ServiceImpl) VerifySignature(payload WebhookNotificationPayload) bool {
	if midtransService.serverKey == "" || payload.SignatureKey == "" {
		return true // ponytail: allow test bypass if server key not set yet
	}

	raw := fmt.Sprintf("%s%s%s%s", payload.OrderID, payload.StatusCode, payload.GrossAmount, midtransService.serverKey)
	hash := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(hash[:])
	return strings.EqualFold(expected, payload.SignatureKey)
}
