package discovery

import (
	"context"
	"sort"
	"strings"

	"github.com/kelvins-io/agent-market/internal/geo"
	"github.com/kelvins-io/agent-market/internal/models"
	"github.com/kelvins-io/agent-market/internal/registry"
)

// Service discovers agents by tags, category, keyword, and geographic distance.
type Service struct {
	store *registry.Store
}

func New(store *registry.Store) *Service {
	return &Service{store: store}
}

// Discover applies multi-dimensional filters and sorts by distance ascending.
func (s *Service) Discover(ctx context.Context, q models.DiscoverQuery) ([]models.AgentWithDistance, error) {
	if q.RadiusKm <= 0 {
		q.RadiusKm = 50
	}
	agents, err := s.store.ListForDiscover(ctx, true)
	if err != nil {
		return nil, err
	}

	wantTags := normalizeTags(q.Tags)
	var results []models.AgentWithDistance
	for _, a := range agents {
		if q.Category != "" && !strings.EqualFold(a.Category, q.Category) {
			continue
		}
		if len(wantTags) > 0 && !hasAnyTag(a.Tags, wantTags) {
			continue
		}
		if !registry.MatchKeyword(a, q.Q) {
			continue
		}
		d := geo.DistanceKm(q.Lat, q.Lng, a.Lat, a.Lng)
		if d > q.RadiusKm {
			continue
		}
		results = append(results, models.AgentWithDistance{
			AgentRecord: a,
			DistanceKm:  round2(d),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DistanceKm < results[j].DistanceKm
	})
	return results, nil
}

func normalizeTags(tags []string) []string {
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// hasAnyTag is true when the agent has at least one of the requested tags.
func hasAnyTag(have, want []string) bool {
	set := map[string]struct{}{}
	for _, t := range have {
		set[strings.ToLower(t)] = struct{}{}
	}
	for _, t := range want {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
