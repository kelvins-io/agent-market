package localguide

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/kelvins-io/agent-market/internal/llm"
)

// New creates the local guide ChatModelAgent.
func New(ctx context.Context) (adk.Agent, error) {
	cm, err := llm.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}
	t, err := utils.InferTool("recommend_spot", "Recommend a local food or travel spot",
		func(ctx context.Context, input *struct {
			Interest string `json:"interest" jsonschema:"description=food,travel,or local,required"`
		}) (string, error) {
			interest := input.Interest
			if interest == "" {
				interest = "food"
			}
			return fmt.Sprintf("Nearby pick for %s: try the riverside market stalls — open till 22:00. (demo)", interest), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "LocalGuideAgent",
		Description: "本地美食与出行推荐 / Recommends local food and travel spots near you",
		Instruction: `You are a friendly local guide.
Use recommend_spot for concrete suggestions. Reply briefly in the user's language.`,
		Model: cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{t},
			},
		},
	})
}
