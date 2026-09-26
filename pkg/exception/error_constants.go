package exception

const (
	ErrInvalidRequestBody  = "Invalid request body"
	ErrUnauthorized        = "Unauthorized access"
	ErrForbidden           = "Forbidden"
	ErrMethodNotAllowed    = "Method not allowed"
	ErrNotAcceptable       = "Not Acceptable"
	ErrProxyAuthRequired   = "Proxy Authentication Required"
	ErrRequestTimeout      = "Request Timeout"
	ErrTooManyRequests     = "Too Many Requests"
	ErrInternalServerError = "Internal Server Error"
	ErrBadRequest          = "Bad Request"
	ErrNotFound            = "Not Found"
)

const (
	ErrSavingResources      = "Error saving resources"
	ErrSomeResourceNotFound = "Some resources not found"
	ErrCreatingResource     = "Error creating resource"
	ErrUpdatingResource     = "Error updating resource"
	ErrDeletingResource     = "Error deleting resource"
	ErrResourceAlreadyExist = "Resource already exists"
	ErrResourceNotFound     = "Resource not found"

	ErrDatabaseConnection  = "Database connection error"
	ErrDatabaseQuery       = "Database query error"
	ErrDatabaseTransaction = "Database transaction error"
	ErrDuplicateEntry      = "Duplicate entry"

	ErrInvalidCredentials     = "Invalid credentials"
	ErrTokenExpiredOrInvalid  = "Token expired or invalid"
	ErrInsufficientPermission = "Insufficient permission"
	ErrAccountLocked          = "Account is locked"
	ErrAccountNotActivated    = "Account is not activated"

	ErrInsufficientBalance = "Insufficient balance"
	ErrInvalidOperation    = "Invalid operation"
	ErrOperationNotAllowed = "Operation not allowed"
	ErrQuotaExceeded       = "Quota exceeded"
	ErrServiceUnavailable  = "Service temporarily unavailable"

	ErrFileUpload       = "Error uploading file"
	ErrFileNotFound     = "File not found"
	ErrInvalidFileType  = "Invalid file type"
	ErrFileSizeTooLarge = "File size too large"

	ErrExternalServiceError = "External service error"
	ErrThirdPartyAPIError   = "Third party API error"
	ErrPaymentFailed        = "Payment processing failed"

	ErrPayloadInvalid           = "Payload invalid or deformed"
	ErrParameterInvalid         = "Parameter invalid or not supplied"
	ErrSystemSettingKeyNotMatch = "The system setting key is invalid"
)

const (
	StatusSuccess            = "1"
	StatusValidationError    = "2"
	StatusAuthError          = "3"
	StatusNotFoundError      = "4"
	StatusDuplicateError     = "5"
	StatusDatabaseError      = "6"
	StatusBusinessLogicError = "7"
	StatusExternalError      = "8"
	StatusInternalError      = "9"
	StatusRateLimitError     = "10"
	StatusTimeoutError       = "11"
	StatusMaintenanceMode    = "12"
)

const (
	MsgSuccess            = "Operation completed successfully"
	MsgValidationError    = "Validation error occurred"
	MsgAuthError          = "Authentication or authorization error"
	MsgNotFoundError      = "Resource not found"
	MsgDuplicateError     = "Duplicate resource detected"
	MsgDatabaseError      = "Database operation failed"
	MsgBusinessLogicError = "Business logic error"
	MsgExternalError      = "External service error"
	MsgInternalError      = "Internal server error"
	MsgRateLimitError     = "Rate limit exceeded"
	MsgTimeoutError       = "Request timeout"
	MsgMaintenanceMode    = "Service under maintenance"
)
