package validator

import (
	"go-rich-buddy-platform/pkg/exception"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	universalTranslator "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type ServiceImpl struct {
	validatorInstance *validator.Validate
	engTranslator     universalTranslator.Translator
}

func NewService(
	validatorInstance *validator.Validate,
	engTranslator universalTranslator.Translator) *ServiceImpl {
	return &ServiceImpl{
		validatorInstance: validatorInstance,
		engTranslator:     engTranslator}
}

func (validatorService *ServiceImpl) ValidateStruct(target interface{}) error {
	return validatorService.validatorInstance.Struct(target)
}

func (validatorService *ServiceImpl) ValidateVar(target interface{}, validatorTags string) error {
	return validatorService.validatorInstance.Var(target, validatorTags)
}

func (validatorService *ServiceImpl) ParseValidationError(validationError error, dtoStruct interface{}) {
	if validationError != nil {
		parsedMap := make(map[string]interface{})
		reflectType := reflect.TypeOf(dtoStruct)
		if reflectType.Kind() == reflect.Ptr {
			reflectType = reflectType.Elem()
		}
		for _, fieldError := range validationError.(validator.ValidationErrors) {
			ns := fieldError.Namespace()
			parts := strings.SplitN(ns, ".", 2)
			var fieldJSON string
			if len(parts) == 2 {
				fieldJSON = parts[1]
			} else {
				fieldJSON = ns
			}

			translatedMessage := fieldError.Translate(validatorService.engTranslator)

			propertyFieldName := getPropertyFieldName(fieldError.StructField(), fieldError.Namespace())
			cleanMessage := strings.Replace(translatedMessage, fieldError.Field(), propertyFieldName, 1)
			cleanMessage = strings.TrimSpace(cleanMessage)

			parsedMap[fieldJSON] = cleanMessage
		}
		panic(exception.NewApplicationErrorSpecific(http.StatusBadRequest, exception.StatusValidationError, exception.MsgValidationError, parsedMap))
	}
}

func getPropertyFieldName(structField string, namespace string) string {
	return splitCamelCase(structField)
}

func splitCamelCase(s string) string {
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	return re.ReplaceAllString(s, "$1 $2")
}
