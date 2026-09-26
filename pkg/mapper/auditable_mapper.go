package mapper

import (
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/pkg/helper"
	"time"
)

func AuditableEntityIntoEntityResponse(auditableEntity *entity.Auditable) *model.AuditableResponse {
	var auditableResponse model.AuditableResponse

	// Format ke ISO 8601 (RFC3339)
	auditableResponse.CreatedBy = auditableEntity.CreatedBy
	auditableResponse.CreatedAt = auditableEntity.CreatedAt.Format(time.RFC3339)
	auditableResponse.UpdatedBy = auditableEntity.UpdatedBy
	auditableResponse.UpdatedAt = auditableEntity.UpdatedAt.Format(time.RFC3339)

	helper.CheckPointerWrapper(auditableEntity.DeletedBy, func() {
		auditableResponse.DeletedBy = *auditableEntity.DeletedBy
	})

	// Jika deletedAt valid, format juga
	if !auditableEntity.DeletedAt.Time.IsZero() {
		auditableResponse.DeletedAt = auditableEntity.DeletedAt.Time.Format(time.RFC3339)
	}

	return &auditableResponse
}

func SimpleAuditableEntityIntoSimpleEntityResponse(simpleAuditableEntity *entity.SimpleAuditable) *model.SimpleAuditableResponse {
	var simpleAuditableResponse model.SimpleAuditableResponse

	// Format ke ISO 8601 (RFC3339)
	simpleAuditableResponse.CreatedBy = simpleAuditableEntity.CreatedBy
	simpleAuditableResponse.CreatedAt = simpleAuditableEntity.CreatedAt.Format(time.RFC3339)

	return &simpleAuditableResponse
}
