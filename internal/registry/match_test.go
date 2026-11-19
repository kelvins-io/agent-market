package registry

import (
	"testing"

	"github.com/kelvins-io/agent-market/internal/models"
)

func TestMatchKeywordSynonym(t *testing.T) {
	a := models.AgentRecord{
		Name:        "WeatherAgent",
		Description: "Provides weather forecasts",
		Tags:        []string{"weather", "forecast"},
	}
	if !MatchKeyword(a, "附近的天气") {
		t.Fatal("expected Chinese weather query to match")
	}
	if MatchKeyword(a, "翻译") {
		t.Fatal("weather agent should not match translate")
	}

	media := models.AgentRecord{
		Name:        "MediaTVAgent",
		Description: "Search movies and TV series",
		Category:    "entertainment",
		Tags:        []string{"movie", "tv", "series"},
	}
	if !MatchKeyword(media, "附近的电影") {
		t.Fatal("expected Chinese movie query to match media agent")
	}
	if !MatchKeyword(media, "电视剧") {
		t.Fatal("expected 电视剧 synonym to match media agent")
	}
}
