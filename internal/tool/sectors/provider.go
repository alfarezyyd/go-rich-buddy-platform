package sectors

import (
	"go-rich-buddy-platform/internal/tool"

	"github.com/go-resty/resty/v2"
)

func ProvideSectorsTools(restySectors *resty.Client) []tool.Tool {
	return []tool.Tool{
		NewCompanyReportTool(restySectors),
		NewDailyPriceTool(restySectors),
		NewTopCompaniesTool(restySectors),
	}
}
