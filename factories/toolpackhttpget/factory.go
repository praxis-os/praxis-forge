// SPDX-License-Identifier: Apache-2.0

// Package toolpackhttpget exposes a single http_get(url) tool that fetches a
// URL over HTTP(S). Host allowlist is enforced at config time; all other size
// governance is the post-tool filter's responsibility.
package toolpackhttpget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/praxis-os/praxis-forge/registry"
	"github.com/praxis-os/praxis/llm"
	"github.com/praxis-os/praxis/tools"
)

type Factory struct{ id registry.ID }

func NewFactory(id registry.ID) *Factory { return &Factory{id: id} }

func (f *Factory) ID() registry.ID     { return f.id }
func (f *Factory) Description() string { return "single http_get tool with host allowlist" }

type cfgShape struct {
	AllowedHosts []string
	Timeout      time.Duration
}

func decode(cfg map[string]any) (cfgShape, error) {
	var c cfgShape
	raw, ok := cfg["allowedHosts"]
	if !ok {
		return c, fmt.Errorf("allowedHosts: required")
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return c, fmt.Errorf("allowedHosts: must be non-empty list")
	}
	for i, h := range list {
		s, ok := h.(string)
		if !ok {
			return c, fmt.Errorf("allowedHosts[%d]: want string, got %T", i, h)
		}
		c.AllowedHosts = append(c.AllowedHosts, s)
	}
	c.Timeout = 5 * time.Second
	if raw, ok := cfg["timeoutMs"]; ok {
		n, err := toInt(raw)
		if err != nil {
			return c, fmt.Errorf("timeoutMs: %w", err)
		}
		c.Timeout = time.Duration(n) * time.Millisecond
	}
	return c, nil
}

func toInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case float64:
		return int(x), nil
	default:
		return 0, fmt.Errorf("want int, got %T", v)
	}
}

func (f *Factory) Build(_ context.Context, cfg map[string]any) (registry.ToolPack, error) {
	c, err := decode(cfg)
	if err != nil {
		return registry.ToolPack{}, fmt.Errorf("%s: %w", f.id, err)
	}
	inv := &invoker{
		client:       &http.Client{Timeout: c.Timeout},
		allowedHosts: c.AllowedHosts,
	}
	def := llm.ToolDefinition{
		Name:        "http_get",
		Description: "Fetch the body of a URL via HTTP GET. Only allow-listed hosts are reachable.",
		InputSchema: []byte(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`),
	}
	desc := registry.ToolDescriptor{
		Name:       "http_get",
		Source:     string(f.id),
		RiskTier:   registry.RiskModerate,
		PolicyTags: []string{"network", "http"},
	}
	return registry.ToolPack{
		Invoker:     inv,
		Definitions: []llm.ToolDefinition{def},
		Descriptors: []registry.ToolDescriptor{desc},
	}, nil
}

type invoker struct {
	client       *http.Client
	allowedHosts []string
}

type args struct {
	URL string `json:"url"`
}

func (i *invoker) Invoke(ctx context.Context, _ tools.InvocationContext, call tools.ToolCall) (tools.ToolResult, error) {
	var a args
	if err := json.Unmarshal(call.ArgumentsJSON, &a); err != nil {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: fmt.Sprintf("invalid args: %s", err)}, nil
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Host == "" {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: "invalid url"}, nil
	}
	if !i.hostAllowed(u.Host) {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: "host not in allowlist: " + u.Host}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: err.Error()}, nil
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: err.Error()}, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tools.ToolResult{Status: tools.ToolStatusError, Content: err.Error()}, nil
	}
	return tools.ToolResult{Status: tools.ToolStatusSuccess, Content: string(body)}, nil
}

func (i *invoker) hostAllowed(host string) bool {
	for _, h := range i.allowedHosts {
		if host == h {
			return true
		}
	}
	return false
}
