package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// NewChatModel builds a chat model from env.
// When MOCK_LLM=1/true/yes, returns an offline mock (no network).
func NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	if mockEnabled(os.Getenv("MOCK_LLM")) {
		return NewMock(), nil
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set (or set MOCK_LLM=1 for offline demo)")
	}
	modelName := os.Getenv("OPENAI_MODEL")
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
	})
}

func mockEnabled(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "mock":
		return true
	default:
		return false
	}
}
