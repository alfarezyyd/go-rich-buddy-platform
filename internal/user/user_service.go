package user

import (
	"go-rich-buddy-platform/internal/model"

	"github.com/gin-gonic/gin"
)

type Service interface {
	FindAll() []*model.UserResponse
	FindById(ginContext *gin.Context, userId uint64) *model.UserResponse
	FindAllPagination(paginationReq *model.PaginationRequest) *model.PaginatedResponse[*model.UserResponse]
	HandleLogin(ginContext *gin.Context, loginUserRequest *model.LoginUserRequest) string
	HandleLogout(ginContext *gin.Context)
	Create(ginContext *gin.Context, createUserRequest *model.CreateUserRequest) *model.PaginatedResponse[*model.UserResponse]
	Update(ginContext *gin.Context, updateUserRequest *model.UpdateUserRequest) *model.PaginatedResponse[*model.UserResponse]
	UpdateProfile(ginContext *gin.Context, updateUserProfileRequest *model.UpdateUserProfileRequest) string
	Delete(ginContext *gin.Context, deleteUserRequest *model.DeleteResourceGeneralRequest) *model.PaginatedResponse[*model.UserResponse]
	FindSelf(ginContext *gin.Context) *model.UserResponse
}
