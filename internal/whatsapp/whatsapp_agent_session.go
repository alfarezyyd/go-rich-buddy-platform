package whatsapp_gateway

import (
	"context"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/model"
	"strings"
	"sync"
)

// WhatsAppAgentSession adalah implementasi agent.Session untuk channel WhatsApp.
// Tidak menggunakan WebSocket — response dari LLM dikumpulkan di buffer
// agar bisa dikirimkan kembali melalui WhatsApp gateway.
type WhatsAppAgentSession struct {
	mu           sync.Mutex
	state        agent.SessionState
	history      []model.ChatMessage
	outputFrames []model.WebsocketServerMessage
}

func newWhatsAppAgentSession() *WhatsAppAgentSession {
	return &WhatsAppAgentSession{
		state:        agent.SessionStateIdle,
		history:      make([]model.ChatMessage, 0),
		outputFrames: make([]model.WebsocketServerMessage, 0),
	}
}

// CollectedResponse menggabungkan semua assistant_message frames menjadi satu
// string yang siap dikirim ke pengguna WhatsApp.
func (s *WhatsAppAgentSession) CollectedResponse() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sb strings.Builder
	for _, frame := range s.outputFrames {
		if frame.Type == "assistant_message" && frame.Content != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(frame.Content)
		}
	}
	return sb.String()
}

// ─── agent.Session implementation ────────────────────────────────────────────

func (s *WhatsAppAgentSession) GetState() agent.SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *WhatsAppAgentSession) SetState(state agent.SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *WhatsAppAgentSession) GetHistory() []model.ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := make([]model.ChatMessage, len(s.history))
	copy(clone, s.history)
	return clone
}

func (s *WhatsAppAgentSession) AppendHistory(msg model.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, msg)
}

func (s *WhatsAppAgentSession) SendFrame(msg model.WebsocketServerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputFrames = append(s.outputFrames, msg)
	return nil
}

func (s *WhatsAppAgentSession) SendBestEffortFrame(msg model.WebsocketServerMessage) {
	_ = s.SendFrame(msg)
}

// EnqueueUserMessage, NextUserMessage, Close, StartPumps adalah no-op karena
// pengiriman pesan di WhatsApp channel dikendalikan oleh state machine,
// bukan oleh session pump loop seperti pada WebSocket.
func (s *WhatsAppAgentSession) EnqueueUserMessage(_ string) {}

func (s *WhatsAppAgentSession) NextUserMessage(_ context.Context) (string, bool) {
	return "", false
}

func (s *WhatsAppAgentSession) Close() {}

func (s *WhatsAppAgentSession) StartPumps() {}
