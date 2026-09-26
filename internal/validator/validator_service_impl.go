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

// ValidateStruct - Validasi struct dengan opsi return error atau panic
func (validatorService *ServiceImpl) ValidateStruct(target interface{}) error {
	return validatorService.validatorInstance.Struct(target)
}

// ValidateVar - Validasi single variable dengan opsi return error atau panic
func (validatorService *ServiceImpl) ValidateVar(target interface{}, validatorTags string) error {
	return validatorService.validatorInstance.Var(target, validatorTags)
}

// ParseValidationError - Parsing error validasi ke dalam format yang lebih mudah dibaca
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
				fieldJSON = parts[1] // Removes the top-level struct name (e.g. CreateMachineRequest)
			} else {
				fieldJSON = ns
			}

			translatedMessage := fieldError.Translate(validatorService.engTranslator)

			// Clean message by replacing the field name with the more friendly property name
			propertyFieldName := getPropertyFieldName(fieldError.StructField(), fieldError.Namespace())
			cleanMessage := strings.Replace(translatedMessage, fieldError.Field(), propertyFieldName, 1)
			cleanMessage = strings.TrimSpace(cleanMessage)

			parsedMap[fieldJSON] = cleanMessage
		}
		panic(exception.NewApplicationErrorSpecific(http.StatusBadRequest, exception.StatusValidationError, exception.MsgValidationError, parsedMap))
	}
}

// getPropertyFieldName memisahkan nama struct field berdasarkan huruf besar.
func getPropertyFieldName(structField string, namespace string) string {
	// Jika kita ingin mencari dari tag `property`, kita perlu manual reflection ke type path.
	// Karena nested struct type tidak diberikan ke fungsi ini secara detail, kita gunakan splitCamelCase sebagai fallback yang baik.
	return splitCamelCase(structField)
}

// splitCamelCase memisahkan nama field yang dalam format CamelCase menjadi kata-kata terpisah.
func splitCamelCase(s string) string {
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	return re.ReplaceAllString(s, "$1 $2")
}
