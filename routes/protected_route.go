package routes

import (
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/user"
	"go-rich-buddy-platform/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type ProtectedRoutes struct {
	viperConfig    *viper.Viper
	redisInstance  *config.RedisInstance
	userController user.Controller
}

func NewProtectedRoutes(
	viperConfig *viper.Viper,
	redisInstance *config.RedisInstance,

	userController user.Controller,

) *ProtectedRoutes {
	return &ProtectedRoutes{
		viperConfig:    viperConfig,
		userController: userController,
	}
}

func (protectedRoutes *ProtectedRoutes) Setup(routerGroup *gin.RouterGroup) {
	routerGroup.Use(middleware.AuthMiddleware(protectedRoutes.viperConfig, protectedRoutes.redisInstance))
}
