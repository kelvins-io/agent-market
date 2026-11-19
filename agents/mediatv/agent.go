package mediatv

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/kelvins-io/agent-market/internal/llm"
	"github.com/kelvins-io/agent-market/internal/mediasearch"
)

// New creates the movie / TV ChatModelAgent wrapping iQIYI, Youku, and Tencent Video search.
func New(ctx context.Context) (adk.Agent, error) {
	cm, err := llm.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}

	searchIQIYI, err := utils.InferTool("search_iqiyi", "Search movies and TV series on iQIYI (爱奇艺)",
		func(ctx context.Context, input *struct {
			Query string `json:"query" jsonschema:"description=title keyword or 电影/电视剧,required"`
			Limit int    `json:"limit" jsonschema:"description=max results 1-10"`
		}) (string, error) {
			hits, err := mediasearch.Search(mediasearch.IQIYI, input.Query, input.Limit)
			if err != nil {
				return "", err
			}
			return mediasearch.FormatItems(mediasearch.IQIYI, input.Query, hits), nil
		},
	)
	if err != nil {
		return nil, err
	}

	searchYouku, err := utils.InferTool("search_youku", "Search movies and TV series on Youku (优酷)",
		func(ctx context.Context, input *struct {
			Query string `json:"query" jsonschema:"description=title keyword or 电影/电视剧,required"`
			Limit int    `json:"limit" jsonschema:"description=max results 1-10"`
		}) (string, error) {
			hits, err := mediasearch.Search(mediasearch.Youku, input.Query, input.Limit)
			if err != nil {
				return "", err
			}
			return mediasearch.FormatItems(mediasearch.Youku, input.Query, hits), nil
		},
	)
	if err != nil {
		return nil, err
	}

	searchTencent, err := utils.InferTool("search_tencent", "Search movies and TV series on Tencent Video (腾讯视频)",
		func(ctx context.Context, input *struct {
			Query string `json:"query" jsonschema:"description=title keyword or 电影/电视剧,required"`
			Limit int    `json:"limit" jsonschema:"description=max results 1-10"`
		}) (string, error) {
			hits, err := mediasearch.Search(mediasearch.Tencent, input.Query, input.Limit)
			if err != nil {
				return "", err
			}
			return mediasearch.FormatItems(mediasearch.Tencent, input.Query, hits), nil
		},
	)
	if err != nil {
		return nil, err
	}

	searchAll, err := utils.InferTool("search_media", "Search movies and TV series across iQIYI, Youku, and Tencent Video",
		func(ctx context.Context, input *struct {
			Query string `json:"query" jsonschema:"description=title keyword or 电影/电视剧,required"`
			Limit int    `json:"limit" jsonschema:"description=max results per platform 1-5"`
		}) (string, error) {
			hits, err := mediasearch.SearchAll(input.Query, input.Limit)
			if err != nil {
				return "", err
			}
			return mediasearch.FormatAll(input.Query, hits), nil
		},
	)
	if err != nil {
		return nil, err
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "MediaTVAgent",
		Description: "电影电视剧搜索（爱奇艺/优酷/腾讯视频） / Search movies and TV series on iQIYI, Youku, Tencent Video",
		Instruction: `You are a movie and TV series search agent for Chinese streaming platforms (爱奇艺 / 优酷 / 腾讯视频).

Always call a search tool before answering. Never say you have no recommendations without calling a tool first.

Query rules for tools:
- Recommendation / "最近好看" / "有什么剧" → search_media with query "电视剧" (or "电影" if they ask for movies). limit=3.
- Specific title (e.g. 狂飙) → search_media with that short title only.
- User names one platform → use search_iqiyi / search_youku / search_tencent.
- Do not pass the full user sentence as query.

After tools return, recommend 2–4 titles: name, year, score, platforms. Reply in the user's language.`,
		Model: cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{searchAll, searchIQIYI, searchYouku, searchTencent},
			},
		},
	})
}
