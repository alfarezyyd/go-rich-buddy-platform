package agent

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(request *http.Request) bool {
		return true
	},
}

type Handler struct {
	agentService Service
}

func NewHandler(agentService Service) *Handler {
	return &Handler{
		agentService: agentService,
	}
}

func (agentHandler *Handler) HandleWebSocket(ginContext *gin.Context) {
	websocketConn, err := websocketUpgrader.Upgrade(ginContext.Writer, ginContext.Request, nil)
	if err != nil {
		logrus.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}

	websocketSession := agentHandler.agentService.CreateSession(websocketConn)
	agentHandler.agentService.RunSession(ginContext.Request.Context(), websocketSession)
}
