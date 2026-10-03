package whatsapp_gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/memory"
	"go-rich-buddy-platform/internal/memory/privacy"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/radar"
	"go-rich-buddy-platform/internal/user"
	whatsappSession "go-rich-buddy-platform/internal/whatsapp_session"
	pkgi18n "go-rich-buddy-platform/pkg/i18n"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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
	if phoneNumber != "6289637577001" {
		return nil
	}
	payloadBody := strings.TrimSpace(strings.ToLower(textMessage.Payload.Body))
	rawBody := strings.TrimSpace(textMessage.Payload.Body)
	waMessageID := textMessage.Payload.Id

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
		usr, _ := whatsappGatewayService.userRepository.FindByPhone(tx, phoneNumber)

		// Persist the incoming message (with PII redaction + dedup).
		if usr != nil && waMessageID != "" {
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
			return whatsappGatewayService.sendMainMenu(phoneNumber)
		}

		switch whatsappSession.CurrentState {
		case stateMainMenu:
			return whatsappGatewayService.handleMainMenu(tx, whatsappSession, phoneNumber, payloadBody)
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
			return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMemoryEmpty))
		}
		return true, whatsappGatewayService.sendMessage(phone, formatMemoryItems(items))

	case privacy.CommandForgetItem:
		if target == "" {
			return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMemoryForgetInvalid))
		}
		_ = whatsappGatewayService.memoryService.ForgetMemoryByKey(ctx, tx, userID, target)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMemoryForgotConfirm))

	case privacy.CommandDeleteChatHistory:
		_ = whatsappGatewayService.memoryService.DeleteChatHistory(ctx, tx, userID)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgChatHistoryDeleted))

	case privacy.CommandDeleteAllMemory:
		_ = whatsappGatewayService.memoryService.DeleteAllMemory(ctx, tx, userID)
		return true, whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgAllMemoryDeleted))
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

// t translates a plain message key with no template data.
func (whatsappGatewayService *ServiceImpl) t(key string) string {
	return whatsappGatewayService.localizer.T(key)
}

// td translates a message key with template data.
func (whatsappGatewayService *ServiceImpl) td(key string, data any) string {
	return whatsappGatewayService.localizer.TData(key, data)
}

// tp translates a plural-aware message key.
func (whatsappGatewayService *ServiceImpl) tp(key string, count int, data any) string {
	return whatsappGatewayService.localizer.TPlural(key, count, data)
}

func (whatsappGatewayService *ServiceImpl) sendMainMenu(phone string) error {
	return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuGreeting))
}

func (whatsappGatewayService *ServiceImpl) handleMainMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	switch body {
	case "1", "register", "daftar":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err == nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuAlreadyRegistered))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateRegisterAwaitName, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuEnterFullName))
		})

	case "2", "order", "paket", "langganan":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuMustRegisterFirst))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateOrderMenu, func() error {
			return whatsappGatewayService.sendOrderMenu(phone)
		})

	case "3", "radar", "radar saham":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuMustRegisterRadar))
		}
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})

	default:
		session.RetryCount++
		if session.RetryCount >= maxRetry {
			session.RetryCount = 0
			session.CurrentState = stateMainMenu
			if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
				return err
			}
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuTooManyWrongInput))
			return whatsappGatewayService.sendMainMenu(phone)
		}
		if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
			return err
		}
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgMainMenuUnrecognizedChoice, map[string]any{
			"Current": session.RetryCount,
			"Max":     maxRetry,
		}))
	}
}

func (whatsappGatewayService *ServiceImpl) handleRegisterAwaitName(tx *gorm.DB, session *entity.WhatsappSession, phone, name string) error {
	if strings.TrimSpace(name) == "" {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgRegisterNameCannotBeEmpty))
	}

	_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err == nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgRegisterAlreadyRegistered))
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
		_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgRegisterSuccess, map[string]any{"Name": name}))
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendOrderMenu(phone string) error {
	return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOrderMenu))
}

func (whatsappGatewayService *ServiceImpl) handleOrderMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	type tierInfo struct {
		name    string
		credits int
	}
	tiers := map[string]tierInfo{
		"1": {"Starter", 10},
		"2": {"Buddy+", 300},
		"3": {"Pro/Analyst", 9999},
		"4": {"Top-up", 50},
	}

	tier, ok := tiers[body]
	if !ok {
		session.RetryCount++
		if session.RetryCount >= maxRetry {
			session.RetryCount = 0
			session.CurrentState = stateMainMenu
			_ = whatsappGatewayService.upsertSession(tx, session)
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgMainMenuTooManyWrongInput))
			return whatsappGatewayService.sendMainMenu(phone)
		}
		_ = whatsappGatewayService.upsertSession(tx, session)
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgOrderInvalidChoice, map[string]any{
			"Current": session.RetryCount,
			"Max":     maxRetry,
		}))
	}

	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOrderAccountNotFound))
		})
	}

	usr.Tier = tier.name
	usr.CreditBalance += tier.credits
	if err := whatsappGatewayService.userRepository.Update(tx, usr); err != nil {
		return err
	}

	payloadMessage := whatsappGatewayService.td(msgOrderSuccess, map[string]any{
		"Tier":    tier.name,
		"Credits": usr.CreditBalance,
	})

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		_ = whatsappGatewayService.sendMessage(phone, payloadMessage)
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendRadarSahamMenu(phone string) error {
	return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgRadarSahamMenu))
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgRadarAccountNotRegistered))
	}

	switch body {
	case "1", "subsektor":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamSubsector, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgSubsectorMenu))
		})

	case "2", "watchlist":
		watchlist, err := whatsappGatewayService.radarService.GetUserWatchlist(context.Background(), tx, usr.Id)
		if err != nil || len(watchlist.Symbols) == 0 {
			return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamWatchlistAdd, func() error {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgWatchlistEmpty))
			})
		}

		result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgWatchlistFetchError, map[string]any{"Error": err.Error()}))
		}

		formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})

	case "3", "manual", "cari":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamManual, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgManualTickerPrompt))
		})

	case "4", "optin", "pengaturan", "notifikasi", "broadcast":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarOptInPrompt, func() error {
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOptInPrompt))
		})

	default:
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgRadarInvalidChoice))
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
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgSubsectorCustomPrompt))
		})
	default:
		// Check if user directly typed subsector name
		subSector = raw
	}

	result, err := whatsappGatewayService.radarService.GetRadarBySubSector(context.Background(), tx, userID, subSector)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgSubsectorFetchError, map[string]any{"Error": err.Error()}))
	}

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
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgSubsectorFetchError, map[string]any{"Error": err.Error()}))
	}

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
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgManualInvalidTicker))
	}

	result, err := whatsappGatewayService.radarService.GetRadarByManualTickers(context.Background(), tx, userID, symbols)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgManualTickerFetchError, map[string]any{"Error": err.Error()}))
	}

	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamWatchlistAdd(tx *gorm.DB, session *entity.WhatsappSession, phone, rawTickers string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgWatchlistUserNotFound))
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
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgWatchlistInvalidTicker))
	}

	_, _ = whatsappGatewayService.radarService.AddToWatchlist(context.Background(), tx, usr.Id, symbols)
	result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgWatchlistAddFetchError, map[string]any{"Error": err.Error()}))
	}

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

	// Drill-down: user mengetik kode saham 4 huruf (misal: BBCA, BMRI, TLKM)
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if len(trimmed) == 4 && !strings.Contains(trimmed, " ") {
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, trimmed)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgDrillDownInvalidTicker, map[string]any{"Symbol": trimmed}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgDrillDownFetchError, map[string]any{"Symbol": trimmed}))
		}
		formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamDrillDown, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})
	}

	// Pertanyaan natural language → Analyst Agent
	return whatsappGatewayService.answerStockInquiry(tx, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamDrillDown(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	if body == "menu" || body == "radar" {
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})
	}

	// Drill-down lanjutan: user mengetik kode saham 4 huruf lain
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if len(trimmed) == 4 && !strings.Contains(trimmed, " ") {
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, trimmed)
		if err != nil {
			if errors.Is(err, radar.ErrTickerNotFound) {
				return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgDrillDownInvalidTicker, map[string]any{"Symbol": trimmed}))
			}
			return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.td(msgDrillDownFetchError, map[string]any{"Symbol": trimmed}))
		}
		if drillDown != nil {
			formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
			return whatsappGatewayService.sendMessage(phone, formatted)
		}
	}

	// Pertanyaan natural language → Analyst Agent
	return whatsappGatewayService.answerStockInquiry(tx, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) answerStockInquiry(tx *gorm.DB, phone, userQuestion string) error {
	agentSession := newWhatsAppAgentSession()
	ctx := context.Background()

	err := whatsappGatewayService.agentService.ProcessTurn(ctx, agentSession, userQuestion)
	if err != nil {
		logrus.WithError(err).Error("Error processing agent response for stock inquiry")
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgAgentError))
	}

	response := agentSession.CollectedResponse()
	if response == "" {
		response = whatsappGatewayService.t(msgAgentEmptyResult)
	}

	response += whatsappGatewayService.t(msgAgentFooter)
	return whatsappGatewayService.sendMessage(phone, response)
}

func (whatsappGatewayService *ServiceImpl) handleRadarOptInPrompt(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgWatchlistUserNotFound))
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
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOptInSubsectorEnabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "2", "watchlist":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "watchlist",
			BroadcastEnabled: &enabled,
			BroadcastTime:    "08:00",
		})
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOptInWatchlistEnabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "3", "manual", "tidak":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "subsector",
			BroadcastEnabled: &disabled,
			BroadcastTime:    "08:00",
		})
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOptInDisabled))
			return whatsappGatewayService.sendMainMenu(phone)
		})

	default:
		return whatsappGatewayService.sendMessage(phone, whatsappGatewayService.t(msgOptInInvalidChoice))
	}
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
