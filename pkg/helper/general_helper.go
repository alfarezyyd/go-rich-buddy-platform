package helper

import (
	"context"
	"encoding/json"
	"errors"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/trait"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/logger"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func CheckErrorOperation(indicatedError error, applicationError *exception.ApplicationError) bool {

	if errors.Is(indicatedError, context.Canceled) {
		return false
	}
	if indicatedError != nil {
		logger.Debug(indicatedError)
		panic(applicationError)
	}

	return false
}

func CheckPointerWrapper[T any](targetChecking *T, renderPayload func()) {
	if targetChecking != nil {
		renderPayload()
	}
}

func ExtractJwtClaimFromContext(ginContext *gin.Context) *model.JwtClaimRequest {
	jwtClaims, isExists := ginContext.Get("claims")
	if !isExists {
		exception.ThrowApplicationError(exception.NewApplicationError(http.StatusUnauthorized, exception.ErrUnauthorized))
	}
	userClaim, isValid := jwtClaims.(*model.JwtClaimRequest)
	if !isValid {
		exception.ThrowApplicationError(exception.NewApplicationError(http.StatusUnauthorized, exception.ErrUnauthorized))
	}

	return userClaim
}

func ExtractRequestMeta(ginContext *gin.Context) (string, string) {
	ipAddress, isIpAddressExists := ginContext.Get("ipAddress")
	userAgent, isUserAgentExists := ginContext.Get("userAgent")
	if isUserAgentExists && isIpAddressExists {
		return ipAddress.(string), userAgent.(string)
	}
	return "", ""
}

func ExtractRequestData(ginContext *gin.Context) (*model.JwtClaimRequest, string, string) {
	userJwtClaims := ExtractJwtClaimFromContext(ginContext)
	ipAddress, userAgent := ExtractRequestMeta(ginContext)
	return userJwtClaims, ipAddress, userAgent
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func TakePointer[T any](value T) *T {
	return &value
}

func DebugArrPointer[T any](arrayOfPointer []*T) {
	for i, pointerData := range arrayOfPointer {
		if pointerData == nil {
			logger.Debugf("[%d] nil pointer", i)
			continue
		}

		jsonBytes, err := json.MarshalIndent(pointerData, "", "  ")
		if err != nil {
			logger.Debugf("[%d] Marshal error: %v", i, err)
			continue
		}

		logger.Debugf("[%d]\n%s", i, string(jsonBytes))
	}
}

func ExtractIds[T trait.HasId](items []T) []uint64 {
	ids := make([]uint64, len(items))
	for i, item := range items {
		ids[i] = item.GetId()
	}
	return ids
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ConvertIntoSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func ExtractKeyOnMap[K comparable, V any](targetMap map[K]V) []K {
	keys := make([]K, 0, len(targetMap))
	for k := range targetMap {
		keys = append(keys, k)
	}
	return keys
}
