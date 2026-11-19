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
	service        *service.QianchuanService
	writeEnabled   bool
	authURL        string
	tenant         TenantIdentity
	accounts       []tenantstore.AccountBinding
	authURLBuilder func(tenantID string) (string, error)
	tokenStorePath string
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
		TenantID:                 t.tenant.ID,
		TenantName:               t.tenant.Name,
		Service:                  "qianchuan-live-operations-mcp",
		ProtocolVersion:          protocolVersion,
		Authenticated:            authorized,
		AdvertiserID:             formatID(advertiserID),
		AwemeID:                  formatID(awemeID),
		LiveRoomID:               formatID(liveRoomID),
		Accounts:                 publicAccounts(accounts),
		AccountSelectionRequired: len(accounts) > 1 && advertiserID == 0,
		ReadEnabled:              true,
		WriteEnabled:             t.writeEnabled,
		AuthorizationURL:         authorizationURL,
		Capabilities:             []string{"account", "plans", "materials", "live", "rules"},
		WriteCapabilities:        []string{"budget", "pause", "material_rotation"},
		OperationMode:            "server-side MCP; no browser cookies",
		RestartRequired:          false,
		TokenEncrypted:           true,
		TokenStorePresent:        t.tokenStoreExists(),
		WriteEnabledReason:       reason,
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
	return map[string]any{
		"account":        t.account(account),
		"ad_id":          formatUintID(adID),
		"window":         map[string]string{"start_date": startDate, "end_date": endDate},
		"materials":      materials,
		"protected_top3": protectedTop3(materials),
		"rule":           "消耗最高的前3素材永久保留；替换仅允许非前3素材",
		"warnings":       warnings,
	}, nil
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
	const dataTopic = "SITE_PROMOTION_POST_DATA_VIDEO"
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
	return map[string]any{
		"account":      t.account(account),
		"aweme_id":     formatID(account.AwemeID),
		"live_room_id": formatID(account.LiveRoomID),
		"window":       map[string]string{"start_time": startTime, "end_time": endTime},
		"live":         response.Data,
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
		case "roi2_material_video_type":
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
