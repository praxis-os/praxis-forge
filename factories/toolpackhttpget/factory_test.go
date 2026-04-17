// SPDX-License-Identifier: Apache-2.0

package toolpackhttpget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxis-os/praxis/tools"
)

func TestTool_Fetches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	host := srv.Listener.Addr().String()

	pack, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{
		"allowedHosts": []any{host},
		"timeoutMs":    2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"url": srv.URL})
	res, err := pack.Invoker.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "http_get", ArgumentsJSON: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "hello" {
		t.Fatalf("got=%q", res.Content)
	}
}

func TestTool_BlocksDisallowedHost(t *testing.T) {
	pack, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{
		"allowedHosts": []any{"example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"url": "http://evil.example/"})
	res, _ := pack.Invoker.Invoke(context.Background(), tools.InvocationContext{}, tools.ToolCall{Name: "http_get", ArgumentsJSON: args})
	if res.Status != tools.ToolStatusError {
		t.Fatalf("status=%v", res.Status)
	}
}

func TestFactory_RequiresAllowedHosts(t *testing.T) {
	_, err := NewFactory("toolpack.http-get@1.0.0").Build(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
