package agent

import (
	"context"
	"fmt"
	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/internal/model"
	"go-rich-buddy-platform/internal/tool"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type ServiceImpl struct {
	agentClient         client.AgentClient
	toolRegistry        tool.Registry
	classifier          Classifier
	maxToolIterations   int
	regularModel        string
	analystModel        string
	regularSystemPrompt string
	analystSystemPrompt string
}

func NewService(
	agentClient client.AgentClient,
	toolRegistry tool.Registry,
	classifier Classifier,
	viperConfig *viper.Viper,
) *ServiceImpl {
	maxIterations := viperConfig.GetInt("AGENT_MAX_TOOL_ROUNDS")
	if maxIterations <= 0 {
		maxIterations = 10
	}

	regularModel := viperConfig.GetString("AGENT_REGULAR")
	if regularModel == "" {
		regularModel = "richBuddyRegular"
	}

	analystModel := viperConfig.GetString("AGENT_ANALYST")
	if analystModel == "" {
		analystModel = "richBuddyAnalyst"
	}

	regularSystemPrompt := viperConfig.GetString("AGENT_REGULAR_SYSTEM_PROMPT")
	if regularSystemPrompt == "" {
		regularSystemPrompt = `You are Rich Buddy (Regular Mode), a friendly, helpful AI assistant for casual conversation and general financial education.
CRITICAL RULES:
1. You are strictly FORBIDDEN from stating, quoting, or guessing specific actual market numbers (e.g. current stock prices, specific financial ratios like actual P/E or PBV, dividend yields, target prices).
2. If the user asks for specific market numbers or current stock data, politely decline and instruct them to ask for stock analysis specifically (e.g., 'Silakan tanyakan analisis saham [TICKER] untuk melihat data terkini') so the Analyst mode can handle it.
3. Keep explanations clear, engaging, and educational.`
	}

	analystSystemPrompt := viperConfig.GetString("AGENT_ANALYST_SYSTEM_PROMPT")
	if analystSystemPrompt == "" {
		analystSystemPrompt = `You are Rich Buddy (Analyst Mode), an expert financial and stock market analyst powered by Sectors API tools.
CRITICAL RULES:
1. Every factual market number, ratio, and financial data point in your final answer MUST be traceable to the tool results returned in this turn. You are strictly forbidden from inventing numbers or relying on memory.
2. If tool data is incomplete or unavailable, explicitly state which parts are unavailable. Never mask missing data with estimates.
3. Provide concise, high-value insights, summaries, and context. Do NOT simply dump raw JSON data. Explain what the numbers mean for the user and compare with historical/industry averages if available.
4. If a tool returns an error, explain it in user-friendly language (e.g., 'Data untuk ticker X tidak ditemukan'), not technical error messages.
5. If the user only greeted or asked something not requiring tools, you may respond directly without calling tools.`
	}

	return &ServiceImpl{
		agentClient:         agentClient,
		toolRegistry:        toolRegistry,
		classifier:          classifier,
		maxToolIterations:   maxIterations,
		regularModel:        regularModel,
		analystModel:        analystModel,
		regularSystemPrompt: regularSystemPrompt,
		analystSystemPrompt: analystSystemPrompt,
	}
}

func (agentService *ServiceImpl) CreateSession(websocketConn *websocket.Conn) Session {
	return NewSession(websocketConn)
}

func (agentService *ServiceImpl) RunSession(ctx context.Context, session Session) {
	session.SetState(SessionStateIdle)
	session.StartPumps()
	defer session.Close()

	for {
		userMessage, isValid := session.NextUserMessage(ctx)
		logrus.Debugf("Received user message: %s", userMessage)
		if !isValid {
			return
		}

		err := agentService.ProcessTurn(ctx, session, userMessage)
		if err != nil {
			logrus.WithError(err).Error("Fatal error encountered in agent loop turn")
			return
		}

		session.SetState(SessionStateIdle)
	}
}

func (agentService *ServiceImpl) ProcessTurn(ctx context.Context, session Session, userMessage string) error {
	session.SetState(SessionStateProcessing)

	trimmedMessage := strings.TrimSpace(userMessage)
	if trimmedMessage == "" {
		_ = session.SendFrame(model.WebsocketServerMessage{
			Type:    "error",
			Message: "Input message cannot be empty or whitespace",
		})
		return fmt.Errorf("empty input message")
	}

	mode := agentService.classifier.Classify(ctx, trimmedMessage)
	logrus.WithFields(logrus.Fields{
		"mode":         mode,
		"user_message": trimmedMessage,
	}).Info("Agent mode classified for turn")

	session.AppendHistory(model.ChatMessage{
		Role:    "user",
		Content: trimmedMessage,
	})

	if mode == ModeRegular {
		return agentService.processRegularTurn(ctx, session)
	}

	return agentService.processAnalystTurn(ctx, session)
}

func (agentService *ServiceImpl) processRegularTurn(ctx context.Context, session Session) error {
	messages := append([]model.ChatMessage{
		{Role: "system", Content: agentService.regularSystemPrompt},
	}, session.GetHistory()...)

	request := &model.ChatCompletionRequest{
		Model:    agentService.regularModel,
		Messages: messages,
		Tools:    nil,
	}

	agentResponse, err := agentService.agentClient.CreateChatCompletion(ctx, request)
	if err != nil {
		_ = session.SendFrame(model.WebsocketServerMessage{
			Type:    "error",
			Message: fmt.Sprintf("Regular LLM completion failed: %v", err),
		})
		return err
	}

	if len(agentResponse.Choices) == 0 {
		_ = session.SendFrame(model.WebsocketServerMessage{
			Type:    "error",
			Message: "Received empty choices from LLM",
		})
		return fmt.Errorf("empty choices from LLM")
	}

	assistantMessage := agentResponse.Choices[0].Message
	session.AppendHistory(model.ChatMessage{
		Role:    "assistant",
		Content: assistantMessage.Content,
	})

	_ = session.SendFrame(model.WebsocketServerMessage{
		Type:    "assistant_message",
		Text:    assistantMessage.Content,
		Content: assistantMessage.Content,
	})

	logrus.WithFields(logrus.Fields{
		"mode": ModeRegular,
	}).Info("Completed regular mode turn")

	return nil
}

func (agentService *ServiceImpl) processAnalystTurn(ctx context.Context, session Session) error {
	tools := agentService.toolRegistry.GetToolDefinitions()

	rountOfIteration := 1
	for {
		if rountOfIteration > agentService.maxToolIterations {

			return agentService.handleMaxIterationsReached(ctx, session)
		}

		messages := append([]model.ChatMessage{
			{Role: "system", Content: agentService.analystSystemPrompt},
		}, session.GetHistory()...)

		request := &model.ChatCompletionRequest{
			Model:    agentService.analystModel,
			Messages: messages,
			Tools:    tools,
		}

		response, err := agentService.agentClient.CreateChatCompletion(ctx, request)
		if err != nil {
			_ = session.SendFrame(model.WebsocketServerMessage{
				Type:    "error",
				Message: fmt.Sprintf("Analyst LLM completion failed: %v", err),
			})
			return err
		}

		if len(response.Choices) == 0 {
			_ = session.SendFrame(model.WebsocketServerMessage{
				Type:    "error",
				Message: "Received empty choices from LLM",
			})
			return fmt.Errorf("empty choices from LLM")
		}

		choice := response.Choices[0]
		assistantMessage := choice.Message

		if len(assistantMessage.ToolCalls) == 0 {
			session.AppendHistory(model.ChatMessage{
				Role:    "assistant",
				Content: assistantMessage.Content,
			})

			_ = session.SendFrame(model.WebsocketServerMessage{
				Type:    "assistant_message",
				Text:    assistantMessage.Content,
				Content: assistantMessage.Content,
			})

			logrus.WithFields(logrus.Fields{
				"mode":   ModeAnalyst,
				"rounds": rountOfIteration,
			}).Info("Completed analyst mode turn")

			return nil
		}

		session.AppendHistory(model.ChatMessage{
			Role:      "assistant",
			Content:   assistantMessage.Content,
			ToolCalls: assistantMessage.ToolCalls,
		})

		type toolExecutionResult struct {
			callID string
			name   string
			output string
		}

		results := make([]toolExecutionResult, len(assistantMessage.ToolCalls))
		var waitGroup sync.WaitGroup

		for index, toolCall := range assistantMessage.ToolCalls {
			waitGroup.Add(1)
			go func(idx int, call model.ChatToolCall) {
				defer waitGroup.Done()

				toolName := call.Function.Name
				toolArgs := call.Function.Arguments

				session.SendBestEffortFrame(model.WebsocketServerMessage{
					Type:  "tool_status",
					Tool:  toolName,
					State: "start",
				})

				output, toolErr := agentService.toolRegistry.Execute(ctx, toolName, toolArgs)
				if toolErr != nil {
					output = fmt.Sprintf(`{"error": %q}`, toolErr.Error())
				}

				session.SendBestEffortFrame(model.WebsocketServerMessage{
					Type:  "tool_status",
					Tool:  toolName,
					State: "done",
				})

				results[idx] = toolExecutionResult{
					callID: call.ID,
					name:   toolName,
					output: output,
				}
			}(index, toolCall)
		}

		waitGroup.Wait()

		for _, result := range results {
			session.AppendHistory(model.ChatMessage{
				Role:       "tool",
				ToolCallID: result.callID,
				Content:    result.output,
			})
		}

		rountOfIteration++
	}
}

func (agentService *ServiceImpl) handleMaxIterationsReached(ctx context.Context, session Session) error {
	synthesisMessages := append([]model.ChatMessage{
		{Role: "system", Content: agentService.analystSystemPrompt},
	}, session.GetHistory()...)

	synthesisMessages = append(synthesisMessages, model.ChatMessage{
		Role:    "user",
		Content: "Batas iterasi pengambilan data telah tercapai. Tolong berikan ringkasan analisis berdasarkan data yang sudah berhasil didapatkan sejauh ini, dan sebutkan secara jelas bagian mana yang belum sempat dicek.",
	})

	synthesisRequest := &model.ChatCompletionRequest{
		Model:    agentService.analystModel,
		Messages: synthesisMessages,
		Tools:    nil,
	}

	response, err := agentService.agentClient.CreateChatCompletion(ctx, synthesisRequest)
	if err == nil && len(response.Choices) > 0 {
		content := response.Choices[0].Message.Content
		session.AppendHistory(model.ChatMessage{
			Role:    "assistant",
			Content: content,
		})
		_ = session.SendFrame(model.WebsocketServerMessage{
			Type:    "assistant_message",
			Text:    content,
			Content: content,
		})
		return nil
	}

	fallbackContent := "Batas iterasi pengambilan data tercapai. Silakan ajukan pertanyaan yang lebih spesifik untuk melanjutkan analisis."
	session.AppendHistory(model.ChatMessage{
		Role:    "assistant",
		Content: fallbackContent,
	})
	_ = session.SendFrame(model.WebsocketServerMessage{
		Type:    "assistant_message",
		Text:    fallbackContent,
		Content: fallbackContent,
	})
	return nil
}
