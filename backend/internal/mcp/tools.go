package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

type TenantIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Tools struct {
	service                   *service.QianchuanService
	writeEnabled              bool
	materialChaseWriteEnabled bool
	planCRUDTestWriteEnabled  bool
	authURL                   string
	tenant                    TenantIdentity
	accounts                  []tenantstore.AccountBinding
	authURLBuilder            func(tenantID string) (string, error)
	tokenStorePath            string
}

func NewTools(
	qianchuanService *service.QianchuanService,
	writeEnabled bool,
	authURL string,
) *Tools {
	return &Tools{
		service:      qianchuanService,
		writeEnabled: writeEnabled,
		authURL:      authURL,
	}
}

func NewTenantTools(
	qianchuanService *service.QianchuanService,
	tenant tenantstore.Tenant,
	writeEnabled bool,
	authURL string,
) *Tools {
	qianchuanService.DefaultAdvertiserID = tenant.AdvertiserID
	qianchuanService.DefaultAwemeID = tenant.AwemeID
	qianchuanService.DefaultLiveRoomID = tenant.LiveRoomID
	tools := NewTools(qianchuanService, writeEnabled, authURL)
	tools.tenant = TenantIdentity{ID: tenant.ID, Name: tenant.Name}
	tools.accounts = tenant.AccountList()
	return tools
}

func (t *Tools) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "qianchuan_get_status":
		return t.status(ctx, args)
	case "qianchuan_list_accounts":
		return t.listAccounts(ctx), nil
	case "qianchuan_list_plans":
		return t.listPlans(ctx, args)
	case "qianchuan_get_plan":
		return t.getPlan(ctx, args)
	case "qianchuan_get_plan_materials":
		return t.planMaterials(ctx, args)
	case "qianchuan_material_report":
		return t.materialReport(ctx, args)
	case "qianchuan_list_material_library":
		return t.materialReport(ctx, args)
	case "qianchuan_live_overview":
		return t.liveOverview(ctx, args)
	case "qianchuan_evaluate_rules":
		return t.evaluateRules(ctx, args)
	case "qianchuan_rotate_materials":
		return t.rotateMaterials(ctx, args)
	case "qianchuan_set_plan_budget":
		return t.setPlanBudget(ctx, args)
	case "qianchuan_pause_plan":
		return t.pausePlan(ctx, args)
	case "qianchuan_create_material_chase":
		return t.createMaterialChase(ctx, args)
	case "qianchuan_list_control_tasks":
		return t.listControlTasks(ctx, args)
	case "qianchuan_list_available_products":
		return t.listAvailableProducts(ctx, args)
	case "qianchuan_list_overall_awemes":
		return t.listOverallAwemes(ctx, args)
	case "qianchuan_create_test_product_plan":
		return t.createTestProductPlan(ctx, args)
	case "qianchuan_rename_test_product_plan":
		return t.renameTestProductPlan(ctx, args)
	case "qianchuan_delete_test_product_plan":
		return t.deleteTestProductPlan(ctx, args)
	case "qianchuan_create_test_overall_product_plan":
		return t.createTestOverallProductPlan(ctx, args)
	case "qianchuan_rename_test_overall_product_plan":
		return t.renameTestOverallProductPlan(ctx, args)
	case "qianchuan_delete_test_overall_product_plan":
		return t.deleteTestOverallProductPlan(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (t *Tools) status(ctx context.Context, args map[string]any) (Status, error) {
	accounts := t.accountList()
	account, accountErr := t.resolveAccount(args)
	advertiserID := int64(0)
	awemeID := int64(0)
	liveRoomID := int64(0)
	if accountErr == nil {
		advertiserID = account.AdvertiserID
		awemeID = account.AwemeID
		liveRoomID = account.LiveRoomID
	} else if len(accounts) == 1 || stringArg(args, "account_id", "") != "" {
		return Status{}, accountErr
	}
	authorized := false
	if advertiserID > 0 {
		if _, tokenErr := t.service.TokenManager.AccessToken(ctx, account.AdvertiserID); tokenErr == nil {
			authorized = true
		}
	}
	reason := "OPERATIONS_WRITE_ENABLED=true 时才允许写操作"
	if t.writeEnabled {
		reason = "写操作已启用，仍需 confirm=true 且服务端保护校验通过"
	}
	authorizationURL := t.authURL
	if t.authURLBuilder != nil {
		selector := t.tenant.ID
		if advertiserID > 0 {
			selector = t.tenant.ID + "|" + formatID(advertiserID)
		}
		if generated, err := t.authURLBuilder(selector); err == nil {
			authorizationURL = generated
		}
	}
	return Status{
		TenantID:                  t.tenant.ID,
		TenantName:                t.tenant.Name,
		Service:                   "qianchuan-live-operations-mcp",
		ProtocolVersion:           protocolVersion,
		Authenticated:             authorized,
		AdvertiserID:              formatID(advertiserID),
		AwemeID:                   formatID(awemeID),
		LiveRoomID:                formatID(liveRoomID),
		Accounts:                  publicAccounts(accounts),
		AccountSelectionRequired:  len(accounts) > 1 && advertiserID == 0,
		ReadEnabled:               true,
		WriteEnabled:              t.writeEnabled,
		MaterialChaseWriteEnabled: t.materialChaseWriteEnabled,
		PlanCRUDTestWriteEnabled:  t.planCRUDTestWriteEnabled,
		AuthorizationURL:          authorizationURL,
		Capabilities:              []string{"account", "plans", "materials", "live", "rules"},
		WriteCapabilities:         []string{"budget", "pause", "material_rotation"},
		MaterialChaseCapabilities: []string{"create_material_chase", "list_control_tasks"},
		PlanCRUDTestCapabilities: []string{
			"list_overall_awemes",
			"create_test_product_plan",
			"rename_test_product_plan",
			"delete_test_product_plan",
			"create_test_overall_product_plan",
			"rename_test_overall_product_plan",
			"delete_test_overall_product_plan",
		},
		OperationMode:      "server-side MCP; no browser cookies",
		RestartRequired:    false,
		TokenEncrypted:     true,
		TokenStorePresent:  t.tokenStoreExists(),
		WriteEnabledReason: reason,
	}, nil
}

func (t *Tools) listAccounts(ctx context.Context) map[string]any {
	accounts := make([]map[string]any, 0, len(t.accountList()))
	for _, account := range t.accountList() {
		authorized := false
		if _, err := t.service.TokenManager.AccessToken(ctx, account.AdvertiserID); err == nil {
			authorized = true
		}
		authorizationURL := ""
		if t.authURLBuilder != nil {
			if generated, err := t.authURLBuilder(t.tenant.ID + "|" + formatID(account.AdvertiserID)); err == nil {
				authorizationURL = generated
			}
		}
		accounts = append(accounts, map[string]any{
			"account_id":        formatID(account.AdvertiserID),
			"advertiser_id":     formatID(account.AdvertiserID),
			"aweme_id":          formatID(account.AwemeID),
			"live_room_id":      formatID(account.LiveRoomID),
			"authenticated":     authorized,
			"default_eligible":  len(t.accountList()) == 1,
			"authorization_url": authorizationURL,
		})
	}
	return map[string]any{
		"tenant_id":                  t.tenant.ID,
		"tenant_name":                t.tenant.Name,
		"accounts":                   accounts,
		"account_selection_required": len(accounts) > 1,
		"selection_rule":             "多个账户时必须向账户类工具传 account_id；可以跨本租户账户选择，不能选择其他租户账户。",
	}
}

func (t *Tools) accountList() []tenantstore.AccountBinding {
	if len(t.accounts) > 0 {
		return append([]tenantstore.AccountBinding{}, t.accounts...)
	}
	if t.service.DefaultAdvertiserID > 0 {
		return []tenantstore.AccountBinding{{
			AdvertiserID: t.service.DefaultAdvertiserID,
			AwemeID:      t.service.DefaultAwemeID,
			LiveRoomID:   t.service.DefaultLiveRoomID,
		}}
	}
	return nil
}

func (t *Tools) resolveAccount(args map[string]any) (tenantstore.AccountBinding, error) {
	accountID := stringArg(args, "account_id", "")
	accounts := t.accountList()
	if accountID == "" {
		if len(accounts) == 1 {
			return accounts[0], nil
		}
		if len(accounts) == 0 {
			return tenantstore.AccountBinding{}, tenantstore.ErrAccountNotFound
		}
		return tenantstore.AccountBinding{}, tenantstore.ErrAccountRequired
	}
	for _, account := range accounts {
		if formatID(account.AdvertiserID) == accountID {
			return account, nil
		}
	}
	return tenantstore.AccountBinding{}, tenantstore.ErrAccountNotFound
}

func publicAccounts(accounts []tenantstore.AccountBinding) []map[string]string {
	result := make([]map[string]string, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, map[string]string{
			"account_id":    formatID(account.AdvertiserID),
			"advertiser_id": formatID(account.AdvertiserID),
			"aweme_id":      formatID(account.AwemeID),
			"live_room_id":  formatID(account.LiveRoomID),
		})
	}
	return result
}

func (t *Tools) tokenStoreExists() bool {
	if t.tokenStorePath == "" {
		return false
	}
	_, err := os.Stat(t.tokenStorePath)
	return err == nil
}

func (t *Tools) listPlans(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(shanghaiLocation)
	startTime := stringArg(args, "start_time", now.Add(-24*time.Hour).Format("2006-01-02 15:04:05"))
	endTime := stringArg(args, "end_time", now.Format("2006-01-02 15:04:05"))
	page := positiveIntArg(args, "page", 1)
	pageSize := clampInt(positiveIntArg(args, "page_size", 20), 1, 100)
	response, err := t.service.Client.UniPromotionList(ctx, sdk.UniPromotionListReq{
		AccessToken:   token,
		AdvertiserId:  uint64(advertiserID),
		StartTime:     startTime,
		EndTime:       endTime,
		MarketingGoal: "LIVE_PROM_GOODS",
		Fields: []string{
			"stat_cost",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
		},
		OrderField: "stat_cost",
		OrderType:  "DESC",
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("获取计划列表失败: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("获取计划列表失败", response.Code, response.Message)
	}
	list := make([]map[string]any, 0, len(response.Data.List))
	for _, plan := range response.Data.List {
		list = append(list, planMap(plan))
	}
	return map[string]any{
		"account":   t.account(account),
		"window":    map[string]string{"start_time": startTime, "end_time": endTime},
		"page_info": response.Data.PageInfo,
		"plans":     list,
	}, nil
}

func (t *Tools) getPlan(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	response, err := t.service.Client.UniPromotionDetail(ctx, sdk.UniPromotionDetailReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		AdId:         adID,
	})
	if err != nil {
		return nil, fmt.Errorf("获取计划详情失败: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("获取计划详情失败", response.Code, response.Message)
	}
	return map[string]any{
		"account": t.account(account),
		"plan":    response.Data,
	}, nil
}

func (t *Tools) planMaterials(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	startDate := stringArg(args, "start_date", today())
	endDate := stringArg(args, "end_date", startDate)
	materials, warnings, err := t.fetchPlanMaterials(ctx, token, advertiserID, adID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	result := materialPage(
		materials,
		positiveIntArg(args, "page", 1),
		clampInt(positiveIntArg(args, "page_size", 20), 1, 100),
	)
	result["account"] = t.account(account)
	result["ad_id"] = formatUintID(adID)
	result["window"] = map[string]string{"start_date": startDate, "end_date": endDate}
	result["rule"] = "消耗最高的前3素材永久保留；替换仅允许非前3素材"
	result["warnings"] = warnings
	return result, nil
}

func (t *Tools) materialReport(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	startDate := stringArg(args, "start_date", today())
	endDate := stringArg(args, "end_date", startDate)
	page := positiveIntArg(args, "page", 1)
	pageSize := reportPageSize(positiveIntArg(args, "page_size", 100))
	const dataTopic = "OVERALL_ROI_LIVE_MATERIAL_VIDEO"
	configResponse, err := t.service.Client.UniPromotionReportConfig(
		ctx,
		sdk.UniPromotionReportConfigReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			DataTopics:   []string{dataTopic},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("获取素材报表配置失败: %w", err)
	}
	if configResponse.Code != 0 {
		return nil, apiError(
			"获取素材报表配置失败",
			configResponse.Code,
			configResponse.Message,
			configResponse.RequestID,
		)
	}
	var topicConfig *sdk.UniPromotionReportTopicConfig
	for index := range configResponse.Data.CustomConfigDatas {
		if configResponse.Data.CustomConfigDatas[index].DataTopic == dataTopic {
			topicConfig = &configResponse.Data.CustomConfigDatas[index]
			break
		}
	}
	if topicConfig == nil {
		return nil, fmt.Errorf("官方报表配置未返回 %s", dataTopic)
	}
	dimensionFields := reportDimensionFields(*topicConfig)
	metricFields := reportMetricFields(*topicConfig)
	if len(dimensionFields) == 0 || len(metricFields) == 0 {
		return nil, fmt.Errorf("官方报表配置缺少素材维度或指标")
	}
	if expected := topicConfig.MaxDimensionLimit(); expected > 0 && len(dimensionFields) > expected {
		return nil, fmt.Errorf("素材报表维度 %d 个，超过官方上限 %d 个", len(dimensionFields), expected)
	}
	anchorID, err := t.resolveReportAnchorID(ctx, token, advertiserID, account)
	if err != nil {
		return nil, err
	}
	filters, err := reportFilters(*topicConfig, startDate, endDate, account, anchorID)
	if err != nil {
		return nil, err
	}
	orderField := reportCostMetric(metricFields)
	response, err := t.service.Client.UniPromotionReportData(
		ctx,
		sdk.UniPromotionReportDataReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			DataTopic:    dataTopic,
			Dimensions:   dimensionFields,
			Metrics:      metricFields,
			Filters:      filters,
			StartTime:    startDate + " 00:00:00",
			EndTime:      endDate + " 23:59:59",
			OrderBy:      []sdk.UniPromotionReportOrder{{Type: 2, Field: orderField}},
			Page:         page,
			PageSize:     pageSize,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("获取素材报表失败: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError(
			"获取素材报表失败",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	materials := make([]MaterialMetrics, 0, len(response.Data.Rows))
	for index, row := range response.Data.Rows {
		materials = append(materials, materialFromReportRow(row, index+1))
	}
	sort.SliceStable(materials, func(i, j int) bool {
		if materials[i].Cost == materials[j].Cost {
			return materials[i].MaterialID > materials[j].MaterialID
		}
		return materials[i].Cost > materials[j].Cost
	})
	for index := range materials {
		materials[index].CostRank = index + 1
		materials[index].ProtectedTop3 = index < DefaultRuleConfig().ProtectedTopN
		materials[index].EligibleReplace = !materials[index].ProtectedTop3
	}
	return map[string]any{
		"account":        t.account(account),
		"window":         map[string]string{"start_date": startDate, "end_date": endDate},
		"page_info":      response.Data.PageInfo,
		"materials":      materials,
		"protected_top3": protectedTop3(materials),
		"data_topic":     dataTopic,
		"anchor_id":      formatUintID(anchorID),
		"dimensions":     dimensionFields,
		"metrics":        metricFields,
		"source":         "qianchuan report uni_promotion data",
		"write_guard":    "素材库只读；本工具不会上传或创建素材",
	}, nil
}

func (t *Tools) liveOverview(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(shanghaiLocation)
	startTime, endTime := businessDayRange(now)
	roomResponse, err := t.service.Client.LiveRoomGet(ctx, sdk.LiveRoomGetReq{
		AccessToken:  token,
		AdvertiserId: advertiserID,
		AwemeId:      account.AwemeID,
		DateTime:     now.Format("2006-01-02"),
		RoomStatus:   "LIVING",
		AdStatus:     "ALL",
		Fields: []string{
			"stat_cost",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"convert_rate",
			"total_live_pay_order_gpm",
			"luban_live_pay_order_gpm",
			"live_pay_order_gmv_alias",
			"luban_live_pay_order_gmv",
			"live_pay_order_gmv_roi",
			"total_live_watch_cnt",
			"live_click_product_count_alias",
		},
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("获取今日直播间列表失败: %w", err)
	}
	if roomResponse.Code != 0 {
		return nil, apiError("获取今日直播间列表失败", roomResponse.Code, roomResponse.Message)
	}
	rooms := make([]map[string]any, 0, len(roomResponse.Data.Rooms))
	for _, encoded := range roomResponse.Data.Rooms {
		room := make(map[string]any, len(encoded))
		for key, value := range encoded {
			room[key] = rawJSONValue(value)
		}
		rooms = append(rooms, room)
	}
	room, roomWarnings := selectLivingRoom(rooms)
	response, err := t.service.Client.LiveGet(ctx, sdk.LiveGetReq{
		AccessToken:  token,
		AdvertiserId: advertiserID,
		AwemeId:      account.AwemeID,
		StartTime:    startTime,
		EndTime:      endTime,
		Fields: []string{
			"stat_cost",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"convert_rate",
			"total_live_pay_order_gpm",
			"luban_live_pay_order_gpm",
			"live_pay_order_gmv_alias",
			"luban_live_pay_order_gmv",
			"live_pay_order_gmv_roi",
			"total_live_watch_cnt",
			"live_click_product_count_alias",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("获取直播数据失败: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("获取直播数据失败", response.Code, response.Message)
	}
	live := normalizeLiveReportAmounts(response.Data)
	liveRoomID := formatID(account.LiveRoomID)
	if room != nil {
		liveRoomID = liveRoomIDFromMap(room)
		room = normalizeLiveReportAmountsMap(room)
	}
	return map[string]any{
		"account":      t.account(account),
		"aweme_id":     formatID(account.AwemeID),
		"live_room_id": liveRoomID,
		"live_room":    room,
		"window":       map[string]string{"start_time": startTime, "end_time": endTime},
		"live":         live,
		"warnings":     roomWarnings,
		"refreshed_at": now.Format(time.RFC3339),
	}, nil
}

func (t *Tools) evaluateRules(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	window := clampInt(positiveIntArg(args, "window_minutes", 5), 1, 60)
	materials, warnings, err := t.fetchPlanMaterials(ctx, token, advertiserID, adID, today(), today())
	if err != nil {
		return nil, err
	}
	config := DefaultRuleConfig()
	protected := protectedTop3(materials)
	topCost := 0.0
	for _, material := range materials {
		topCost += material.Cost
	}
	minuteCost := topCost / float64(window)
	ctr := weightedRate(materials, "ctr")
	conversion := weightedRate(materials, "conversion_rate")
	gpm := 0.0
	if topCost > 0 {
		for _, material := range materials {
			gpm += material.GPM
		}
	}
	ctrPassed := ctr >= config.MinimumCTR
	conversionPassed := conversion >= config.MinimumConversionRate
	minutePassed := minuteCost >= config.MinimumMinuteCost
	candidates := make([]MaterialMetrics, 0)
	for _, material := range materials {
		if material.ProtectedTop3 {
			continue
		}
		if material.Cost > 0 && (material.CTR < config.MinimumCTR || material.ConversionRate < config.MinimumConversionRate) {
			candidates = append(candidates, material)
		}
	}
	action := "hold"
	if !ctrPassed || !conversionPassed {
		if len(candidates) > 0 {
			action = "rotate_non_protected_materials"
		} else {
			action = "manual_review_no_material_candidate"
		}
	} else if !minutePassed {
		action = "increase_budget_or_traffic"
	}
	status := "healthy"
	if action != "hold" {
		status = "attention_required"
	}
	return Evaluation{
		AdvertiserID:          formatID(advertiserID),
		AdID:                  formatUintID(adID),
		WindowMinutes:         window,
		Cost:                  round(topCost, 2),
		MinuteCost:            round(minuteCost, 2),
		MinuteCostPassed:      minutePassed,
		CTR:                   round(ctr, 6),
		CTRPassed:             ctrPassed,
		ConversionRate:        round(conversion, 6),
		ConversionPassed:      conversionPassed,
		GPM:                   round(gpm, 2),
		Status:                status,
		Action:                action,
		ProtectedTop3:         protected,
		ReplacementCandidates: candidates,
		RuleConfig:            config,
		MetricsComplete:       len(warnings) == 0,
		Warnings:              warnings,
	}, nil
}

func (t *Tools) rotateMaterials(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requireWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	materialIDStrings, err := stringIDListArg(args, "material_ids")
	if err != nil || len(materialIDStrings) == 0 {
		return nil, errors.New("material_ids must contain at least one non-protected material")
	}
	materialIDs, err := numericIDList(materialIDStrings)
	if err != nil {
		return nil, err
	}
	videoIDs, err := stringIDListArg(args, "replacement_video_ids")
	if err != nil {
		return nil, err
	}
	if len(videoIDs) == 0 {
		return nil, errors.New("replacement_video_ids is required")
	}
	materials, _, err := t.fetchPlanMaterials(ctx, token, advertiserID, adID, today(), today())
	if err != nil {
		return nil, err
	}
	protected := make(map[string]bool)
	for _, item := range protectedTop3(materials) {
		protected[item.MaterialID] = true
	}
	for _, id := range materialIDStrings {
		if protected[id] {
			return nil, fmt.Errorf("material %s is protected: top 3 by spend cannot be removed", id)
		}
	}
	deleteResponse, err := t.service.Client.UniPromotionMaterialDelete(ctx, sdk.UniPromotionMaterialDeleteReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		AdId:         adID,
		MaterialIds:  materialIDs,
		MaterialType: "VIDEO",
	})
	if err != nil {
		return nil, fmt.Errorf("remove materials failed: %w", err)
	}
	if deleteResponse.Code != 0 {
		return nil, apiError("删除素材失败", deleteResponse.Code, deleteResponse.Message)
	}
	addPayload := make([]sdk.UniPromotionMaterial, 0, len(videoIDs))
	for _, id := range videoIDs {
		addPayload = append(addPayload, sdk.UniPromotionMaterial{VideoId: id})
	}
	addResponse, err := t.service.Client.UniPromotionMaterialAdd(ctx, sdk.UniPromotionMaterialAddReq{
		AccessToken:   token,
		AdvertiserId:  uint64(advertiserID),
		AdId:          adID,
		VideoMaterial: addPayload,
	})
	if err != nil {
		return nil, fmt.Errorf("add existing materials failed: %w", err)
	}
	if addResponse.Code != 0 {
		return nil, apiError("加入素材失败", addResponse.Code, addResponse.Message)
	}
	return map[string]any{
		"ad_id":          formatUintID(adID),
		"removed":        materialIDStrings,
		"added":          videoIDs,
		"protected_top3": protectedTop3(materials),
		"source":         "existing material library only",
	}, nil
}

func (t *Tools) setPlanBudget(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requireWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	budget, ok := floatArg(args, "budget")
	if !ok || budget <= 0 {
		return nil, errors.New("budget must be positive")
	}
	response, err := t.service.Client.UniPromotionBudgetUpdate(ctx, sdk.UniPromotionBudgetUpdateReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		Data: []sdk.UniPromotionBudgetUpdateItem{{
			AdId:   adID,
			Budget: budget,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("update plan budget failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("更新计划预算失败", response.Code, response.Message)
	}
	return map[string]any{"ad_id": formatUintID(adID), "budget": budget, "result": response.Data.Results}, nil
}

func (t *Tools) pausePlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requireWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	response, err := t.service.Client.UniPromotionStatusUpdate(ctx, sdk.UniPromotionStatusUpdateReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		AdIds:        []uint64{adID},
		OptStatus:    "DISABLE",
	})
	if err != nil {
		return nil, fmt.Errorf("pause plan failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("暂停计划失败", response.Code, response.Message)
	}
	return map[string]any{"ad_id": formatUintID(adID), "status": "DISABLE", "result": response.Data}, nil
}

func (t *Tools) createMaterialChase(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requireMaterialChaseWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	budget, ok := floatArg(args, "budget")
	if !ok || !almostEqual(budget, 200, 0.000001) {
		return nil, errors.New("budget must equal 200")
	}
	bid, ok := floatArg(args, "bid")
	if !ok || !almostEqual(bid, 14.32, 0.000001) {
		return nil, errors.New("bid must equal 14.32")
	}
	materialIDs, err := stringIDListArg(args, "material_ids")
	if err != nil {
		return nil, err
	}
	materialIDs = uniqueStrings(materialIDs)
	if len(materialIDs) == 0 || len(materialIDs) > 20 {
		return nil, errors.New("material_ids must contain between 1 and 20 unique IDs")
	}
	planResponse, err := t.service.Client.UniPromotionDetail(ctx, sdk.UniPromotionDetailReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		AdId:         adID,
	})
	if err != nil {
		return nil, fmt.Errorf("read parent plan failed: %w", err)
	}
	if planResponse.Code != 0 || planResponse.Data == nil {
		return nil, apiError("read parent plan failed", planResponse.Code, planResponse.Message, planResponse.RequestID)
	}
	if planResponse.Data.MarketingGoal != "LIVE_PROM_GOODS" {
		return nil, fmt.Errorf("parent plan marketing_goal=%q, required LIVE_PROM_GOODS", planResponse.Data.MarketingGoal)
	}
	materials, _, err := t.fetchPlanMaterials(ctx, token, advertiserID, adID, today(), today())
	if err != nil {
		return nil, err
	}
	protected := make(map[string]bool, 3)
	for _, item := range protectedTop3(materials) {
		protected[item.MaterialID] = true
	}
	library, err := t.fetchMaterialLibrary(ctx, token, advertiserID, today(), today())
	if err != nil {
		return nil, err
	}
	libraryByID := make(map[string]MaterialMetrics, len(library))
	for _, item := range library {
		libraryByID[item.MaterialID] = item
	}
	selected := make([]MaterialMetrics, 0, len(materialIDs))
	for _, id := range materialIDs {
		if protected[id] {
			return nil, fmt.Errorf("material %s is protected: top 3 by spend cannot enter the chase task", id)
		}
		material, ok := libraryByID[id]
		if !ok {
			return nil, fmt.Errorf("material %s is not present in today's official video material report", id)
		}
		if !materialThresholdPassed(material, DefaultRuleConfig()) {
			return nil, fmt.Errorf(
				"material %s did not meet either threshold: ctr=%.4f%% (min 7%%), conversion_rate=%.4f%% (min 8%%)",
				id,
				material.CTR,
				material.ConversionRate,
			)
		}
		selected = append(selected, material)
	}
	now := time.Now().In(shanghaiLocation)
	startTime, endTime := businessDayRange(now)
	existing, err := t.service.Client.UniPromotionControlTaskList(ctx, sdk.UniPromotionControlTaskListReq{
		AccessToken:   token,
		AdvertiserId:  uint64(advertiserID),
		MarketingGoal: "LIVE_PROM_GOODS",
		AdId:          adID,
		StartTime:     startTime,
		EndTime:       endTime,
		Scene:         "MATERIAL_ADD_BUDGET",
		Filtering: &sdk.UniPromotionControlTaskListFiltering{
			TaskStatus: "PROCESSING",
		},
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("read existing material chase tasks failed: %w", err)
	}
	if existing.Code != 0 {
		return nil, apiError("read existing material chase tasks failed", existing.Code, existing.Message, existing.RequestID)
	}
	for _, task := range existing.Data.TaskList {
		if task.TaskStatus != "PROCESSING" {
			continue
		}
		if sameIDSet(task.MaterialList, materialIDs) {
			return nil, fmt.Errorf("an identical processing material chase already exists: task_id=%d", task.Id)
		}
	}
	taskName := fmt.Sprintf("20261120_单阈值追投_%s", now.Format("1504"))
	createResponse, err := t.service.Client.UniPromotionControlTaskCreate(
		ctx,
		sdk.UniPromotionControlTaskCreateReq{
			AccessToken:    token,
			AdvertiserId:   uint64(advertiserID),
			AdId:           adID,
			Name:           taskName,
			Scene:          "MATERIAL_ADD_BUDGET",
			Budget:         budget,
			MaterialIds:    mustNumericIDs(materialIDs),
			ExternalAction: "AD_CONVERT_TYPE_LIVE_SUCCESSORDER_PAY",
			SmartBidType:   "SMART_BID_CUSTOM",
			Bid:            bid,
			MaterialType:   "VIDEO",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create material chase failed: %w", err)
	}
	if createResponse.Code != 0 || createResponse.Data.Id == 0 {
		return nil, apiError(
			"create material chase failed",
			createResponse.Code,
			createResponse.Message,
			createResponse.RequestID,
		)
	}
	taskID := createResponse.Data.Id
	var verified *sdk.UniPromotionControlTask
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		listResponse, listErr := t.service.Client.UniPromotionControlTaskList(
			ctx,
			sdk.UniPromotionControlTaskListReq{
				AccessToken:   token,
				AdvertiserId:  uint64(advertiserID),
				MarketingGoal: "LIVE_PROM_GOODS",
				AdId:          adID,
				StartTime:     startTime,
				EndTime:       endTime,
				Scene:         "MATERIAL_ADD_BUDGET",
				Filtering: &sdk.UniPromotionControlTaskListFiltering{
					SearchKeyword: formatUintID(taskID),
				},
				Page:     1,
				PageSize: 10,
			},
		)
		if listErr != nil {
			continue
		}
		if listResponse.Code != 0 {
			continue
		}
		for index := range listResponse.Data.TaskList {
			if listResponse.Data.TaskList[index].Id == taskID {
				verified = &listResponse.Data.TaskList[index]
				break
			}
		}
		if verified != nil {
			break
		}
	}
	if verified == nil {
		return map[string]any{
			"created":      true,
			"verified":     false,
			"task_id":      formatUintID(taskID),
			"task_name":    taskName,
			"budget":       budget,
			"bid":          bid,
			"material_ids": materialIDs,
			"warning":      "task creation succeeded, but it has not appeared in the control-task list yet",
		}, nil
	}
	verifiedIDs := controlTaskMaterialIDs(verified.MaterialList)
	if !almostEqual(verified.Budget, budget, 0.000001) ||
		!almostEqual(verified.Bid, bid, 0.000001) ||
		!sameStrings(verifiedIDs, materialIDs) {
		return map[string]any{
			"created":            true,
			"verified":           false,
			"task_id":            formatUintID(taskID),
			"task_name":          taskName,
			"expected_budget":    budget,
			"actual_budget":      verified.Budget,
			"expected_bid":       bid,
			"actual_bid":         verified.Bid,
			"expected_materials": materialIDs,
			"actual_materials":   verifiedIDs,
			"warning":            "task exists, but server-side verification found a setting mismatch",
		}, nil
	}
	return map[string]any{
		"created":                  true,
		"verified":                 true,
		"task_id":                  formatUintID(taskID),
		"task":                     verified,
		"account":                  t.account(account),
		"parent_plan_id":           formatUintID(adID),
		"budget":                   budget,
		"bid":                      bid,
		"material_count":           len(materialIDs),
		"material_ids":             materialIDs,
		"selected_materials":       selected,
		"protected_top3":           protectedTop3(materials),
		"dual_threshold_materials": dualThresholdMaterials(selected, DefaultRuleConfig()),
		"source":                   "existing official video material library only",
	}, nil
}

func (t *Tools) listControlTasks(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	now := time.Now().In(shanghaiLocation)
	startTime := stringArg(args, "start_time", now.Format("2006-01-02")+" 00:00:00")
	endTime := stringArg(args, "end_time", now.Format("2006-01-02")+" 23:59:59")
	filtering := &sdk.UniPromotionControlTaskListFiltering{
		SearchKeyword: stringArg(args, "task_id", ""),
		TaskStatus:    stringArg(args, "task_status", ""),
	}
	response, err := t.service.Client.UniPromotionControlTaskList(
		ctx,
		sdk.UniPromotionControlTaskListReq{
			AccessToken:   token,
			AdvertiserId:  uint64(advertiserID),
			MarketingGoal: "LIVE_PROM_GOODS",
			AdId:          adID,
			StartTime:     startTime,
			EndTime:       endTime,
			Scene:         "MATERIAL_ADD_BUDGET",
			Fields:        controlTaskFields(),
			Filtering:     filtering,
			Page:          positiveIntArg(args, "page", 1),
			PageSize:      closestControlTaskPageSize(positiveIntArg(args, "page_size", 20)),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list control tasks failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("list control tasks failed", response.Code, response.Message, response.RequestID)
	}
	return map[string]any{
		"account":   t.account(account),
		"ad_id":     formatUintID(adID),
		"scene":     "MATERIAL_ADD_BUDGET",
		"tasks":     response.Data.TaskList,
		"page_info": response.Data.PageInfo,
	}, nil
}

func (t *Tools) listAvailableProducts(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	page := positiveIntArg(args, "page", 1)
	pageSize := closestProductPageSize(positiveIntArg(args, "page_size", 20))
	response, err := t.service.Client.UniPromotionProductAvailableGet(
		ctx,
		sdk.UniPromotionProductAvailableGetReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AwemeId:      uint64(account.AwemeID),
			Page:         page,
			PageSize:     pageSize,
			ProductName:  stringArg(args, "product_name", ""),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list available products failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError(
			"list available products failed",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	products := make([]map[string]any, 0, len(response.Data.ProductList))
	for _, product := range response.Data.ProductList {
		products = append(products, map[string]any{
			"product_id":    formatUintID(product.ID),
			"name":          product.Name,
			"img":           product.Img,
			"category_name": product.CategoryName,
			"sell_num":      product.SellNum,
			"stock_num":     product.StockNum,
			"audit_time":    product.AuditTime,
			"gray_reason":   product.GrayReason,
		})
	}
	return map[string]any{
		"account":   t.account(account),
		"products":  products,
		"page_info": response.Data.PageInfo,
	}, nil
}

func (t *Tools) listOverallAwemes(ctx context.Context, args map[string]any) (any, error) {
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	productID, err := optionalID(args, "product_id")
	if err != nil {
		return nil, err
	}
	page := positiveIntArg(args, "page", 1)
	pageSize := clampInt(positiveIntArg(args, "page_size", 30), 1, 30)
	response, err := t.service.Client.OverallAwemeList(
		ctx,
		sdk.OverallAwemeListReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			ProductId:    productID,
			Page:         page,
			PageSize:     pageSize,
			CanApplyOnly: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("list overall awemes failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError(
			"list overall awemes failed",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	items := make([]map[string]any, 0, len(response.Data.BindAwemeUserInfos))
	for _, item := range response.Data.BindAwemeUserInfos {
		items = append(items, map[string]any{
			"aweme_id":                         formatUintID(item.AwemeUserInfo.AwemeID),
			"aweme_name":                       item.AwemeUserInfo.AwemeName,
			"aweme_show_id":                    item.AwemeUserInfo.AwemeShowID,
			"aweme_avatar":                     item.AwemeUserInfo.AwemeAvatar,
			"auth_types":                       item.AuthTypes,
			"has_authorized":                   item.HasAuthorized,
			"can_control_uniprom":              item.CanControlUniProm,
			"can_apply_uniprom":                item.CanApplyUniProm,
			"has_shop_permission":              item.HasShopPermission,
			"is_aweme_c":                       item.IsAwemeC,
			"is_product_uni_prom_disabled":     item.ProductUniPromDisabled,
			"product_disable_reasons":          item.ProductDisableReasons,
			"multi_aweme_uni_prom_auth_status": item.MultiAwemeAuthStatus,
			"is_allow_mall":                    item.ProductUniPromInfo.IsAllowMall,
		})
	}
	return map[string]any{
		"account":    t.account(account),
		"product_id": formatUintID(productID),
		"awemes":     items,
		"pagination": response.Data.Pagination,
	}, nil
}

const planCRUDTestPrefix = "Codex-CRUD-TEST-"
const overallPlanCRUDTestPrefix = "Codex-CRUD-OVERALL-TEST-"

func (t *Tools) createTestProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	name := stringArg(args, "name", "")
	if !validPlanCRUDTestName(name) {
		return nil, fmt.Errorf("name must start with %s", planCRUDTestPrefix)
	}
	productID, err := requiredID(args, "product_id")
	if err != nil {
		return nil, err
	}
	budget, ok := floatArg(args, "budget")
	if !ok || budget <= 0 || budget > 1 {
		return nil, errors.New("budget must be greater than 0 and at most 1 CNY for CRUD testing")
	}
	startDate, endDate, err := planCRUDTestDates(args)
	if err != nil {
		return nil, err
	}
	if err := t.verifyTestProduct(ctx, token, advertiserID, account.AwemeID, productID); err != nil {
		return nil, err
	}
	createResponse, err := t.service.Client.UniPromotionProductPlanCreate(
		ctx,
		sdk.UniPromotionProductPlanCreateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			Name:         name,
			AwemeId:      uint64(account.AwemeID),
			ProductId:    productID,
			Budget:       budget,
			StartDate:    startDate,
			EndDate:      endDate,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create test product plan failed: %w", err)
	}
	if createResponse.Code != 0 || createResponse.Data.AdId == 0 {
		return nil, apiError(
			"create test product plan failed",
			createResponse.Code,
			createResponse.Message,
			createResponse.RequestID,
		)
	}
	adID := createResponse.Data.AdId
	pauseResponse, pauseErr := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DISABLE",
		},
	)
	detail, detailErr := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	result := map[string]any{
		"created":            true,
		"ad_id":              formatUintID(adID),
		"name":               name,
		"marketing_goal":     "VIDEO_PROM_GOODS",
		"product_id":         formatUintID(productID),
		"budget":             budget,
		"start_date":         startDate,
		"end_date":           endDate,
		"paused_immediately": pauseErr == nil && pauseResponse != nil && pauseResponse.Code == 0,
		"account":            t.account(account),
	}
	if detailErr != nil {
		result["verified"] = false
		result["verification_error"] = detailErr.Error()
	} else if detail.Code != 0 || detail.Data == nil {
		result["verified"] = false
		result["verification_error"] = apiError(
			"read created test plan failed",
			detail.Code,
			detail.Message,
			detail.RequestID,
		).Error()
	} else {
		result["verified"] = testPlanIdentityValid(detail.Data)
		result["plan"] = detail.Data
		result["verified_test_identity"] = testPlanIdentityValid(detail.Data)
	}
	return result, nil
}

func (t *Tools) renameTestProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	newName := stringArg(args, "new_name", "")
	if !validPlanCRUDTestName(newName) {
		return nil, fmt.Errorf("new_name must start with %s", planCRUDTestPrefix)
	}
	before, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read test plan before rename failed: %w", err)
	}
	if before.Code != 0 || before.Data == nil {
		return nil, apiError("read test plan before rename failed", before.Code, before.Message, before.RequestID)
	}
	if !testPlanIdentityValid(before.Data) {
		return nil, fmt.Errorf(
			"refusing to rename non-test plan: ad_id=%s marketing_goal=%q name=%q",
			formatUintID(adID),
			before.Data.MarketingGoal,
			before.Data.Name,
		)
	}
	updateResponse, err := t.service.Client.UniPromotionNameUpdate(
		ctx,
		sdk.UniPromotionNameUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
			Name:         newName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("rename test product plan failed: %w", err)
	}
	if updateResponse.Code != 0 {
		return nil, apiError(
			"rename test product plan failed",
			updateResponse.Code,
			updateResponse.Message,
			updateResponse.RequestID,
		)
	}
	after, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return map[string]any{
			"renamed":      true,
			"verified":     false,
			"ad_id":        formatUintID(adID),
			"old_name":     before.Data.Name,
			"new_name":     newName,
			"verify_error": err.Error(),
		}, nil
	}
	if after.Code != 0 || after.Data == nil {
		return map[string]any{
			"renamed":  true,
			"verified": false,
			"ad_id":    formatUintID(adID),
			"old_name": before.Data.Name,
			"new_name": newName,
			"verify_error": apiError(
				"read renamed test plan failed",
				after.Code,
				after.Message,
				after.RequestID,
			).Error(),
		}, nil
	}
	return map[string]any{
		"renamed":     true,
		"verified":    after.Data.Name == newName,
		"ad_id":       formatUintID(adID),
		"old_name":    before.Data.Name,
		"new_name":    newName,
		"actual_name": after.Data.Name,
		"plan":        after.Data,
	}, nil
}

func (t *Tools) deleteTestProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	before, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read test plan before delete failed: %w", err)
	}
	if before.Code != 0 || before.Data == nil {
		return nil, apiError("read test plan before delete failed", before.Code, before.Message, before.RequestID)
	}
	if !testPlanIdentityValid(before.Data) {
		return nil, fmt.Errorf(
			"refusing to delete non-test plan: ad_id=%s marketing_goal=%q name=%q",
			formatUintID(adID),
			before.Data.MarketingGoal,
			before.Data.Name,
		)
	}
	pauseResponse, err := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DISABLE",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("pause test plan before delete failed: %w", err)
	}
	if pauseResponse.Code != 0 {
		return nil, apiError("pause test plan before delete failed", pauseResponse.Code, pauseResponse.Message)
	}
	deleteResponse, err := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DELETE",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("delete test product plan failed: %w", err)
	}
	if deleteResponse.Code != 0 {
		return nil, apiError("delete test product plan failed", deleteResponse.Code, deleteResponse.Message)
	}
	result := map[string]any{
		"deleted":        true,
		"ad_id":          formatUintID(adID),
		"name":           before.Data.Name,
		"marketing_goal": before.Data.MarketingGoal,
		"delete_result":  deleteResponse.Data,
	}
	verified, evidence := t.verifyDeletedPlan(ctx, token, advertiserID, adID)
	result["verified"] = verified
	result["verify_evidence"] = evidence
	return result, nil
}

func (t *Tools) createTestOverallProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	name := stringArg(args, "name", "")
	if !validOverallPlanCRUDTestName(name) {
		return nil, fmt.Errorf("name must start with %s", overallPlanCRUDTestPrefix)
	}
	productID, err := requiredID(args, "product_id")
	if err != nil {
		return nil, err
	}
	awemeID, err := requiredID(args, "aweme_id")
	if err != nil {
		return nil, err
	}
	budget, ok := floatArg(args, "budget")
	if !ok || !almostEqual(budget, sdk.OverallProductPlanMinimumBudget, 0.000001) {
		return nil, errors.New("budget must equal 300 CNY because overall product plans enforce a 300 CNY minimum")
	}
	roi2Goal, err := overallPlanCRUDTestROIGoal(args)
	if err != nil {
		return nil, err
	}
	startDate, endDate, err := planCRUDTestDates(args)
	if err != nil {
		return nil, err
	}
	if err := t.verifyTestProduct(ctx, token, advertiserID, account.AwemeID, productID); err != nil {
		return nil, err
	}
	if err := t.verifyOverallAweme(ctx, token, advertiserID, productID, awemeID); err != nil {
		return nil, err
	}
	createResponse, err := t.service.Client.OverallProductPlanCreate(
		ctx,
		sdk.OverallProductPlanCreateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			Name:         name,
			ProductId:    productID,
			AwemeId:      awemeID,
			Budget:       budget,
			Roi2Goal:     roi2Goal,
			StartDate:    startDate,
			EndDate:      endDate,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create test overall product plan failed: %w", err)
	}
	if createResponse.Code != 0 || createResponse.Data.AdId == 0 {
		return nil, apiError(
			"create test overall product plan failed",
			createResponse.Code,
			createResponse.Message,
			createResponse.RequestID,
		)
	}
	adID := createResponse.Data.AdId
	pauseResponse, pauseErr := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DISABLE",
		},
	)
	detail, detailErr := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	result := map[string]any{
		"created":        true,
		"ad_id":          formatUintID(adID),
		"name":           name,
		"plan_type":      "OVERALL_PROJECT",
		"marketing_goal": "VIDEO_PROM_GOODS",
		"product_id":     formatUintID(productID),
		"aweme_id":       formatUintID(awemeID),
		"budget":         budget,
		"roi2_goal":      roi2Goal,
		"start_date":     startDate,
		"end_date":       endDate,
		"paused_immediately": pauseErr == nil &&
			uniPromotionStatusUpdateSucceeded(pauseResponse, adID),
		"account": t.account(account),
	}
	if pauseErr != nil {
		result["pause_error"] = pauseErr.Error()
	} else if !uniPromotionStatusUpdateSucceeded(pauseResponse, adID) {
		result["pause_error"] = apiError(
			"pause created overall test plan failed",
			pauseResponse.Code,
			pauseResponse.Message,
			pauseResponse.RequestID,
		).Error()
	}
	if detailErr != nil {
		result["verified"] = false
		result["verification_error"] = detailErr.Error()
		return result, nil
	}
	if detail.Code != 0 || detail.Data == nil {
		result["verified"] = false
		result["verification_error"] = apiError(
			"read created overall test plan failed",
			detail.Code,
			detail.Message,
			detail.RequestID,
		).Error()
		return result, nil
	}
	result["verified"] = overallTestPlanPausedAndValid(detail.Data)
	result["verified_test_identity"] = overallTestPlanIdentityValid(detail.Data)
	result["plan"] = detail.Data
	return result, nil
}

func (t *Tools) renameTestOverallProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	newName := stringArg(args, "new_name", "")
	if !validOverallPlanCRUDTestName(newName) {
		return nil, fmt.Errorf("new_name must start with %s", overallPlanCRUDTestPrefix)
	}
	before, err := t.readTestOverallPlan(ctx, token, advertiserID, adID, "rename")
	if err != nil {
		return nil, err
	}
	productID, awemeID, err := overallPlanCreativeIdentity(before)
	if err != nil {
		return nil, err
	}
	updateResponse, err := t.service.Client.OverallProductPlanNameUpdate(
		ctx,
		sdk.OverallProductPlanNameUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
			Name:         newName,
			ProductId:    productID,
			AwemeId:      awemeID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("rename test overall product plan failed: %w", err)
	}
	if updateResponse.Code != 0 {
		return nil, apiError(
			"rename test overall product plan failed",
			updateResponse.Code,
			updateResponse.Message,
			updateResponse.RequestID,
		)
	}
	after, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	result := map[string]any{
		"renamed":  true,
		"ad_id":    formatUintID(adID),
		"old_name": before.Name,
		"new_name": newName,
	}
	if err != nil {
		result["verified"] = false
		result["verification_error"] = err.Error()
		return result, nil
	}
	if after.Code != 0 || after.Data == nil {
		result["verified"] = false
		result["verification_error"] = apiError(
			"read renamed overall test plan failed",
			after.Code,
			after.Message,
			after.RequestID,
		).Error()
		return result, nil
	}
	result["verified"] = overallTestPlanIdentityValid(after.Data) &&
		after.Data.Name == newName
	result["actual_name"] = after.Data.Name
	result["adlab_scene"] = after.Data.AdlabScene
	result["plan"] = after.Data
	return result, nil
}

func (t *Tools) deleteTestOverallProductPlan(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	before, err := t.readTestOverallPlan(ctx, token, advertiserID, adID, "delete")
	if err != nil {
		return nil, err
	}
	pauseResponse, err := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DISABLE",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("pause test overall plan before delete failed: %w", err)
	}
	if !uniPromotionStatusUpdateSucceeded(pauseResponse, adID) {
		return nil, apiError(
			"pause test overall plan before delete failed",
			pauseResponse.Code,
			pauseResponse.Message,
			pauseResponse.RequestID,
		)
	}
	deleteResponse, err := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "DELETE",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("delete overall test plan failed: %w", err)
	}
	if !uniPromotionStatusUpdateSucceeded(deleteResponse, adID) {
		return nil, apiError(
			"delete overall test plan failed",
			deleteResponse.Code,
			deleteResponse.Message,
			deleteResponse.RequestID,
		)
	}
	verified, evidence := t.verifyDeletedPlan(ctx, token, advertiserID, adID)
	return map[string]any{
		"deleted":         true,
		"ad_id":           formatUintID(adID),
		"name":            before.Name,
		"plan_type":       "OVERALL_PROJECT",
		"delete_result":   deleteResponse.Data,
		"verified":        verified,
		"verify_evidence": evidence,
	}, nil
}

func (t *Tools) verifyOverallAweme(
	ctx context.Context,
	token string,
	advertiserID int64,
	productID uint64,
	awemeID uint64,
) error {
	const pageSize = 30
	for page := 1; page <= 100; page++ {
		response, err := t.service.Client.OverallAwemeList(
			ctx,
			sdk.OverallAwemeListReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				ProductId:    productID,
				Page:         page,
				PageSize:     pageSize,
				CanApplyOnly: true,
			},
		)
		if err != nil {
			return fmt.Errorf("verify overall aweme list failed: %w", err)
		}
		if response.Code != 0 {
			return apiError(
				"verify overall aweme list failed",
				response.Code,
				response.Message,
				response.RequestID,
			)
		}
		for _, item := range response.Data.BindAwemeUserInfos {
			if item.AwemeUserInfo.AwemeID != awemeID {
				continue
			}
			if item.ProductUniPromDisabled {
				return fmt.Errorf(
					"aweme_id %d is disabled for overall product delivery: %s",
					awemeID,
					strings.Join(item.ProductDisableReasons, ","),
				)
			}
			if !item.HasAuthorized && !item.CanControlUniProm && !item.CanApplyUniProm &&
				item.MultiAwemeAuthStatus != "APPROVED" &&
				item.MultiAwemeAuthStatus != "ALLIANCE_AUTO_AUTH" {
				return fmt.Errorf(
					"aweme_id %d has no overall product authorization; status=%q",
					awemeID,
					item.MultiAwemeAuthStatus,
				)
			}
			return nil
		}
		if len(response.Data.BindAwemeUserInfos) < pageSize {
			break
		}
	}
	return fmt.Errorf(
		"aweme_id %d is not in the official overall available list for product_id %d",
		awemeID,
		productID,
	)
}

func (t *Tools) readTestOverallPlan(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	action string,
) (*sdk.UniPromotionDetail, error) {
	response, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read test overall plan before %s failed: %w", action, err)
	}
	if response.Code != 0 || response.Data == nil {
		return nil, apiError(
			"read test overall plan before "+action+" failed",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	if !overallTestPlanIdentityValid(response.Data) {
		return nil, fmt.Errorf(
			"refusing to %s non-overall-test plan: ad_id=%s adlab_scene=%q marketing_goal=%q name=%q",
			action,
			formatUintID(adID),
			response.Data.AdlabScene,
			response.Data.MarketingGoal,
			response.Data.Name,
		)
	}
	return response.Data, nil
}

func (t *Tools) verifyDeletedPlan(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
) (bool, map[string]any) {
	evidence := map[string]any{}
	detail, detailErr := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if detailErr != nil {
		evidence["detail_error"] = detailErr.Error()
	} else {
		evidence["detail_code"] = detail.Code
		evidence["detail_message"] = detail.Message
		if detail.Code != 0 || detail.Data == nil {
			evidence["detail_absent"] = true
		}
		if detail.Data != nil {
			evidence["detail_status"] = detail.Data.Status
			evidence["detail_opt_status"] = detail.Data.OptStatus
			if isDeletedStatus(detail.Data.Status) || isDeletedStatus(detail.Data.OptStatus) {
				return true, evidence
			}
		}
	}
	list, listErr := t.service.Client.UniPromotionList(
		ctx,
		sdk.UniPromotionListReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			StartTime:    time.Now().In(shanghaiLocation).AddDate(0, 0, -1).Format("2006-01-02 15:04:05"),
			EndTime:      time.Now().In(shanghaiLocation).AddDate(0, 0, 1).Format("2006-01-02 15:04:05"),
			Page:         1,
			PageSize:     100,
			Filtering:    &sdk.UniPromotionListFiltering{AdIds: []uint64{adID}},
		},
	)
	if listErr != nil {
		evidence["list_error"] = listErr.Error()
		return false, evidence
	}
	evidence["list_code"] = list.Code
	evidence["list_message"] = list.Message
	if list.Code != 0 {
		return false, evidence
	}
	for _, plan := range list.Data.List {
		if plan.AdId != adID {
			continue
		}
		evidence["list_status"] = plan.Status
		evidence["list_opt_status"] = plan.OptStatus
		return isDeletedStatus(plan.Status) || isDeletedStatus(plan.OptStatus), evidence
	}
	evidence["list_absent"] = true
	return false, evidence
}

func isDeletedStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DELETE", "DELETED":
		return true
	default:
		return false
	}
}

func planCRUDTestDates(args map[string]any) (string, string, error) {
	todayDate := time.Now().In(shanghaiLocation)
	startDate := stringArg(args, "start_date", todayDate.AddDate(0, 0, 1).Format("2006-01-02"))
	endDate := stringArg(args, "end_date", startDate)
	startParsed, err := time.ParseInLocation("2006-01-02", startDate, shanghaiLocation)
	if err != nil {
		return "", "", errors.New("start_date must use YYYY-MM-DD")
	}
	endParsed, err := time.ParseInLocation("2006-01-02", endDate, shanghaiLocation)
	if err != nil {
		return "", "", errors.New("end_date must use YYYY-MM-DD")
	}
	todayStart := time.Date(
		todayDate.Year(),
		todayDate.Month(),
		todayDate.Day(),
		0,
		0,
		0,
		0,
		shanghaiLocation,
	)
	if !startParsed.After(todayStart) {
		return "", "", errors.New("start_date must be tomorrow or later to prevent test consumption")
	}
	if endParsed.Before(startParsed) {
		return "", "", errors.New("end_date cannot be earlier than start_date")
	}
	return startDate, endDate, nil
}

func overallPlanCRUDTestROIGoal(args map[string]any) (float64, error) {
	const defaultROI2Goal = 1.90
	if _, provided := args["roi2_goal"]; !provided {
		return defaultROI2Goal, nil
	}
	value, ok := args["roi2_goal"].(float64)
	if !ok {
		return 0, errors.New("roi2_goal must be a number")
	}
	if value <= 0 || value > 100 {
		return 0, errors.New("roi2_goal must be greater than 0 and at most 100")
	}
	return value, nil
}

func validPlanCRUDTestName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), planCRUDTestPrefix) &&
		len(strings.TrimSpace(name)) > len(planCRUDTestPrefix)
}

func validOverallPlanCRUDTestName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), overallPlanCRUDTestPrefix) &&
		len(strings.TrimSpace(name)) > len(overallPlanCRUDTestPrefix)
}

func testPlanIdentityValid(plan *sdk.UniPromotionDetail) bool {
	return plan != nil &&
		plan.MarketingGoal == "VIDEO_PROM_GOODS" &&
		validPlanCRUDTestName(plan.Name)
}

func overallTestPlanIdentityValid(plan *sdk.UniPromotionDetail) bool {
	return plan != nil &&
		plan.MarketingGoal == "VIDEO_PROM_GOODS" &&
		plan.AdlabScene == sdk.AdlabSceneOverall &&
		validOverallPlanCRUDTestName(plan.Name)
}

func overallTestPlanPausedAndValid(plan *sdk.UniPromotionDetail) bool {
	return overallTestPlanIdentityValid(plan) &&
		strings.EqualFold(plan.Status, "DISABLE") &&
		strings.EqualFold(plan.OptStatus, "DISABLE")
}

func overallPlanCreativeIdentity(plan *sdk.UniPromotionDetail) (uint64, uint64, error) {
	if plan == nil {
		return 0, 0, errors.New("overall plan detail is empty")
	}
	var productID uint64
	for _, product := range plan.ProductInfos {
		if product.ProductID != 0 {
			productID = product.ProductID
			break
		}
	}
	if productID == 0 {
		return 0, 0, errors.New("overall plan detail does not contain product_id")
	}
	awemeID := plan.AwemeId
	if awemeID == 0 {
		for _, aweme := range plan.AwemeInfos {
			if aweme.AwemeUID != 0 {
				awemeID = aweme.AwemeUID
				break
			}
		}
	}
	if awemeID == 0 {
		return 0, 0, errors.New("overall plan detail does not contain aweme_id")
	}
	return productID, awemeID, nil
}

func uniPromotionStatusUpdateSucceeded(response *sdk.UniPromotionStatusUpdateRes, adID uint64) bool {
	if response == nil || response.Code != 0 {
		return false
	}
	for _, result := range response.Data.Results {
		if result.AdId == adID && (result.Success || result.Flag) &&
			result.ErrorCode == 0 && result.ErrorMessage == "" {
			return true
		}
	}
	for _, updatedID := range response.Data.AdIds {
		if updatedID == adID {
			return true
		}
	}
	return false
}

func closestProductPageSize(requested int) int {
	for _, size := range []int{10, 20, 30} {
		if requested <= size {
			return size
		}
	}
	return 30
}

func (t *Tools) requireWrite(args map[string]any) error {
	if !t.writeEnabled {
		return errors.New("operations write is paused: OPERATIONS_WRITE_ENABLED must be true")
	}
	confirm, ok := args["confirm"].(bool)
	if !ok || !confirm {
		return errors.New("confirm=true is required")
	}
	return nil
}

func (t *Tools) requireMaterialChaseWrite(args map[string]any) error {
	if !t.materialChaseWriteEnabled {
		return errors.New("material chase write is paused: MATERIAL_CHASE_WRITE_ENABLED must be true")
	}
	confirm, ok := args["confirm"].(bool)
	if !ok || !confirm {
		return errors.New("confirm=true is required")
	}
	return nil
}

func (t *Tools) requirePlanCRUDTestWrite(args map[string]any) error {
	if !t.planCRUDTestWriteEnabled {
		return errors.New("plan CRUD test writes are paused: PLAN_CRUD_TEST_WRITE_ENABLED must be true")
	}
	confirm, ok := args["confirm"].(bool)
	if !ok || !confirm {
		return errors.New("confirm=true is required")
	}
	return nil
}

func (t *Tools) tokenFor(ctx context.Context, account tenantstore.AccountBinding) (string, int64, error) {
	if t.service.TokenManager == nil {
		return "", 0, errors.New("token manager is not initialized")
	}
	return t.service.AccessTokenFor(ctx, account.AdvertiserID)
}

func (t *Tools) resolveReportAnchorID(
	ctx context.Context,
	token string,
	advertiserID int64,
	account tenantstore.AccountBinding,
) (uint64, error) {
	const pageSize = 100
	var all []sdk.UniPromotionAweme
	for page := 1; page <= 100; page++ {
		response, err := t.service.Client.UniPromotionAuthorizedGet(
			ctx,
			sdk.UniPromotionAuthorizedGetReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				Page:         page,
				PageSize:     pageSize,
			},
		)
		if err != nil {
			return 0, fmt.Errorf("获取官方报表可用抖音号失败: %w", err)
		}
		if response.Code != 0 {
			return 0, apiError(
				"获取官方报表可用抖音号失败",
				response.Code,
				response.Message,
			)
		}
		all = append(all, response.Data.List...)
		if len(response.Data.List) < pageSize ||
			(response.Data.PageInfo.TotalPage > 0 && page >= response.Data.PageInfo.TotalPage) {
			break
		}
	}
	return selectReportAnchorID(all, account.AwemeID)
}

func selectReportAnchorID(list []sdk.UniPromotionAweme, bindingID int64) (uint64, error) {
	binding := strings.TrimSpace(formatID(bindingID))
	for _, item := range list {
		if item.AwemeId == 0 {
			continue
		}
		if strings.TrimSpace(formatUintID(item.AwemeId)) == binding ||
			strings.TrimSpace(item.AwemeShowId) == binding {
			return item.AwemeId, nil
		}
	}
	if len(list) == 1 && list[0].AwemeId > 0 {
		return list[0].AwemeId, nil
	}
	if len(list) == 0 {
		return 0, fmt.Errorf("账户 %s 没有可用于官方素材报表的抖音号", formatID(bindingID))
	}
	return 0, fmt.Errorf("账户 %s 绑定的抖音号未出现在官方全域授权列表，共 %d 个可用抖音号",
		formatID(bindingID), len(list))
}

func (t *Tools) account(account tenantstore.AccountBinding) map[string]string {
	return map[string]string{
		"account_id":    formatID(account.AdvertiserID),
		"advertiser_id": formatID(account.AdvertiserID),
		"aweme_id":      formatID(account.AwemeID),
		"live_room_id":  formatID(account.LiveRoomID),
	}
}

func (t *Tools) fetchPlanMaterials(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	startDate string,
	endDate string,
) ([]MaterialMetrics, []string, error) {
	const perPage = 100
	all := make([]MaterialMetrics, 0, perPage)
	warnings := make([]string, 0)
	for page := 1; page <= 100; page++ {
		response, err := t.service.Client.UniPromotionMaterialGet(ctx, sdk.UniPromotionMaterialGetReq{
			AccessToken:        token,
			AdvertiserId:       uint64(advertiserID),
			AdId:               adID,
			MaterialType:       "VIDEO",
			Status:             "ALL",
			MaterialSelectType: "ALL",
			StartDate:          startDate,
			EndDate:            endDate,
			Fields: []string{
				"stat_cost_for_roi2",
				"live_show_count_for_roi2_v2",
				"live_watch_count_for_roi2_v2",
				"live_cvr_rate_for_roi2_v2",
				"live_convert_rate_for_roi2_v2",
				"total_pay_order_gmv_for_roi2",
				"total_pay_order_count_for_roi2",
				"total_prepay_and_pay_order_roi2",
			},
			OrderField: "stat_cost_for_roi2",
			OrderType:  "DESC",
			Page:       page,
			PageSize:   perPage,
		})
		if err != nil {
			return nil, warnings, fmt.Errorf("get plan materials page %d: %w", page, err)
		}
		if response.Code != 0 {
			return nil, warnings, apiError("获取计划素材失败", response.Code, response.Message)
		}
		for _, raw := range response.Data.Materials {
			all = append(all, normalizeMaterial(raw, len(all)+1))
		}
		if len(response.Data.Materials) < perPage ||
			response.Data.PageInfo.TotalPage <= page {
			break
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Cost == all[j].Cost {
			return all[i].MaterialID > all[j].MaterialID
		}
		return all[i].Cost > all[j].Cost
	})
	for index := range all {
		all[index].CostRank = index + 1
		all[index].ProtectedTop3 = index < DefaultRuleConfig().ProtectedTopN
		all[index].EligibleReplace = !all[index].ProtectedTop3
	}
	if len(all) < 3 {
		warnings = append(warnings, "当前计划素材不足3个，没有形成完整保护名单")
	}
	return all, warnings, nil
}

func normalizeMaterial(raw sdk.UniPromotionMaterialInfo, rank int) MaterialMetrics {
	return MaterialMetrics{
		MaterialID:      formatUintID(raw.MaterialId),
		Title:           raw.MaterialName,
		VideoID:         raw.VideoId,
		AwemeItemID:     formatUintID(raw.AwemeItemId),
		Status:          raw.MaterialStatus,
		Cost:            raw.StatCost,
		ShowCount:       raw.ShowCnt,
		ClickCount:      raw.ClickCnt,
		CTR:             raw.Ctr,
		ConvertCount:    raw.ConvertCnt,
		ConversionRate:  raw.ConversionRate,
		PayOrderCount:   float64(raw.ConvertCnt),
		PayOrderGMV:     raw.PayOrderAmount,
		ROI:             raw.Roi2,
		GPM:             raw.Gpm,
		CostRank:        rank,
		ProtectedTop3:   rank <= 3,
		EligibleReplace: rank > 3,
	}
}

func reportDimensionFields(config sdk.UniPromotionReportTopicConfig) []string {
	preferred := []string{
		"material_id",
		"roi2_material_video_name",
		"material_name",
		"video_id",
		"roi2_material_status",
		"material_status",
		"roi2_material_show_status",
		"roi2_material_show_status_reason",
		"roi2_material_video_type",
	}
	available := make(map[string]sdk.UniPromotionReportDimension, len(config.Dimensions))
	for _, dimension := range config.Dimensions {
		if dimension.Field == "" || dimension.FilterOnly {
			continue
		}
		available[dimension.Field] = dimension
	}
	fields := make([]string, 0, len(preferred))
	for _, field := range preferred {
		if _, ok := available[field]; ok && !containsString(fields, field) {
			fields = append(fields, field)
		}
	}
	for _, dimension := range config.Dimensions {
		if dimension.Field == "" || dimension.FilterOnly || !dimension.IsRequired {
			continue
		}
		if !containsString(fields, dimension.Field) {
			fields = append(fields, dimension.Field)
		}
	}
	limit := config.MaxDimensionLimit()
	if limit > 0 && len(fields) > limit {
		fields = fields[:limit]
	}
	return fields
}

func reportMetricFields(config sdk.UniPromotionReportTopicConfig) []string {
	preferred := []string{
		"stat_cost_for_roi2",
		"stat_cost",
		"live_show_count_for_roi2_v2",
		"live_show_count_for_roi2",
		"live_watch_count_for_roi2_v2",
		"live_watch_count_for_roi2",
		"live_cvr_rate_for_roi2_v2",
		"live_convert_rate_for_roi2_v2",
		"total_pay_order_count_for_roi2",
		"total_pay_order_gmv_include_coupon_for_roi2",
		"total_pay_order_gmv_for_roi2",
		"total_prepay_and_pay_order_roi2",
		"total_ecpm_for_roi2",
	}
	available := make(map[string]sdk.UniPromotionReportMetric, len(config.Metrics))
	for _, metric := range config.Metrics {
		if metric.Field != "" {
			available[metric.Field] = metric
		}
	}
	fields := make([]string, 0, len(preferred))
	for _, field := range preferred {
		if _, ok := available[field]; ok {
			fields = append(fields, field)
		}
	}
	return fields
}

func reportPageSize(value int) int {
	allowed := []int{10, 20, 50, 100, 200}
	if value <= allowed[0] {
		return allowed[0]
	}
	for _, size := range allowed {
		if value <= size {
			return size
		}
	}
	return allowed[len(allowed)-1]
}

func reportFilters(
	config sdk.UniPromotionReportTopicConfig,
	startDate string,
	endDate string,
	account tenantstore.AccountBinding,
	anchorID uint64,
) ([]sdk.UniPromotionReportFilter, error) {
	filters := make([]sdk.UniPromotionReportFilter, 0)
	optional := []sdk.UniPromotionReportFilter{}
	for _, dimension := range config.Dimensions {
		if dimension.FilterConfig == nil {
			continue
		}
		values := reportFilterValues(dimension, startDate, endDate, account, anchorID)
		if len(values) == 0 && dimension.FilterConfig.FilterRequired {
			return nil, fmt.Errorf("官方报表必填筛选 %s 没有可用值", dimension.Field)
		}
		operator := dimension.FilterConfig.Operator
		if operator == 0 {
			operator = 7
		}
		filter := sdk.UniPromotionReportFilter{
			Field:    dimension.Field,
			Operator: operator,
			Values:   values,
		}
		if dimension.FilterConfig.FilterRequired {
			filters = append(filters, filter)
			continue
		}
		switch strings.ToLower(dimension.Field) {
		case "roi2_material_video_type", "roi2_material_type_v3":
			optional = append(optional, filter)
		}
	}
	filters = append(filters, optional...)
	return filters, nil
}

func reportFilterValues(
	dimension sdk.UniPromotionReportDimension,
	startDate string,
	endDate string,
	account tenantstore.AccountBinding,
	anchorID uint64,
) []string {
	if dimension.FilterConfig == nil {
		return nil
	}
	field := strings.ToLower(dimension.Field)
	switch field {
	case "start_date":
		return []string{startDate}
	case "end_date":
		return []string{endDate}
	case "anchor_id", "aweme_id":
		if anchorID > 0 {
			return []string{formatUintID(anchorID)}
		}
	case "advertiser_id":
		if account.AdvertiserID > 0 {
			return []string{formatID(account.AdvertiserID)}
		}
	}

	// 千川全域配置的枚举格式为 label=接口代码、value=中文展示值。
	codes := make([]string, 0, len(dimension.FilterConfig.RangeValues))
	for _, option := range dimension.FilterConfig.RangeValues {
		if code := strings.TrimSpace(option.Label); code != "" {
			codes = append(codes, code)
		}
	}
	if len(codes) > 0 {
		preferred := preferredReportFilterValue(field, codes)
		if preferred != "" {
			return []string{preferred}
		}
		return codes[:1]
	}
	return nil
}

func preferredReportFilterValue(field string, values []string) string {
	preferences := map[string][]string{
		"query_type":               {"all"},
		"marketing_goal":           {"2"},
		"aggregate_smart_bid_type": {"0", "7"},
		"ecp_app_id":               {"1", "2"},
		"roi2_material_status":     {"1", "2"},
		"roi2_material_type_v3":    {"3"},
		"roi2_material_video_type": {"1", "2", "3"},
		"roi2_material_advice":     {"6", "8", "10", "4", "5"},
	}
	for _, candidate := range preferences[field] {
		if containsString(values, candidate) {
			return candidate
		}
	}
	return ""
}

func reportCostMetric(fields []string) string {
	for _, candidate := range []string{
		"stat_cost_for_roi2",
		"stat_cost",
		"total_cost",
	} {
		if containsString(fields, candidate) {
			return candidate
		}
	}
	return fields[0]
}

func materialFromReportRow(row sdk.UniPromotionReportRow, rank int) MaterialMetrics {
	dimensions := row.Dimensions
	metrics := row.Metrics
	materialID := anyID(dimensions, "material_id", "video_material_id", "mid")
	showCount := anyInt64(metrics, "live_show_count_for_roi2_v2", "show_cnt", "show_count", "product_show_count_for_roi2")
	clickCount := anyInt64(metrics, "live_watch_count_for_roi2_v2", "click_cnt", "watch_count", "product_click_count_for_roi2")
	convertCount := anyInt64(metrics, "total_pay_order_count_for_roi2", "convert_cnt", "conversion_count")
	cost := anyFloat64(metrics, "stat_cost_for_roi2", "stat_cost", "cost")
	gmv := anyFloat64(
		metrics,
		"total_pay_order_gmv_for_roi2",
		"total_pay_order_gmv_include_coupon_for_roi2",
		"pay_order_gmv",
	)
	ctr := anyFloat64(metrics, "live_cvr_rate_for_roi2_v2", "ctr", "click_rate")
	conversionRate := anyFloat64(metrics, "live_convert_rate_for_roi2_v2", "conversion_rate", "convert_rate")
	if ctr == 0 && showCount > 0 {
		ctr = float64(clickCount) / float64(showCount) * 100
	}
	if conversionRate == 0 && clickCount > 0 {
		conversionRate = float64(convertCount) / float64(clickCount) * 100
	}
	roi := anyFloat64(metrics, "total_prepay_and_pay_order_roi2", "pay_order_roi", "roi")
	gpm := anyFloat64(metrics, "gpm", "total_live_pay_order_gpm")
	if gpm == 0 && showCount > 0 {
		gpm = gmv / float64(showCount) * 1000
	}
	return MaterialMetrics{
		MaterialID:      materialID,
		Title:           anyString(dimensions, "roi2_material_video_name", "material_name", "title", "video_title"),
		VideoID:         anyString(dimensions, "video_id"),
		AwemeItemID:     anyID(dimensions, "aweme_item_id"),
		Status:          anyString(dimensions, "roi2_material_status", "material_status"),
		Cost:            cost,
		ShowCount:       showCount,
		ClickCount:      clickCount,
		CTR:             ctr,
		ConvertCount:    convertCount,
		ConversionRate:  conversionRate,
		PayOrderCount:   float64(convertCount),
		PayOrderGMV:     gmv,
		ROI:             roi,
		GPM:             gpm,
		CostRank:        rank,
		ProtectedTop3:   rank <= 3,
		EligibleReplace: rank > 3,
		MetricSource:    "qianchuan_report_uni_promotion",
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func anyValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return unwrapReportValue(value)
		}
	}
	return nil
}

func unwrapReportValue(value any) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	for _, key := range []string{"Value", "ValueStr", "ShortValueStr"} {
		if nested, exists := object[key]; exists && nested != nil {
			return nested
		}
	}
	return value
}

func anyID(values map[string]any, keys ...string) string {
	value := anyValue(values, keys...)
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return ""
	}
}

func anyString(values map[string]any, keys ...string) string {
	value := anyValue(values, keys...)
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func anyFloat64(values map[string]any, keys ...string) float64 {
	value := anyValue(values, keys...)
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case uint64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	default:
		return 0
	}
}

func anyInt64(values map[string]any, keys ...string) int64 {
	value := anyValue(values, keys...)
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func selectLivingRoom(rooms []map[string]any) (map[string]any, []string) {
	warnings := make([]string, 0)
	var selected map[string]any
	for _, room := range rooms {
		if !strings.EqualFold(anyString(room, "room_status"), "LIVING") {
			continue
		}
		if selected == nil || roomStartTime(room) > roomStartTime(selected) {
			selected = room
		}
	}
	if selected == nil {
		warnings = append(warnings, "今日直播间列表中没有 LIVING 状态房间，已回退到今日直播汇总数据")
	}
	if len(rooms) == 0 {
		warnings = append(warnings, "今日直播间列表返回空数据")
	}
	return selected, warnings
}

func materialPage(materials []MaterialMetrics, page, pageSize int) map[string]any {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > len(materials) {
		start = len(materials)
	}
	end := start + pageSize
	if end > len(materials) {
		end = len(materials)
	}
	totalPage := 0
	if len(materials) > 0 {
		totalPage = (len(materials) + pageSize - 1) / pageSize
	}
	return map[string]any{
		"materials":      materials[start:end],
		"page_info":      sdk.PageInfo{Page: page, PageSize: pageSize, TotalNumber: int64(len(materials)), TotalPage: totalPage},
		"material_count": len(materials),
		"protected_top3": protectedTop3(materials),
	}
}

func roomStartTime(room map[string]any) string {
	return anyString(room, "start_time")
}

func rawJSONValue(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	if value[0] == '"' {
		var text string
		if json.Unmarshal(value, &text) == nil {
			return text
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return string(value)
	}
	return decoded
}

func liveRoomIDFromMap(room map[string]any) string {
	return anyID(room, "room_id")
}

func normalizeLiveReportAmounts(value any) any {
	object, ok := value.(map[string]any)
	if !ok || object == nil {
		return value
	}
	return normalizeLiveReportAmountsMap(object)
}

func normalizeLiveReportAmountsMap(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	for _, key := range []string{
		"stat_cost",
		"luban_live_pay_order_gmv",
		"live_pay_order_gmv_alias",
		"live_order_pay_coupon_amount",
	} {
		if amount, ok := result[key]; ok {
			number := anyFloat64(map[string]any{"value": amount}, "value")
			if number != 0 {
				result[key] = number / 100000
			}
		}
	}
	return result
}

func protectedTop3(materials []MaterialMetrics) []MaterialMetrics {
	result := make([]MaterialMetrics, 0, 3)
	for _, material := range materials {
		if material.ProtectedTop3 {
			result = append(result, material)
		}
	}
	return result
}

func weightedRate(materials []MaterialMetrics, field string) float64 {
	show, click, convert := int64(0), int64(0), int64(0)
	for _, material := range materials {
		show += material.ShowCount
		click += material.ClickCount
		convert += material.ConvertCount
	}
	switch field {
	case "ctr":
		if show > 0 {
			return float64(click) / float64(show)
		}
	case "conversion_rate":
		if click > 0 {
			return float64(convert) / float64(click)
		}
	}
	return 0
}

func (t *Tools) fetchMaterialLibrary(
	ctx context.Context,
	token string,
	advertiserID int64,
	startDate string,
	endDate string,
) ([]MaterialMetrics, error) {
	const dataTopic = "OVERALL_ROI_LIVE_MATERIAL_VIDEO"
	configResponse, err := t.service.Client.UniPromotionReportConfig(
		ctx,
		sdk.UniPromotionReportConfigReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			DataTopics:   []string{dataTopic},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read material report config failed: %w", err)
	}
	if configResponse.Code != 0 {
		return nil, apiError(
			"read material report config failed",
			configResponse.Code,
			configResponse.Message,
			configResponse.RequestID,
		)
	}
	var topicConfig *sdk.UniPromotionReportTopicConfig
	for index := range configResponse.Data.CustomConfigDatas {
		if configResponse.Data.CustomConfigDatas[index].DataTopic == dataTopic {
			topicConfig = &configResponse.Data.CustomConfigDatas[index]
			break
		}
	}
	if topicConfig == nil {
		return nil, fmt.Errorf("official report config omitted %s", dataTopic)
	}
	dimensionFields := reportDimensionFields(*topicConfig)
	metricFields := reportMetricFields(*topicConfig)
	account := tenantstore.AccountBinding{AdvertiserID: advertiserID}
	for _, configured := range t.accountList() {
		if configured.AdvertiserID == advertiserID {
			account = configured
			break
		}
	}
	anchorID, err := t.resolveReportAnchorID(ctx, token, advertiserID, account)
	if err != nil {
		return nil, err
	}
	filters, err := reportFilters(*topicConfig, startDate, endDate, account, anchorID)
	if err != nil {
		return nil, err
	}
	response, err := t.service.Client.UniPromotionReportData(
		ctx,
		sdk.UniPromotionReportDataReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			DataTopic:    dataTopic,
			Dimensions:   dimensionFields,
			Metrics:      metricFields,
			Filters:      filters,
			StartTime:    startDate + " 00:00:00",
			EndTime:      endDate + " 23:59:59",
			OrderBy:      []sdk.UniPromotionReportOrder{{Type: 2, Field: reportCostMetric(metricFields)}},
			Page:         1,
			PageSize:     200,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read material report failed: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("read material report failed", response.Code, response.Message, response.RequestID)
	}
	materials := make([]MaterialMetrics, 0, len(response.Data.Rows))
	for index, row := range response.Data.Rows {
		material := materialFromReportRow(row, index+1)
		config := DefaultRuleConfig()
		material.CTRPassed = boolPointer(thresholdReached(material.CTR, config.MinimumCTR))
		material.ConversionPassed = boolPointer(
			thresholdReached(material.ConversionRate, config.MinimumConversionRate),
		)
		material.BothThresholdsPassed = *material.CTRPassed && *material.ConversionPassed
		materials = append(materials, material)
	}
	sort.SliceStable(materials, func(i, j int) bool {
		if materials[i].Cost == materials[j].Cost {
			return materials[i].MaterialID > materials[j].MaterialID
		}
		return materials[i].Cost > materials[j].Cost
	})
	for index := range materials {
		materials[index].CostRank = index + 1
		materials[index].ProtectedTop3 = index < DefaultRuleConfig().ProtectedTopN
		materials[index].EligibleReplace = !materials[index].ProtectedTop3
	}
	return materials, nil
}

func materialThresholdPassed(material MaterialMetrics, config RuleConfig) bool {
	return thresholdReached(material.CTR, config.MinimumCTR) ||
		thresholdReached(material.ConversionRate, config.MinimumConversionRate)
}

func dualThresholdMaterials(materials []MaterialMetrics, config RuleConfig) []MaterialMetrics {
	result := make([]MaterialMetrics, 0)
	for _, material := range materials {
		if thresholdReached(material.CTR, config.MinimumCTR) &&
			thresholdReached(material.ConversionRate, config.MinimumConversionRate) {
			result = append(result, material)
		}
	}
	return result
}

func thresholdReached(actual, minimum float64) bool {
	return actual+0.000000001 >= minimum
}

func almostEqual(actual, expected, tolerance float64) bool {
	difference := actual - expected
	if difference < 0 {
		difference = -difference
	}
	return difference <= tolerance
}

func boolPointer(value bool) *bool {
	return &value
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func sameStrings(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	left := append([]string(nil), first...)
	right := append([]string(nil), second...)
	sort.Strings(left)
	sort.Strings(right)
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameIDSet(materials []sdk.UniPromotionControlTaskMaterial, expected []string) bool {
	return sameStrings(controlTaskMaterialIDs(materials), expected)
}

func controlTaskMaterialIDs(materials []sdk.UniPromotionControlTaskMaterial) []string {
	result := make([]string, 0, len(materials))
	for _, material := range materials {
		result = append(result, formatUintID(material.MaterialId))
	}
	return result
}

func mustNumericIDs(values []string) []uint64 {
	result, err := numericIDList(values)
	if err != nil {
		panic(err)
	}
	return result
}

func controlTaskFields() []string {
	return []string{
		"stat_cost_for_roi2_assist",
		"show_cnt_for_roi2_assist",
		"click_cnt_for_roi2_assist",
		"ctr_for_roi2_assist",
		"convert_rate_for_roi2_assist",
		"total_pay_order_count_for_roi2_assist",
		"total_pay_order_gmv_for_roi2_assist",
		"total_prepay_and_pay_order_roi2_assist",
		"total_prepay_and_pay_settle_roi2_1h_assist",
		"total_order_settle_count_for_roi2_1h_assist",
		"total_refund_order_gmv_for_roi2_1h_rate_assist",
	}
}

func closestControlTaskPageSize(value int) int {
	for _, allowed := range []int{10, 20, 50, 100} {
		if value <= allowed {
			return allowed
		}
	}
	return 100
}

func planMap(plan sdk.UniPromotion) map[string]any {
	return map[string]any{
		"ad_id":           formatUintID(plan.AdId),
		"name":            plan.AdName,
		"status":          plan.Status,
		"opt_status":      plan.OptStatus,
		"budget":          plan.Budget,
		"roi2_goal":       plan.Roi2Goal,
		"aweme_id":        formatUintID(plan.AwemeId),
		"cost":            plan.StatCost,
		"pay_order_count": plan.PayOrderCount,
		"pay_order_gmv":   plan.PayOrderGMV,
		"roi":             plan.PayOrderROI,
	}
}

func apiError(prefix string, code int64, message string, requestIDs ...string) error {
	for _, requestID := range requestIDs {
		if requestID = strings.TrimSpace(requestID); requestID != "" {
			return fmt.Errorf("%s[%d]: %s (request_id=%s)", prefix, code, message, requestID)
		}
	}
	return fmt.Errorf("%s[%d]: %s", prefix, code, message)
}

func formatID(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func formatUintID(id uint64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

func today() string {
	return time.Now().In(shanghaiLocation).Format("2006-01-02")
}

var shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func businessDayRange(now time.Time) (string, string) {
	local := now.In(shanghaiLocation)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, shanghaiLocation)
	return start.Format("2006-01-02 15:04:05"),
		start.Add(24*time.Hour - time.Second).Format("2006-01-02 15:04:05")
}

func stringArg(args map[string]any, key, fallback string) string {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func positiveIntArg(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case float64:
		if value > 0 {
			return int(value)
		}
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func requiredID(args map[string]any, key string) (uint64, error) {
	value, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func optionalID(args map[string]any, key string) (uint64, error) {
	value, ok := args[key]
	if !ok {
		return 0, nil
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("%s must be a positive integer when provided", key)
	}
	return parsed, nil
}

func (t *Tools) verifyTestProduct(
	ctx context.Context,
	token string,
	advertiserID int64,
	awemeID int64,
	productID uint64,
) error {
	const pageSize = 30
	for page := 1; page <= 100; page++ {
		response, err := t.service.Client.UniPromotionProductAvailableGet(
			ctx,
			sdk.UniPromotionProductAvailableGetReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AwemeId:      uint64(awemeID),
				Page:         page,
				PageSize:     pageSize,
				ProductIDs:   []uint64{productID},
			},
		)
		if err != nil {
			return fmt.Errorf("verify product ID failed: %w", err)
		}
		if response.Code != 0 {
			return apiError(
				"verify product ID failed",
				response.Code,
				response.Message,
				response.RequestID,
			)
		}
		for _, product := range response.Data.ProductList {
			if product.ID == productID {
				return nil
			}
		}
		if len(response.Data.ProductList) < pageSize ||
			(response.Data.PageInfo.TotalPage > 0 && page >= response.Data.PageInfo.TotalPage) {
			break
		}
	}
	return fmt.Errorf("product_id %d is not in the account's official available product list", productID)
}

func numericIDList(values []string) ([]uint64, error) {
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			return nil, fmt.Errorf("invalid ID: %s", value)
		}
		result = append(result, parsed)
	}
	return result, nil
}

func floatArg(args map[string]any, key string) (float64, bool) {
	switch value := args[key].(type) {
	case float64:
		return value, true
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round(value float64, precision int) float64 {
	scale := math.Pow10(precision)
	return math.Round(value*scale) / scale
}
