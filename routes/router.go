package routes

import (
	"go-rich-buddy-platform/pkg/middleware"

	"github.com/gin-gonic/gin"
)

type ApplicationRoutes struct {
	ginEngine            *gin.Engine
	RouterGroup          *gin.RouterGroup
	PublicRoutes         *PublicRoutes
	AuthenticationRoutes *AuthenticationRoutes
	ProtectedRoutes      *ProtectedRoutes
	AgentRoutes          *AgentRoutes
	WhatsappRoutes       *WhatsappRoutes
	RadarRoutes          *RadarRoutes
	PaymentRoutes        *PaymentRoutes
}

func NewApplicationRoutes(
	ginEngine *gin.Engine,
	publicRoutes *PublicRoutes,
	authenticationRoutes *AuthenticationRoutes,
	protectedRoutes *ProtectedRoutes,
	agentRoutes *AgentRoutes,
	whatsappRoutes *WhatsappRoutes,
	radarRoutes *RadarRoutes,
	paymentRoutes *PaymentRoutes,
) *ApplicationRoutes {
	parentRouterGroup := ginEngine.Group("/api/")
	parentRouterGroup.Use(middleware.RequestMetaMiddleware())
	return &ApplicationRoutes{
		ginEngine:            ginEngine,
		RouterGroup:          parentRouterGroup,
		PublicRoutes:         publicRoutes,
		AuthenticationRoutes: authenticationRoutes,
		ProtectedRoutes:      protectedRoutes,
		AgentRoutes:          agentRoutes,
		WhatsappRoutes:       whatsappRoutes,
		RadarRoutes:          radarRoutes,
		PaymentRoutes:        paymentRoutes,
	}
}

func (applicationRoutes *ApplicationRoutes) Setup() {
	if applicationRoutes.PublicRoutes != nil {
		applicationRoutes.PublicRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.PaymentRoutes != nil {
		applicationRoutes.PaymentRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.WhatsappRoutes != nil {
		applicationRoutes.WhatsappRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.RadarRoutes != nil {
		applicationRoutes.RadarRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.AuthenticationRoutes != nil {
		applicationRoutes.AuthenticationRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.ProtectedRoutes != nil {
		applicationRoutes.ProtectedRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.AgentRoutes != nil {
		applicationRoutes.AgentRoutes.Setup(applicationRoutes.RouterGroup)
		applicationRoutes.AgentRoutes.SetupRoot(applicationRoutes.ginEngine)
	}

}
