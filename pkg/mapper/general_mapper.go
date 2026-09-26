package mapper

import (
	"go-rich-buddy-platform/internal/entity"
	"go-rich-buddy-platform/internal/model"
)

func FuncMapAuditable[S entity.HasAuditable, R model.HasAuditableResponse](
	entityObject S,
	responseObject R,
) {
	responseObject.SetAuditableResponse(
		AuditableEntityIntoEntityResponse(entityObject.GetAuditable()),
	)
}

func FuncMapSimpleAuditable[S entity.HasSimpleAuditable, R model.HasSimpleAuditableResponse](
	entityObject S,
	responseObject R,
) {
	responseObject.SetSimpleAuditableResponse(
		SimpleAuditableEntityIntoSimpleEntityResponse(entityObject.GetSimpleAuditable()),
	)
}
