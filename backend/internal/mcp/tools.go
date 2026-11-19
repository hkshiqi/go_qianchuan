package mcp

import (
	"context"
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
	return tools
}

func (t *Tools) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "qianchuan_get_status":
		return t.status(ctx), nil
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
		return t.liveOverview(ctx)
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

func (t *Tools) status(ctx context.Context) Status {
	advertiserID := t.service.DefaultAdvertiserID
	authorized := false
	if advertiserID > 0 {
		if _, tokenErr := t.service.TokenManager.AccessToken(ctx, advertiserID); tokenErr == nil {
			authorized = true
		}
	}
	reason := "OPERATIONS_WRITE_ENABLED=true 时才允许写操作"
	if t.writeEnabled {
		reason = "写操作已启用，仍需 confirm=true 且服务端保护校验通过"
	}
	authorizationURL := t.authURL
	if t.authURLBuilder != nil {
		if generated, err := t.authURLBuilder(t.tenant.ID); err == nil {
			authorizationURL = generated
		}
	}
	return Status{
		TenantID:            t.tenant.ID,
		TenantName:          t.tenant.Name,
		Service:            "qianchuan-live-operations-mcp",
		ProtocolVersion:    protocolVersion,
		Authenticated:      authorized,
		AdvertiserID:       formatID(advertiserID),
		AwemeID:            formatID(t.service.DefaultAwemeID),
		LiveRoomID:         formatID(t.service.DefaultLiveRoomID),
		ReadEnabled:        true,
		WriteEnabled:       t.writeEnabled,
		AuthorizationURL:   authorizationURL,
		Capabilities:       []string{"account", "plans", "materials", "live", "rules"},
		WriteCapabilities:  []string{"budget", "pause", "material_rotation"},
		OperationMode:      "server-side MCP; no browser cookies",
		RestartRequired:    false,
		TokenEncrypted:      true,
		TokenStorePresent:  t.tokenStoreExists(),
		WriteEnabledReason: reason,
	}
}

func (t *Tools) tokenStoreExists() bool {
	if t.tokenStorePath == "" {
		return false
	}
	_, err := os.Stat(t.tokenStorePath)
	return err == nil
}

func (t *Tools) listPlans(ctx context.Context, args map[string]any) (any, error) {
	token, advertiserID, err := t.token(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
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
		"account":   t.account(advertiserID),
		"window":    map[string]string{"start_time": startTime, "end_time": endTime},
		"page_info": response.Data.PageInfo,
		"plans":     list,
	}, nil
}

func (t *Tools) getPlan(ctx context.Context, args map[string]any) (any, error) {
	token, advertiserID, err := t.token(ctx)
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
		"account": t.account(advertiserID),
		"plan":    response.Data,
	}, nil
}

func (t *Tools) planMaterials(ctx context.Context, args map[string]any) (any, error) {
	token, advertiserID, err := t.token(ctx)
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
		"account":   t.account(advertiserID),
		"ad_id":     formatUintID(adID),
		"window":    map[string]string{"start_date": startDate, "end_date": endDate},
		"materials": materials,
		"protected_top3": protectedTop3(materials),
		"rule":      "消耗最高的前3素材永久保留；替换仅允许非前3素材",
		"warnings":  warnings,
	}, nil
}

func (t *Tools) materialReport(ctx context.Context, args map[string]any) (any, error) {
	token, advertiserID, err := t.token(ctx)
	if err != nil {
		return nil, err
	}
	startDate := stringArg(args, "start_date", today())
	endDate := stringArg(args, "end_date", startDate)
	page := positiveIntArg(args, "page", 1)
	pageSize := clampInt(positiveIntArg(args, "page_size", 100), 1, 200)
	response, err := t.service.Client.ReportMaterialGet(ctx, sdk.ReportMaterialGetReq{
		AccessToken:  token,
		AdvertiserId: advertiserID,
		StartDate:    startDate,
		EndDate:      endDate,
		Fields: []string{
			"stat_cost",
			"show_cnt",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"conversion_rate",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
		},
		Page:       int64(page),
		PageSize:   int64(pageSize),
		OrderField: "stat_cost",
		OrderType:  "DESC",
	})
	if err != nil {
		return nil, fmt.Errorf("获取素材报表失败: %w", err)
	}
	if response.Code != 0 {
		return nil, apiError("获取素材报表失败", response.Code, response.Message)
	}
	return map[string]any{
		"account":    t.account(advertiserID),
		"window":     map[string]string{"start_date": startDate, "end_date": endDate},
		"page_info":  response.Data.PageInfo,
		"materials":  response.Data.List,
		"source":     "qianchuan report material",
		"write_guard": "素材库只读；本工具不会上传或创建素材",
	}, nil
}

func (t *Tools) liveOverview(ctx context.Context) (any, error) {
	token, advertiserID, err := t.token(ctx)
	if err != nil {
		return nil, err
	}
	response, err := t.service.Client.LiveGet(ctx, sdk.LiveGetReq{
		AccessToken:  token,
		AdvertiserId: advertiserID,
		AwemeId:      t.service.DefaultAwemeID,
		Fields: []string{
			"stat_cost",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"convert_rate",
			"today_live_pay_order_gpm",
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
		"account":     t.account(advertiserID),
		"aweme_id":    formatID(t.service.DefaultAwemeID),
		"live_room_id": formatID(t.service.DefaultLiveRoomID),
		"live":        response.Data,
		"refreshed_at": time.Now().Format(time.RFC3339),
	}, nil
}

func (t *Tools) evaluateRules(ctx context.Context, args map[string]any) (any, error) {
	token, advertiserID, err := t.token(ctx)
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
		AdvertiserID:         formatID(advertiserID),
		AdID:                 formatUintID(adID),
		WindowMinutes:        window,
		Cost:                 round(topCost, 2),
		MinuteCost:           round(minuteCost, 2),
		MinuteCostPassed:     minutePassed,
		CTR:                  round(ctr, 6),
		CTRPassed:            ctrPassed,
		ConversionRate:       round(conversion, 6),
		ConversionPassed:     conversionPassed,
		GPM:                  round(gpm, 2),
		Status:               status,
		Action:               action,
		ProtectedTop3:        protected,
		ReplacementCandidates: candidates,
		RuleConfig:           config,
		MetricsComplete:      len(warnings) == 0,
		Warnings:             warnings,
	}, nil
}

func (t *Tools) rotateMaterials(ctx context.Context, args map[string]any) (any, error) {
	if err := t.requireWrite(args); err != nil {
		return nil, err
	}
	token, advertiserID, err := t.token(ctx)
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
	token, advertiserID, err := t.token(ctx)
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
	token, advertiserID, err := t.token(ctx)
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

func (t *Tools) token(ctx context.Context) (string, int64, error) {
	if t.service.TokenManager == nil {
		return "", 0, errors.New("token manager is not initialized")
	}
	return t.service.AccessTokenFor(ctx, t.service.DefaultAdvertiserID)
}

func (t *Tools) account(advertiserID int64) map[string]string {
	return map[string]string{
		"advertiser_id": formatID(advertiserID),
		"aweme_id":      formatID(t.service.DefaultAwemeID),
		"live_room_id":  formatID(t.service.DefaultLiveRoomID),
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
		MaterialID:     formatUintID(raw.MaterialId),
		Title:          raw.MaterialName,
		VideoID:        raw.VideoId,
		AwemeItemID:    formatUintID(raw.AwemeItemId),
		Status:         raw.MaterialStatus,
		Cost:           raw.StatCost,
		ShowCount:      raw.ShowCnt,
		ClickCount:     raw.ClickCnt,
		CTR:            raw.Ctr,
		ConvertCount:   raw.ConvertCnt,
		ConversionRate: raw.ConversionRate,
		PayOrderCount:  raw.PayOrderAmount,
		PayOrderGMV:    raw.PayOrderAmount,
		ROI:            raw.Roi2,
		GPM:            raw.Gpm,
		CostRank:       rank,
		ProtectedTop3:  rank <= 3,
		EligibleReplace: rank > 3,
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
		"ad_id":         formatUintID(plan.AdId),
		"name":          plan.AdName,
		"status":        plan.Status,
		"opt_status":    plan.OptStatus,
		"budget":        plan.Budget,
		"roi2_goal":     plan.Roi2Goal,
		"aweme_id":      formatUintID(plan.AwemeId),
		"cost":          plan.StatCost,
		"pay_order_count": plan.PayOrderCount,
		"pay_order_gmv": plan.PayOrderGMV,
		"roi":           plan.PayOrderROI,
	}
}

func apiError(prefix string, code int64, message string) error {
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
	return time.Now().Format("2006-01-02")
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
