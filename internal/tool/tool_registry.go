package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"go-rich-buddy-platform/internal/model"
	"sync"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, arguments string) (string, error)
}

type Registry interface {
	Register(tool Tool)
	RegisterAll(tools ...Tool)
	GetTool(name string) (Tool, bool)
	GetToolDefinitions() []model.ChatTool
	Execute(ctx context.Context, name string, arguments string) (string, error)
}

type RegistryImpl struct {
	toolsMutex sync.RWMutex
	toolsMap   map[string]Tool
}

func NewRegistry() Registry {
	return &RegistryImpl{
		toolsMap: make(map[string]Tool),
	}
}

func (registryImpl *RegistryImpl) Register(tool Tool) {
	registryImpl.toolsMutex.Lock()
	defer registryImpl.toolsMutex.Unlock()
	registryImpl.toolsMap[tool.Name()] = tool
}

func (registryImpl *RegistryImpl) RegisterAll(availableTools ...Tool) {
	registryImpl.toolsMutex.Lock()
	defer registryImpl.toolsMutex.Unlock()
	for _, availableTool := range availableTools {
		registryImpl.toolsMap[availableTool.Name()] = availableTool
	}
}

func (registryImpl *RegistryImpl) GetTool(name string) (Tool, bool) {
	registryImpl.toolsMutex.RLock()
	defer registryImpl.toolsMutex.RUnlock()
	tool, exists := registryImpl.toolsMap[name]
	return tool, exists
}

func (registryImpl *RegistryImpl) GetToolDefinitions() []model.ChatTool {
	registryImpl.toolsMutex.RLock()
	defer registryImpl.toolsMutex.RUnlock()

	toolDefinitions := make([]model.ChatTool, 0, len(registryImpl.toolsMap))
	for _, tool := range registryImpl.toolsMap {
		toolDefinitions = append(toolDefinitions, model.ChatTool{
			Type: "function",
			Function: model.ChatToolFunctionDetail{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}
	return toolDefinitions
}

func (registryImpl *RegistryImpl) Execute(ctx context.Context, name string, arguments string) (string, error) {
	targetTool, isExists := registryImpl.GetTool(name)
	if !isExists {
		errorJson, _ := json.Marshal(map[string]string{
			"error": fmt.Sprintf("Tool %s not found in registry", name),
		})
		return string(errorJson), nil
	}

	resultTool, err := targetTool.Execute(ctx, arguments)
	if err != nil {
		errorJson, _ := json.Marshal(map[string]string{
			"error": err.Error(),
		})
		return string(errorJson), nil
	}

	return resultTool, nil
}
