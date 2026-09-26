package config

import (
	"fmt"
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/trait"
	"go-rich-buddy-platform/pkg/helper"
	"mime/multipart"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	engTranslation "github.com/go-playground/validator/v10/translations/en"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// InitializeValidator setup validator instance dengan semua custom validation
func InitializeValidator(dbConnection *gorm.DB) (*validator.Validate, ut.Translator) {
	validate := validator.New()

	// Use form/json tag for names instead of struct field names
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("form")
		if name == "" {
			name = fld.Tag.Get("json")
		}
		if name == "-" {
			return ""
		}
		name = strings.Split(name, ",")[0]
		return name
	})

	// Universal Translator
	english := en.New()
	uni := ut.New(english, english)
	trans, _ := uni.GetTranslator("en")
	_ = engTranslation.RegisterDefaultTranslations(validate, trans)

	// ======================
	// Register custom validators
	// ======================
	validate.RegisterValidation("maxSize", fileSizeValidator)
	validate.RegisterValidation("fileExtension", fileExtensionValidator)
	validate.RegisterValidation("requiredFile", requiredFileValidator)
	validate.RegisterValidation("phoneNumber", phoneNumberValidator)
	validate.RegisterValidation("weakPassword", weakPasswordValidator)
	validate.RegisterValidation("decimalValue", decimalValueValidator)
	validate.RegisterValidation("unique", uniqueValidator(dbConnection))
	validate.RegisterValidation("exists", existsValidator(dbConnection))
	validate.RegisterValidation("date", dateValidator("2006-01-02"))
	validate.RegisterValidation("datetime", dateValidator("2006-01-02 15:04"))
	validate.RegisterValidation("time", timeValidator("15:04"))
	validate.RegisterValidation("matchPassword", matchPasswordValidator(dbConnection))

	// ======================
	// Register translations
	// ======================
	registerTranslations(validate, trans)

	return validate, trans
}

// ======================
// Translations
// ======================
func registerTranslations(validate *validator.Validate, trans ut.Translator) {
	type translation struct {
		tag     string
		message func(fe validator.FieldError) string
	}

	translations := []translation{
		{
			tag: "maxSize",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("File size must be at most %s MB", fe.Param())
			},
		},
		{
			tag: "fileExtension",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("File extension must be one of %s", fe.Param())
			},
		},
		{
			tag: "requiredFile",
			message: func(fe validator.FieldError) string {
				return "File is required"
			},
		},
		{
			tag: "phoneNumber",
			message: func(fe validator.FieldError) string {
				return "Phone number format is invalid"
			},
		},
		{
			tag: "weakPassword",
			message: func(fe validator.FieldError) string {
				return "Password is weak, must contain upper, lower, digit and special char"
			},
		},
		{
			tag: "decimalValue",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s must be decimal", formatFieldName(fe.Field()))
			},
		},
		{
			tag: "unique",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s must contain unique values", fe.Field())
			},
		},
		{
			tag: "date",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s must be a valid date (YYYY-MM-DD)", fe.Field())
			},
		},
		{
			tag: "datetime",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s must be a valid datetime (YYYY-MM-DD HH:mm)", formatFieldName(fe.Field()))
			},
		},
		{
			tag: "exists",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s must refer to an existing record", formatFieldName(fe.Field()))
			},
		},
		{
			tag: "matchPassword",
			message: func(fe validator.FieldError) string {
				return fmt.Sprintf("%s is incorrect", formatFieldName(fe.Field()))
			},
		},
	}

	for _, tr := range translations {
		tag := tr.tag
		msg := tr.message
		validate.RegisterTranslation(tag, trans,
			func(ut ut.Translator) error {
				return ut.Add(tag, "{0}", true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				return msg(fe)
			},
		)
	}
}

// ======================
// Validators
// ======================
func uniqueValidator(db *gorm.DB) validator.Func {
	return func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		params := strings.Split(fl.Param(), ";")
		if len(params) < 2 { // minimal tableName dan columnName
			return false
		}

		tableName := params[0]
		columnName := params[1]

		var pkField string
		var pkVal interface{}
		if len(params) >= 3 {
			pkField = params[2]
			parent := fl.Parent()
			if parent.IsValid() {
				field := parent.FieldByName(pkField)
				if field.IsValid() {
					pkVal = field.Interface()
				}
			}
		}

		var count int64
		query := db.Debug().Table(tableName).Where(fmt.Sprintf("%s = ?", columnName), val)

		// Jika pkField & pkVal valid, tambahkan kondisi exclude record yang sama
		if pkField != "" && pkVal != nil {
			query = query.Where(fmt.Sprintf("%s <> ?", helper.ConvertIntoSnakeCase(pkField)), pkVal)
		}
		query.Count(&count)
		return count == 0
	}
}

func existsValidator(db *gorm.DB) validator.Func {
	return func(fl validator.FieldLevel) bool {
		val := fl.Field().Uint()
		params := strings.Split(fl.Param(), ";")
		if len(params) < 1 {
			return false
		}
		tableName, columnName := params[0], params[1]
		var count int64
		db.Table(tableName).Where(fmt.Sprintf("%s = ?", columnName), val).Count(&count)
		return count != 0
	}
}

func dateValidator(format string) validator.Func {
	return func(fl validator.FieldLevel) bool {
		if fl.Field().String() == "" {
			return true
		}
		_, err := time.Parse(format, fl.Field().String())
		return err == nil
	}
}

func timeValidator(format string) validator.Func {
	return func(fl validator.FieldLevel) bool {
		value := fl.Field().String()

		// allow empty value (optional field)
		if value == "" {
			return true
		}

		// strict time validation (HH:mm)
		parsedTime, err := time.Parse(format, value)
		if err != nil {
			return false
		}

		// enforce exact format (no auto-correction by time.Parse)
		return parsedTime.Format(format) == value
	}
}

func phoneNumberValidator(fl validator.FieldLevel) bool {
	s, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	r := regexp.MustCompile(`^(\+62|62|0)8[1-9][0-9]{6,9}$`)
	return r.MatchString(s)
}

func requiredFileValidator(fl validator.FieldLevel) bool {
	_, ok := fl.Field().Interface().(multipart.FileHeader)
	return ok
}

func fileSizeValidator(fl validator.FieldLevel) bool {
	f, ok := fl.Field().Interface().(multipart.FileHeader)
	if !ok {
		return false
	}
	maxMB, _ := strconv.ParseInt(fl.Param(), 10, 64)
	return f.Size <= maxMB*1024*1024
}

func fileExtensionValidator(fl validator.FieldLevel) bool {
	f, ok := fl.Field().Interface().(multipart.FileHeader)
	if !ok {
		return false
	}
	allowed := strings.Split(fl.Param(), " ")
	fn := strings.ToLower(f.Filename)
	for _, ext := range allowed {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}

func decimalValueValidator(fl validator.FieldLevel) bool {
	switch v := fl.Field().Interface().(type) {
	case float32, float64, int, int32, int64:
		return true
	case string:
		_, err := strconv.ParseFloat(v, 64)
		return err == nil
	default:
		return false
	}
}

func weakPasswordValidator(fl validator.FieldLevel) bool {
	pwd := fl.Field().String()
	if len(pwd) < 8 {
		return false
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(pwd) {
		return false
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(pwd) {
		return false
	}
	if !regexp.MustCompile(`\d`).MatchString(pwd) {
		return false
	}
	if !regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(pwd) {
		return false
	}
	return true
}

func matchPasswordValidator(dbConnection *gorm.DB) validator.Func {
	return func(fl validator.FieldLevel) bool {
		currentPassword := fl.Field().String()

		// ambil struct req sebagai interface
		requestModel, isValid := fl.Top().Interface().(trait.HasId)
		if !isValid {
			return false // struct tidak mendukung HasId
		}

		// ambil user dari DB menggunakan ID
		var user entity.User
		if err := dbConnection.First(&user, requestModel.GetId()).Error; err != nil {
			return false
		}

		// compare currentPassword == actualPassword (hashed)
		if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)) != nil {
			return false
		}

		return true
	}
}

// ======================
// Utility
// ======================
func formatFieldName(field string) string {
	if field == "" {
		return ""
	}
	return strings.ToLower(field[:1]) + field[1:]
}
