package middleware

import (
	"go-rich-buddy-platform/pkg/exception"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
	"github.com/gin-gonic/gin"
)

func RateLimitMiddleware(limiters ...*limiter.Limiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, lmt := range limiters {
			httpError := tollbooth.LimitByRequest(lmt, ctx.Writer, ctx.Request)
			if httpError != nil {
				ctx.AbortWithStatusJSON(httpError.StatusCode, exception.NewApplicationError(httpError.StatusCode, httpError.Message))
				return
			}
		}
		ctx.Next()
	}
}
