package helper

import (
	"fmt"
	"go-rich-buddy-platform/pkg/exception"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

func StringIntoTypeHookFunc(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
	switch to {
	case reflect.TypeOf(uint64(0)):
		if str, ok := data.(string); ok {
			return strconv.ParseUint(str, 10, 64)
		}
	case reflect.TypeOf(uint32(0)):
		if str, ok := data.(string); ok {
			val, err := strconv.ParseUint(str, 10, 32)
			return uint32(val), err
		}
	case reflect.TypeOf(int(0)):
		if str, ok := data.(string); ok {
			return strconv.Atoi(str)
		}
	case reflect.TypeOf(float32(0)):
		if str, ok := data.(string); ok {
			val, err := strconv.ParseFloat(str, 32)
			return float32(val), err
		}
	case reflect.TypeOf(time.Time{}):
		if str, ok := data.(string); ok {
			// Trim untuk keamanan input
			str = strings.TrimSpace(str)

			if str == "" {
				return time.Time{}, nil
			}

			// Daftar layout yang diizinkan
			layouts := []string{
				"2006-01-02",       // YYYY-MM-DD
				"2006-01-02 15:04", // YYYY-MM-DD HH:mm
			}

			var parsed time.Time
			var err error

			for _, layout := range layouts {
				parsed, err = time.Parse(layout, str)
				if err == nil {
					return parsed, nil
				}
			}

			return nil, fmt.Errorf("invalid date format: %s", str)
		}
	case reflect.TypeOf(&time.Time{}):
		if str, ok := data.(string); ok {
			str = strings.TrimSpace(str)

			// 👉 kalau kosong → return nil
			if str == "" {
				return nil, nil
			}

			layouts := []string{
				"2006-01-02",
				"2006-01-02 15:04",
			}

			for _, layout := range layouts {
				parsed, err := time.Parse(layout, str)
				if err == nil {
					return &parsed, nil // 👉 return pointer
				}
			}

			return nil, fmt.Errorf("invalid date format: %s", str)
		}
	}

	return data, nil
}

func PointerDecodeHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		// 1. Handle nil data
		if data == nil {
			if t.Kind() == reflect.Ptr {
				return nil, nil
			}
			return data, nil
		}

		// 2. Dereference pointer source jika perlu
		dataValue := reflect.ValueOf(data)
		if !dataValue.IsValid() {
			return nil, nil
		}

		// Handle pointer-to-pointer atau pointer-to-struct
		if dataValue.Kind() == reflect.Ptr {
			if dataValue.IsNil() {
				if t.Kind() == reflect.Ptr {
					return nil, nil
				}
				return data, nil
			}

			// PENTING: Jika target juga pointer dan source adalah pointer ke struct
			// Biarkan mapstructure handle nested decoding
			if t.Kind() == reflect.Ptr && dataValue.Elem().Kind() == reflect.Struct {
				// Jangan return langsung, biarkan mapstructure decode field by field
				return data, nil
			}
		}

		// 3. Handle zero value struct -> nil pointer
		if t.Kind() == reflect.Ptr && f.Kind() == reflect.Struct {
			if dataValue.IsValid() && dataValue.IsZero() {
				return nil, nil
			}
		}

		// 4. Handle map to struct (untuk nested objects)
		if f.Kind() == reflect.Map && t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
			// Biarkan mapstructure handle conversion
			return data, nil
		}

		return data, nil
	}
}
func DecodeFromSource[S any, T any](sourceMapping S, targetMapping T) T {

	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			PointerDecodeHookFunc(),
			StringIntoTypeHookFunc,
		), Result: &targetMapping,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	err = decoder.Decode(sourceMapping)

	return targetMapping
}

func DecodeIntoSource[S any, T any](sourceMapping S, targetMapping T) T {
	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook:       StringIntoTypeHookFunc,
		Result:           &targetMapping,
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	CheckErrorOperation(err, exception.NewApplicationError(http.StatusBadRequest, exception.ErrBadRequest))
	err = decoder.Decode(sourceMapping)
	return targetMapping
}

func MapEntitiesIntoResponses[S any, R any](entityObjects []S) []R {
	var responseObjects []R
	for _, entityObject := range entityObjects {
		responseObjects = append(responseObjects, MapEntityIntoResponse[S, R](entityObject, nil))
	}
	return responseObjects
}

func MapEntitiesIntoResponsesWithFunc[S any, R any](
	entityObjects []S,
	renderPayloads ...func(S, R),
) []R {
	var responseObjects []R
	for _, entityObject := range entityObjects {
		responseObjects = append(
			responseObjects,
			MapEntityIntoResponse[S, R](entityObject, renderPayloads...),
		)
	}
	return responseObjects
}

func MapEntitiesIntoResponsesWithIgnoredFieldsWithFunc[S any, R any](
	entityObjects []S,
	ignoredFields []string,
	renderPayloads ...func(S, R),
) []R {
	var responseObjects []R
	for _, entityObject := range entityObjects {
		responseObjects = append(
			responseObjects,
			MapEntityIntoResponseWithIgnoredFields[S, R](entityObject, ignoredFields, renderPayloads...),
		)
	}
	return responseObjects
}

func MapEntityIntoResponseWithIgnoredFields[S any, R any](
	entityObject S,
	ignoredFields []string,
	renderPayloads ...func(S, R),
) R {
	var responseObject R
	if reflect.TypeOf(responseObject).Kind() == reflect.Ptr {
		responseObject = reflect.New(reflect.TypeOf(responseObject).Elem()).Interface().(R)
	}
	decoded := DecodeFromSource[S, R](entityObject, responseObject)
	responseObject = decoded

	// Remove fields if necessary
	if len(ignoredFields) > 0 {
		removeIgnoredFields(responseObject, ignoredFields)
	}
	if len(renderPayloads) > 0 {
		for _, renderPayload := range renderPayloads {
			if renderPayload == nil { // <-- cegah panic
				continue
			}
			renderPayload(entityObject, responseObject)
		}
	}
	return responseObject
}

func MapEntityIntoResponse[S any, R any](
	entityObject S,
	renderPayloads ...func(S, R),
) R {
	var responseObject R
	if reflect.TypeOf(responseObject).Kind() == reflect.Ptr {
		responseObject = reflect.New(reflect.TypeOf(responseObject).Elem()).Interface().(R)
	}
	decoded := DecodeFromSource[S, R](entityObject, responseObject)
	responseObject = decoded

	if len(renderPayloads) > 0 {
		for _, renderPayload := range renderPayloads {
			if renderPayload == nil {
				continue
			}
			renderPayload(entityObject, responseObject)
		}
	}
	return responseObject
}

func MapCreateRequestIntoEntity[S any, R any](createRequest *S) *R {
	var entityObject R
	return DecodeIntoSource[*S, *R](createRequest, &entityObject)
}

func MapUpdateRequestIntoEntity[S any, R any](updateRequest S, existingEntity *R) {
	existingEntity = DecodeIntoSource[S, *R](updateRequest, existingEntity)
}

func ParsingHashMapIntoStruct[R any](sourceHashMap map[string]interface{}, rawStruct R) *R {
	parsedRawStruct := DecodeFromSource[map[string]interface{}, *R](sourceHashMap, &rawStruct)
	return parsedRawStruct
}

func removeIgnoredFields[R any](obj R, ignored []string) {
	reflectVal := reflect.ValueOf(obj)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	for _, field := range ignored {
		fieldName := reflectVal.FieldByName(field)
		if fieldName.IsValid() && fieldName.CanSet() {
			zero := reflect.Zero(fieldName.Type())
			fieldName.Set(zero)
		}
	}
}
