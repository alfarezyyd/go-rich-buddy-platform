package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-rich-buddy-platform/internal/model"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type AgentClient interface {
	CreateChatCompletion(ctx context.Context, chatCompletionRequest *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error)
}

type AgentClientImpl struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func NewAgentClient(baseURL string, apiKey string) AgentClient {
	return &AgentClientImpl{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

func (agentClient *AgentClientImpl) CreateChatCompletion(ctx context.Context, chatCompletionRequest *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	endpoint := fmt.Sprintf("%s/chat/completions", agentClient.baseURL)

	requestBodyBytes, err := json.Marshal(chatCompletionRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")

	authHeader := fmt.Sprintf("Bearer %s", agentClient.apiKey)
	httpRequest.Header.Set("Authorization", authHeader)

	httpResponse, err := agentClient.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode >= 400 {
		errorBody, _ := io.ReadAll(httpResponse.Body)
		var errorResponse struct {
			Error model.ChatError `json:"error"`
		}
		if jsonErr := json.Unmarshal(errorBody, &errorResponse); jsonErr == nil && errorResponse.Error.Message != "" {
			return nil, fmt.Errorf("llm api error: %s (type: %s, code: %s)", errorResponse.Error.Message, errorResponse.Error.Type, errorResponse.Error.Code)
		}
		return nil, fmt.Errorf("llm api returned status %d: %s", httpResponse.StatusCode, string(errorBody))
	}

	var (
		contentBuilder   strings.Builder
		toolCallsMap     = make(map[int]*model.ChatToolCall)
		toolCallIndices  []int
		responseID       string
		responseModel    string
		finishReason     string
		receivedAnyChunk bool
		rawBuffer        bytes.Buffer
	)

	teeReader := io.TeeReader(httpResponse.Body, &rawBuffer)
	reader := bufio.NewReader(teeReader)

	for {
		line, readErr := reader.ReadString('\n')
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "data:"))
			if data == "" {
				if readErr != nil {
					break
				}
				continue
			}
			if data == "[DONE]" {
				break
			}

			var chunk model.ChatCompletionResponse
			if unmarshalErr := json.Unmarshal([]byte(data), &chunk); unmarshalErr != nil {
				logrus.WithError(unmarshalErr).Debugf("Failed to unmarshal SSE chunk: %s", data)
				if readErr != nil {
					break
				}
				continue
			}

			receivedAnyChunk = true
			if chunk.ID != "" {
				responseID = chunk.ID
			}
			if chunk.Model != "" {
				responseModel = chunk.Model
			}

			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}

				if choice.Delta.Content != "" {
					contentBuilder.WriteString(choice.Delta.Content)
				}

				for _, toolCall := range choice.Delta.ToolCalls {
					existing, exists := toolCallsMap[toolCall.Index]
					if !exists {
						existing = &model.ChatToolCall{
							ID:   toolCall.ID,
							Type: toolCall.Type,
							Function: model.ChatFunctionCall{
								Name:      toolCall.Function.Name,
								Arguments: toolCall.Function.Arguments,
							},
						}
						if existing.Type == "" {
							existing.Type = "function"
						}
						toolCallsMap[toolCall.Index] = existing
						toolCallIndices = append(toolCallIndices, toolCall.Index)
					} else {
						if toolCall.ID != "" {
							existing.ID = toolCall.ID
						}
						if toolCall.Type != "" {
							existing.Type = toolCall.Type
						}
						if toolCall.Function.Name != "" {
							existing.Function.Name += toolCall.Function.Name
						}
						existing.Function.Arguments += toolCall.Function.Arguments
					}
				}
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, fmt.Errorf("reading sse stream failed: %w", readErr)
		}
	}

	if !receivedAnyChunk {
		var singleResponse model.ChatCompletionResponse
		if err := json.Unmarshal(rawBuffer.Bytes(), &singleResponse); err == nil && len(singleResponse.Choices) > 0 {
			return &singleResponse, nil
		}
		return nil, fmt.Errorf("no sse chunks received from llm. Raw body: %s", rawBuffer.String())
	}

	finalToolCalls := make([]model.ChatToolCall, 0, len(toolCallIndices))
	for _, idx := range toolCallIndices {
		finalToolCalls = append(finalToolCalls, *toolCallsMap[idx])
	}

	return &model.ChatCompletionResponse{
		ID:    responseID,
		Model: responseModel,
		Choices: []model.ChatChoice{
			{
				Index: 0,
				Message: model.ChatMessage{
					Role:      "assistant",
					Content:   contentBuilder.String(),
					ToolCalls: finalToolCalls,
				},
				FinishReason: finishReason,
			},
		},
	}, nil
}
