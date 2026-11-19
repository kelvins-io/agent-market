package weather

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/kelvins-io/agent-market/internal/llm"
)

// New creates the weather ChatModelAgent.
func New(ctx context.Context) (adk.Agent, error) {
	cm, err := llm.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}
	t, err := utils.InferTool("get_weather", "Get a simple weather summary for a city",
		func(ctx context.Context, input *struct {
			City string `json:"city" jsonschema:"description=city name,required"`
		}) (string, error) {
			city := input.City
			if city == "" {
				city = "Beijing"
			}
			return fmt.Sprintf("%s: 22°C, partly cloudy, light breeze. (demo forecast)", city), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "WeatherAgent",
		Description: "附近天气与预报 / Provides weather forecasts for nearby locations",
		Instruction: `You are a helpful weather agent.
Use get_weather when the user asks about weather. Reply briefly in the user's language.`,
		Model: cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{t},
			},
		},
	})
}
