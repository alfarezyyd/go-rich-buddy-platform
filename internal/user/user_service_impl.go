package user

import (
	"fmt"
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/validator"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/helper"
	"go-rich-buddy-platform/pkg/mapper"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ServiceImpl struct {
	userRepository   Repository
	validatorService validator.Service
	dbConnection     *gorm.DB
	viperConfig      *viper.Viper
	redisInstance    *config.RedisInstance
}

func NewService(userRepository Repository, validatorService validator.Service, dbConnection *gorm.DB,
	viperConfig *viper.Viper,
	redisInstance *config.RedisInstance,
) *ServiceImpl {
	return &ServiceImpl{
		userRepository:   userRepository,
		validatorService: validatorService,
		dbConnection:     dbConnection,
		viperConfig:      viperConfig,
		redisInstance:    redisInstance,
	}
}

func (userService *ServiceImpl) FindAll() []*model.UserResponse {
	var allUser []*model.UserResponse
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userResponse, err := userService.userRepository.FindAll(gormTransaction)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		allUser = helper.MapEntitiesIntoResponsesWithFunc[*entity.User, *model.UserResponse](userResponse, mapper.FuncMapAuditable)
		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return allUser
}

func (userService *ServiceImpl) FindById(ginContext *gin.Context, userId uint64) *model.UserResponse {
	var userResponse *model.UserResponse
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userEntity, err := userService.userRepository.FindById(gormTransaction, userId)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		userResponse = helper.MapEntityIntoResponse[*entity.User, *model.UserResponse](userEntity,
			mapper.FuncMapAuditable)
		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return userResponse
}

func (userService *ServiceImpl) FindSelf(ginContext *gin.Context) *model.UserResponse {
	userJwtClaims := helper.ExtractJwtClaimFromContext(ginContext)
	var userResponse *model.UserResponse
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userEntity, err := userService.userRepository.FindById(gormTransaction, userJwtClaims.Id)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		userResponse = helper.MapEntityIntoResponse[*entity.User, *model.UserResponse](userEntity,
			mapper.FuncMapAuditable)
		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return userResponse
}

// Create - Membuat user baru
func (userService *ServiceImpl) Create(ginContext *gin.Context, createUserRequest *model.CreateUserRequest) *model.PaginatedResponse[*model.UserResponse] {
	userJwtClaims := helper.ExtractJwtClaimFromContext(ginContext)
	var paginationResp *model.PaginatedResponse[*model.UserResponse]
	valErr := userService.validatorService.ValidateStruct(createUserRequest)
	userService.validatorService.ParseValidationError(valErr, *createUserRequest)
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userEntity := helper.MapCreateRequestIntoEntity[model.CreateUserRequest, entity.User](createUserRequest)
		userEntity.Auditable = entity.NewAuditable(userJwtClaims.Name)
		err := userService.userRepository.Create(gormTransaction, userEntity)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))

		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return paginationResp
}

func (userService *ServiceImpl) HandleLogin(ginContext *gin.Context, loginUserRequest *model.LoginUserRequest) string {
	err := userService.validatorService.ValidateStruct(loginUserRequest)
	userService.validatorService.ParseValidationError(err, *loginUserRequest)
	var tokenString string
	err = userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userEntity, err := userService.userRepository.FindByIdentifier(gormTransaction, loginUserRequest.UserIdentifier)

		if err = bcrypt.CompareHashAndPassword([]byte(userEntity.Password), []byte(loginUserRequest.Password)); err != nil {
			exception.ThrowApplicationError(exception.NewApplicationError(http.StatusBadRequest, "User credentials invalid"))
		}
		jwtHour := userService.viperConfig.GetInt64("JWT_HOUR")
		if jwtHour == 0 {
			jwtHour = 72
		}
		tokenInstance := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"id":    userEntity.Id,
			"email": userEntity.Email,
			"name":  userEntity.Name,
			"exp":   time.Now().Add(time.Hour * time.Duration(jwtHour)).Unix(),
		})
		tokenString, err = tokenInstance.SignedString([]byte(userService.viperConfig.GetString("JWT_SECRET")))
		helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))
		if claims, ok := tokenInstance.Claims.(jwt.MapClaims); ok {
			userJwtClaim := helper.MapCreateRequestIntoEntity[jwt.MapClaims, model.JwtClaimRequest](&claims)
			ginContext.Set("claims", userJwtClaim)
		}
		redisKey := fmt.Sprintf("auth:token:%d", userEntity.Id)
		err = userService.redisInstance.RedisClient.Set(
			ginContext.Request.Context(),
			redisKey,
			tokenString,
			time.Hour*time.Duration(jwtHour),
		).Err()
		helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))

		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		return nil
	})
	return tokenString
}
func (userService *ServiceImpl) HandleLogout(ginContext *gin.Context) {
	userJwtClaims := helper.ExtractJwtClaimFromContext(ginContext)
	redisKey := fmt.Sprintf("auth:token:%d", userJwtClaims.Id)
	_, err := userService.redisInstance.RedisClient.Del(ginContext.Request.Context(), redisKey).Result()
	helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))
}
func (userService *ServiceImpl) Update(ginContext *gin.Context, updateUserRequest *model.UpdateUserRequest) *model.PaginatedResponse[*model.UserResponse] {
	var paginationResp *model.PaginatedResponse[*model.UserResponse]
	valErr := userService.validatorService.ValidateStruct(updateUserRequest)
	userService.validatorService.ParseValidationError(valErr, *updateUserRequest)
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		userEntity, err := userService.userRepository.FindById(gormTransaction, updateUserRequest.Id)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		helper.MapUpdateRequestIntoEntity(updateUserRequest, userEntity)
		if updateUserRequest.Password != "" {
			userEntity.Password, err = helper.HashPassword(updateUserRequest.Password)
			helper.CheckErrorOperation(err, exception.NewApplicationError(http.StatusInternalServerError, exception.ErrInternalServerError))
		}
		err = userService.userRepository.Update(gormTransaction, userEntity)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))

		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return paginationResp
}

func (userService *ServiceImpl) UpdateProfile(ginContext *gin.Context, updateUserProfileRequest *model.UpdateUserProfileRequest) string {
	userJwtClaims := helper.ExtractJwtClaimFromContext(ginContext)
	updateUserProfileRequest.Id = userJwtClaims.Id
	valErr := userService.validatorService.ValidateStruct(updateUserProfileRequest)
	userService.validatorService.ParseValidationError(valErr, *updateUserProfileRequest)
	var tokenString string
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		var err error
		userEntity, err := userService.userRepository.FindById(gormTransaction, updateUserProfileRequest.Id)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		helper.MapUpdateRequestIntoEntity(updateUserProfileRequest, userEntity)
		if updateUserProfileRequest.Password != nil {
			userEntity.Password, _ = helper.HashPassword(*updateUserProfileRequest.Password)
		}

		err = userService.userRepository.Update(gormTransaction, userEntity)
		tokenString = userService.HandleLogin(ginContext, &model.LoginUserRequest{
			UserIdentifier: userEntity.Email,
			Password:       updateUserProfileRequest.CurrentPassword,
		})
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return tokenString
}

func (userService *ServiceImpl) Delete(ginContext *gin.Context, deleteUserRequest *model.DeleteResourceGeneralRequest) *model.PaginatedResponse[*model.UserResponse] {
	var paginationResp *model.PaginatedResponse[*model.UserResponse]
	valErr := userService.validatorService.ValidateStruct(deleteUserRequest)
	userService.validatorService.ParseValidationError(valErr, *deleteUserRequest)
	err := userService.dbConnection.Transaction(func(gormTransaction *gorm.DB) error {
		_, err := userService.userRepository.FindById(gormTransaction, deleteUserRequest.Id)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))
		err = userService.userRepository.Delete(gormTransaction, deleteUserRequest.Id)
		helper.CheckErrorOperation(err, exception.ParseGormError(err))

		return nil
	})
	helper.CheckErrorOperation(err, exception.ParseGormError(err))
	return paginationResp
}
