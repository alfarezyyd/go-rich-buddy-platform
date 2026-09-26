package middleware

import "github.com/gin-gonic/gin"

func RequestMetaMiddleware() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		ipAddress := ginContext.ClientIP()
		userAgent := ginContext.GetHeader("User-Agent")

		// Simpan ke context
		ginContext.Set("ipAddress", ipAddress)
		ginContext.Set("userAgent", userAgent)

		ginContext.Next()
	}
}
