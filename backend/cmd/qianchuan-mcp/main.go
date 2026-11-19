package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const protocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type gateway struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func main() {
	log.SetOutput(os.Stderr)
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("QIANCHUAN_MCP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:18081/api/mcp"
	}
	apiKey := strings.TrimSpace(os.Getenv("MCP_API_KEY"))
	if apiKey == "" {
		log.Fatal("MCP_API_KEY is required")
	}

	serve(os.Stdin, os.Stdout, &gateway{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 40 * time.Second,
		},
	})
}

func serve(input io.Reader, output io.Writer, g *gateway) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error: &rpcError{
					Code:    -32700,
					Message: "Parse error",
				},
			})
			continue
		}
		if len(req.ID) == 0 {
			// MCP notifications do not receive a response.
			continue
		}
		result, rerr := dispatch(&req, g)
		response := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr}
		if err := encoder.Encode(response); err != nil {
			log.Printf("write response failed: %v", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("read stdio failed: %v", err)
	}
}

func dispatch(req *rpcRequest, g *gateway) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "qianchuan-operation-mcp",
				"version": "0.1.0",
			},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "shutdown":
		return nil, nil
	case "tools/list":
		return map[string]any{"tools": availableTools()}, nil
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid tools/call params"}
		}
		result, err := g.callTool(params.Name, params.Arguments)
		if err != nil {
			return map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": err.Error(),
				}},
				"isError": true,
			}, nil
		}
		return toolSuccess(result), nil
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found: " + req.Method}
	}
}

func availableTools() []tool {
	return []tool{
		{
			Name:        "qianchuan_get_status",
			Description: "读取千川账户上下文、只读开关和写操作开关。",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "qianchuan_list_plans",
			Description: "读取全域推广计划列表，默认按统计消耗降序。",
			InputSchema: objectSchema(map[string]any{
				"start_time": stringSchema("开始时间，格式 YYYY-MM-DD HH:mm:ss"),
				"end_time":   stringSchema("结束时间，格式 YYYY-MM-DD HH:mm:ss"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量，可选 10/20/50/100", 20),
			}),
		},
		{
			Name:        "qianchuan_get_plan",
			Description: "读取单个全域推广计划详情。",
			InputSchema: objectSchema(map[string]any{
				"ad_id": integerSchema("计划 ID", 1),
			}, "ad_id"),
		},
		{
			Name:        "qianchuan_list_plan_materials",
			Description: "读取计划素材及服务端锁定的当日消耗前三素材。",
			InputSchema: objectSchema(map[string]any{
				"ad_id":           integerSchema("计划 ID", 1),
				"material_type":   stringSchema("素材类型，默认 VIDEO"),
				"material_status": stringSchema("素材状态，例如 DELIVERY_OK 或 ALL"),
				"start_date":      stringSchema("开始日期 YYYY-MM-DD"),
				"end_date":        stringSchema("结束日期 YYYY-MM-DD"),
				"page":            integerSchema("页码", 1),
				"page_size":       integerSchema("每页数量，可选 10/20/50/100", 20),
			}, "ad_id"),
		},
		{
			Name:        "qianchuan_material_report",
			Description: "读取素材报表，支持点击率、转化率、GPM、消耗和成交数据。",
			InputSchema: objectSchema(map[string]any{
				"start_date": stringSchema("开始日期 YYYY-MM-DD"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量", 100),
				"fields": map[string]any{
					"type":        "array",
					"description": "需要返回的指标字段",
					"items":       map[string]any{"type": "string"},
				},
				"filtering": map[string]any{
					"type":        "object",
					"description": "素材报表过滤条件",
					"additionalProperties": true,
				},
			}, "start_date", "end_date"),
		},
		{
			Name:        "qianchuan_live_overview",
			Description: "读取当天直播间投放概览，包括消耗、点击率、转化率、GPM 和成交。",
			InputSchema: objectSchema(map[string]any{
				"fields": map[string]any{
					"type":        "array",
					"description": "需要返回的直播指标字段",
					"items":       map[string]any{"type": "string"},
				},
			}),
		},
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerSchema(description string, minimum int) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
		"minimum":     minimum,
	}
}

func toolSuccess(data any) map[string]any {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "序列化结果失败",
			}},
			"isError": true,
		}
	}
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": string(encoded),
		}},
	}
}

func (g *gateway) callTool(name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "qianchuan_get_status":
		return g.request(http.MethodGet, "/status", nil, nil)
	case "qianchuan_list_plans":
		query := queryValues(args, "start_time", "end_time", "page", "page_size")
		return g.request(http.MethodGet, "/plans", query, nil)
	case "qianchuan_get_plan":
		adID, err := requiredPositiveInt64(args, "ad_id")
		if err != nil {
			return nil, err
		}
		return g.request(http.MethodGet, fmt.Sprintf("/plans/%d", adID), nil, nil)
	case "qianchuan_list_plan_materials":
		adID, err := requiredPositiveInt64(args, "ad_id")
		if err != nil {
			return nil, err
		}
		query := queryValues(
			args,
			"material_type",
			"material_status",
			"start_date",
			"end_date",
			"page",
			"page_size",
		)
		return g.request(
			http.MethodGet,
			fmt.Sprintf("/plans/%d/materials", adID),
			query,
			nil,
		)
	case "qianchuan_material_report":
		if err := requiredString(args, "start_date"); err != nil {
			return nil, err
		}
		if err := requiredString(args, "end_date"); err != nil {
			return nil, err
		}
		return g.request(http.MethodPost, "/material-report", nil, args)
	case "qianchuan_live_overview":
		return g.request(
			http.MethodGet,
			"/live-overview",
			queryValues(args, "fields"),
			nil,
		)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (g *gateway) request(method, path string, query url.Values, body any) (any, error) {
	target := g.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-MCP-Key", g.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Qianchuan MCP API: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read Qianchuan MCP API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Qianchuan MCP API HTTP %d: %s", resp.StatusCode, string(payload))
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode Qianchuan MCP API response: %w", err)
	}
	return decoded, nil
}

func queryValues(args map[string]any, keys ...string) url.Values {
	query := url.Values{}
	for _, key := range keys {
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				query.Set(key, typed)
			}
		case float64:
			query.Set(key, strconv.FormatFloat(typed, 'f', -1, 64))
		case []any:
			parts := make([]string, 0, len(typed))
			for _, item := range typed {
				parts = append(parts, fmt.Sprint(item))
			}
			if len(parts) > 0 {
				query.Set(key, strings.Join(parts, ","))
			}
		default:
			query.Set(key, fmt.Sprint(typed))
		}
	}
	return query
}

func requiredPositiveInt64(args map[string]any, key string) (int64, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch typed := value.(type) {
	case float64:
		if typed <= 0 || typed != float64(int64(typed)) {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return int64(typed), nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return parsed, nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
}

func requiredString(args map[string]any, key string) error {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", key)
	}
	return nil
}
