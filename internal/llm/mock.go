package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// Mock is an offline ToolCallingChatModel for demos when OpenAI is unreachable.
type Mock struct {
	tools []*schema.ToolInfo
}

func NewMock() model.ToolCallingChatModel {
	return &Mock{}
}

func (m *Mock) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cp := &Mock{tools: append([]*schema.ToolInfo(nil), tools...)}
	return cp, nil
}

func (m *Mock) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	tools := m.tools
	if o := model.GetCommonOptions(nil, opts...); o != nil && len(o.Tools) > 0 {
		tools = o.Tools
	}
	if len(input) == 0 {
		return &schema.Message{Role: schema.Assistant, Content: "你好，我是演示 Agent（MOCK_LLM）。"}, nil
	}

	userText := latestUserText(input)
	last := input[len(input)-1]
	if last.Role == schema.Tool {
		// After discover_agents, continue into talk_to_agent when the user asked a question.
		if talk := findTool(tools, "talk_to_agent"); talk != nil &&
			looksLikeDiscoverJSON(last.Content) && wantsAgentReply(userText) {
			args := mockArgsForTool(talk.Name, userText)
			return &schema.Message{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID:   "mock-" + uuid.NewString()[:8],
					Type: "function",
					Function: schema.FunctionCall{
						Name:      talk.Name,
						Arguments: args,
					},
				}},
			}, nil
		}
		return &schema.Message{
			Role:    schema.Assistant,
			Content: fmt.Sprintf("根据工具结果：%s", strings.TrimSpace(last.Content)),
		}, nil
	}

	// If history already has a non-discover tool result, summarize it.
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.Tool && strings.TrimSpace(input[i].Content) != "" &&
			!looksLikeDiscoverJSON(input[i].Content) {
			return &schema.Message{
				Role:    schema.Assistant,
				Content: fmt.Sprintf("根据工具结果：%s", strings.TrimSpace(input[i].Content)),
			}, nil
		}
	}

	if tool := pickTool(tools, userText); tool != nil {
		args := mockArgsForTool(tool.Name, userText)
		return &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "mock-" + uuid.NewString()[:8],
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tool.Name,
					Arguments: args,
				},
			}},
		}, nil
	}

	if userText == "" {
		userText = "你好"
	}
	return &schema.Message{
		Role:    schema.Assistant,
		Content: fmt.Sprintf("（演示模式 MOCK_LLM）已收到：%s。当前未配置可达的大模型接口，已用本地模拟回复。", userText),
	}, nil
}

func (m *Mock) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func pickTool(tools []*schema.ToolInfo, userText string) *schema.ToolInfo {
	if len(tools) == 0 {
		return nil
	}
	lower := strings.ToLower(userText)
	if d := findTool(tools, "discover_agents"); d != nil &&
		(strings.Contains(userText, "附近") || strings.Contains(userText, "找") ||
			strings.Contains(userText, "天气") || strings.Contains(userText, "美食") ||
			strings.Contains(userText, "翻译") || strings.Contains(userText, "电影") ||
			strings.Contains(userText, "电视剧") || strings.Contains(userText, "影视") ||
			strings.Contains(lower, "agent")) {
		return d
	}
	for _, t := range tools {
		name := strings.ToLower(t.Name)
		switch {
		case strings.Contains(name, "weather") && (strings.Contains(userText, "天气") || strings.Contains(lower, "weather")):
			return t
		case strings.Contains(name, "recommend") && (strings.Contains(userText, "美食") || strings.Contains(userText, "推荐") || strings.Contains(lower, "food") || strings.Contains(lower, "travel")):
			return t
		case strings.Contains(name, "translate") && (strings.Contains(userText, "翻译") || strings.Contains(lower, "translate")):
			return t
		case (strings.Contains(name, "search_media") || strings.Contains(name, "search_iqiyi") ||
			strings.Contains(name, "search_youku") || strings.Contains(name, "search_tencent") ||
			strings.Contains(name, "media")) &&
			(strings.Contains(userText, "电影") || strings.Contains(userText, "电视剧") ||
				strings.Contains(userText, "影视") || strings.Contains(userText, "爱奇艺") ||
				strings.Contains(userText, "优酷") || strings.Contains(userText, "腾讯") ||
				strings.Contains(userText, "好看") || strings.Contains(userText, "追剧") ||
				strings.Contains(lower, "movie") || strings.Contains(lower, "series")):
			return t
		}
	}
	// Default: call the first tool so ReAct agents still complete a demo turn.
	return tools[0]
}

func findTool(tools []*schema.ToolInfo, name string) *schema.ToolInfo {
	for _, t := range tools {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func looksLikeDiscoverJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "[") && (strings.Contains(s, `"id"`) || s == "[]")
}

func wantsAgentReply(userText string) bool {
	return strings.Contains(userText, "问") || strings.Contains(userText, "今天") ||
		strings.Contains(userText, "聊聊") || strings.Contains(userText, "对话") ||
		strings.Contains(strings.ToLower(userText), "ask")
}

func mockArgsForTool(name, userText string) string {
	lowerName := strings.ToLower(name)
	switch {
	case strings.Contains(lowerName, "discover"):
		q := ""
		switch {
		case strings.Contains(userText, "天气") || strings.Contains(strings.ToLower(userText), "weather"):
			q = "天气"
		case strings.Contains(userText, "美食") || strings.Contains(userText, "吃"):
			q = "美食"
		case strings.Contains(userText, "翻译") || strings.Contains(strings.ToLower(userText), "translate"):
			q = "翻译"
		case strings.Contains(userText, "电影") || strings.Contains(userText, "电视剧") ||
			strings.Contains(userText, "影视") || strings.Contains(strings.ToLower(userText), "movie"):
			q = "电影"
		}
		b, _ := json.Marshal(map[string]any{"q": q, "radius_km": 50})
		return string(b)
	case strings.Contains(lowerName, "talk"):
		agentID := "weather"
		msg := userText
		switch {
		case strings.Contains(userText, "电影") || strings.Contains(userText, "电视剧") ||
			strings.Contains(userText, "影视") || strings.Contains(userText, "爱奇艺") ||
			strings.Contains(userText, "优酷") || strings.Contains(userText, "腾讯") ||
			strings.Contains(userText, "追剧"):
			agentID = "mediatv"
		case strings.Contains(userText, "美食") || strings.Contains(userText, "推荐"):
			agentID = "localguide"
		case strings.Contains(userText, "翻译"):
			agentID = "translator"
		}
		if strings.Contains(userText, "天气") {
			msg = "今天北京天气怎么样"
		}
		if agentID == "mediatv" && !strings.Contains(msg, "搜") {
			msg = "帮我搜狂飙在哪些平台能看"
		}
		b, _ := json.Marshal(map[string]string{"agent_id": agentID, "message": msg})
		return string(b)
	case strings.Contains(lowerName, "weather"):
		city := "Beijing"
		if strings.Contains(userText, "上海") {
			city = "Shanghai"
		}
		b, _ := json.Marshal(map[string]string{"city": city})
		return string(b)
	case strings.Contains(lowerName, "recommend"):
		interest := "food"
		if strings.Contains(userText, "旅游") || strings.Contains(userText, "景点") {
			interest = "travel"
		}
		b, _ := json.Marshal(map[string]string{"interest": interest})
		return string(b)
	case strings.Contains(lowerName, "translate"):
		b, _ := json.Marshal(map[string]string{"text": userText, "target_lang": "en"})
		return string(b)
	case strings.Contains(lowerName, "search_media") || strings.Contains(lowerName, "search_iqiyi") ||
		strings.Contains(lowerName, "search_youku") || strings.Contains(lowerName, "search_tencent"):
		query := "热门"
		for _, title := range []string{"狂飙", "繁花", "三体", "流浪地球", "庆余年", "长相思", "热辣滚烫", "奥本海默"} {
			if strings.Contains(userText, title) {
				query = title
				break
			}
		}
		if query == "热门" {
			switch {
			case strings.Contains(userText, "电视剧") || strings.Contains(userText, "追剧") || strings.Contains(userText, "剧"):
				query = "电视剧"
			case strings.Contains(userText, "电影"):
				query = "电影"
			}
		}
		b, _ := json.Marshal(map[string]any{"query": query, "limit": 3})
		return string(b)
	default:
		return "{}"
	}
}

func latestUserText(input []*schema.Message) string {
	for i := len(input) - 1; i >= 0; i-- {
		if input[i].Role == schema.User && strings.TrimSpace(input[i].Content) != "" {
			return strings.TrimSpace(input[i].Content)
		}
	}
	return ""
}
