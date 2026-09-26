package routes

import (
	"github.com/gin-gonic/gin"
)

type PublicRoutes struct {
}

func NewPublicRoutes(routerGroup *gin.RouterGroup) *PublicRoutes {
	return &PublicRoutes{}
}

func (publicRoutes *PublicRoutes) Setup(routerGroup *gin.RouterGroup) {
	publicRouterGroup := routerGroup.Group("/public")

	// Serve static files from "./uploads"
	publicRouterGroup.Static("/uploads", "./uploads")

}
