package model

// ResponseContractModel Old response contract
type ResponseContractModel struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

func NewResponseContractModel(statusResp bool, messageResp string, dataResp *interface{}, Error *interface{}) *ResponseContractModel {
	return &ResponseContractModel{
		Status:  statusResp,
		Message: messageResp,
		Data:    dataResp,
		Error:   Error,
	}
}

type ResponseContract[T any] struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Entry      *T              `json:"entry"`
	Entries    []T             `json:"entries"`
	Error      *ErrorDetail    `json:"error"`
	Pagination *PaginationMeta `json:"pagination"`
}

// ErrorDetail berisi detail error
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details"`
}

const (
	RESPONSE_SUCCESS = "Success"
)
