package agent

import "github.com/gin-gonic/gin"

type Controller interface {
	HandleWebSocket(ginContext *gin.Context)
}
