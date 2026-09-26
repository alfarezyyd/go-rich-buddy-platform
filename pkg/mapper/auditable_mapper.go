package mapper

import (
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/helper"
	"time"
)

func AuditableEntityIntoEntityResponse(auditableEntity *entity.Auditable) *model.AuditableResponse {
	var auditableResponse model.AuditableResponse

	auditableResponse.CreatedBy = auditableEntity.CreatedBy
	auditableResponse.CreatedAt = auditableEntity.CreatedAt.Format(time.RFC3339)
	auditableResponse.UpdatedBy = auditableEntity.UpdatedBy
	auditableResponse.UpdatedAt = auditableEntity.UpdatedAt.Format(time.RFC3339)

	helper.CheckPointerWrapper(auditableEntity.DeletedBy, func() {
		auditableResponse.DeletedBy = *auditableEntity.DeletedBy
	})

	if !auditableEntity.DeletedAt.Time.IsZero() {
		auditableResponse.DeletedAt = auditableEntity.DeletedAt.Time.Format(time.RFC3339)
	}

	return &auditableResponse
}

func SimpleAuditableEntityIntoSimpleEntityResponse(simpleAuditableEntity *entity.SimpleAuditable) *model.SimpleAuditableResponse {
	var simpleAuditableResponse model.SimpleAuditableResponse

	simpleAuditableResponse.CreatedBy = simpleAuditableEntity.CreatedBy
	simpleAuditableResponse.CreatedAt = simpleAuditableEntity.CreatedAt.Format(time.RFC3339)

	return &simpleAuditableResponse
}
