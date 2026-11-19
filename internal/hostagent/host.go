package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/kelvins-io/agent-market/internal/a2a"
	"github.com/kelvins-io/agent-market/internal/discovery"
	"github.com/kelvins-io/agent-market/internal/geo"
	"github.com/kelvins-io/agent-market/internal/llm"
	"github.com/kelvins-io/agent-market/internal/models"
	"github.com/kelvins-io/agent-market/internal/registry"
)

// Host is the market-side Eino ChatModelAgent that can discover and talk to A2A agents.
type Host struct {
	runner *adk.Runner
	store  *registry.Store
	disc   *discovery.Service
}

// New creates the host agent. Returns (nil, nil) when neither MOCK_LLM nor OPENAI_API_KEY is set.
func New(ctx context.Context, store *registry.Store, disc *discovery.Service) (*Host, error) {
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("MOCK_LLM") == "" {
		return nil, nil
	}
	cm, err := llm.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("chat model: %w", err)
	}

	h := &Host{store: store, disc: disc}
	discoverTool, err := h.discoverTool()
	if err != nil {
		return nil, err
	}
	talkTool, err := h.talkTool()
	if err != nil {
		return nil, err
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "MarketHost",
		Description: "Agent Market host that discovers nearby agents and talks to them over A2A",
		Instruction: `You are the Agent Market host assistant.
Help users find nearby agents and talk to them over A2A.

Tool rules:
- discover_agents: prefer the q keyword (e.g. 天气, 美食, 翻译, 电影, 电视剧). Optional category is "utility", "lifestyle", or "entertainment". Optional tags examples: weather, forecast, food, travel, translate, movie, tv, 天气, 美食, 翻译, 电影, 电视剧. Never invent category/tag values that are not in those lists. Default radius_km to 50. Omit category/tags when unsure.
- talk_to_agent: after discover returns an agent, call it with that agent's id to fulfill the user's request. For weather, include the user's city in the message (e.g. "今天北京天气怎么样"). For movie/TV (影视) agents, pass the title or 电影/电视剧 keyword (e.g. "帮我搜狂飙在哪些平台能看"). For takeout/merchant (外卖) agents, ask about the menu in Chinese (e.g. "今天有什么菜") and summarize the returned menu/prices for the user.

Reply in the user's language. Keep answers concise.`,
		Model: cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{discoverTool, talkTool},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	h.runner = adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	return h, nil
}

type emitFn func(event string, data any)

// ChatSSE runs one turn and streams events via emit.
func (h *Host) ChatSSE(ctx context.Context, message string, lat, lng float64, emit emitFn) error {
	if h == nil || h.runner == nil {
		return fmt.Errorf("host agent unavailable: set OPENAI_API_KEY")
	}
	ctx = withUserLoc(ctx, lat, lng)
	place := geo.ReversePlace(lat, lng)
	prompt := fmt.Sprintf("用户位置：%s (lat=%.4f, lng=%.4f)\n%s", place.Name, lat, lng, message)
	iter := h.runner.Query(ctx, prompt)
	for {
		event, ok := iter.Next()
		if !ok {
			return nil
		}
		if event.Err != nil {
			emit("error", event.Err.Error())
			return event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mo := event.Output.MessageOutput
		if mo.Message != nil {
			if len(mo.Message.ToolCalls) > 0 {
				for _, tc := range mo.Message.ToolCalls {
					emit("tool", map[string]string{"name": tc.Function.Name, "arguments": tc.Function.Arguments})
				}
			}
			if mo.Message.Content != "" && mo.Message.Role != schema.Tool {
				emit("message", mo.Message.Content)
			}
			if mo.Message.Role == schema.Tool && mo.Message.Content != "" {
				emit("tool_result", mo.Message.Content)
			}
		}
		if mo.MessageStream != nil {
			var b strings.Builder
			for {
				chunk, err := mo.MessageStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if chunk == nil {
					continue
				}
				if len(chunk.ToolCalls) > 0 {
					for _, tc := range chunk.ToolCalls {
						if tc.Function.Name != "" {
							emit("tool", map[string]string{"name": tc.Function.Name, "arguments": tc.Function.Arguments})
						}
					}
				}
				if chunk.Content != "" && chunk.Role != schema.Tool {
					b.WriteString(chunk.Content)
					emit("delta", chunk.Content)
				}
			}
			if s := b.String(); s != "" {
				emit("message", s)
			}
		}
	}
}

type locKey struct{}

type userLoc struct{ Lat, Lng float64 }

func withUserLoc(ctx context.Context, lat, lng float64) context.Context {
	return context.WithValue(ctx, locKey{}, userLoc{Lat: lat, Lng: lng})
}

func locFrom(ctx context.Context) (float64, float64) {
	if v, ok := ctx.Value(locKey{}).(userLoc); ok {
		return v.Lat, v.Lng
	}
	return 39.9042, 116.4074
}

type discoverArgs struct {
	Q        string  `json:"q" jsonschema:"description=keyword such as 天气/美食/翻译/电影/电视剧 or weather/food/translate/movie"`
	Category string  `json:"category" jsonschema:"description=optional category: utility or lifestyle only"`
	Tags     string  `json:"tags" jsonschema:"description=optional comma-separated tags; match any one tag"`
	RadiusKm float64 `json:"radius_km" jsonschema:"description=search radius in km, default 50"`
}

func (h *Host) discoverTool() (tool.InvokableTool, error) {
	return utils.InferTool("discover_agents",
		"Discover agents in the market by keyword, category, tags, and geographic radius from the user",
		func(ctx context.Context, args *discoverArgs) (string, error) {
			lat, lng := locFrom(ctx)
			tags := splitCSV(args.Tags)
			radius := args.RadiusKm
			if radius <= 0 {
				radius = 50
			}
			category := strings.TrimSpace(args.Category)
			// LLMs often confuse tag values with category (e.g. category=weather).
			if category != "" && !validCategory(category) {
				if args.Q == "" {
					args.Q = category
				}
				tags = append(tags, category)
				category = ""
			}

			queries := []models.DiscoverQuery{
				{Lat: lat, Lng: lng, RadiusKm: radius, Category: category, Tags: tags, Q: args.Q},
			}
			// Progressive broaden when strict filters miss demo agents.
			if category != "" || len(tags) > 0 {
				queries = append(queries, models.DiscoverQuery{
					Lat: lat, Lng: lng, RadiusKm: maxFloat(radius, 50), Q: args.Q,
				})
			}
			if args.Q != "" {
				queries = append(queries, models.DiscoverQuery{
					Lat: lat, Lng: lng, RadiusKm: maxFloat(radius, 50),
				})
			}

			var results []models.AgentWithDistance
			var err error
			for _, q := range queries {
				results, err = h.disc.Discover(ctx, q)
				if err != nil {
					return "", err
				}
				if len(results) > 0 {
					break
				}
			}
			type brief struct {
				ID          string   `json:"id"`
				Name        string   `json:"name"`
				Category    string   `json:"category"`
				Tags        []string `json:"tags"`
				DistanceKm  float64  `json:"distance_km"`
				Description string   `json:"description"`
			}
			out := make([]brief, 0, len(results))
			for _, r := range results {
				out = append(out, brief{
					ID: r.ID, Name: r.Name, Category: r.Category, Tags: r.Tags,
					DistanceKm: r.DistanceKm, Description: r.Description,
				})
			}
			b, _ := json.Marshal(out)
			return string(b), nil
		},
	)
}

func splitCSV(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func validCategory(c string) bool {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "utility", "lifestyle", "entertainment":
		return true
	default:
		return false
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

type talkArgs struct {
	AgentID string `json:"agent_id" jsonschema:"description=registered agent id,required"`
	Message string `json:"message" jsonschema:"description=message to send,required"`
}

func (h *Host) talkTool() (tool.InvokableTool, error) {
	return utils.InferTool("talk_to_agent",
		"Send a text message to a registered agent over A2A and return its reply",
		func(ctx context.Context, args *talkArgs) (string, error) {
			rec, err := h.store.Get(ctx, args.AgentID)
			if err != nil {
				return "", err
			}
			msg := strings.TrimSpace(args.Message)
			lat, lng := locFrom(ctx)
			place := geo.ReversePlace(lat, lng)
			if place.City != "" && !strings.Contains(msg, place.City) && !strings.Contains(msg, place.Name) {
				msg = fmt.Sprintf("%s（地点：%s）", msg, place.City)
			}
			return a2a.SendText(ctx, a2a.SendOptions{
				BaseURL:     rec.A2AURL,
				HandlerPath: rec.HandlerPath,
				Message:     msg,
				AgentCard:   rec.AgentCard,
			})
		},
	)
}
