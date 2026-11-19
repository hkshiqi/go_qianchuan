package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitializeAndToolsList(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	serve(strings.NewReader(input), &output, &gateway{})

	decoder := json.NewDecoder(&output)
	var initialize rpcResponse
	if err := decoder.Decode(&initialize); err != nil {
		t.Fatal(err)
	}
	if initialize.Error != nil {
		t.Fatalf("initialize failed: %#v", initialize.Error)
	}

	var toolsList struct {
		Tools []tool `json:"tools"`
	}
	var listResponse struct {
		Result json.RawMessage `json:"result"`
	}
	if err := decoder.Decode(&listResponse); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(listResponse.Result, &toolsList); err != nil {
		t.Fatal(err)
	}
	if len(toolsList.Tools) < 6 {
		t.Fatalf("expected read-only tools, got %d", len(toolsList.Tools))
	}
}

func TestToolCallUsesServerSideAPIKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-MCP-Key"); got != "test-key" {
			t.Fatalf("unexpected API key: %q", got)
		}
		if r.URL.Path != "/plans/123/materials" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("material_type"); got != "VIDEO" {
			t.Fatalf("unexpected material_type: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"data":{"materials":[]}}`)
	}))
	defer upstream.Close()

	g := &gateway{
		baseURL:    upstream.URL,
		apiKey:     "test-key",
		httpClient: upstream.Client(),
	}
	result, err := g.callTool("qianchuan_list_plan_materials", map[string]any{
		"ad_id":         float64(123),
		"material_type": "VIDEO",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"code":0`) {
		t.Fatalf("unexpected result: %s", encoded)
	}
}

func TestUnknownToolReturnsMCPErrorContent(t *testing.T) {
	request := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"missing","arguments":{}}`),
	}
	result, rerr := dispatch(&request, &gateway{})
	if rerr != nil {
		t.Fatalf("unexpected RPC error: %#v", rerr)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"isError":true`) {
		t.Fatalf("expected tool error result: %s", payload)
	}
}
