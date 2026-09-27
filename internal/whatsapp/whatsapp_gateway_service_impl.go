package whatsapp_gateway

import (
	"fmt"
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/user"
	whatsappSession "go-rich-buddy-platform/internal/whatsapp_session"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// States
const (
	stateMainMenu          = "MAIN_MENU"
	stateRegisterAwaitName = "REGISTER_AWAIT_NAME"
	stateOrderMenu         = "ORDER_MENU"
	stateFreeChatFlow      = "FREE_CHAT_FLOW"
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
	restyModule       *config.RestyModule
}

func NewService(dbConnection *gorm.DB, sessionRepository whatsappSession.SessionRepository, userRepository user.Repository, restyModule *config.RestyModule) *ServiceImpl {
	return &ServiceImpl{
		dbConnection:      dbConnection,
		sessionRepository: sessionRepository,
		userRepository:    userRepository,
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
			// No session: create MAIN_MENU and process input in same turn
			whatsappSession = &entity.WhatsappSession{
				Phone:        phoneNumber,
				CurrentState: stateMainMenu,
			}
			if err := whatsappGatewayService.sessionRepository.Upsert(tx, whatsappSession); err != nil {
				return err
			}
			// Fall through to handle the input that triggered session creation
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
		case stateFreeChatFlow:
			return whatsappGatewayService.handleFreeChatFlow(tx, whatsappSession, phoneNumber, rawBody)
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

// upsertSession saves session and returns error.
func (whatsappGatewayService *ServiceImpl) upsertSession(tx *gorm.DB, session *entity.WhatsappSession) error {
	return whatsappGatewayService.sessionRepository.Upsert(tx, session)
}

// transitionTo updates state+retry, persists, then calls next.
func (whatsappGatewayService *ServiceImpl) transitionTo(tx *gorm.DB, session *entity.WhatsappSession, state string, next func() error) error {
	session.CurrentState = state
	session.RetryCount = 0
	if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
		return err
	}
	return next()
}

func (whatsappGatewayService *ServiceImpl) sendMainMenu(phone string) error {
	msg := "Halo, aku RichBuddy 👋\n" +
		"Silakan pilih:\n" +
		"1️⃣ Register — buat akun & persona\n" +
		"2️⃣ Order — lihat paket subscription/kredit\n" +
		"3️⃣ Free Chat — ngobrol bebas soal saham/sektor\n" +
		"Balas dengan angka (1/2/3)."
	return whatsappGatewayService.sendMessage(phone, msg)
}

func (whatsappGatewayService *ServiceImpl) handleMainMenu(gormTransaction *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	switch body {
	case "1", "register", "daftar":
		_, err := whatsappGatewayService.userRepository.FindByPhone(gormTransaction, phone)
		if err == nil {
			return whatsappGatewayService.sendMessage(phone, "Anda sudah terdaftar! Silakan pilih menu lain (2 untuk Order, 3 untuk Chat).")
		}
		return whatsappGatewayService.transitionTo(gormTransaction, session, stateRegisterAwaitName, func() error {
			return whatsappGatewayService.sendMessage(phone, "Masukkan nama Anda:")
		})

	case "2", "order", "paket", "langganan":
		_, err := whatsappGatewayService.userRepository.FindByPhone(gormTransaction, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Maaf, Anda harus daftar dulu. Balas 1 untuk Register.")
		}
		return whatsappGatewayService.transitionTo(gormTransaction, session, stateOrderMenu, func() error {
			return whatsappGatewayService.sendOrderMenu(phone)
		})

	case "3", "chat", "bebas":
		_, err := whatsappGatewayService.userRepository.FindByPhone(gormTransaction, phone)
		if err != nil {
			return whatsappGatewayService.sendMessage(phone, "Maaf, Anda harus daftar dulu. Balas 1 untuk Register.")
		}
		return whatsappGatewayService.transitionTo(gormTransaction, session, stateFreeChatFlow, func() error {
			return whatsappGatewayService.sendMessage(phone, "Anda memasuki Free Chat. Ketikkan pertanyaan seputar saham/sektor:\n_(Ketik *batal* kapan saja untuk kembali ke menu utama)_")
		})

	default:
		session.RetryCount++
		if session.RetryCount >= maxRetry {
			session.RetryCount = 0
			session.CurrentState = stateMainMenu
			if err := whatsappGatewayService.upsertSession(gormTransaction, session); err != nil {
				return err
			}
			_ = whatsappGatewayService.sendMessage(phone, "Terlalu banyak input salah. Kembali ke menu utama...")
			return whatsappGatewayService.sendMainMenu(phone)
		}
		if err := whatsappGatewayService.upsertSession(gormTransaction, session); err != nil {
			return err
		}
		return whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Pilihan tidak dikenali. Balas 1, 2, atau 3. (Percobaan %d/%d)", session.RetryCount, maxRetry))
	}
}

// handleRegisterAwaitName: any non-empty trimmed text is accepted as name.
func (whatsappGatewayService *ServiceImpl) handleRegisterAwaitName(tx *gorm.DB, session *entity.WhatsappSession, phone, name string) error {
	if name == "" {
		return whatsappGatewayService.sendMessage(phone, "Nama tidak boleh kosong. Silakan masukkan nama Anda:")
	}

	// Guard: already registered
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
		CreditBalance: 5,
	}
	if err := whatsappGatewayService.userRepository.Create(tx, newUser); err != nil {
		return err
	}

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		_ = whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Registrasi berhasil, %s!\nAkun dibuat dengan tier Starter (5 kredit).", name))
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) sendOrderMenu(phone string) error {
	msg := "Pilih paket:\n" +
		"1️⃣ Starter (Gratis) — 5 kredit/hari\n" +
		"2️⃣ Buddy+ — 300 kredit/bulan\n" +
		"3️⃣ Pro/Analyst — unlimited fair-use\n" +
		"4️⃣ Top-up kredit\n" +
		"Balas dengan angka (1/2/3/4).\n_(Ketik *batal* untuk kembali ke menu utama)_"
	return whatsappGatewayService.sendMessage(phone, msg)
}

func (whatsappGatewayService *ServiceImpl) handleOrderMenu(tx *gorm.DB, session *entity.WhatsappSession, phone, body string) error {
	type tierInfo struct {
		name    string
		credits int
	}
	tiers := map[string]tierInfo{
		"1": {"Starter", 5},
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
			if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
				return err
			}
			_ = whatsappGatewayService.sendMessage(phone, "Terlalu banyak input salah. Kembali ke menu utama...")
			return whatsappGatewayService.sendMainMenu(phone)
		}
		if err := whatsappGatewayService.upsertSession(tx, session); err != nil {
			return err
		}
		return whatsappGatewayService.sendMessage(phone, fmt.Sprintf("Pilihan tidak valid. Balas angka 1-4. (Percobaan %d/%d)", session.RetryCount, maxRetry))
	}

	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			return whatsappGatewayService.sendMessage(phone, "Akun tidak ditemukan. Sesi dikembalikan ke menu utama.")
		})
	}

	usr.Tier = tier.name
	usr.CreditBalance += tier.credits
	if err := whatsappGatewayService.userRepository.Update(tx, usr); err != nil {
		return err
	}

	// ponytail: mock QRIS; replace with real Midtrans payment link when webhook ready
	qrCodeLink := "https://api.sandbox.midtrans.com/v2/qris/dummy"
	payloadMessage := fmt.Sprintf(
		"Anda memilih paket %s.\nSilakan bayar menggunakan QRIS berikut: %s\n\n(Simulasi: Pembayaran sukses! Saldo Anda sekarang: %d kredit)",
		tier.name, qrCodeLink, usr.CreditBalance,
	)

	return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
		_ = whatsappGatewayService.sendMessage(phone, payloadMessage)
		return whatsappGatewayService.sendMainMenu(phone)
	})
}

func (whatsappGatewayService *ServiceImpl) handleFreeChatFlow(tx *gorm.DB, session *entity.WhatsappSession, phone, message string) error {
	usr, err := whatsappGatewayService.userRepository.FindByPhone(tx, phone)
	if err != nil {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, "Anda harus Register terlebih dahulu. Kembali ke menu utama...")
			return whatsappGatewayService.sendMainMenu(phone)
		})
	}

	if usr.CreditBalance <= 0 {
		return whatsappGatewayService.transitionTo(tx, session, stateMainMenu, func() error {
			_ = whatsappGatewayService.sendMessage(phone, "Kredit Anda habis. Silakan pilih menu Order untuk top-up.")
			return whatsappGatewayService.sendMainMenu(phone)
		})
	}

	usr.CreditBalance--
	if err := whatsappGatewayService.userRepository.Update(tx, usr); err != nil {
		return err
	}

	// ponytail: echo only; replace with real AI call when agent ready
	return whatsappGatewayService.sendMessage(phone, fmt.Sprintf("AI Buddy merespons: Anda bertanya '%s'. (Sisa kredit: %d)", message, usr.CreditBalance))
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
