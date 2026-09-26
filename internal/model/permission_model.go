package model

type PermissionResponse struct {
	Id                uint64             `json:"id"`
	Code              string             `json:"code"`
	Name              string             `json:"name"`
	Category          string             `json:"category"`
	Description       string             `json:"description"`
	AuditableResponse *AuditableResponse `json:"auditable" mapstructure:"Auditable"`
}

func (permissionResponse *PermissionResponse) GetAuditableResponse() *AuditableResponse {
	return permissionResponse.AuditableResponse
}

func (permissionResponse *PermissionResponse) SetAuditableResponse(auditableResponse *AuditableResponse) {
	permissionResponse.AuditableResponse = auditableResponse
}
