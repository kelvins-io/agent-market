package agentboot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/a2a/extension/eino"
	a2amodels "github.com/cloudwego/eino-ext/a2a/models"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	hertz_server "github.com/cloudwego/hertz/pkg/app/server"

	"github.com/kelvins-io/agent-market/internal/models"
)

// Spec describes one sample agent process.
type Spec struct {
	ID          string
	Name        string
	Description string
	Category    string
	Tags        []string
	Lat         float64
	Lng         float64
	Port        int
	HandlerPath string
	MarketURL   string
	// AdvertiseHost is the hostname/IP other services use to reach this agent
	// (e.g. Docker Compose service name). Empty defaults to 127.0.0.1.
	AdvertiseHost string
	Agent         adk.Agent
	Skills        []a2amodels.AgentSkill
}

// Run starts Hertz + A2A handlers, registers with the market, and blocks.
func Run(ctx context.Context, spec Spec) error {
	if spec.HandlerPath == "" {
		spec.HandlerPath = "/a2a"
	}
	host := spec.AdvertiseHost
	if host == "" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf(":%d", spec.Port)
	baseURL := fmt.Sprintf("http://%s:%d", host, spec.Port)

	h := hertz_server.Default(hertz_server.WithHostPorts(addr))
	reg, err := jsonrpc.NewRegistrar(ctx, &jsonrpc.ServerConfig{
		Router:      h,
		HandlerPath: spec.HandlerPath,
	})
	if err != nil {
		return fmt.Errorf("a2a registrar: %w", err)
	}

	desc := spec.Description
	skills := spec.Skills
	if len(skills) == 0 {
		skills = []a2amodels.AgentSkill{{
			ID:          spec.ID + "-skill",
			Name:        spec.Name,
			Description: &desc,
			Tags:        spec.Tags,
		}}
	}

	err = eino.RegisterServerHandlers(ctx, spec.Agent, &eino.ServerConfig{
		Registrar:          reg,
		URL:                baseURL + spec.HandlerPath,
		Version:            "1.0.0",
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             skills,
		CheckPointStore:    &memStore{},
	})
	if err != nil {
		return fmt.Errorf("register a2a handlers: %w", err)
	}

	go func() {
		time.Sleep(400 * time.Millisecond)
		if err := registerLoop(ctx, spec, baseURL); err != nil {
			log.Printf("[%s] register error: %v", spec.ID, err)
		}
	}()

	log.Printf("[%s] A2A listening on %s%s", spec.ID, baseURL, spec.HandlerPath)
	return h.Run()
}

func registerLoop(ctx context.Context, spec Spec, baseURL string) error {
	card := map[string]interface{}{
		"name":        spec.Name,
		"description": spec.Description,
		"url":         baseURL + spec.HandlerPath,
		"version":     "1.0.0",
	}
	req := models.RegisterRequest{
		ID:          spec.ID,
		Name:        spec.Name,
		Description: spec.Description,
		Category:    spec.Category,
		Tags:        spec.Tags,
		Lat:         spec.Lat,
		Lng:         spec.Lng,
		A2AURL:      baseURL,
		HandlerPath: spec.HandlerPath,
		AgentCard:   card,
	}

	for attempt := 1; ; attempt++ {
		if err := postRegister(spec.MarketURL, req); err != nil {
			log.Printf("[%s] register attempt %d failed: %v", spec.ID, attempt, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}
		log.Printf("[%s] registered at market %s (%s)", spec.ID, spec.MarketURL, baseURL)
		break
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := postRegister(spec.MarketURL, req); err != nil {
				log.Printf("[%s] heartbeat failed: %v", spec.ID, err)
			}
		}
	}
}

func postRegister(marketURL string, req models.RegisterRequest) error {
	body, _ := json.Marshal(req)
	resp, err := http.Post(marketURL+"/api/v1/agents/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("register status %d", resp.StatusCode)
	}
	return nil
}

type memStore struct {
	m sync.Map
}

func (i *memStore) Get(_ context.Context, checkPointID string) ([]byte, bool, error) {
	v, ok := i.m.Load(checkPointID)
	if !ok {
		return nil, false, nil
	}
	return v.([]byte), true, nil
}

func (i *memStore) Set(_ context.Context, checkPointID string, checkPoint []byte) error {
	i.m.Store(checkPointID, checkPoint)
	return nil
}

var _ compose.CheckPointStore = (*memStore)(nil)
