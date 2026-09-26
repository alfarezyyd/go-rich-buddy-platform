package sectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type TopCompaniesTool struct {
	restyClient *resty.Client
}

type topCompaniesArgument struct {
	Sector string `json:"sector"`
	Limit  int    `json:"limit,omitempty"`
}

func NewTopCompaniesTool(restyClient *resty.Client) *TopCompaniesTool {
	return &TopCompaniesTool{
		restyClient: restyClient,
	}
}

func (tool *TopCompaniesTool) Name() string {
	return "get_top_companies"
}

func (tool *TopCompaniesTool) Description() string {
	return "Get top companies ranked by market cap within a specific sector or overall index (e.g. finance, energy, technology)"
}

func (tool *TopCompaniesTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sector": map[string]interface{}{
				"type":        "string",
				"description": "Sector name to filter by (e.g. finance, energy, basic-materials, technology, or all)",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of companies to return (default 5, max 20)",
			},
		},
		"required": []string{"sector"},
	}
}

func (tool *TopCompaniesTool) Execute(ctx context.Context, arguments string) (string, error) {
	var arg topCompaniesArgument
	if err := json.Unmarshal([]byte(arguments), &arg); err != nil {
		return "", fmt.Errorf("invalid arguments JSON: %w", err)
	}

	arg.Sector = strings.TrimSpace(strings.ToLower(arg.Sector))
	if arg.Sector == "" {
		arg.Sector = "finance"
	}
	if arg.Limit <= 0 || arg.Limit > 20 {
		arg.Limit = 5
	}

	if tool.restyClient == nil {
		return fmt.Sprintf(`{"sector": "%s", "status": "simulated", "top_companies": ["BBCA", "BBRI", "BMRI"]}`, arg.Sector), nil
	}

	endpoint := fmt.Sprintf("/companies/top/?sector=%s&limit=%d", arg.Sector, arg.Limit)
	httpResponse, err := tool.restyClient.R().
		SetContext(ctx).
		Get(endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to fetch top companies: %w", err)
	}

	if httpResponse.IsError() {
		return "", fmt.Errorf("sectors API returned status %d: %s", httpResponse.StatusCode(), httpResponse.String())
	}

	return httpResponse.String(), nil
}
