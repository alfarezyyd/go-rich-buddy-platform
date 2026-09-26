package user

import "github.com/gin-gonic/gin"

type Controller interface {
	LoginUser(ginContext *gin.Context)
	LogoutUser(ginContext *gin.Context)
	FindAllUser(ginContext *gin.Context)
	FindById(ginContext *gin.Context)
	FindSelf(ginContext *gin.Context)
	FindAllUserPagination(ginContext *gin.Context)
	CreateUser(ginContext *gin.Context)
	UpdateUser(ginContext *gin.Context)
	UpdateProfile(ginContext *gin.Context)
	DeleteUser(ginContext *gin.Context)
}
