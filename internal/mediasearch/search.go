// Package mediasearch wraps demo search APIs for iQIYI, Youku, and Tencent Video.
package mediasearch

import (
	"fmt"
	"strings"
	"unicode"
)

// Platform identifies a Chinese video streaming service.
type Platform string

const (
	IQIYI   Platform = "iqiyi"
	Youku   Platform = "youku"
	Tencent Platform = "tencent"
)

// Item is one search hit from a platform.
type Item struct {
	Platform    Platform `json:"platform"`
	Title       string   `json:"title"`
	Kind        string   `json:"kind"` // movie | series
	Year        int      `json:"year"`
	Score       float64  `json:"score"`
	Episodes    string   `json:"episodes,omitempty"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
}

type catalogEntry struct {
	Title       string
	Aliases     []string
	Kind        string
	Year        int
	Score       float64
	Episodes    string
	Description string
	Platforms   map[Platform]string // platform -> relative path / id
}

var catalog = []catalogEntry{
	{
		Title: "狂飙", Aliases: []string{"狂飙", "狂飙剧", "crime drama"}, Kind: "series", Year: 2023, Score: 9.1,
		Episodes: "39集全", Description: "扫黑反腐题材剧，张颂文、张译主演。",
		Platforms: map[Platform]string{IQIYI: "a_kuangbiao", Youku: "id_kuangbiao", Tencent: "cover_kuangbiao"},
	},
	{
		Title: "繁花", Aliases: []string{"繁花", "王家卫"}, Kind: "series", Year: 2023, Score: 8.2,
		Episodes: "30集全", Description: "上世纪九十年代上海商战传奇。",
		Platforms: map[Platform]string{IQIYI: "a_fanhua", Youku: "id_fanhua", Tencent: "cover_fanhua"},
	},
	{
		Title: "三体", Aliases: []string{"三体", "科幻", "刘慈欣"}, Kind: "series", Year: 2023, Score: 8.0,
		Episodes: "30集全", Description: "根据刘慈欣同名小说改编的科幻剧。",
		Platforms: map[Platform]string{Tencent: "cover_santi", IQIYI: "a_santi"},
	},
	{
		Title: "流浪地球2", Aliases: []string{"流浪地球", "流浪地球2", "科幻电影"}, Kind: "movie", Year: 2023, Score: 8.3,
		Description: "刘慈欣科幻宇宙电影续作，吴京、刘德华主演。",
		Platforms: map[Platform]string{IQIYI: "m_liulang2", Youku: "id_liulang2", Tencent: "cover_liulang2"},
	},
	{
		Title: "热辣滚烫", Aliases: []string{"热辣滚烫", "贾玲"}, Kind: "movie", Year: 2024, Score: 7.8,
		Description: "贾玲执导并主演的励志喜剧。",
		Platforms: map[Platform]string{IQIYI: "m_rela", Youku: "id_rela", Tencent: "cover_rela"},
	},
	{
		Title: "长相思", Aliases: []string{"长相思", "古装", "仙侠"}, Kind: "series", Year: 2023, Score: 7.5,
		Episodes: "39集全", Description: "古装仙侠爱情剧，杨紫、张晚意主演。",
		Platforms: map[Platform]string{Tencent: "cover_changxiangsi", Youku: "id_changxiangsi"},
	},
	{
		Title: "庆余年第二季", Aliases: []string{"庆余年", "庆余年2", "范闲"}, Kind: "series", Year: 2024, Score: 8.1,
		Episodes: "36集全", Description: "权谋喜剧续季，张若昀主演。",
		Platforms: map[Platform]string{Tencent: "cover_qingyunian2", IQIYI: "a_qingyunian2"},
	},
	{
		Title: "奥本海默", Aliases: []string{"奥本海默", "Oppenheimer", "诺兰"}, Kind: "movie", Year: 2023, Score: 8.8,
		Description: "克里斯托弗·诺兰传记片。",
		Platforms: map[Platform]string{IQIYI: "m_oppenheimer", Youku: "id_oppenheimer", Tencent: "cover_oppenheimer"},
	},
	{
		Title: "去有风的地方", Aliases: []string{"去有风的地方", "刘亦菲", "云南"}, Kind: "series", Year: 2023, Score: 7.9,
		Episodes: "40集全", Description: "都市治愈剧，刘亦菲、李现主演。",
		Platforms: map[Platform]string{IQIYI: "a_youfeng", Youku: "id_youfeng"},
	},
	{
		Title: "封神第一部", Aliases: []string{"封神", "封神第一部", "神话"}, Kind: "movie", Year: 2023, Score: 7.6,
		Description: "乌尔善执导神话史诗电影。",
		Platforms: map[Platform]string{Tencent: "cover_fengshen", Youku: "id_fengshen", IQIYI: "m_fengshen"},
	},
}

var platformMeta = map[Platform]struct {
	Name string
	Host string
}{
	IQIYI:   {Name: "爱奇艺", Host: "https://www.iqiyi.com"},
	Youku:   {Name: "优酷", Host: "https://www.youku.com"},
	Tencent: {Name: "腾讯视频", Host: "https://v.qq.com"},
}

// Search calls the encapsulated platform search API (demo catalog match).
func Search(platform Platform, query string, limit int) ([]Item, error) {
	if _, ok := platformMeta[platform]; !ok {
		return nil, fmt.Errorf("unsupported platform %q", platform)
	}
	query = NormalizeQuery(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var hits []Item
	q := strings.ToLower(query)
	for _, e := range catalog {
		path, ok := e.Platforms[platform]
		if !ok {
			continue
		}
		if !matchEntry(e, q) {
			continue
		}
		meta := platformMeta[platform]
		hits = append(hits, Item{
			Platform:    platform,
			Title:       e.Title,
			Kind:        e.Kind,
			Year:        e.Year,
			Score:       e.Score,
			Episodes:    e.Episodes,
			Description: e.Description,
			URL:         meta.Host + "/demo/" + path,
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

// NormalizeQuery maps recommendation / browse phrases onto catalog keywords.
// e.g. "最近有什么好看电视剧推荐" → "电视剧".
func NormalizeQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	lower := strings.ToLower(query)

	// Exact / short keywords keep as-is after lower for English.
	switch query {
	case "电影", "电视剧", "剧", "影视":
		return query
	}
	switch lower {
	case "movie", "movies", "series", "tv", "drama":
		return lower
	}

	wantsSeries := containsAny(query, "电视剧", "连续剧", "追剧", "剧集") ||
		containsAny(lower, "series", "drama", "tv show", "tv series")
	wantsMovie := containsAny(query, "电影", "影片") || containsAny(lower, "movie", "film")
	browse := containsAny(query, "推荐", "好看", "热门", "最近", "近期", "有什么", "榜", "流行") ||
		containsAny(lower, "recommend", "popular", "trending", "best")

	if browse {
		switch {
		case wantsSeries && !wantsMovie:
			return "电视剧"
		case wantsMovie && !wantsSeries:
			return "电影"
		case wantsSeries && wantsMovie:
			return "影视"
		default:
			// 「有什么好看的」等未指明类型 → 全库热门
			return "热门"
		}
	}

	// Soften long natural-language queries that still mention a kind.
	if wantsSeries && !looksLikeTitleQuery(query) {
		return "电视剧"
	}
	if wantsMovie && !looksLikeTitleQuery(query) {
		return "电影"
	}
	return query
}

// SearchAll queries iQIYI, Youku, and Tencent Video and merges results.
func SearchAll(query string, limitPerPlatform int) ([]Item, error) {
	if limitPerPlatform <= 0 {
		limitPerPlatform = 3
	}
	var all []Item
	for _, p := range []Platform{IQIYI, Youku, Tencent} {
		hits, err := Search(p, query, limitPerPlatform)
		if err != nil {
			return nil, err
		}
		all = append(all, hits...)
	}
	return all, nil
}

// FormatItems renders search hits for agent replies.
func FormatItems(platform Platform, query string, items []Item) string {
	meta, ok := platformMeta[platform]
	name := string(platform)
	if ok {
		name = meta.Name
	}
	if len(items) == 0 {
		return fmt.Sprintf("%s 搜索「%s」：暂无结果（demo API）。", name, query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s 搜索「%s」（demo API，共 %d 条）：\n", name, query, len(items))
	for i, it := range items {
		kindCN := "电影"
		if it.Kind == "series" {
			kindCN = "电视剧"
		}
		fmt.Fprintf(&b, "%d. [%s] %s（%d，%.1f分", i+1, kindCN, it.Title, it.Year, it.Score)
		if it.Episodes != "" {
			fmt.Fprintf(&b, "，%s", it.Episodes)
		}
		fmt.Fprintf(&b, "）— %s\n   %s\n", it.Description, it.URL)
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatAll renders cross-platform search hits.
func FormatAll(query string, items []Item) string {
	if len(items) == 0 {
		return fmt.Sprintf("全平台搜索「%s」：暂无结果（爱奇艺 / 优酷 / 腾讯视频 demo API）。", query)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "全平台搜索「%s」（爱奇艺 / 优酷 / 腾讯视频 demo API，共 %d 条）：\n", query, len(items))
	for i, it := range items {
		meta := platformMeta[it.Platform]
		kindCN := "电影"
		if it.Kind == "series" {
			kindCN = "电视剧"
		}
		fmt.Fprintf(&b, "%d. [%s·%s] %s（%d，%.1f分", i+1, meta.Name, kindCN, it.Title, it.Year, it.Score)
		if it.Episodes != "" {
			fmt.Fprintf(&b, "，%s", it.Episodes)
		}
		fmt.Fprintf(&b, "）— %s\n   %s\n", it.Description, it.URL)
	}
	return strings.TrimRight(b.String(), "\n")
}

func matchEntry(e catalogEntry, q string) bool {
	switch q {
	case "电影", "movie", "movies", "film":
		return e.Kind == "movie"
	case "电视剧", "剧", "series", "tv", "drama":
		return e.Kind == "series"
	case "影视", "热门", "推荐", "好看", "popular", "trending":
		return true
	}
	fields := []string{e.Title, e.Kind, e.Description}
	fields = append(fields, e.Aliases...)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	// Chinese substring without case fold when query has CJK.
	if hasCJK(q) {
		for _, f := range fields {
			if strings.Contains(f, q) {
				return true
			}
		}
	}
	return false
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if p != "" && strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// looksLikeTitleQuery is true when the text is short enough to be a title/keyword.
func looksLikeTitleQuery(q string) bool {
	r := []rune(strings.TrimSpace(q))
	return len(r) > 0 && len(r) <= 12 && !containsAny(q, "什么", "哪些", "推荐", "有没有", "帮我")
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
