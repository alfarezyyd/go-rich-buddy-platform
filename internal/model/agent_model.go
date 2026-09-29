package model

type WSClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Text    string `json:"text,omitempty"`
	Message string `json:"message,omitempty"`
}

func (clientMessage *WSClientMessage) GetUserMessage() string {
	if clientMessage.Content != "" {
		return clientMessage.Content
	}
	if clientMessage.Text != "" {
		return clientMessage.Text
	}
	return clientMessage.Message
}

type WebsocketServerMessage struct {
	Type    string `json:"type"`
	Tool    string `json:"tool,omitempty"`
	State   string `json:"state,omitempty"`
	Text    string `json:"text,omitempty"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
}

type ChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type ChatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ChatFunctionCall `json:"function"`
}

type ChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatTool struct {
	Type     string                 `json:"type"`
	Function ChatToolFunctionDetail `json:"function"`
}

type ChatToolFunctionDetail struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Tools       []ChatTool    `json:"tools,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Error   *ChatError   `json:"error,omitempty"`
	Usage   *ChatUsage   `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int         `json:"index"`
	Delta        ChatDelta   `json:"delta"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatToolCallChunk struct {
	Index    int              `json:"index"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ChatFunctionCall `json:"function,omitempty"`
}

type ChatDelta struct {
	Role      string              `json:"role,omitempty"`
	Content   string              `json:"content,omitempty"`
	ToolCalls []ChatToolCallChunk `json:"tool_calls,omitempty"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
