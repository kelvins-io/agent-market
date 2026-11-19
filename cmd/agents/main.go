package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/cloudwego/eino/adk"
	"gopkg.in/yaml.v3"

	"github.com/kelvins-io/agent-market/agents/localguide"
	"github.com/kelvins-io/agent-market/agents/mediatv"
	"github.com/kelvins-io/agent-market/agents/translator"
	"github.com/kelvins-io/agent-market/agents/weather"
	"github.com/kelvins-io/agent-market/internal/agentboot"
)

type config struct {
	MarketURL     string      `yaml:"market_url"`
	AdvertiseHost string      `yaml:"advertise_host"`
	Agents        []agentConf `yaml:"agents"`
}

type agentConf struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Category    string   `yaml:"category"`
	Tags        []string `yaml:"tags"`
	Lat         float64  `yaml:"lat"`
	Lng         float64  `yaml:"lng"`
	Port        int      `yaml:"port"`
	HandlerPath string   `yaml:"handler_path"`
}

func main() {
	cfgPath := flag.String("config", "configs/agents.yaml", "agents config")
	flag.Parse()

	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("MOCK_LLM") == "" {
		log.Fatal("set OPENAI_API_KEY or MOCK_LLM=1 to run sample agents")
	}

	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var cfg config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	if v := os.Getenv("MARKET_URL"); v != "" {
		cfg.MarketURL = v
	}
	if v := os.Getenv("ADVERTISE_HOST"); v != "" {
		cfg.AdvertiseHost = v
	}
	if cfg.MarketURL == "" {
		cfg.MarketURL = "http://127.0.0.1:13100"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	for _, ac := range cfg.Agents {
		agent, err := buildAgent(ctx, ac.ID)
		if err != nil {
			log.Fatalf("build agent %s: %v", ac.ID, err)
		}
		spec := agentboot.Spec{
			ID:            ac.ID,
			Name:          ac.Name,
			Description:   ac.Description,
			Category:      ac.Category,
			Tags:          ac.Tags,
			Lat:           ac.Lat,
			Lng:           ac.Lng,
			Port:          ac.Port,
			HandlerPath:   ac.HandlerPath,
			MarketURL:     cfg.MarketURL,
			AdvertiseHost: cfg.AdvertiseHost,
			Agent:         agent,
		}
		wg.Add(1)
		go func(spec agentboot.Spec) {
			defer wg.Done()
			if err := agentboot.Run(ctx, spec); err != nil {
				log.Printf("[%s] stopped: %v", spec.ID, err)
			}
		}(spec)
	}

	<-ctx.Done()
	log.Printf("shutting down sample agents...")
	wg.Wait()
}

func buildAgent(ctx context.Context, id string) (adk.Agent, error) {
	switch id {
	case "weather":
		return weather.New(ctx)
	case "localguide":
		return localguide.New(ctx)
	case "translator":
		return translator.New(ctx)
	case "mediatv":
		return mediatv.New(ctx)
	default:
		return nil, fmt.Errorf("unknown agent id %q", id)
	}
}
