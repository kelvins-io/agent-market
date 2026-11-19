package translator

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/kelvins-io/agent-market/internal/llm"
)

// New creates the translator ChatModelAgent.
func New(ctx context.Context) (adk.Agent, error) {
	cm, err := llm.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}
	t, err := utils.InferTool("translate_text", "Translate text between languages (demo)",
		func(ctx context.Context, input *struct {
			Text       string `json:"text" jsonschema:"description=text to translate,required"`
			TargetLang string `json:"target_lang" jsonschema:"description=target language code like en or zh,required"`
		}) (string, error) {
			text := strings.TrimSpace(input.Text)
			lang := strings.TrimSpace(input.TargetLang)
			if lang == "" {
				lang = "en"
			}
			return fmt.Sprintf("[%s] %s (demo translation)", lang, text), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "TranslatorAgent",
		Description: "多语言翻译 / Translates text between languages",
		Instruction: `You are a translation agent.
Use translate_text for translations. Reply briefly in the user's language.`,
		Model: cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{t},
			},
		},
	})
}
