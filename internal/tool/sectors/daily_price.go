package sectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type DailyPriceTool struct {
	restyClient *resty.Client
}

type dailyPriceArgument struct {
	Ticker string `json:"ticker"`
}

func NewDailyPriceTool(restyClient *resty.Client) *DailyPriceTool {
	return &DailyPriceTool{
		restyClient: restyClient,
	}
}

func (tool *DailyPriceTool) Name() string {
	return "get_daily_price"
}

func (tool *DailyPriceTool) Description() string {
	return "Get latest daily trading and price information for a given stock ticker symbol (e.g. BBCA, BBRI)"
}

func (tool *DailyPriceTool) Parameters() map[string]interface{} {
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

func (tool *DailyPriceTool) Execute(ctx context.Context, arguments string) (string, error) {
	var arg dailyPriceArgument
	if err := json.Unmarshal([]byte(arguments), &arg); err != nil {
		return "", fmt.Errorf("invalid arguments JSON: %w", err)
	}

	arg.Ticker = strings.TrimSpace(strings.ToUpper(arg.Ticker))
	if arg.Ticker == "" {
		return "", fmt.Errorf("ticker argument is required")
	}

	if tool.restyClient == nil {
		return fmt.Sprintf(`{"ticker": "%s", "status": "simulated", "price": 9500, "change": "+1.2%%"}`, arg.Ticker), nil
	}

	endpoint := fmt.Sprintf("/daily/%s/", arg.Ticker)
	httpResponse, err := tool.restyClient.R().
		SetContext(ctx).
		Get(endpoint)

	if err != nil {
		return "", fmt.Errorf("failed to fetch daily price: %w", err)
	}

	if httpResponse.IsError() {
		return "", fmt.Errorf("sectors API returned status %d: %s", httpResponse.StatusCode(), httpResponse.String())
	}

	return httpResponse.String(), nil
}
