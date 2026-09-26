package middleware

import (
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/helper"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func AuthMiddleware(viperConfig *viper.Viper, redisConfig *config.RedisInstance) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		authHeader := ginContext.GetHeader("Authorization")
		apiKeyHeader := ginContext.GetHeader("X-API-HEADER")
		if authHeader == "" && apiKeyHeader == "" {
			ginContext.JSON(http.StatusUnauthorized, helper.NewErrorResponse("", helper.NewErrorDetail(
				exception.StatusAuthError, exception.MsgAuthError, nil)))
			ginContext.Abort()
			return
		}

		if apiKeyHeader == viperConfig.GetString("API_KEY") {
			ginContext.Next()
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, http.ErrAbortHandler
			}
			return []byte(viperConfig.GetString("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			ginContext.JSON(http.StatusUnauthorized, helper.NewErrorResponse("", helper.NewErrorDetail(
				exception.StatusAuthError, exception.MsgAuthError, nil)))
			ginContext.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			ginContext.JSON(http.StatusUnauthorized, helper.NewErrorResponse("", helper.NewErrorDetail(
				exception.StatusAuthError, exception.MsgAuthError, nil)))
			ginContext.Abort()
			return
		}

		if err == redis.Nil {
			ginContext.JSON(http.StatusUnauthorized, helper.NewErrorResponse("", helper.NewErrorDetail(
				exception.StatusAuthError, exception.ErrTokenExpiredOrInvalid, nil)))
			ginContext.Abort()
			return
		} else if err != nil {
			ginContext.JSON(http.StatusInternalServerError, helper.NewErrorResponse("", helper.NewErrorDetail(
				exception.StatusInternalError, "Redis error/down", nil)))

			ginContext.Abort()
			return
		}

		userJwtClaim := helper.MapCreateRequestIntoEntity[jwt.MapClaims, model.JwtClaimRequest](&claims)

		ginContext.Set("claims", userJwtClaim)
		ginContext.Next()
	}
}
