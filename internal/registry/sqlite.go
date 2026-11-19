package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kelvins-io/agent-market/internal/models"
)

// Store persists agent registrations in SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database and ensures the schema exists.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '[]',
  lat REAL NOT NULL DEFAULT 0,
  lng REAL NOT NULL DEFAULT 0,
  a2a_url TEXT NOT NULL,
  handler_path TEXT NOT NULL DEFAULT '/',
  agent_card TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'online',
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agents_category ON agents(category);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
`)
	return err
}

// Upsert inserts or updates an agent registration.
func (s *Store) Upsert(ctx context.Context, req models.RegisterRequest) (*models.AgentRecord, error) {
	if req.ID == "" || req.Name == "" || req.A2AURL == "" {
		return nil, fmt.Errorf("id, name, and a2a_url are required")
	}
	if req.HandlerPath == "" {
		req.HandlerPath = "/"
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, err
	}
	card := req.AgentCard
	if card == nil {
		card = map[string]interface{}{}
	}
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO agents (id, name, description, category, tags, lat, lng, a2a_url, handler_path, agent_card, status, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'online', ?)
ON CONFLICT(id) DO UPDATE SET
  name=excluded.name,
  description=excluded.description,
  category=excluded.category,
  tags=excluded.tags,
  lat=excluded.lat,
  lng=excluded.lng,
  a2a_url=excluded.a2a_url,
  handler_path=excluded.handler_path,
  agent_card=excluded.agent_card,
  status='online',
  updated_at=excluded.updated_at
`, req.ID, req.Name, req.Description, req.Category, string(tagsJSON),
		req.Lat, req.Lng, req.A2AURL, req.HandlerPath, string(cardJSON), now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, req.ID)
}

// Get returns one agent by id.
func (s *Store) Get(ctx context.Context, id string) (*models.AgentRecord, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, description, category, tags, lat, lng, a2a_url, handler_path, agent_card, status, updated_at
FROM agents WHERE id = ?`, id)
	return scanAgent(row)
}

// Delete removes an agent registration.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %s not found", id)
	}
	return nil
}

// List returns all agents, optionally online-only.
func (s *Store) List(ctx context.Context, onlineOnly bool) ([]models.AgentRecord, error) {
	q := `SELECT id, name, description, category, tags, lat, lng, a2a_url, handler_path, agent_card, status, updated_at FROM agents`
	if onlineOnly {
		q += ` WHERE status = 'online'`
	}
	q += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgents(rows)
}

// ListForDiscover returns candidates for discovery filtering (status-aware).
func (s *Store) ListForDiscover(ctx context.Context, onlineOnly bool) ([]models.AgentRecord, error) {
	return s.List(ctx, onlineOnly)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanAgent(row scannable) (*models.AgentRecord, error) {
	var (
		r          models.AgentRecord
		tagsJSON   string
		cardJSON   string
		updatedRaw string
	)
	err := row.Scan(&r.ID, &r.Name, &r.Description, &r.Category, &tagsJSON,
		&r.Lat, &r.Lng, &r.A2AURL, &r.HandlerPath, &cardJSON, &r.Status, &updatedRaw)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tagsJSON), &r.Tags)
	if r.Tags == nil {
		r.Tags = []string{}
	}
	_ = json.Unmarshal([]byte(cardJSON), &r.AgentCard)
	if t, err := time.Parse(time.RFC3339Nano, updatedRaw); err == nil {
		r.UpdatedAt = t
	}
	return &r, nil
}

func scanAgents(rows *sql.Rows) ([]models.AgentRecord, error) {
	var out []models.AgentRecord
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// MatchKeyword returns true if q appears in name, description, or tags (case-insensitive).
// Also expands a few Chinese demo synonyms onto English tags/fields.
func MatchKeyword(a models.AgentRecord, q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		a.Name, a.Description, a.Category, strings.Join(a.Tags, " "),
	}, " "))
	if strings.Contains(hay, q) {
		return true
	}
	for _, term := range expandQuery(q) {
		if strings.Contains(hay, term) {
			return true
		}
	}
	return false
}

func expandQuery(q string) []string {
	var out []string
	synonyms := []struct {
		keys []string
		vals []string
	}{
		{[]string{"天气", "气象", "预报"}, []string{"weather", "forecast"}},
		{[]string{"美食", "吃饭", "餐厅", "附近吃"}, []string{"food", "local", "travel"}},
		{[]string{"旅游", "景点", "本地"}, []string{"travel", "local", "food"}},
		{[]string{"翻译", "英文", "中文"}, []string{"translate", "language"}},
		{[]string{"电影", "电视剧", "影视", "爱奇艺", "优酷", "腾讯视频", "追剧"}, []string{"movie", "tv", "series", "iqiyi", "youku", "tencent", "entertainment"}},
	}
	for _, s := range synonyms {
		for _, k := range s.keys {
			if strings.Contains(q, k) {
				out = append(out, s.vals...)
				break
			}
		}
	}
	return out
}
