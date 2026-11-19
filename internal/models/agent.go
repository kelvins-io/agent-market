package models

import "time"

// AgentRecord is the market registry entry for a discoverable A2A agent.
type AgentRecord struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags"`
	Lat         float64                `json:"lat"`
	Lng         float64                `json:"lng"`
	A2AURL      string                 `json:"a2a_url"`
	HandlerPath string                 `json:"handler_path"`
	AgentCard   map[string]interface{} `json:"agent_card,omitempty"`
	Status      string                 `json:"status"` // online | offline
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AgentWithDistance is a discovery result with haversine distance attached.
type AgentWithDistance struct {
	AgentRecord
	DistanceKm float64 `json:"distance_km"`
}

// RegisterRequest is the payload agents POST to self-register / heartbeat.
type RegisterRequest struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags"`
	Lat         float64                `json:"lat"`
	Lng         float64                `json:"lng"`
	A2AURL      string                 `json:"a2a_url"`
	HandlerPath string                 `json:"handler_path"`
	AgentCard   map[string]interface{} `json:"agent_card,omitempty"`
}

// DiscoverQuery holds multi-dimensional discovery filters.
type DiscoverQuery struct {
	Lat       float64
	Lng       float64
	RadiusKm  float64
	Category  string
	Tags      []string
	Q         string
	OnlineOnly bool
}

// ChatRequest is a human message to the market host or a specific agent.
type ChatRequest struct {
	Message string `json:"message"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
}

// A2ASendRequest proxies a text message to an agent's A2A endpoint.
type A2ASendRequest struct {
	Message string `json:"message"`
}
