package routes

import (
	"go-rich-buddy-platform/pkg/middleware"

	"github.com/gin-gonic/gin"
)

// ApplicationRoutes holds all route groups
type ApplicationRoutes struct {
	RouterGroup          *gin.RouterGroup
	PublicRoutes         *PublicRoutes
	AuthenticationRoutes *AuthenticationRoutes
	ProtectedRoutes      *ProtectedRoutes
	// Add other route groups here
}

func NewApplicationRoutes(
	ginEngine *gin.Engine,
	publicRoutes *PublicRoutes,
	authenticationRoutes *AuthenticationRoutes,
	protectedRoutes *ProtectedRoutes) *ApplicationRoutes {
	parentRouterGroup := ginEngine.Group("/api/")
	parentRouterGroup.Use(middleware.RequestMetaMiddleware())
	return &ApplicationRoutes{
		RouterGroup:          parentRouterGroup,
		PublicRoutes:         publicRoutes,
		AuthenticationRoutes: authenticationRoutes,
		ProtectedRoutes:      protectedRoutes,
	}
}

func (applicationRoutes *ApplicationRoutes) Setup() {
	if applicationRoutes.PublicRoutes != nil {
		applicationRoutes.PublicRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.AuthenticationRoutes != nil {
		applicationRoutes.AuthenticationRoutes.Setup(applicationRoutes.RouterGroup)
	}
	if applicationRoutes.ProtectedRoutes != nil {
		applicationRoutes.ProtectedRoutes.Setup(applicationRoutes.RouterGroup)
	}
}
