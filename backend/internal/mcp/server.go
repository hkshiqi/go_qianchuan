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
			"instructions": "先调用 qianchuan_list_accounts 查询本 Key 可操作的账户。多账户时必须给账户类工具传 account_id；单账户可省略。所有指标读取后再执行策略评估；计划写操作受服务端开关限制。",
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
		if forbiddenTenantFieldRequested(params.Arguments) {
			return toolFailure("租户上下文由 Bearer Key 决定；使用 account_id 在本租户账户中选择，不能传 advertiser_id、tenant_id 或密钥"), nil
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

func forbiddenTenantFieldRequested(args map[string]any) bool {
	return containsForbiddenTenantField(args)
}

func containsForbiddenTenantField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			switch key {
			case "advertiser_id", "advertiserId", "tenant_id", "tenantId", "api_key", "apiKey":
				return true
			}
			if containsForbiddenTenantField(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if containsForbiddenTenantField(nested) {
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
			Description: "核对 MCP 协议、可操作账户、千川授权状态、当前账户与直播间上下文、读写开关。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("可选；多账户时传入本租户内的广告账户 ID 查看该账户状态"),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_accounts",
			Title:       "查询可切换投放账户",
			Description: "列出当前个人 Key 可操作的广告账户、抖音号、直播间和授权状态；不会返回其他租户账户。",
			InputSchema: objectSchema(nil),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_plans",
			Title:       "查询投放计划",
			Description: "查询全域直播推广计划，按消耗排序；默认最近24小时。大整数 ID 以字符串返回。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
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
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("计划 ID"),
			}, "ad_id"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_get_plan_materials",
			Title:       "查询计划素材",
			Description: "读取计划素材及全部指标，服务端按消耗排名并锁定前3名。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
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
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
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
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_evaluate_rules",
			Title:       "评估投放规则",
			Description: "按点击率>=7%、点击转化率>=8%、分钟消耗>=20元评估计划，返回可替换素材；前3消耗素材不可候选。",
			InputSchema: objectSchema(map[string]any{
				"account_id":     stringSchema("本租户内的广告账户 ID；多账户时必填"),
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
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
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
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("计划 ID"),
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
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("计划 ID"),
				"budget":     numberSchema("新日预算，人民币元", 0.01),
				"confirm":    booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "budget", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_pause_plan",
			Title:       "暂停计划",
			Description: "暂停指定计划；写操作开关关闭时拒绝执行。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("计划 ID"),
				"confirm":    booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "confirm"),
			Annotations: mutatingAnnotations(true),
		},
		{
			Name:        "qianchuan_create_material_chase",
			Title:       "创建单阈值素材追投",
			Description: "用素材库现有视频创建直播成交素材追投。固定预算200元、出价14.32元；自动排除母计划消耗前3素材，只接受当天官方报表中点击率≥7%或点击转化率≥8%的素材，最多20条。成功后立即回读核对。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("直播全域母计划 ID"),
				"material_ids": map[string]any{
					"type":        "array",
					"description": "素材库现有视频素材 ID，1-20 条；须有至少一个阈值达标且不能包含消耗前3",
					"minItems":    1,
					"maxItems":    20,
					"items":       map[string]any{"type": "string"},
				},
				"budget":  numberSchema("追投预算，人民币元；固定 200", 200),
				"bid":     numberSchema("直播间成交转化出价，人民币元；固定 14.32", 14.32),
				"confirm": booleanSchema("必须为 true 才执行写操作"),
			}, "ad_id", "material_ids", "budget", "bid", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_list_control_tasks",
			Title:       "查询素材追投任务",
			Description: "读取指定母计划的素材追投任务及消耗、点击率、转化率、GPM、支付ROI和净成交ROI。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("直播全域母计划 ID"),
				"task_id":    stringSchema("可选；按调控任务 ID 搜索"),
				"task_status": stringEnumSchema(
					"可选；任务状态",
					"PROCESSING",
					"FROZEN",
					"DISABLE",
					"DELETED",
					"OFFLINE_BALANCE",
					"OFFLINE_BUDGET",
					"OFFLINE_TIME",
					"ROI2_DISABLE",
				),
				"start_time": stringSchema("开始时间 YYYY-MM-DD HH:mm:ss；默认今天 00:00:00"),
				"end_time":   stringSchema("结束时间 YYYY-MM-DD HH:mm:ss；默认今天 23:59:59"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量；服务端取 10/20/50/100", 1),
			}, "ad_id"),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_available_products",
			Title:       "查询全域可投商品",
			Description: "只读查询官方全域商家可选商品列表，供隔离 CRUD 测试计划选择现有商品。",
			InputSchema: objectSchema(map[string]any{
				"account_id":   stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"product_name": stringSchema("可选；按商品名称搜索"),
				"page":         integerSchema("页码", 1),
				"page_size":    integerSchema("每页数量；服务端取 10/20/30", 1),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_list_overall_awemes",
			Title:       "查询乘方可投抖音号",
			Description: "只读查询官方乘方商品计划可投抖音号。默认只返回可申请或可管理的抖音号，并按商品过滤。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"product_id": stringSchema("可选；只查询该商品的乘方可投抖音号"),
				"page":       integerSchema("页码", 1),
				"page_size":  integerSchema("每页数量；官方实际最大值 30", 1),
			}),
			Annotations: readOnlyAnnotations(),
		},
		{
			Name:        "qianchuan_create_test_product_plan",
			Title:       "创建隔离测试商品计划",
			Description: "仅用于计划 CRUD 验收。名称必须以 Codex-CRUD-TEST- 开头，创建商品全域计划，预算不超过1元，开始日期不早于明天，创建后立即暂停并回读。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"name":       stringSchema("测试计划名称；必须以 Codex-CRUD-TEST- 开头"),
				"product_id": stringSchema("官方全域可选商品列表中的现有商品 ID"),
				"budget":     numberSchema("测试日预算，人民币元；必须不超过1元", 0.01),
				"start_date": stringSchema("开始日期 YYYY-MM-DD；默认明天"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD；默认与开始日期相同"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "name", "product_id", "budget", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_rename_test_product_plan",
			Title:       "修改隔离测试计划名称",
			Description: "只允许修改名称以 Codex-CRUD-TEST- 开头的商品全域测试计划。修改前后均回读核验。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("本次测试计划 ID"),
				"new_name":   stringSchema("新名称；必须以 Codex-CRUD-TEST- 开头"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "ad_id", "new_name", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_delete_test_product_plan",
			Title:       "删除隔离测试计划",
			Description: "仅允许删除名称以 Codex-CRUD-TEST- 开头的商品全域测试计划。删除前先暂停，再删除并回读。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("本次测试计划 ID"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "ad_id", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_create_test_overall_product_plan",
			Title:       "创建隔离乘方商品测试计划",
			Description: "仅用于乘方计划 CRUD 验收。名称必须以 Codex-CRUD-OVERALL-TEST- 开头；平台最低预算为300元，因此测试预算只能等于300元；开始日期不早于明天；抖音号必须来自官方乘方可投列表；创建后立即暂停并回读。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"name":       stringSchema("测试计划名称；必须以 Codex-CRUD-OVERALL-TEST- 开头"),
				"product_id": stringSchema("官方全域可选商品列表中的现有商品 ID"),
				"aweme_id":   stringSchema("官方 overall_aweme/list 返回的乘方可投抖音号 ID"),
				"budget":     numberSchema("测试日预算，人民币元；平台最低300元，测试时只能等于300", 300),
				"roi2_goal":  numberSchema("乘方支付 ROI 目标；默认 1.90", 0.01),
				"start_date": stringSchema("开始日期 YYYY-MM-DD；默认明天"),
				"end_date":   stringSchema("结束日期 YYYY-MM-DD；默认与开始日期相同"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "name", "product_id", "aweme_id", "budget", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_rename_test_overall_product_plan",
			Title:       "修改隔离乘方测试计划名称",
			Description: "只允许修改名称以 Codex-CRUD-OVERALL-TEST- 开头的乘方商品测试计划。修改前必须回读确认 adlab_scene=1/OVERALL_PROJECT。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("本次乘方测试计划 ID"),
				"new_name":   stringSchema("新名称；必须以 Codex-CRUD-OVERALL-TEST- 开头"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "ad_id", "new_name", "confirm"),
			Annotations: mutatingAnnotations(false),
		},
		{
			Name:        "qianchuan_delete_test_overall_product_plan",
			Title:       "删除隔离乘方测试计划",
			Description: "仅允许删除名称以 Codex-CRUD-OVERALL-TEST- 开头的乘方商品测试计划。删除前先暂停；删除后必须由详情或列表明确返回删除态。",
			InputSchema: objectSchema(map[string]any{
				"account_id": stringSchema("本租户内的广告账户 ID；多账户时必填"),
				"ad_id":      stringSchema("本次乘方测试计划 ID"),
				"confirm":    booleanSchema("必须为 true 才执行测试写操作"),
			}, "ad_id", "confirm"),
			Annotations: mutatingAnnotations(false),
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
