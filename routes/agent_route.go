package routes

import (
	"go-rich-buddy-platform/internal/agent"

	"github.com/gin-gonic/gin"
)

type AgentRoutes struct {
	agentController agent.Controller
}

func NewAgentRoutes(agentController agent.Controller) *AgentRoutes {
	return &AgentRoutes{
		agentController: agentController,
	}
}

func (agentRoutes *AgentRoutes) Setup(routerGroup *gin.RouterGroup) {
	routerGroup.GET("/ws", agentRoutes.agentController.HandleWebSocket)
	routerGroup.GET("/agent/ws", agentRoutes.agentController.HandleWebSocket)
}

func (agentRoutes *AgentRoutes) SetupRoot(ginEngine *gin.Engine) {
	ginEngine.GET("/ws", agentRoutes.agentController.HandleWebSocket)
}
