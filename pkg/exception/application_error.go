package exception

import "fmt"

type ApplicationError struct {
	HttpStatusCode       int                    `json:"status_code"`
	ConventionStatusCode string                 `json:"convention_status_code"`
	Message              string                 `json:"message"`
	Details              map[string]interface{} `json:"details"`
}

func (applicationError *ApplicationError) Error() string {
	return fmt.Sprintf("Error %d-%s: %s", applicationError.HttpStatusCode, applicationError.ConventionStatusCode, applicationError.Message)
}

func NewApplicationError(statusCode int, message string) *ApplicationError {
	return &ApplicationError{
		HttpStatusCode: statusCode,
		Message:        message,
	}
}

func NewApplicationErrorWithDetails(statusCode int, message string, details map[string]interface{}) *ApplicationError {
	return &ApplicationError{
		HttpStatusCode: statusCode,
		Message:        message,
		Details:        details,
	}
}

func NewApplicationErrorSpecific(statusCode int, conventionStatusCode string, message string, details map[string]interface{}) *ApplicationError {
	return &ApplicationError{
		HttpStatusCode:       statusCode,
		ConventionStatusCode: conventionStatusCode,
		Message:              message,
		Details:              details,
	}
}

func ThrowApplicationError(applicationError *ApplicationError) {
	panic(applicationError)
}
