package agent

import (
	"context"
	"encoding/json"
	"go-rich-buddy-platform/internal/model"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type SessionState string

const (
	SessionStateConnected  SessionState = "Connected"
	SessionStateIdle       SessionState = "Idle"
	SessionStateProcessing SessionState = "Processing"
	SessionStateClosed     SessionState = "Closed"
)

type Session interface {
	GetState() SessionState
	SetState(state SessionState)
	GetHistory() []model.ChatMessage
	AppendHistory(message model.ChatMessage)
	SendFrame(serverMessage model.WebsocketServerMessage) error
	SendBestEffortFrame(serverMessage model.WebsocketServerMessage)
	EnqueueUserMessage(userMessage string)
	NextUserMessage(ctx context.Context) (string, bool)
	Close()
	StartPumps()
}

type SessionImpl struct {
	websocketConn    *websocket.Conn
	sessionState     SessionState
	chatHistory      []model.ChatMessage
	userMessageQueue chan string
	sendQueue        chan []byte
	closeSignal      chan struct{}
	closeOnce        sync.Once
	stateMutex       sync.RWMutex
	historyMutex     sync.RWMutex
}

func NewSession(websocketConn *websocket.Conn) Session {
	return &SessionImpl{
		websocketConn:    websocketConn,
		sessionState:     SessionStateConnected,
		chatHistory:      make([]model.ChatMessage, 0),
		userMessageQueue: make(chan string, 100),
		sendQueue:        make(chan []byte, 256),
		closeSignal:      make(chan struct{}),
	}
}

func (sessionImpl *SessionImpl) GetState() SessionState {
	sessionImpl.stateMutex.RLock()
	defer sessionImpl.stateMutex.RUnlock()
	return sessionImpl.sessionState
}

func (sessionImpl *SessionImpl) SetState(newState SessionState) {
	sessionImpl.stateMutex.Lock()
	defer sessionImpl.stateMutex.Unlock()
	sessionImpl.sessionState = newState
}

func (sessionImpl *SessionImpl) GetHistory() []model.ChatMessage {
	sessionImpl.historyMutex.RLock()
	defer sessionImpl.historyMutex.RUnlock()
	clonedHistory := make([]model.ChatMessage, len(sessionImpl.chatHistory))
	copy(clonedHistory, sessionImpl.chatHistory)
	return clonedHistory
}

func (sessionImpl *SessionImpl) AppendHistory(chatMessage model.ChatMessage) {
	sessionImpl.historyMutex.Lock()
	defer sessionImpl.historyMutex.Unlock()
	sessionImpl.chatHistory = append(sessionImpl.chatHistory, chatMessage)
}

func (sessionImpl *SessionImpl) SendFrame(serverMessage model.WebsocketServerMessage) error {
	payload, err := json.Marshal(serverMessage)
	if err != nil {
		return err
	}

	select {
	case <-sessionImpl.closeSignal:
		return websocket.ErrCloseSent
	case sessionImpl.sendQueue <- payload:
		return nil
	}
}

func (sessionImpl *SessionImpl) SendBestEffortFrame(serverMessage model.WebsocketServerMessage) {
	payload, err := json.Marshal(serverMessage)
	if err != nil {
		return
	}

	select {
	case <-sessionImpl.closeSignal:
		return
	case sessionImpl.sendQueue <- payload:
		return
	default:
		logrus.Warn("Session sendQueue full, best-effort frame dropped")
	}
}

func (sessionImpl *SessionImpl) EnqueueUserMessage(userMessage string) {
	select {
	case <-sessionImpl.closeSignal:
		return
	case sessionImpl.userMessageQueue <- userMessage:
		return
	default:
		logrus.Warn("Session userMessageQueue full, message dropped")
	}
}

func (sessionImpl *SessionImpl) NextUserMessage(ctx context.Context) (string, bool) {
	select {
	case <-ctx.Done():
		return "", false
	case <-sessionImpl.closeSignal:
		return "", false
	case msg, ok := <-sessionImpl.userMessageQueue:
		return msg, ok
	}
}

func (sessionImpl *SessionImpl) Close() {
	sessionImpl.closeOnce.Do(func() {
		sessionImpl.SetState(SessionStateClosed)
		close(sessionImpl.closeSignal)
		if sessionImpl.websocketConn != nil {
			_ = sessionImpl.websocketConn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second),
			)
			_ = sessionImpl.websocketConn.Close()
		}
	})
}

func (sessionImpl *SessionImpl) StartPumps() {
	go sessionImpl.writePump()
	go sessionImpl.readPump()
}

func (sessionImpl *SessionImpl) writePump() {
	defer sessionImpl.Close()

	for {
		select {
		case <-sessionImpl.closeSignal:
			return
		case message, ok := <-sessionImpl.sendQueue:
			if !ok {
				return
			}
			if err := sessionImpl.websocketConn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

func (sessionImpl *SessionImpl) readPump() {
	defer sessionImpl.Close()

	sessionImpl.websocketConn.SetReadLimit(512 * 1024)
	_ = sessionImpl.websocketConn.SetReadDeadline(time.Now().Add(60 * time.Second))
	sessionImpl.websocketConn.SetPongHandler(func(string) error {
		_ = sessionImpl.websocketConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messagePayload, err := sessionImpl.websocketConn.ReadMessage()
		if err != nil {
			break
		}
		_ = sessionImpl.websocketConn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var clientMessage model.WSClientMessage
		if unmarshalErr := json.Unmarshal(messagePayload, &clientMessage); unmarshalErr != nil {
			continue
		}

		switch clientMessage.Type {
		case "ping":
			sessionImpl.SendBestEffortFrame(model.WebsocketServerMessage{
				Type: "pong",
			})
		case "user_message":
			text := clientMessage.GetUserMessage()
			if text != "" {
				sessionImpl.EnqueueUserMessage(text)
			}
		}
	}
}
