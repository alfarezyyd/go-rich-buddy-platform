package agent

import (
	"context"

	"github.com/gorilla/websocket"
)

type Service interface {
	CreateSession(websocketConn *websocket.Conn) Session
	RunSession(ctx context.Context, session Session)
	ProcessTurn(ctx context.Context, session Session, userMessage string) error
	Classify(ctx context.Context, userMessage string) AgentMode
}
