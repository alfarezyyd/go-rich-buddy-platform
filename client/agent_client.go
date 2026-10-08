package client

import (
	"context"
	"fmt"
	"go-rich-buddy-platform/internal/model"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type AgentClient interface {
	CreateChatCompletion(ctx context.Context, chatCompletionRequest *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error)
}

type AgentClientImpl struct {
	restyClient *resty.Client
}

func NewAgentClient(baseURL string, apiKey string) AgentClient {
	client := resty.New().
		SetBaseURL(strings.TrimRight(baseURL, "/")).
		SetTimeout(120*time.Second).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetAuthToken(apiKey)

	return &AgentClientImpl{restyClient: client}
}

func (agentClient *AgentClientImpl) CreateChatCompletion(ctx context.Context, chatCompletionRequest *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {

	chatCompletionRequest.Stream = false
	var completionResponse model.ChatCompletionResponse
	var errorResponse struct {
		Error model.ChatError `json:"error"`
	}

	resp, err := agentClient.restyClient.R().
		SetContext(ctx).
		SetBody(chatCompletionRequest).
		SetResult(&completionResponse).
		SetError(&errorResponse).
		Post("/chat/completions")

	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}

	if resp.IsError() {
		if errorResponse.Error.Message != "" {
			return nil, fmt.Errorf("llm api error: %s (type: %s, code: %s)",
				errorResponse.Error.Message, errorResponse.Error.Type, errorResponse.Error.Code)
		}
		return nil, fmt.Errorf("llm api returned status %d: %s", resp.StatusCode(), resp.String())
	}

	if len(completionResponse.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices, body: %s", resp.String())
	}

	return &completionResponse, nil
}
