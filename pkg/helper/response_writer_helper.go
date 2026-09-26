package helper

import "go-rich-buddy-platform/internal/model"

func WriteSuccess(message string, entry interface{}) model.ResponseContractModel {
	return model.ResponseContractModel{
		Status:  true,
		Message: message,
		Data:    &entry,
		Error:   nil,
	}
}

func NewSuccessResponse[T any](message string, entry T) *model.ResponseContract[T] {
	return &model.ResponseContract[T]{
		Success: true,
		Message: message,
		Entry:   &entry,
		Error:   nil,
	}
}

func NewSuccessResponseWithEntries[T any](message string, entries []T) *model.ResponseContract[T] {
	return &model.ResponseContract[T]{
		Success: true,
		Message: message,
		Entry:   nil,
		Entries: entries,
		Error:   nil,
	}
}

func NewErrorResponse(message string, errorDetail *model.ErrorDetail) *model.ResponseContract[any] {
	return &model.ResponseContract[any]{
		Success: false,
		Message: message,
		Entry:   nil,
		Entries: nil,
		Error:   errorDetail,
	}
}

func NewPaginatedResponse[T any](
	message string,
	entries []T,
	currentPage, pageSize int,
	totalItems int64,
) *model.ResponseContract[T] {
	totalPages := int(totalItems) / pageSize
	if int(totalItems)%pageSize != 0 {
		totalPages++
	}

	return &model.ResponseContract[T]{
		Success: true,
		Message: message,
		Entries: entries,
		Entry:   nil,
		Pagination: &model.PaginationMeta{
			CurrentPage: currentPage,
			PageSize:    pageSize,
			TotalPages:  totalPages,
			TotalItems:  totalItems,
		},
	}
}

func NewPaginatedResponseFromResult[T any](
	message string,
	entries []T,
	paginationReq *model.PaginationRequest,
	totalItems int64,
) *model.ResponseContract[T] {
	return NewPaginatedResponse(
		message,
		entries,
		paginationReq.Page,
		paginationReq.Size,
		totalItems,
	)
}

func NewErrorDetail(conventionStatusCode string, errorMessage string, detailsError map[string]interface{}) *model.ErrorDetail {
	return &model.ErrorDetail{
		Code:    conventionStatusCode,
		Message: errorMessage,
		Details: detailsError,
	}
}
