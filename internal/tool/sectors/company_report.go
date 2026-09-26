package sectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type CompanyReportTool struct {
	restyClient *resty.Client
}

type companyReportArgument struct {
	Ticker string `json:"ticker"`
}

func NewCompanyReportTool(restyClient *resty.Client) *CompanyReportTool {
	return &CompanyReportTool{
		restyClient: restyClient,
	}
}

func (tool *CompanyReportTool) Name() string {
	return "get_company_report"
}

func (tool *CompanyReportTool) Description() string {
	return "Get comprehensive company report and overview for a given stock ticker symbol (e.g. BBCA, BBRI)"
}

func (tool *CompanyReportTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticker": map[string]interface{}{
				"type":        "string",
				"description": "The stock ticker symbol (e.g. BBCA, BBRI, TLKM)",
			},
		},
		"required": []string{"ticker"},
	}
}

func (tool *CompanyReportTool) Execute(ctx context.Context, arguments string) (string, error) {
	var arg companyReportArgument
	if err := json.Unmarshal([]byte(arguments), &arg); err != nil {
		return "", fmt.Errorf("invalid arguments JSON: %w", err)
	}

	arg.Ticker = strings.TrimSpace(strings.ToUpper(arg.Ticker))
	if arg.Ticker == "" {
		return "", fmt.Errorf("ticker argument is required")
	}

	if tool.restyClient == nil {
		return fmt.Sprintf(`{"ticker": "%s", "status": "simulated", "overview": "Company report for %s"}`, arg.Ticker, arg.Ticker), nil
	}

	endpoint := fmt.Sprintf("/company/report/%s/", arg.Ticker)
	httpResponse, err := tool.restyClient.R().
		SetContext(ctx).
		Get(endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to fetch company report: %w", err)
	}

	if httpResponse.IsError() {
		return "", fmt.Errorf("sectors API returned status %d: %s", httpResponse.StatusCode(), httpResponse.String())
	}

	return httpResponse.String(), nil
}
