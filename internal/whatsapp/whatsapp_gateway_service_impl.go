package whatsapp_gateway

import (
	"context"
	"fmt"
	"strings"

	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/radar"
	"go-rich-buddy-platform/internal/user"
	whatsappSession "go-rich-buddy-platform/internal/whatsapp_session"

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
	restyModule       *config.RestyModule
}

func NewService(
	dbConnection *gorm.DB,
	sessionRepository whatsappSession.SessionRepository,
	userRepository user.Repository,
	radarService radar.Service,
	agentService agent.Service,
	restyModule *config.RestyModule,
) Service {
	return &ServiceImpl{
		dbConnection:      dbConnection,
		sessionRepository: sessionRepository,
		userRepository:    userRepository,
		radarService:      radarService,
		agentService:      agentService,
		restyModule:       restyModule,
	}
}

func (whatsappGatewayService *ServiceImpl) HandleIncoming(ginContext *gin.Context, textMessage model.TextMessage) error {
	if textMessage.Event != "message" || textMessage.Payload.IsFromMe || textMessage.Payload.Body == "" {
		return nil
	}

	phoneNumber := strings.Split(textMessage.Payload.From, "@")[0]
	payloadBody := strings.TrimSpace(strings.ToLower(textMessage.Payload.Body))
	rawBody := strings.TrimSpace(textMessage.Payload.Body)

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

func (whatsappGatewayService *ServiceImpl) sendMainMenu(phone string) error {
	msg := "Halo, aku RichBuddy 👋 Sahabat analisis pasar saham IDX kamu.\n\n" +
		"Silakan pilih menu:\n" +
		"1️⃣ Register — Buat akun & persona\n" +
		"2️⃣ Order — Lihat paket subscription/kredit\n" +
		"3️⃣ 📊 Radar Saham — 5 saham pilihan harian berbasis data\n\n" +
		"Balas dengan angka (*1*, *2*, atau *3*)."
	return whatsappGatewayService.sendMessage(phone, msg)
}

func (whatsappGatewayService *ServiceImpl) handleMainMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	switch body {
	case "1", "register", "daftar":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err == nil {
			return whatsappGatewayService.sendMessage(phone, "Anda sudah terdaftar! Silakan pilih menu lain:\n*2* untuk Order\n*3* untuk 📊 Radar Saham.")
		}
		return whatsappGatewayService.transitionTo(tx, session, stateRegisterAwaitName, func() error {
			return whatsappGatewayService.sendMessage(phone, "Masukkan nama lengkap Anda:")
		})

	case "2", "order", "paket", "langganan":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Maaf, Anda harus mendaftar dulu. Balas *1* untuk Register.")
		}
		return whatsappGatewayService.transitionTo(tx, session, stateOrderMenu, func() error {
			return whatsappGatewayService.sendOrderMenu(phone)
		})

	case "3", "radar", "radar saham":
		_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Maaf, Anda harus mendaftar dulu sebelum menggunakan Radar Saham. Balas *1* untuk Register.")
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
			_ = whatsappGatewayService.sendMessage(phone, "Terlalu banyak input salah. Kembali ke menu utama...")
			return whatsappGatewayService.sendMainMenu(phone)
		}
		if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
			return err
		}
		return whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Pilihan tidak dikenali. Balas 1, 2, atau 3. (Percobaan %d/%d)", session.RetryCount, maxRetry))
	}
}

func (whatsappGatewayService *ServiceImpl) handleRegisterAwaitName(tx *gorm.DB, session *entity.WhatsappSession, phone, name string) error {
	if strings.TrimSpace(name) == "" {
		return whatsappGatewayService.sendMessage(phone, "Nama tidak boleh kosong. Silakan masukkan nama Anda:")
	}

	_, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err == nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, "Anda sudah terdaftar! Kembali ke menu utama...")
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
		_ = whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Registrasi berhasil, %s!\nAkun dibuat dengan tier Starter (10 kredit).", name))
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendOrderMenu(phone string) error {
	msg := "Pilih paket subscription:\n" +
		"1️⃣ Starter (Gratis) — 10 kredit/hari\n" +
		"2️⃣ Buddy+ — 300 kredit/bulan\n" +
		"3️⃣ Pro/Analyst — unlimited fair-use\n" +
		"4️⃣ Top-up 50 kredit\n\n" +
		"Balas dengan angka (1/2/3/4).\n_(Ketik *batal* untuk kembali ke menu utama)_"
	return whatsappGatewayService.sendMessage(phone, msg)
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
			_ = whatsappGatewayService.sendMessage(phone, "Terlalu banyak input salah. Kembali ke menu utama...")
			return whatsappGatewayService.sendMainMenu(phone)
		}
		_ = whatsappGatewayService.upsertSession(tx, session)
		return whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Pilihan tidak valid. Balas angka 1-4. (Percobaan %d/%d)", session.RetryCount, maxRetry))
	}

	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			return whatsappGatewayService.sendMessage(phone, "Akun tidak ditemukan. Silakan daftar dulu.")
		})
	}

	usr.Tier = tier.name
	usr.CreditBalance += tier.credits
	if err := whatsappGatewayService.userRepository.Update(tx, usr); err != nil {
		return err
	}

	payloadMessage := fmt.Sprintf(
		"🎉 Anda memilih paket *%s*.\n\n(Simulasi: Pembayaran sukses! Saldo kredit Anda sekarang: %d kredit)",
		tier.name, usr.CreditBalance,
	)

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		_ = whatsappGatewayService.sendMessage(phone, payloadMessage)
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendRadarSahamMenu(phone string) error {
	msg := "📊 *Radar Saham*\n" +
		"Mau lihat saham pilihan berdasarkan apa?\n\n" +
		"1️⃣ Subsektor\n" +
		"2️⃣ Watchlist saya\n" +
		"3️⃣ Cari ticker manual (ketik kode saham)\n" +
		"4️⃣ ⚙️ Pengaturan Notifikasi Pagi (Opt-in)\n\n" +
		"Balas dengan angka (*1*, *2*, *3*, atau *4*).\n" +
		"_(Ketik *batal* kapan saja untuk kembali ke menu utama)_"
	return whatsappGatewayService.sendMessage(phone, msg)
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "Akun belum terdaftar.")
	}

	switch body {
	case "1", "subsektor":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamSubsector, func() error {
			msg := "Pilih subsektor yang ingin dipantau:\n\n" +
				"1. 🏦 Banks\n" +
				"2. 🍔 Food & Beverage\n" +
				"3. ⚡ Coal\n" +
				"4. 📡 Telco\n" +
				"5. 🔍 Lainnya (ketik nama subsektor)\n\n" +
				"Balas dengan angka 1-5 atau ketik nama subsektor."
			return whatsappGatewayService.sendMessage(phone, msg)
		})

	case "2", "watchlist":
		watchlist, err := whatsappGatewayService.radarService.GetUserWatchlist(context.Background(), tx, usr.Id)
		if err != nil || len(watchlist.Symbols) == 0 {
			return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamWatchlistAdd, func() error {
				msg := "Watchlist kamu saat ini masih kosong 📝\n\n" +
					"Silakan ketik kode saham yang ingin kamu pantau (pisahkan dengan koma, contoh: *BBCA, BBRI, ANTM, GOTO*):"
				return whatsappGatewayService.sendMessage(phone, msg)
			})
		}

		result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Gagal mengambil data Radar Watchlist: "+err.Error())
		}

		formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})

	case "3", "manual", "cari":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamManual, func() error {
			msg := "Ketikkan kode saham yang ingin dipantau (pisahkan dengan koma, contoh: *BBCA, BBRI, ANTM, GOTO*):"
			return whatsappGatewayService.sendMessage(phone, msg)
		})

	case "4", "optin", "pengaturan", "notifikasi", "broadcast":
		return whatsappGatewayService.transitionTo(tx, session, stateRadarOptInPrompt, func() error {
			msg := "Mau RichBuddy kirim Radar otomatis tiap pagi jam 08:00 WIB? ☀️\n\n" +
				"1️⃣ Ya, kirim untuk subsektor pilihan saya (Banks)\n" +
				"2️⃣ Ya, kirim untuk watchlist saya\n" +
				"3️⃣ Tidak usah, saya cek manual saja\n\n" +
				"Balas dengan angka (1/2/3)."
			return whatsappGatewayService.sendMessage(phone, msg)
		})

	default:
		return whatsappGatewayService.sendMessage(phone, "Pilihan tidak valid. Balas 1 untuk Subsektor, 2 untuk Watchlist, 3 untuk Manual, atau 4 untuk Pengaturan Notifikasi.")
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
			return whatsappGatewayService.sendMessage(phone, "Ketikkan nama subsektor yang ingin dipantau (contoh: *Basic Materials*, *Automotive*, *Technology*):")
		})
	default:
		// Check if user directly typed subsector name
		subSector = raw
	}

	result, err := whatsappGatewayService.radarService.GetRadarBySubSector(context.Background(), tx, userID, subSector)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "Gagal mengambil data subsektor: "+err.Error())
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
		return whatsappGatewayService.sendMessage(phone, "Gagal mengambil data subsektor: "+err.Error())
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
		return whatsappGatewayService.sendMessage(phone, "Kode saham tidak valid. Silakan ketik kode saham (contoh: *BBCA, BBRI, ANTM*):")
	}

	result, err := whatsappGatewayService.radarService.GetRadarByManualTickers(context.Background(), tx, userID, symbols)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "Gagal memproses data ticker: "+err.Error())
	}

	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamWatchlistAdd(tx *gorm.DB, session *entity.WhatsappSession, phone, rawTickers string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "User tidak ditemukan.")
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
		return whatsappGatewayService.sendMessage(phone, "Kode saham tidak valid. Ketik kode saham (contoh: *BBCA, BBRI, ANTM*):")
	}

	_, _ = whatsappGatewayService.radarService.AddToWatchlist(context.Background(), tx, usr.Id, symbols)
	result, err := whatsappGatewayService.radarService.GetRadarByWatchlist(context.Background(), tx, usr.Id)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "Gagal mengambil radar watchlist: "+err.Error())
	}

	formatted := whatsappGatewayService.radarService.FormatRadarMessage(result)
	return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamResults, func() error {
		_ = whatsappGatewayService.sendMessage(phone, fmt.Sprintf("✅ Berhasil menambahkan %d saham ke watchlist kamu!\n", len(symbols)))
		return whatsappGatewayService.sendMessage(phone, formatted)
	})
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamResults(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	if body == "menu" || body == "radar" {
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})
	}

	// Drill down check by number or ticker symbol
	var targetSymbol string
	numberToSymbol := map[string]string{
		"1": "", "2": "", "3": "", "4": "", "5": "",
	}

	date, _ := whatsappGatewayService.radarService.RunTier1(context.Background(), "")
	targetDate := ""
	if date != nil {
		targetDate = date.Date
	}

	// Check if input is a known IDX symbol (e.g. BBCA, BBRI, ANTM, BMRI, TLKM)
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if len(trimmed) == 4 && !strings.Contains(trimmed, " ") {
		targetSymbol = trimmed
	} else if body == "1" || body == "2" || body == "3" || body == "4" || body == "5" {
		// Lookup top signals for default subsector
		signals, _ := whatsappGatewayService.radarService.GetRadarBySubSector(context.Background(), tx, 1, "Banks")
		if signals != nil && len(signals.Tickers) > 0 {
			idx := 0
			fmt.Sscanf(body, "%d", &idx)
			idx--
			if idx >= 0 && idx < len(signals.Tickers) {
				targetSymbol = signals.Tickers[idx].Symbol
			}
		}
		if targetSymbol == "" {
			defaultSymbols := []string{"BBCA", "BMRI", "BRIS", "BBRI", "BNGA"}
			idx := 0
			fmt.Sscanf(body, "%d", &idx)
			idx--
			if idx >= 0 && idx < len(defaultSymbols) {
				targetSymbol = defaultSymbols[idx]
			}
		}
	}
	_ = numberToSymbol
	_ = targetDate

	if targetSymbol != "" {
		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, targetSymbol)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Gagal mengambil data drill-down untuk "+targetSymbol)
		}
		formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamDrillDown, func() error {
			return whatsappGatewayService.sendMessage(phone, formatted)
		})
	}

	// User asked natural language question about recommended stocks - answer using Analyst Agent!
	return whatsappGatewayService.answerStockInquiry(tx, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) handleRadarSahamDrillDown(tx *gorm.DB, session *entity.WhatsappSession, phone, body, raw string) error {
	if body == "menu" || body == "radar" {
		return whatsappGatewayService.transitionTo(tx, session, stateRadarSahamMenu, func() error {
			return whatsappGatewayService.sendRadarSahamMenu(phone)
		})
	}

	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if (len(trimmed) == 4 && !strings.Contains(trimmed, " ")) || body == "1" || body == "2" || body == "3" || body == "4" || body == "5" {
		targetSymbol := trimmed
		if body == "1" || body == "2" || body == "3" || body == "4" || body == "5" {
			defaultSymbols := []string{"BBCA", "BMRI", "BRIS", "BBRI", "BNGA"}
			idx := 0
			fmt.Sscanf(body, "%d", &idx)
			idx--
			if idx >= 0 && idx < len(defaultSymbols) {
				targetSymbol = defaultSymbols[idx]
			}
		}

		drillDown, err := whatsappGatewayService.radarService.GetDrillDownExplanation(context.Background(), tx, targetSymbol)
		if err == nil && drillDown != nil {
			formatted := whatsappGatewayService.radarService.FormatDrillDownMessage(drillDown)
			return whatsappGatewayService.sendMessage(phone, formatted)
		}
	}

	// Natural language follow-up discussion regarding the stock
	return whatsappGatewayService.answerStockInquiry(tx, phone, raw)
}

func (whatsappGatewayService *ServiceImpl) answerStockInquiry(tx *gorm.DB, phone, userQuestion string) error {
	agentSession := newWhatsAppAgentSession()
	ctx := context.Background()

	err := whatsappGatewayService.agentService.ProcessTurn(ctx, agentSession, userQuestion)
	if err != nil {
		logrus.WithError(err).Error("Error processing agent response for stock inquiry")
		return whatsappGatewayService.sendMessage(phone, "Maaf, terjadi kendala saat menganalisis pertanyaan Anda. Silakan tanyakan hal lain seputar saham rekomendasi atau ketik *menu*.")
	}

	response := agentSession.CollectedResponse()
	if response == "" {
		response = "Berikut adalah data pasar terkini untuk saham yang Anda tanyakan. Ketik *menu* untuk melihat Radar Saham lainnya."
	}

	response += "\n\n_(Ketik *menu* untuk kembali ke menu Radar Saham atau *batal* untuk menu utama)_"
	return whatsappGatewayService.sendMessage(phone, response)
}

func (whatsappGatewayService *ServiceImpl) handleRadarOptInPrompt(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.sendMessage(phone, "User tidak ditemukan.")
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
		msg := "✅ *Notifikasi Pagi Aktif!* RichBuddy akan mengirimkan Radar Subsektor Banks otomatis setiap pagi jam 08:00 WIB."
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, msg)
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "2", "watchlist":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "watchlist",
			BroadcastEnabled: &enabled,
			BroadcastTime:    "08:00",
		})
		msg := "✅ *Notifikasi Pagi Aktif!* RichBuddy akan mengirimkan Radar untuk Watchlist kamu otomatis setiap pagi jam 08:00 WIB."
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, msg)
			return whatsappGatewayService.sendMainMenu(phone)
		})

	case "3", "manual", "tidak":
		_, _ = whatsappGatewayService.radarService.SetUserPreference(context.Background(), tx, usr.Id, model.SetRadarPreferenceRequest{
			Mode:             "subsector",
			BroadcastEnabled: &disabled,
			BroadcastTime:    "08:00",
		})
		msg := "Pengiriman otomatis dinonaktifkan. Anda tetap dapat mengakses Radar Saham kapan saja melalui menu."
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, msg)
			return whatsappGatewayService.sendMainMenu(phone)
		})

	default:
		return whatsappGatewayService.sendMessage(phone, "Pilihan tidak valid. Balas 1 untuk Subsektor, 2 untuk Watchlist, atau 3 untuk Manual.")
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
