package user

import (
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/helper"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/form"
	"github.com/spf13/viper"
)

type Handler struct {
	userService Service
	formDecoder *form.Decoder
	viperConfig *viper.Viper
}

func NewHandler(userService Service, viperConfig *viper.Viper) *Handler {
	return &Handler{
		userService: userService,
		viperConfig: viperConfig,
		formDecoder: form.NewDecoder(),
	}
}

func (userHandler *Handler) FindAllUser(ginContext *gin.Context) {
	userResponses := userHandler.userService.FindAll()
	ginContext.JSON(http.StatusOK, helper.WriteSuccess("User has been fetched", userResponses))
}

func (userHandler *Handler) FindById(ginContext *gin.Context) {
	userId := ginContext.Param("id")
	parsedUserId, err := strconv.ParseUint(userId, 10, 64)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrParameterInvalid))
	userResponse := userHandler.userService.FindById(ginContext, parsedUserId)
	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("User fetched successfully", userResponse))
}

func (userHandler *Handler) FindSelf(ginContext *gin.Context) {
	userResponse := userHandler.userService.FindSelf(ginContext)
	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("User fetched successfully", userResponse))
}

func (userHandler *Handler) LoginUser(ginContext *gin.Context) {
	var loginUserRequest model.LoginUserRequest
	err := ginContext.ShouldBindBodyWithJSON(&loginUserRequest)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	generatedToken := userHandler.userService.HandleLogin(ginContext, &loginUserRequest)
	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("User logged successfully", gin.H{
		"token": generatedToken,
	}))
}

func (userHandler *Handler) LogoutUser(ginContext *gin.Context) {
	userHandler.userService.HandleLogout(ginContext)
	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse[interface{}]("User logout successfully", nil))
}

func (userHandler *Handler) CreateUser(ginContext *gin.Context) {
	var createUserModel model.CreateUserRequest
	err := ginContext.ShouldBindBodyWithJSON(&createUserModel)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	paginatedResponse := userHandler.userService.Create(ginContext, &createUserModel)
	ginContext.JSON(http.StatusOK, paginatedResponse)
}

func (userHandler *Handler) UpdateUser(ginContext *gin.Context) {
	var updateUserModel model.UpdateUserRequest
	err := ginContext.ShouldBindBodyWithJSON(&updateUserModel)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	userId := ginContext.Param("id")
	parsedUserId, err := strconv.ParseUint(userId, 10, 64)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	updateUserModel.Id = parsedUserId
	paginatedResponse := userHandler.userService.Update(ginContext, &updateUserModel)
	ginContext.JSON(http.StatusOK, paginatedResponse)
}

func (userHandler *Handler) UpdateProfile(ginContext *gin.Context) {
	var updateUserProfileRequest model.UpdateUserProfileRequest
	err := ginContext.Request.ParseMultipartForm(20 << 20) // 32MB maxMemory
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	err = userHandler.formDecoder.Decode(&updateUserProfileRequest, ginContext.Request.PostForm)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	avatarFile, _ := ginContext.FormFile("avatar")
	updateUserProfileRequest.Avatar = avatarFile
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	newGeneratedToken := userHandler.userService.UpdateProfile(ginContext, &updateUserProfileRequest)
	ginContext.JSON(http.StatusOK, helper.NewSuccessResponse("Update profile success", gin.H{
		"token": newGeneratedToken,
	}))
}

func (userHandler *Handler) DeleteUser(ginContext *gin.Context) {
	var deleteUserModel model.DeleteResourceGeneralRequest
	err := ginContext.ShouldBindBodyWithJSON(&deleteUserModel)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	userId := ginContext.Param("id")
	parsedUserId, err := strconv.ParseUint(userId, 10, 64)
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	deleteUserModel.Id = parsedUserId
	paginatedResponse := userHandler.userService.Delete(ginContext, &deleteUserModel)
	ginContext.JSON(http.StatusOK, paginatedResponse)
}
