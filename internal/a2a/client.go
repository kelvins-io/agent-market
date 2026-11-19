package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	a2av2 "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
	"github.com/cloudwego/eino-ext/a2a/client"
	einox "github.com/cloudwego/eino-ext/a2a/extension/eino"
	"github.com/cloudwego/eino-ext/a2a/transport/jsonrpc"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const bearerScheme = a2av2.SecuritySchemeName("bearer")

// SendOptions configures an outbound A2A text turn.
type SendOptions struct {
	BaseURL     string
	HandlerPath string
	Message     string
	// AgentCard is the registry-stored card (used to pick protocol / takeout intents).
	AgentCard map[string]any
	// BearerToken overrides A2A_BEARER_TOKEN when non-empty.
	BearerToken string
}

// SendText sends a user text message to a remote A2A agent and returns the final reply text.
// a2a-go v2 agents (protocol 1.0, e.g. takeout-agent) use SendMessage + optional Bearer;
// older eino-ext sample agents keep the legacy transport.
func SendText(ctx context.Context, opt SendOptions) (string, error) {
	if opt.HandlerPath == "" {
		opt.HandlerPath = "/"
	}
	if prefersV2(opt.AgentCard) {
		return sendV2(ctx, opt)
	}
	reply, err := sendLegacy(ctx, opt)
	if err == nil {
		return reply, nil
	}
	if looksLikeProtocolMismatch(err) {
		return sendV2(ctx, opt)
	}
	return "", err
}

func looksLikeProtocolMismatch(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "method not found") ||
		strings.Contains(s, "version not supported") ||
		strings.Contains(s, "version_not_supported")
}

// prefersV2 reports whether the stored agent card looks like a2a-go v2 / protocol 1.x.
func prefersV2(card map[string]any) bool {
	if card == nil {
		return false
	}
	if v, ok := card["protocolVersion"].(string); ok && isV1Protocol(v) {
		return true
	}
	if ifaces, ok := card["supportedInterfaces"].([]any); ok {
		for _, raw := range ifaces {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if v, ok := m["protocolVersion"].(string); ok && isV1Protocol(v) {
				return true
			}
		}
	}
	if _, ok := card["securitySchemes"]; ok {
		return true
	}
	if _, ok := card["securityRequirements"]; ok {
		return true
	}
	return false
}

func isV1Protocol(v string) bool {
	v = strings.TrimSpace(v)
	return strings.HasPrefix(v, "1.") || v == "1"
}

func bearerToken(opt SendOptions) string {
	if t := strings.TrimSpace(opt.BearerToken); t != "" {
		return t
	}
	return strings.TrimSpace(os.Getenv("A2A_BEARER_TOKEN"))
}

func sendV2(ctx context.Context, opt SendOptions) (string, error) {
	base := strings.TrimRight(opt.BaseURL, "/")
	httpClient := &http.Client{Timeout: 45 * time.Second}

	card, err := agentcard.DefaultResolver.Resolve(ctx, base)
	if err != nil {
		// Fall back to constructing a minimal card from registry URL.
		invoke := base + opt.HandlerPath
		card = &a2av2.AgentCard{
			Name: "remote",
			SupportedInterfaces: []*a2av2.AgentInterface{
				a2av2.NewAgentInterface(invoke, a2av2.TransportProtocolJSONRPC),
			},
		}
		if needsBearer(opt.AgentCard) || bearerToken(opt) != "" {
			card.SecuritySchemes = a2av2.NamedSecuritySchemes{
				bearerScheme: a2av2.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "opaque"},
			}
			card.SecurityRequirements = []a2av2.SecurityRequirements{{bearerScheme: {}}}
		}
		// Prefer registry card fields for takeout intent detection.
		if opt.AgentCard != nil {
			_ = mergeCardHints(card, opt.AgentCard)
		}
	}

	// Point the interface URL at the registry-advertised base (docker DNS / host.docker.internal).
	rewriteInterfaceURL(card, base, opt.HandlerPath)

	factoryOpts := []a2aclient.FactoryOption{
		a2aclient.WithJSONRPCTransport(httpClient),
	}
	token := bearerToken(opt)
	if token != "" && (len(card.SecuritySchemes) > 0 || needsBearer(opt.AgentCard)) {
		sessionID := a2aclient.SessionID("agent-market")
		credStore := a2aclient.NewInMemoryCredentialsStore()
		credStore.Set(sessionID, bearerScheme, a2aclient.AuthCredential(token))
		factoryOpts = append(factoryOpts, a2aclient.WithCallInterceptors(&a2aclient.AuthInterceptor{Service: credStore}))
		ctx = a2aclient.AttachSessionID(ctx, sessionID)
		if len(card.SecuritySchemes) == 0 {
			card.SecuritySchemes = a2av2.NamedSecuritySchemes{
				bearerScheme: a2av2.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "opaque"},
			}
			card.SecurityRequirements = []a2av2.SecurityRequirements{{bearerScheme: {}}}
		}
	}

	cli, err := a2aclient.NewFromCard(ctx, card, factoryOpts...)
	if err != nil {
		return "", fmt.Errorf("a2a v2 client: %w", err)
	}

	msg := buildV2UserMessage(opt.Message, card, opt.AgentCard)
	resp, err := cli.SendMessage(ctx, &a2av2.SendMessageRequest{Message: msg})
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	return formatV2Result(resp)
}

func needsBearer(card map[string]any) bool {
	if card == nil {
		return false
	}
	_, a := card["securitySchemes"]
	_, b := card["securityRequirements"]
	return a || b
}

func mergeCardHints(dst *a2av2.AgentCard, src map[string]any) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	var tmp a2av2.AgentCard
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return err
	}
	if tmp.Name != "" {
		dst.Name = tmp.Name
	}
	if len(tmp.Skills) > 0 {
		dst.Skills = tmp.Skills
	}
	if len(tmp.SecuritySchemes) > 0 {
		dst.SecuritySchemes = tmp.SecuritySchemes
		dst.SecurityRequirements = tmp.SecurityRequirements
	}
	return nil
}

func rewriteInterfaceURL(card *a2av2.AgentCard, base, handlerPath string) {
	if card == nil {
		return
	}
	invoke := strings.TrimRight(base, "/") + handlerPath
	if len(card.SupportedInterfaces) == 0 {
		card.SupportedInterfaces = []*a2av2.AgentInterface{
			a2av2.NewAgentInterface(invoke, a2av2.TransportProtocolJSONRPC),
		}
		return
	}
	// Keep protocol metadata; force URL to the reachable registry address.
	for _, iface := range card.SupportedInterfaces {
		if iface == nil {
			continue
		}
		iface.URL = invoke
		if iface.ProtocolVersion == "" {
			iface.ProtocolVersion = a2av2.Version
		}
	}
}

func buildV2UserMessage(message string, card *a2av2.AgentCard, cardMap map[string]any) *a2av2.Message {
	message = strings.TrimSpace(message)
	if intent, payload, ok := inferTakeoutIntent(message, card, cardMap); ok {
		msg := a2av2.NewMessage(
			a2av2.MessageRoleUser,
			a2av2.NewTextPart("intent="+intent),
			a2av2.NewDataPart(map[string]any{
				"intent":   intent,
				"from_bot": "agent-market",
				"payload":  payload,
			}),
		)
		msg.SetMeta("takeout.a2a.intent", intent)
		return msg
	}
	return a2av2.NewMessage(a2av2.MessageRoleUser, a2av2.NewTextPart(message))
}

func isTakeoutMerchant(card *a2av2.AgentCard, cardMap map[string]any) bool {
	if card != nil {
		for _, sk := range card.Skills {
			if sk.ID == "takeout.merchant" {
				return true
			}
			for _, ex := range sk.Examples {
				if ex == "menu.search" {
					return true
				}
			}
			for _, t := range sk.Tags {
				if t == "merchant" || t == "外卖" {
					return true
				}
			}
		}
	}
	if cardMap == nil {
		return false
	}
	if skills, ok := cardMap["skills"].([]any); ok {
		for _, raw := range skills {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := m["id"].(string); id == "takeout.merchant" {
				return true
			}
			if name, _ := m["name"].(string); name == "外卖商家" {
				return true
			}
		}
	}
	return false
}

// inferTakeoutIntent maps free-text host messages onto takeout A2A intents.
func inferTakeoutIntent(message string, card *a2av2.AgentCard, cardMap map[string]any) (intent string, payload map[string]any, ok bool) {
	if !isTakeoutMerchant(card, cardMap) {
		return "", nil, false
	}
	payload = map[string]any{}
	if strings.HasPrefix(message, "intent=") {
		intent = strings.TrimSpace(strings.TrimPrefix(message, "intent="))
		if i := strings.IndexAny(intent, " \n\t"); i > 0 {
			intent = intent[:i]
		}
		if intent == "" {
			intent = "handshake"
		}
		return intent, payload, true
	}
	msg := stripHostLocationSuffix(message)
	lower := strings.ToLower(msg)
	switch {
	case containsAny(msg, "菜单", "有什么菜", "今日菜", "今天有什么", "吃什么", "搜菜", "菜品") ||
		strings.Contains(lower, "menu"):
		payload["query"] = menuSearchQuery(msg)
		return "menu.search", payload, true
	case containsAny(msg, "询价", "多少钱", "报价") || strings.Contains(lower, "quote"):
		return "quote.inquire", payload, true
	case containsAny(msg, "下单", "点餐") || strings.Contains(lower, "order"):
		return "order.create", payload, true
	case containsAny(msg, "偏好", "口味", "交换"):
		return "preference.exchange", payload, true
	default:
		// Default merchant free-text to a full-menu browse (empty query).
		payload["query"] = menuSearchQuery(msg)
		return "menu.search", payload, true
	}
}

func stripHostLocationSuffix(s string) string {
	if i := strings.Index(s, "（地点："); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	if i := strings.Index(s, "(地点:"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// menuSearchQuery turns browse-style prompts into an empty SearchMenu query
// so the merchant returns the full catalog; keeps specific dish keywords.
func menuSearchQuery(msg string) string {
	browse := []string{"菜单", "有什么菜", "今日菜", "今天有什么", "吃什么", "搜菜", "菜品", "menu", "今天的菜"}
	for _, b := range browse {
		if strings.EqualFold(strings.TrimSpace(msg), b) || msg == b {
			return ""
		}
	}
	if containsAny(msg, browse...) {
		// "今天有什么菜" / "看看菜单" — no concrete dish token → full list.
		dishHints := []string{"水煮", "回锅", "豆腐", "鱼", "肉", "饭", "可乐", "辣", "素", "川菜"}
		if !containsAny(msg, dishHints...) {
			return ""
		}
	}
	return msg
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func formatV2Result(resp a2av2.SendMessageResult) (string, error) {
	switch v := resp.(type) {
	case *a2av2.Message:
		return formatV2Message(v), nil
	case *a2av2.Task:
		parts := []string{fmt.Sprintf("task=%s state=%s", v.ID, v.Status.State)}
		if v.Status.Message != nil {
			parts = append(parts, formatV2Message(v.Status.Message))
		}
		return strings.Join(parts, "\n"), nil
	default:
		b, _ := json.Marshal(resp)
		if len(b) == 0 {
			return "", fmt.Errorf("empty response from a2a v2 agent")
		}
		return string(b), nil
	}
}

func formatV2Message(msg *a2av2.Message) string {
	if msg == nil {
		return ""
	}
	var texts []string
	var dataParts []any
	for _, p := range msg.Parts {
		if t := p.Text(); t != "" {
			texts = append(texts, t)
		}
		if d := p.Data(); d != nil {
			dataParts = append(dataParts, d)
		}
	}
	var b strings.Builder
	if len(texts) > 0 {
		b.WriteString(strings.Join(texts, "\n"))
	}
	if len(dataParts) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		enc, _ := json.MarshalIndent(dataParts, "", "  ")
		b.Write(enc)
	}
	if b.Len() == 0 {
		return "(empty message)"
	}
	return b.String()
}

func sendLegacy(ctx context.Context, opt SendOptions) (string, error) {
	streaming := false
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     opt.BaseURL,
		HandlerPath: opt.HandlerPath,
	})
	if err != nil {
		return "", fmt.Errorf("a2a transport: %w", err)
	}
	cli, err := client.NewA2AClient(ctx, &client.Config{Transport: t})
	if err != nil {
		return "", fmt.Errorf("a2a client: %w", err)
	}
	agent, err := einox.NewAgent(ctx, einox.AgentConfig{
		Client:    cli,
		Streaming: &streaming,
	})
	if err != nil {
		return "", fmt.Errorf("a2a agent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Run(ctx, []adk.Message{schema.UserMessage(opt.Message)})

	var parts []string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mo := event.Output.MessageOutput
		if mo.Message != nil && mo.Message.Content != "" && mo.Message.Role != schema.Tool {
			parts = append(parts, mo.Message.Content)
		}
		if mo.MessageStream != nil {
			var b strings.Builder
			for {
				chunk, err := mo.MessageStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return "", err
				}
				if chunk != nil && chunk.Content != "" && chunk.Role != schema.Tool {
					b.WriteString(chunk.Content)
				}
			}
			if s := b.String(); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty response from agent at %s (check MOCK_LLM=1 or reachable OPENAI_BASE_URL / API key)", opt.BaseURL)
	}
	return strings.Join(parts, "\n"), nil
}

// FetchAgentCardJSON fetches the remote agent card via HTTP well-known or legacy A2A client.
func FetchAgentCardJSON(ctx context.Context, baseURL, handlerPath string) (map[string]interface{}, error) {
	base := strings.TrimRight(baseURL, "/")
	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/.well-known/agent-card.json", nil)
	if err == nil {
		resp, err := httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var out map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&out); err == nil {
					return out, nil
				}
			}
		}
	}

	if handlerPath == "" {
		handlerPath = "/"
	}
	t, err := jsonrpc.NewTransport(ctx, &jsonrpc.ClientConfig{
		BaseURL:     baseURL,
		HandlerPath: handlerPath,
	})
	if err != nil {
		return nil, err
	}
	cli, err := client.NewA2AClient(ctx, &client.Config{Transport: t})
	if err != nil {
		return nil, err
	}
	card, err := cli.AgentCard(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":        card.Name,
		"description": card.Description,
		"url":         card.URL,
		"version":     card.Version,
		"skills":      card.Skills,
	}, nil
}
