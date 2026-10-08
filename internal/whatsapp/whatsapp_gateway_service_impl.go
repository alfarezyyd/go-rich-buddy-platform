package whatsapp_gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/memory"
	"go-rich-buddy-platform/internal/memory/privacy"
	"go-rich-buddy-platform/internal/midtrans"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/order"
	"go-rich-buddy-platform/internal/radar"
	"go-rich-buddy-platform/internal/user"
	whatsappSession "go-rich-buddy-platform/internal/whatsapp_session"
	pkgi18n "go-rich-buddy-platform/pkg/i18n"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var inFlightMessages sync.Map

// States
const (
	stateMainMenu                = "MAIN_MENU"
	stateRegisterAwaitName       = "REGISTER_AWAIT_NAME"
	stateOrderMenu               = "ORDER_MENU"
	stateRadarSahamMenu          = "RADAR_SAHAM_MENU"
	stateRadarSahamSubsector     = "RADAR_SAHAM_SUBSECTOR"
	stateRadarSahamSubsectorCust = "RADAR_SAHAM_SUBSECTOR_CUSTOM"
	stateRadarSahamManual        = "RADAR_SAHAM_MANUAL"
	stateRadarSahamWatchlistAdd  = "RADAR_SAHAM_WATCHLIST_ADD"
	stateRadarSahamResults       = "RADAR_SAHAM_RESULTS"
	stateRadarSahamDrillDown     = "RADAR_SAHAM_DRILLDOWN"
	stateRadarOptInPrompt        = "RADAR_OPTIN_PROMPT"
)

const maxRetry = 3

type SendMessageRequest struct {
	Phone       string   `json:"phone"`
	Message     string   `json:"message"`
	Mentions    []string `json:"mentions,omitempty"`
	IsForwarded bool     `json:"is_forwarded,omitempty"`
}

type ServiceImpl struct {
	dbConnection      *gorm.DB
	sessionRepository whatsappSession.SessionRepository
	userRepository    user.Repository
	orderRepository   order.Repository
	midtransService   midtrans.Service
	radarService      radar.Service
	agentService      agent.Service
	memoryService     memory.Service
	restyModule       *config.RestyModule
	localizer         *pkgi18n.Localizer
}

func NewService(
	dbConnection *gorm.DB,
	sessionRepository whatsappSession.SessionRepository,
	userRepository user.Repository,
	orderRepository order.Repository,
	midtransService midtrans.Service,
	radarService radar.Service,
	agentService agent.Service,
	memoryService memory.Service,
	restyModule *config.RestyModule,
	i18nBundle *pkgi18n.Bundle,
) Service {
	return &ServiceImpl{
		dbConnection:      dbConnection,
		sessionRepository: sessionRepository,
		userRepository:    userRepository,
		orderRepository:   orderRepository,
		midtransService:   midtransService,
		radarService:      radarService,
		agentService:      agentService,
		memoryService:     memoryService,
		restyModule:       restyModule,
		localizer:         i18nBundle.NewLocalizer("id"),
	}
}

func (whatsappGatewayService *ServiceImpl) HandleIncoming(ginContext *gin.Context, textMessage model.TextMessage) error {
	if textMessage.Event != "message" || textMessage.Payload.IsFromMe || textMessage.Payload.Body == "" {
		return nil
	}

	phoneNumber := strings.Split(textMessage.Payload.From, "@")[0]
	logrus.Debugf("Incoming WhatsApp message from: %s", phoneNumber)

	payloadBody := strings.TrimSpace(strings.ToLower(textMessage.Payload.Body))
	rawBody := strings.TrimSpace(textMessage.Payload.Body)
	waMessageID := textMessage.Payload.Id

	// Cek in-flight / duplicate message ID sebelum masuk database transaction & LLM call.
	// Jika gateway melakukan retry saat request pertama masih diproses, request kedua langsung diabaikan.
	if waMessageID != "" {
		if _, loaded := inFlightMessages.LoadOrStore(waMessageID, time.Now()); loaded {
			logrus.WithField("wa_message_id", waMessageID).Info("Duplicate in-flight WhatsApp webhook received, skipping processing")
			return nil
		}
		// Hapus dari in-flight cache setelah 30 detik agar memory tidak bocor.
		time.AfterFunc(30*time.Second, func() {
			inFlightMessages.Delete(waMessageID)
		})
	}

	return whatsappGatewayService.dbConnection.Transaction(func(tx *gorm.DB) error {
		whatsappSession, err := whatsappGatewayService.sessionRepository.FindByPhone(tx, phoneNumber)
		if err != nil {
			whatsappSession = &entity.WhatsappSession{
				Phone:        phoneNumber,
				CurrentState: stateMainMenu,
			}
			if err := whatsappGatewayService.sessionRepository.Upsert(tx, whatsappSession); err != nil {
				return err
			}
		}

		// Resolve user for memory operations.
		usr, usrErr := whatsappGatewayService.userRepository.FindByPhone(tx, phoneNumber)

		// Persist the incoming message (with PII redaction + dedup).
		if usrErr == nil && usr != nil && usr.Id > 0 && waMessageID != "" {
			existing, _ := whatsappGatewayService.memoryService.GetMessageRepository().FindMessageByWaID(tx, waMessageID)
			if existing != nil {
				logrus.WithField("wa_message_id", waMessageID).Info("Duplicate WhatsApp webhook received, skipping processing")
				return nil
			}

			_, chatSession, saveErr := whatsappGatewayService.memoryService.SaveIncomingMessage(
				context.Background(), tx, usr.Id, waMessageID, rawBody, "user",
			)
			if saveErr != nil {
				logrus.WithError(saveErr).Warn("failed to persist incoming message to memory store")
			}
			_ = chatSession // used by async jobs below if needed
		}

		// Handle deterministic memory control commands before the state machine.
		if usr != nil {
			if handled, handleErr := whatsappGatewayService.handleMemoryCommand(tx, usr.Id, phoneNumber, payloadBody); handled {
				return handleErr
			}
		}

		if payloadBody == "batal" || payloadBody == "cancel" {
			whatsappSession.CurrentState = stateMainMenu
			whatsappSession.RetryCount = 0
			if err := whatsappGatewayService.sessionRepository.Upsert(tx, whatsappSession); err != nil {
				return err
			}

			// If user has an active pending order, cancel it in Midtrans and update DB
			if usr != nil {
				if pendingOrder, err := whatsappGatewayService.orderRepository.FindLatestPendingOrderByUserId(tx, usr.Id); err == nil && pendingOrder != nil {
					if _, cancelErr := whatsappGatewayService.midtransService.CancelOrder(context.Background(), pendingOrder.OrderId); cancelErr != nil {
						logrus.Warnf("Failed to cancel midtrans order %s: %v", pendingOrder.OrderId, cancelErr)
					}
					pendingOrder.TransactionStatus = "cancel"
					_ = whatsappGatewayService.orderRepository.UpdateOrder(tx, pendingOrder)

					_ = whatsappGatewayService.sendMessage(phoneNumber, whatsappGatewayService.translateWithTemplateData(msgOrderCancelled, map[string]any{
						"OrderID": pendingOrder.OrderId,
					}))
					return whatsappGatewayService.sendMainMenu(phoneNumber)
				}
			}

			return whatsappGatewayService.sendMainMenu(phoneNumber)
		}

		switch whatsappSession.CurrentState {
		case stateMainMenu:
			return whatsappGatewayService.handleMainMenu(tx, whatsappSession, phoneNumber, payloadBody, rawBody)
		case stateRegisterAwaitName:
			return whatsappGatewayService.handleRegisterAwaitName(tx, whatsappSession, phoneNumber, rawBody)
		case stateOrderMenu:
			return whatsappGatewayService.handleOrderMenu(tx, whatsappSession, phoneNumber, payloadBody)
		case stateRadarSahamMenu:
			return whatsappGatewayService.handleRadarSahamMenu(tx, whatsappSession, phoneNumber, payloadBody)
		case stateRadarSahamSubsector:
			return whatsappGatewayService.handleRadarSahamSubsector(tx, whatsappSession, phoneNumber, payloadBody, rawBody)
		case stateRadarSahamSubsectorCust:
			return whatsappGatewayService.handleRadarSahamSubsectorCustom(tx, whatsappSession, phoneNumber, rawBody)
		case stateRadarSahamManual:
			return whatsappGatewayService.handleRadarSahamManual(tx, whatsappSession, phoneNumber, rawBody)
		case stateRadarSahamWatchlistAdd:
			return whatsappGatewayService.handleRadarSahamWatchlistAdd(tx, whatsappSession, phoneNumber, rawBody)
		case stateRadarSahamResults:
			return whatsappGatewayService.handleRadarSahamResults(tx, whatsappSession, phoneNumber, payloadBody, rawBody)
		case stateRadarSahamDrillDown:
			return whatsappGatewayService.handleRadarSahamDrillDown(tx, whatsappSession, phoneNumber, payloadBody, rawBody)
		case stateRadarOptInPrompt:
			return whatsappGatewayService.handleRadarOptInPrompt(tx, whatsappSession, phoneNumber, payloadBody)
		default:
			whatsappSession.CurrentState = stateMainMenu
			whatsappSession.RetryCount = 0
			if err := whatsappGatewayService.sessionRepository.Upsert(tx, whatsappSession); err != nil {
				return err
			}
			return whatsappGatewayService.sendMainMenu(phoneNumber)
		}
	})
}

// handleMemoryCommand processes deterministic memory control commands.
// Returns (true, err) if a memory command was matched, (false, nil) otherwise.
func (whatsappGatewayService *ServiceImpl) handleMemoryCommand(tx *gorm.DB, userID uint64, phone, payloadBody string) (bool, error) {
	cmd, target := privacy.ParseMemoryCommand(payloadBody)
	if cmd == privacy.CommandNone {
		return false, nil
	}

	ctx := context.Background()

	switch cmd {
	case privacy.CommandViewMemory:
		items, err := whatsappGatewayService.memoryService.GetUserMemoryItems(ctx, tx, userID)
		if err != nil || len(items) == 0 {
			return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMemoryEmpty))
		}
		return true, whatsappGatewayService.sendMessage(phone, formatMemoryItems(items))

	case privacy.CommandForgetItem:
		if target == "" {
			return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMemoryForgetInvalid))
		}
		_ = whatsappGatewayService.memoryService.ForgetMemoryByKey(ctx, tx, userID, target)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMemoryForgotConfirm))

	case privacy.CommandDeleteChatHistory:
		_ = whatsappGatewayService.memoryService.DeleteChatHistory(ctx, tx, userID)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgChatHistoryDeleted))

	case privacy.CommandDeleteAllMemory:
		_ = whatsappGatewayService.memoryService.DeleteAllMemory(ctx, tx, userID)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgAllMemoryDeleted))
	}

	return false, nil
}

// formatMemoryItems formats memory items as a human-readable message for the user.
func formatMemoryItems(items []*entity.UserMemoryItem) string {
	var sb strings.Builder
	sb.WriteString("📋 *Preferensi & Memori Kamu*\n\n")
	keyLabels := map[string]string{
		"profile.experience_level": "Tingkat pengalaman",
		"pref.explanation_depth":   "Kedalaman penjelasan",
		"pref.tone":                "Nada percakapan",
		"pref.emoji":               "Emoji",
		"pref.language":            "Bahasa",
		"interest.topics":          "Minat",
		"goal.learning":            "Tujuan belajar",
	}
	for _, item := range items {
		label := item.Key
		if l, ok := keyLabels[item.Key]; ok {
			label = l
		}
		sb.WriteString(fmt.Sprintf("• %s: %s\n", label, item.Value))
	}
	sb.WriteString("\nKetik *lupakan <preferensi>* untuk menghapus, atau *hapus semua memori* untuk menghapus semua.")
	return sb.String()
}

func (whatsappGatewayService *ServiceImpl) upsertSession(tx *gorm.DB, session *entity.WhatsappSession) error {
	return whatsappGatewayService.sessionRepository.Upsert(tx, session)
}

func (whatsappGatewayService *ServiceImpl) transitionTo(tx *gorm.DB, session *entity.WhatsappSession, state string, next func() error) error {
	session.CurrentState = state
	session.RetryCount = 0
	if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
		return err
	}
	return next()
}

// translateLocalization translates a plain message key with no template data.
func (whatsappGatewayService *ServiceImpl) translateLocalization(key string) string {
	return whatsappGatewayService.localizer.T(key)
}

// translateWithTemplateData translates a message key with template data.
func (whatsappGatewayService *ServiceImpl) translateWithTemplateData(key string, data any) string {
	return whatsappGatewayService.localizer.TData(key, data)
}

// tp translates a plural-aware message key.
func (whatsappGatewayService *ServiceImpl) tp(key string, count int, data any) string {
	return whatsappGatewayService.localizer.TPlural(key, count, data)
}

func (whatsappGatewayService *ServiceImpl) SendMainMenu(phone string) error {
	return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuGreeting))
}

func (whatsappGatewayService *ServiceImpl) sendMainMenu(phone string) error {
	return whatsappGatewayService.SendMainMenu(phone)
}

var regularGreetings = []string{
	"halo", "hallo", "hello", "hi", "hai", "hei", "hey",
	"selamat pagi", "selamat siang", "selamat sore", "selamat malam",
	"assalamualaikum", "pagi", "siang", "sore", "malam", "ping", "p",
}

func isGreeting(body string) bool {
	for _, g := range regularGreetings {
		if body == g {
			return true
		}
	}
	return false
}

func (whatsappGatewayService *ServiceImpl) handleMainMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string, rawBody string) error {
	switch body {
	case "1", "register", "daftar":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err == nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuAlreadyRegistered))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateRegisterAwaitName, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuEnterFullName))
		})

	case "2", "order", "paket", "langganan":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuMustRegisterFirst))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateOrderMenu, func() error {
			return whatsappGatewayService.sendOrderMenu(phone)
		})

	case "3", "radar", "radar saham":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuMustRegisterRadar))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})

	case "status", "cek status", "status order":
		usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuMustRegisterFirst))
		}
		pendingOrder, err := whatsappGatewayService.orderRepository.FindLatestPendingOrderByUserId(tx, usr.Id)
		if err != nil || pendingOrder == nil {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOrderNoPending))
			return whatsappGatewayService.sendMainMenu(phone)
		}

		// Call Midtrans get-status
		statusResp, statusErr := whatsappGatewayService.midtransService.CheckStatus(context.Background(), pendingOrder.OrderId)
		if statusErr == nil && statusResp != nil {
			pendingOrder.TransactionStatus = statusResp.TransactionStatus
			if statusResp.TransactionStatus == "settlement" || statusResp.TransactionStatus == "capture" {
				now := time.Now()
				pendingOrder.PaidAt = &now
				if pendingOrder.Package != nil {
					usr.Tier = pendingOrder.Package.Tier
					usr.CreditBalance += pendingOrder.Package.Credits
					_ = whatsappGatewayService.userRepository.Update(tx, usr)
				}
				_ = whatsappGatewayService.orderRepository.UpdateOrder(tx, pendingOrder)
				tierName := ""
				if pendingOrder.Package != nil {
					tierName = pendingOrder.Package.Tier
				}
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgOrderStatusSettled, map[string]any{
					"OrderID": pendingOrder.OrderId,
					"Tier":    tierName,
					"Credits": usr.CreditBalance,
				}))
			}
			_ = whatsappGatewayService.orderRepository.UpdateOrder(tx, pendingOrder)
		}

		pkgName := ""
		if pendingOrder.Package != nil {
			pkgName = pendingOrder.Package.Name
		}
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgOrderStatusPending, map[string]any{
			"OrderID":     pendingOrder.OrderId,
			"PackageName": pkgName,
			"Amount":      fmt.Sprintf("%d", pendingOrder.GrossAmount),
		}))

	default:
		// Sapaan murni (greeting): reset retry count & kirim menu utama RichBuddy
		if isGreeting(body) {
			session.RetryCount = 0
			_ = whatsappGatewayService.upsertSession(tx, session)
			return whatsappGatewayService.sendMainMenu(phone)
		}

		ctx := context.Background()
		mode := whatsappGatewayService.agentService.Classify(ctx, rawBody)

		usr, _ := whatsappGatewayService.userRepository.FindByPhone(tx, phone)

		// Mode Analyst: cegah jika user belum terdaftar
		if mode == agent.ModeAnalyst {
			if usr == nil {
				session.RetryCount = 0
				_ = whatsappGatewayService.upsertSession(tx, session)
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuMustRegisterFirst))
			}
			// User sudah terdaftar: layani analisis langsung
			return whatsappGatewayService.answerStockInquiry(tx, session, phone, rawBody)
		}

		// Mode Regular (chit-chat, edukasi, obrolan santai): layani via Agent Regular
		session.RetryCount = 0
		_ = whatsappGatewayService.upsertSession(tx, session)
		return whatsappGatewayService.handleRegularChat(phone, rawBody)
	}
}

func (whatsappGatewayService *ServiceImpl) handleRegularChat(phone, userMessage string) error {
	agentSession := newWhatsAppAgentSession()
	ctx := context.Background()

	err := whatsappGatewayService.agentService.ProcessTurn(ctx, agentSession, userMessage)
	if err != nil {
		logrus.WithError(err).Error("Error processing regular chat response")
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgAgentError))
	}

	response := agentSession.CollectedResponse()
	if response == "" {
		response = whatsappGatewayService.translateLocalization(msgAgentEmptyResult)
	}
	return whatsappGatewayService.sendMessage(phone, response)
}

func (whatsappGatewayService *ServiceImpl) handleRegisterAwaitName(tx *gorm.DB, session *entity.WhatsappSession, phone, name string) error {
	if strings.TrimSpace(name) == "" {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgRegisterNameCannotBeEmpty))
	}

	_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err == nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgRegisterAlreadyRegistered))
			return whatsappGatewayService.sendMainMenu(phone)
		})
	}

	newUser := &entity.User{
		Name:          name,
		Phone:         phone,
		Tier:          "Starter",
		CreditBalance: 10,
	}
	if err := whatsappGatewayService.userRepository.Create(tx, newUser); err != nil {
		return err
	}

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgRegisterSuccess, map[string]any{"Name": name}))
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendOrderMenu(phone string) error {
	packages, err := whatsappGatewayService.orderRepository.FindAllActivePackages(whatsappGatewayService.dbConnection)
	if err != nil || len(packages) == 0 {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOrderMenu))
	}

	var sb strings.Builder
	sb.WriteString("Pilih paket subscription:\n")
	for idx, pkg := range packages {
		if pkg.Price == 0 {
			sb.WriteString(fmt.Sprintf("%d️⃣ %s — %d kredit/hari (Gratis)\n", idx+1, pkg.Name, pkg.Credits))
		} else {
			sb.WriteString(fmt.Sprintf("%d️⃣ %s — Rp %d (%d kredit)\n", idx+1, pkg.Name, pkg.Price, pkg.Credits))
		}
	}
	sb.WriteString(fmt.Sprintf("\nBalas dengan angka (1-%d).\n_(Ketik *batal* untuk kembali ke menu utama)_", len(packages)))

	return whatsappGatewayService.sendMessage(phone, sb.String())
}

func (whatsappGatewayService *ServiceImpl) handleOrderMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	packages, err := whatsappGatewayService.orderRepository.FindAllActivePackages(tx)
	if err != nil || len(packages) == 0 {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, "Paket belum tersedia saat ini.")
			return whatsappGatewayService.sendMainMenu(phone)
		})
	}

	choiceIdx, parseErr := strconv.Atoi(strings.TrimSpace(body))
	if parseErr != nil || choiceIdx < 1 || choiceIdx > len(packages) {
		session.RetryCount++
		if session.RetryCount >= maxRetry {
			session.RetryCount = 0
			session.CurrentState = stateMainMenu
			_ = whatsappGatewayService.upsertSession(tx, session)
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgMainMenuTooManyWrongInput))
			return whatsappGatewayService.sendMainMenu(phone)
		}
		_ = whatsappGatewayService.upsertSession(tx, session)
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgOrderInvalidChoice, map[string]any{
			"Current": session.RetryCount,
			"Max":     maxRetry,
		}))
	}

	selectedPkg := packages[choiceIdx-1]

	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOrderAccountNotFound))
		})
	}

	// Paket gratis langsung diproses
	if selectedPkg.Price <= 0 {
		usr.Tier = selectedPkg.Tier
		usr.CreditBalance += selectedPkg.Credits
		if err := whatsappGatewayService.userRepository.Update(tx, usr); err != nil {
			return err
		}

		payloadMessage := whatsappGatewayService.translateWithTemplateData(msgOrderFreeSuccess, map[string]any{
			"Tier":    selectedPkg.Tier,
			"Credits": usr.CreditBalance,
		})

		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, payloadMessage)
			return whatsappGatewayService.sendMainMenu(phone)
		})
	}

	// Paket berbayar: Buat order & panggil Midtrans QRIS
	orderID := fmt.Sprintf("RB-%d-%d", usr.Id, time.Now().Unix())
	midtransResp, err := whatsappGatewayService.midtransService.ChargeQRIS(context.Background(), orderID, selectedPkg.Price)
	if err != nil {
		logrus.Errorf("Failed to charge QRIS from Midtrans: %v", err)
		return whatsappGatewayService.sendMessage(phone, "Gagal membuat kode pembayaran QRIS. Silakan coba beberapa saat lagi.")
	}

	qrURL := midtransResp.GetQRCodeURL()
	deeplinkURL := midtransResp.GetDeeplinkURL()

	rawMidtrans, _ := json.Marshal(midtransResp)

	newOrder := &entity.Order{
		OrderId:           orderID,
		UserId:            usr.Id,
		PackageId:         selectedPkg.Id,
		GrossAmount:       selectedPkg.Price,
		PaymentType:       "qris",
		TransactionStatus: "pending",
		QrString:          midtransResp.QRString,
		QrUrl:             qrURL,
		MidtransResponse:  string(rawMidtrans),
	}

	if err := whatsappGatewayService.orderRepository.CreateOrder(tx, newOrder); err != nil {
		logrus.Errorf("Failed to persist order: %v", err)
		return whatsappGatewayService.sendMessage(phone, "Gagal menyimpan data pesanan.")
	}

	deeplinkInfo := ""
	if deeplinkURL != "" {
		deeplinkInfo = whatsappGatewayService.translateWithTemplateData(msgOrderDeeplink, map[string]any{
			"URL": deeplinkURL,
		})
	}

	qrisMsg := whatsappGatewayService.translateWithTemplateData(msgOrderQRISPrompt, map[string]any{
		"PackageName":  selectedPkg.Name,
		"Amount":       fmt.Sprintf("%d", selectedPkg.Price),
		"OrderID":      orderID,
		"DeeplinkInfo": deeplinkInfo,
	})

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		if qrURL != "" {
			return whatsappGatewayService.sendImage(phone, qrURL, qrisMsg)
		}
		return whatsappGatewayService.sendMessage(phone, qrisMsg)
	})
}

func (whatsappGatewayService *ServiceImpl) sendRadarSahamMenu(phone string) error {
	return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgRadarSahamMenu))
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgRadarAccountNotRegistered))
	}

	switch body {
	case "1", "subsektor":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamSubsector, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgSubsectorMenu))
		})

	case "2", "watchlist":
		watchlist, err := whatsappGatewayService.radarService.GetUserWatchlist(context.Background(), tx, usr.Id)
		if err != nil || len(watchlist.Symbols) == 0 {
			return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamWatchlistAdd, func() error {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgWatchlistEmpty))
			})
		}

		result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgWatchlistFetchError, map[string]any{"Error": err.Error()}))
		}

		// Simpan urutan ticker ke session sebelum transisi.
		session.SetRadarSymbols(extractSymbols(result))
		formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})

	case "3", "manual", "cari":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamManual, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgManualTickerPrompt))
		})

	case "4", "optin", "pengaturan", "notifikasi", "broadcast":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarOptInPrompt, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOptInPrompt))
		})

	default:
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgRadarInvalidChoice))
	}
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamSubsector(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	usr, _ := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	userID := uint64(1)
	if usr != nil {
		userID = usr.Id
	}

	var subSector string
	switch body {
	case "1", "banks", "bank":
		subSector = "Banks"
	case "2", "food & beverage", "food and beverage", "f&b", "food":
		subSector = "Food & Beverage"
	case "3", "coal", "batu bara", "batubara", "energy":
		subSector = "Coal"
	case "4", "telco", "telekomunikasi", "telecom":
		subSector = "Telco"
	case "5", "lainnya", "other":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamSubsectorCust, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgSubsectorCustomPrompt))
		})
	default:
		// Check if user directly typed subsector name
		subSector = raw
	}

	result, err := whatsappGatewayService.radarService.GetRadarBySubSector(context.Background(), tx, userID, subSector)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgSubsectorFetchError, map[string]any{"Error": err.Error()}))
	}

	// Simpan urutan ticker ke session sebelum transisi agar angka pilihan user bisa di-resolve.
	session.SetRadarSymbols(extractSymbols(result))
	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamSubsectorCustom(tx *gorm.DB, session *entity.WhatsappSession, phone, subSectorName string) error {
	usr, _ := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	userID := uint64(1)
	if usr != nil {
		userID = usr.Id
	}

	result, err := whatsappGatewayService.radarService.GetRadarBySubSector(context.Background(), tx, userID, strings.TrimSpace(subSectorName))
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgSubsectorFetchError, map[string]any{"Error": err.Error()}))
	}

	// Simpan urutan ticker ke session sebelum transisi.
	session.SetRadarSymbols(extractSymbols(result))
	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamManual(tx *gorm.DB, session *entity.WhatsappSession, phone, rawTickers string) error {
	usr, _ := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	userID := uint64(1)
	if usr != nil {
		userID = usr.Id
	}

	rawSymbols := strings.Split(rawTickers, ",")
	var symbols []string
	for _, s := range rawSymbols {
		clean := strings.TrimSpace(strings.ToUpper(s))
		if clean != "" {
			symbols = append(symbols, clean)
		}
	}

	if len(symbols) == 0 {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgManualInvalidTicker))
	}

	result, err := whatsappGatewayService.radarService.GetRadarByManualTickers(context.Background(), tx, userID, symbols)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgManualTickerFetchError, map[string]any{"Error": err.Error()}))
	}

	// Simpan urutan ticker ke session sebelum transisi.
	session.SetRadarSymbols(extractSymbols(result))
	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamWatchlistAdd(tx *gorm.DB, session *entity.WhatsappSession, phone, rawTickers string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgWatchlistUserNotFound))
	}

	rawSymbols := strings.Split(rawTickers, ",")
	var symbols []string
	for _, s := range rawSymbols {
		clean := strings.TrimSpace(strings.ToUpper(s))
		if clean != "" {
			symbols = append(symbols, clean)
		}
	}

	if len(symbols) == 0 {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgWatchlistInvalidTicker))
	}

	_, _ = whatsappGatewayService.radarService.AddToWatchlist(context.Background(), tx, usr.Id, symbols)
	result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgWatchlistAddFetchError, map[string]any{"Error": err.Error()}))
	}

	// Simpan urutan ticker ke session sebelum transisi.
	session.SetRadarSymbols(extractSymbols(result))
	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.tp(msgWatchlistAddSuccess, len(symbols), map[string]any{"Count": len(symbols)}))
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamResults(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	if body == "menu" || body == "radar" {
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})
	}

	// Pilihan angka: user memilih nomor urut dari daftar hasil radar (misal: "1", "2", "3")
	if idx, err := strconv.Atoi(body); err == nil {
		symbol := session.RadarSymbolByIndex(idx)
		if symbol == "" {
			// Angka di luar range atau sesi tidak punya data radar — tampilkan pesan kontekstual
			// agar tidak jatuh ke agent yang tidak punya konteks pilihan menu.
			n := len(session.GetRadarSymbols())
			if n == 0 {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgDrillDownNoContext))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownOutOfRange, map[string]any{"Max": n}))
		}
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, symbol)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownInvalidTicker, map[string]any{"Symbol": symbol}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownFetchError, map[string]any{"Symbol": symbol}))
		}
		session.ActiveSymbol = symbol
		formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamDrillDown, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})
	}

	// Drill-down langsung: user mengetik kode saham 4 huruf (misal: BBCA, BMRI, TLKM)
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if len(trimmed) == 4 && !strings.Contains(trimmed, " ") {
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, trimmed)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownInvalidTicker, map[string]any{"Symbol": trimmed}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownFetchError, map[string]any{"Symbol": trimmed}))
		}
		session.ActiveSymbol = trimmed
		formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamDrillDown, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})
	}

	// Pertanyaan natural language → Analyst Agent
	return whatsappGatewayService.answerStockInquiry(tx, session, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamDrillDown(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	if body == "menu" || body == "radar" {
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})
	}

	// Pilihan angka: user memilih saham lain berdasarkan nomor urut dari hasil radar sebelumnya
	if idx, err := strconv.Atoi(body); err == nil {
		symbol := session.RadarSymbolByIndex(idx)
		if symbol == "" {
			// Angka di luar range atau sesi tidak punya data radar — tampilkan pesan kontekstual.
			n := len(session.GetRadarSymbols())
			if n == 0 {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgDrillDownNoContext))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownOutOfRange, map[string]any{"Max": n}))
		}
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, symbol)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownInvalidTicker, map[string]any{"Symbol": symbol}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownFetchError, map[string]any{"Symbol": symbol}))
		}
		if drillDown != nil {
			session.ActiveSymbol = symbol
			_ = whatsappGatewayService.upsertSession(tx, session)
			formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
			return whatsappGatewayService.sendMessage(phone, formatted)
		}
	}

	// Drill-down lanjutan: user mengetik kode saham 4 huruf lain
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if len(trimmed) == 4 && !strings.Contains(trimmed, " ") {
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, trimmed)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownInvalidTicker, map[string]any{"Symbol": trimmed}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateWithTemplateData(msgDrillDownFetchError, map[string]any{"Symbol": trimmed}))
		}
		if drillDown != nil {
			session.ActiveSymbol = trimmed
			_ = whatsappGatewayService.upsertSession(tx, session)
			formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
			return whatsappGatewayService.sendMessage(phone, formatted)
		}
	}

	// Pertanyaan natural language → Analyst Agent
	return whatsappGatewayService.answerStockInquiry(tx, session, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) answerStockInquiry(tx *gorm.DB, session *entity.WhatsappSession, phone, userQuestion string) error {
	agentSession := newWhatsAppAgentSession()
	ctx := context.Background()

	effectiveQuestion := userQuestion
	if session != nil && session.ActiveSymbol != "" {
		agentSession.AppendHistory(model.ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("Konteks saham yang sedang dibuka dan ditanyakan pengguna saat ini adalah %s. Jika pengguna menyebut 'saham ini', 'ini', atau tidak menyebut kode saham secara spesifik, fokuskan analisis dan panggil tools Sectors API untuk ticker %s.", session.ActiveSymbol, session.ActiveSymbol),
		})
	}

	err := whatsappGatewayService.agentService.ProcessTurn(ctx, agentSession, effectiveQuestion)
	if err != nil {
		logrus.WithError(err).Error("Error processing agent response for stock inquiry")
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgAgentError))
	}

	response := agentSession.CollectedResponse()
	if response == "" {
		response = whatsappGatewayService.translateLocalization(msgAgentEmptyResult)
	}

	response += whatsappGatewayService.translateLocalization(msgAgentFooter)
	return whatsappGatewayService.sendMessage(phone, response)
}

func (whatsappGatewayService *ServiceImpl) handleRadarOptInPrompt(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgWatchlistUserNotFound))
	}

	enabled := true
	disabled := false

	switch body {
	case "1", "subsektor":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:               "subsector",
			PreferredSubSector: "Banks",
			BroadcastEnabled:   &enabled,
			BroadcastTime:      "08:00",
		})
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOptInSubsectorEnabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "2", "watchlist":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "watchlist",
			BroadcastEnabled: &enabled,
			BroadcastTime:    "08:00",
		})
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOptInWatchlistEnabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "3", "manual", "tidak":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "subsector",
			BroadcastEnabled: &disabled,
			BroadcastTime:    "08:00",
		})
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOptInDisabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	default:
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.translateLocalization(msgOptInInvalidChoice))
	}
}

// extractSymbols extracts ticker symbols from RadarResult into a string slice.
// Used to store the sequence into session so user numeric selections (1, 2, 3...)
// map accurately to the intended stock ticker.
func extractSymbols(result *model.RadarResult) []string {
	if result == nil {
		return nil
	}
	symbols := make([]string, 0, len(result.Tickers))
	for _, t := range result.Tickers {
		symbols = append(symbols, t.Symbol)
	}
	return symbols
}

func (whatsappGatewayService *ServiceImpl) sendMessage(phone, message string) error {
	req := SendMessageRequest{
		Phone:       phone,
		Message:     message,
		Mentions:    []string{},
		IsForwarded: false,
	}
	if !strings.Contains(phone, "@s.whatsapp.net") {
		req.Phone = phone + "@s.whatsapp.net"
	}

	_, err := whatsappGatewayService.restyModule.GetRestyWhatsappGateway().R().
		SetBody(req).
		Post("/send/message")
	return err
}

func (whatsappGatewayService *ServiceImpl) sendImage(phone, imageURL, caption string) error {
	// Download image bytes from URL
	resp, err := resty.New().R().Get(imageURL)
	if err != nil {
		return err
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("failed to fetch image from %s: status %d", imageURL, resp.StatusCode())
	}

	// Send as multipart/form-data to /send/image
	_, err = whatsappGatewayService.restyModule.GetRestyWhatsappGateway().R().
		SetFormData(map[string]string{
			"phone":   phone,
			"caption": caption,
		}).
		SetFileReader("image", "qr.png", bytes.NewReader(resp.Body())).
		Post("/send/image")
	return err
}

func (whatsappGatewayService *ServiceImpl) SendDirectMessage(phone, message string) error {
	return whatsappGatewayService.sendMessage(phone, message)
}
