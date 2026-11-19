package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

type Server struct {
	tenants  *tenantstore.Store
	resolver ToolResolver
}

type ToolResolver interface {
	ToolsFor(ctx context.Context, tenant *tenantstore.Tenant) (ToolCaller, error)
}

type ToolCaller interface {
	Call(ctx context.Context, name string, args map[string]any) (any, error)
}

func NewServer(tenants *tenantstore.Store, resolver ToolResolver) *Server {
	return &Server{tenants: tenants, resolver: resolver}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	tenant, err := s.authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="qianchuan-mcp"`)
		writeJSON(w, http.StatusUnauthorized, rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32001, Message: "MCP 鉴权失败"},
		})
		return
	}

	var req rpcRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		})
		return
	}
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		writeJSON(w, http.StatusBadRequest, rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32600, Message: "Invalid Request"},
		})
		return
	}
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, rpcErr := s.dispatch(r.Context(), tenant, &req)
	writeJSON(w, http.StatusOK, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	})
}

func (s *Server) authenticate(r *http.Request) (*tenantstore.Tenant, error) {
	if s.tenants == nil {
		return nil, fmt.Errorf("tenant store unavailable")
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, fmt.Errorf("missing bearer key")
	}
	key := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if key == "" {
		return nil, fmt.Errorf("missing bearer key")
	}
	return s.tenants.Authenticate(key)
}

func (s *Server) dispatch(ctx context.Context, tenant *tenantstore.Tenant, req *rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var params initializedParams
		_ = json.Unmarshal(req.Params, &params)
		version := protocolVersion
		if params.ProtocolVersion != "" {
			version = params.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "qianchuan-live-operations",
				"version": "1.0.0",
				"title":   "千川直播投放控制 MCP",
			},
			"instructions": "先调用 qianchuan_get_status 核对账户。所有指标读取后再执行策略评估；计划写操作受服务端开关限制。",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": availableTools()}, nil
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "Invalid tools/call params"}
		}
		if tenantFieldRequested(params.Arguments) {
			return toolFailure("账户上下文由 Bearer Key 决定，请求参数不能指定 advertiser_id、tenant_id 或密钥"), nil
		}
		tools, err := s.resolver.ToolsFor(ctx, tenant)
		if err != nil {
			return toolFailure(err.Error()), nil
		}
		result, err := tools.Call(ctx, params.Name, params.Arguments)
		if err != nil {
			return toolFailure(err.Error()), nil
		}
		return toolSuccess(result), nil
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found: " + req.Method}
	}
}

func tenantFieldRequested(args map[string]any) bool {
	return containsIdentifierField(args)
}

func containsIdentifierField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			switch key {
			case "advertiser_id", "advertiserId", "tenant_id", "tenantId", "api_key", "apiKey":
				return true
			}
			if containsIdentifierField(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsIdentifierField(nested) {
				return true
			}
		}
	}
	return false
}

func toolFailure(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": message,
		}},
		"isError": true,
	}
}

func availableTools() []tool {
	return []tool{
		{
			Name:        "qianchuan_get_status",
			Title:       "账户与 MCP 状态",
			Description: "核对 MCP 协议、千川授权状态、当前账户与直播间上下文、读写开关。",
			InputSchema: objectSchema(nil),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_plans",
			Title:       "查询投放计划",
			Description: "查询全域直播推广计划，按消耗排序；默认最近24小时。大整数 ID 以字符串返回。",
			InputSchema: objectSchema(map[string]any{
				"start_time": stringSchema("开始时间 YYYY-MM-DD HH:mm:ss；默认24小时前"),
				"end_time":   stringSchema("结束时间 YYYY-MM-DD HH:mm:ss；默认现在"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量，1-100", 1),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_get_plan",
			Title:       "查询计划详情",
			Description: "读取单个计划详情、预算、ROI 目标和关联素材。",
			InputSchema: objectSchema(map[string]any{
				"ad_id": stringSchema("计划 ID"),
			}, "ad_id"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_get_plan_materials",
			Title:       "查询计划素材",
			Description: "读取计划素材及全部指标，服务端按消耗排名并锁定前3名。",
			InputSchema: objectSchema(map[string]any{
				"ad_id":      stringSchema("计划 ID"),
				"start_date": stringSchema("开始日期 YYYY-MM-DD；默认今天"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD；默认今天"),
			}, "ad_id"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_material_report",
			Title:       "素材库效果报表",
			Description: "读取账户素材库效果数据，返回消耗、点击率、转化率、GPM、ROI，并标记消耗前3素材。",
			InputSchema: objectSchema(map[string]any{
				"start_date": stringSchema("开始日期 YYYY-MM-DD；默认今天"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD；默认今天"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量，1-200", 1),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_live_overview",
			Title:       "直播间投放概览",
			Description: "读取当前直播间当日投放数据，包含消耗、点击率、转化率、GPM、成交与分钟消耗。",
			InputSchema: objectSchema(nil),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_evaluate_rules",
			Title:       "评估投放规则",
			Description: "按点击率>=7%、点击转化率>=8%、分钟消耗>=20元评估计划，返回可替换素材；前3消耗素材不可候选。",
			InputSchema: objectSchema(map[string]any{
				"ad_id":          stringSchema("计划 ID"),
				"window_minutes": integerSchema("消耗统计窗口分钟数，默认5", 1),
			}, "ad_id"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_material_library",
			Title:       "查询可复用素材库",
			Description: "读取账户现有素材库；替换素材只能从返回结果中选择，不创建或上传新素材。",
			InputSchema: objectSchema(map[string]any{
				"start_date": stringSchema("开始日期 YYYY-MM-DD；默认今天"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD；默认今天"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量，1-200", 1),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_rotate_materials",
			Title:       "替换非保护素材",
			Description: "从现有素材库选择素材替换计划中的非前3素材。服务端再次校验保护名单并只允许加入素材库现有素材。",
			InputSchema: objectSchema(map[string]any{
				"ad_id": stringSchema("计划 ID"),
				"material_ids": map[string]any{
					"type":        "array",
					"description": "要移除的非保护素材 ID",
					"items":       map[string]any{"type": "string"},
				},
				"replacement_video_ids": map[string]any{
					"type":        "array",
					"description": "从素材库现有素材中选择的视频 ID",
					"items":       map[string]any{"type": "string"},
				},
				"confirm": booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "material_ids", "confirm"),
			Annotations: mutatingAnnotations(true),
		},
		{
			Name:        "qianchuan_set_plan_budget",
			Title:       "调整计划预算",
			Description: "调整单个计划日预算；写操作开关关闭时拒绝执行。",
			InputSchema: objectSchema(map[string]any{
				"ad_id":   stringSchema("计划 ID"),
				"budget":  numberSchema("新日预算，人民币元", 0.01),
				"confirm": booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "budget", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_pause_plan",
			Title:       "暂停计划",
			Description: "暂停指定计划；写操作开关关闭时拒绝执行。",
			InputSchema: objectSchema(map[string]any{
				"ad_id":   stringSchema("计划 ID"),
				"confirm": booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "confirm"),
			Annotations: mutatingAnnotations(true),
		},
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"encode error"}}`)
	}
}
