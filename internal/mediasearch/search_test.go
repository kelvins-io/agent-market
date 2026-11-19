package mediasearch

import "testing"

func TestSearchIQIYI(t *testing.T) {
	hits, err := Search(IQIYI, "狂飙", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits for 狂飙 on iqiyi")
	}
	if hits[0].Platform != IQIYI {
		t.Fatalf("platform = %s", hits[0].Platform)
	}
}

func TestSearchAll(t *testing.T) {
	hits, err := SearchAll("电影", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected movie hits")
	}
	for _, h := range hits {
		if h.Kind != "movie" {
			t.Fatalf("want movie, got %s", h.Kind)
		}
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	if _, err := Search(Youku, "  ", 3); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestNormalizeRecommendQuery(t *testing.T) {
	got := NormalizeQuery("最近有什么好看电视剧推荐")
	if got != "电视剧" {
		t.Fatalf("NormalizeQuery = %q, want 电视剧", got)
	}
	hits, err := SearchAll("最近有什么好看电视剧推荐", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("recommendation query should return series hits")
	}
	for _, h := range hits {
		if h.Kind != "series" {
			t.Fatalf("want series, got %s (%s)", h.Kind, h.Title)
		}
	}
}
