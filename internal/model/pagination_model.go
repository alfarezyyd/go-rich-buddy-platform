package model

type PaginatedResponse[T any] = ResponseContract[T]

type PaginationRequest struct {
	Page        int    `form:"page,default=1"`
	Size        int    `form:"size,default=10"`
	Sort        string `form:"sort,default=id"`
	Order       string `form:"order,default=asc"`
	SearchQuery string `form:"query"`
}

type PaginationQuery struct {
	Offset      int
	Limit       int
	OrderClause string
	SearchQuery string
}

type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	PageSize    int   `json:"page_size"`
	TotalPages  int   `json:"total_pages"`
	TotalItems  int64 `json:"total_items"`
}

func NewPaginationRequest() PaginationRequest {
	return PaginationRequest{
		Page:  1,
		Size:  10,
		Sort:  "id",
		Order: "asc",
	}
}
