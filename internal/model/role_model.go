package model

type CreateRoleRequest struct {
	Name          string   `json:"name" validate:"required,min=3,max=50"`
	Description   string   `json:"description" validate:""`
	PermissionIds []uint64 `json:"permission_ids" validate:"required,dive,number,gte=1"`
}

type UpdateRoleRequest struct {
	Id            uint64   `json:"-" validate:"required,number,gte=1,exists=roles;id"`
	Name          string   `json:"name" validate:"required,min=3,max=50"`
	Description   string   `json:"description" validate:""`
	PermissionIds []uint64 `json:"permission_ids" validate:"required,dive,number,gte=1"`
}

type RoleResponse struct {
	Id                uint64               `json:"id" `
	Name              string               `json:"name" `
	Description       string               `json:"description"`
	Permissions       []PermissionResponse `json:"permissions"`
	AuditableResponse *AuditableResponse   `json:"auditable_response"`
}

func (roleResponse *RoleResponse) GetAuditableResponse() *AuditableResponse {
	return roleResponse.AuditableResponse
}

func (roleResponse *RoleResponse) SetAuditableResponse(auditableResponse *AuditableResponse) {
	roleResponse.AuditableResponse = auditableResponse
}
