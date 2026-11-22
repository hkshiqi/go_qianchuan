package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

func TestBusinessDayRangeUsesShanghaiDate(t *testing.T) {
	utc := time.Date(2026, 11, 19, 18, 38, 0, 0, time.UTC)
	start, end := businessDayRange(utc)

	if start != "2026-11-20 00:00:00" {
		t.Fatalf("start = %q, want Shanghai day start", start)
	}
	if end != "2026-11-20 23:59:59" {
		t.Fatalf("end = %q, want Shanghai day end", end)
	}
}

func TestReportDimensionFieldsSkipFilterOnlyFields(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{Field: "material_id", IsRequired: true},
			{Field: "material_status", FilterOnly: true},
			{Field: "ad_id"},
		},
	}
	fields := reportDimensionFields(config)
	if len(fields) != 1 || fields[0] != "material_id" {
		t.Fatalf("dimension fields = %#v", fields)
	}
}

func TestReportFiltersUseRequiredConfigValues(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{
				Field: "material_status",
				FilterConfig: &sdk.UniPromotionReportFilterConfig{
					FilterRequired: true,
					Operator:       7,
					RangeValues: []sdk.UniPromotionReportFilterRangeValue{
						{Label: "DELIVERY_OK", Value: "投放中"},
					},
				},
			},
		},
	}
	filters, err := reportFilters(config, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 || filters[0].Field != "material_status" ||
		len(filters[0].Values) != 1 || filters[0].Values[0] != "DELIVERY_OK" {
		t.Fatalf("filters = %#v", filters)
	}
}

func TestReportFiltersUseSingleCanonicalEnumValue(t *testing.T) {
	dimension := sdk.UniPromotionReportDimension{
		Field: "aggregate_smart_bid_type",
		FilterConfig: &sdk.UniPromotionReportFilterConfig{
			FilterRequired: true,
			Operator:       7,
			RangeValues: []sdk.UniPromotionReportFilterRangeValue{
				{Label: "7", Value: "放量投放"},
				{Label: "0", Value: "控成本投放"},
			},
		},
	}
	values := reportFilterValues(
		dimension,
		"2026-11-20",
		"2026-11-20",
		tenantstore.AccountBinding{},
		0,
	)
	if len(values) != 1 || values[0] != "0" {
		t.Fatalf("smart bid values = %#v", values)
	}
}

func TestReportFiltersUseSingleSmartBidMode(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{
				Field: "aggregate_smart_bid_type",
				FilterConfig: &sdk.UniPromotionReportFilterConfig{
					FilterRequired: true,
					Operator:       7,
					RangeValues: []sdk.UniPromotionReportFilterRangeValue{
						{Label: "0", Value: "控成本投放"},
						{Label: "7", Value: "放量投放"},
					},
				},
			},
		},
	}
	filters, err := reportFilters(config, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 || filters[0].Field != "aggregate_smart_bid_type" ||
		len(filters[0].Values) != 1 || filters[0].Values[0] != "0" {
		t.Fatalf("filters = %#v", filters)
	}
}

func TestReportFiltersPreferCodesStoredInLabels(t *testing.T) {
	dimension := sdk.UniPromotionReportDimension{
		Field: "aggregate_smart_bid_type",
		FilterConfig: &sdk.UniPromotionReportFilterConfig{
			FilterRequired: true,
			Operator:       7,
			RangeValues: []sdk.UniPromotionReportFilterRangeValue{
				{Label: "0", Value: "控成本投放"},
				{Label: "7", Value: "放量投放"},
			},
		},
	}
	values := reportFilterValues(dimension, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if len(values) != 1 || values[0] != "0" {
		t.Fatalf("filter values = %#v", values)
	}
}

func TestReportFiltersIncludeMaterialVideoTypeCode(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{
				Field: "roi2_material_video_type",
				FilterConfig: &sdk.UniPromotionReportFilterConfig{
					Operator: 7,
					RangeValues: []sdk.UniPromotionReportFilterRangeValue{
						{Label: "1", Value: "全部"},
						{Label: "2", Value: "自选投放视频"},
						{Label: "3", Value: "智能优选视频"},
					},
				},
			},
		},
	}
	filters, err := reportFilters(config, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 || filters[0].Field != "roi2_material_video_type" ||
		len(filters[0].Values) != 1 || filters[0].Values[0] != "1" {
		t.Fatalf("filters = %#v", filters)
	}
}

func TestReportFiltersSelectOfficialLiveVideoMaterialType(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{
				Field: "roi2_material_type_v3",
				FilterConfig: &sdk.UniPromotionReportFilterConfig{
					Operator: 7,
					RangeValues: []sdk.UniPromotionReportFilterRangeValue{
						{Label: "1", Value: "标题"},
						{Label: "3", Value: "视频"},
						{Label: "6", Value: "图片"},
					},
				},
			},
		},
	}
	filters, err := reportFilters(config, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 || filters[0].Field != "roi2_material_type_v3" ||
		len(filters[0].Values) != 1 || filters[0].Values[0] != "3" {
		t.Fatalf("filters = %#v", filters)
	}
}

func TestReportFiltersPreferQianchuanPCOrderPlatformCode(t *testing.T) {
	dimension := sdk.UniPromotionReportDimension{
		Field: "ecp_app_id",
		FilterConfig: &sdk.UniPromotionReportFilterConfig{
			FilterRequired: true,
			Operator:       7,
			RangeValues: []sdk.UniPromotionReportFilterRangeValue{
				{Label: "2", Value: "小店随心推"},
				{Label: "1", Value: "千川pc"},
			},
		},
	}
	values := reportFilterValues(dimension, "2026-11-20", "2026-11-20", tenantstore.AccountBinding{}, 0)
	if len(values) != 1 || values[0] != "1" {
		t.Fatalf("filter values = %#v", values)
	}
}

func TestReportDimensionsRespectOfficialLimit(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{Field: "material_id", IsRequired: true},
			{Field: "roi2_material_video_name", IsRequired: true},
			{Field: "ad_id", IsRequired: true},
			{Field: "video_id", IsRequired: true},
		},
	}
	config.QueryLimit.MaxDimensionLimit = "2"
	fields := reportDimensionFields(config)
	if len(fields) != 2 {
		t.Fatalf("dimension fields = %#v", fields)
	}
}

func TestReportFieldsSelectSupportedMaterialMetrics(t *testing.T) {
	config := sdk.UniPromotionReportTopicConfig{
		Dimensions: []sdk.UniPromotionReportDimension{
			{Field: "material_id", IsRequired: true},
			{Field: "ad_id"},
			{Field: "roi2_material_video_name"},
			{Field: "roi2_material_upload_time"},
			{Field: "create_time"},
			{Field: "roi2_material_status", FilterOnly: true},
		},
		Metrics: []sdk.UniPromotionReportMetric{
			{Field: "basic_stat_cost_for_roi2_v2"},
			{Field: "stat_cost_for_roi2"},
			{Field: "total_prepay_and_pay_order_roi2"},
			{Field: "total_pay_order_gmv_include_coupon_for_roi2"},
			{Field: "total_ecpm_for_roi2"},
			{Field: "gpm"},
			{Field: "live_pay_order_gpm"},
			{Field: "total_live_pay_order_gpm"},
			{Field: "some_unrelated_metric"},
		},
	}
	dimensions := reportDimensionFields(config)
	if len(dimensions) != 2 || dimensions[0] != "material_id" ||
		dimensions[1] != "roi2_material_video_name" {
		t.Fatalf("dimension fields = %#v", dimensions)
	}
	metrics := reportMetricFields(config)
	if len(metrics) != 5 || metrics[0] != "basic_stat_cost_for_roi2_v2" ||
		metrics[1] != "stat_cost_for_roi2" ||
		metrics[2] != "total_pay_order_gmv_include_coupon_for_roi2" ||
		metrics[3] != "total_prepay_and_pay_order_roi2" ||
		metrics[4] != "total_ecpm_for_roi2" {
		t.Fatalf("metric fields = %#v", metrics)
	}
	for _, unsupported := range []string{"gpm", "live_pay_order_gpm", "total_live_pay_order_gpm"} {
		if containsString(metrics, unsupported) {
			t.Fatalf("unsupported metric %q entered official request: %#v", unsupported, metrics)
		}
	}
}

func TestReportFilterAnchorUsesResolvedInternalID(t *testing.T) {
	dimension := sdk.UniPromotionReportDimension{
		Field: "anchor_id",
		FilterConfig: &sdk.UniPromotionReportFilterConfig{
			FilterRequired: true,
			Operator:       7,
		},
	}
	values := reportFilterValues(
		dimension,
		"2026-11-20",
		"2026-11-20",
		tenantstore.AccountBinding{AwemeID: 63307887035},
		45525620241147,
	)
	if len(values) != 1 || values[0] != "45525620241147" {
		t.Fatalf("anchor filter values = %#v", values)
	}
}

func TestSelectReportAnchorIDMatchesShowID(t *testing.T) {
	id, err := selectReportAnchorID([]sdk.UniPromotionAweme{
		{
			AwemeId:     45525620241147,
			AwemeShowId: "63307887035",
			AwemeName:   "红卫监理旗舰店",
		},
	}, 63307887035)
	if err != nil {
		t.Fatal(err)
	}
	if id != 45525620241147 {
		t.Fatalf("anchor_id = %d", id)
	}
}

func TestMaterialFromReportRowMapsLiveMaterialMetrics(t *testing.T) {
	row := sdk.UniPromotionReportRow{
		Dimensions: map[string]any{
			"material_id":   "7654214461952425993",
			"material_name": "test",
			"create_time": map[string]any{
				"value":    "2026-09-08 17:33:23",
				"valueStr": "2026-09-08 17:33:23",
			},
		},
		Metrics: map[string]any{
			"basic_stat_cost_for_roi2_v2":                 357.05,
			"stat_cost_for_roi2":                          387.05,
			"live_show_count_for_roi2_v2":                 float64(10000),
			"live_watch_count_for_roi2_v2":                float64(98),
			"live_convert_rate_for_roi2_v2":               4.5,
			"live_cvr_rate_for_roi2_v2":                   0.98,
			"total_pay_order_count_for_roi2":              float64(4),
			"total_pay_order_gmv_for_roi2":                120.0,
			"total_pay_order_gmv_include_coupon_for_roi2": 131.6,
		},
	}
	material := materialFromReportRow(row, 1)
	if material.MaterialID != "7654214461952425993" ||
		material.BaseCost != 357.05 || material.TotalCost != 387.05 ||
		material.Cost != 357.05 ||
		material.CTR != 0.98 || material.ConversionRate != 4.5 ||
		material.OverallCTR != 0.98 || material.OverallConversionRate != 4.5 ||
		material.BaseCTR != nil || material.BaseConversionRate != nil ||
		material.PayOrderCount != 4 || material.PayOrderGMV != 120 ||
		material.GPM != 12 {
		t.Fatalf("material = %#v", material)
	}
}

func TestMaterialFromReportRowDerivesRatesOnlyFromBasicCounts(t *testing.T) {
	row := sdk.UniPromotionReportRow{
		Dimensions: map[string]any{
			"material_id": "7654214461952425993",
		},
		Metrics: map[string]any{
			"basic_stat_cost_for_roi2_v2":          100.0,
			"basic_live_show_count_for_roi2_v2":    1000,
			"basic_live_watch_count_for_roi2_v2":   80,
			"basic_total_pay_order_count_for_roi2": 8,
			"stat_cost_for_roi2":                   150.0,
			"live_cvr_rate_for_roi2_v2":            9.9,
			"live_convert_rate_for_roi2_v2":        19.9,
		},
	}
	material := materialFromReportRow(row, 1)
	if material.BaseCTR == nil || material.BaseConversionRate == nil ||
		math.Abs(*material.BaseCTR-8) > 1e-9 ||
		math.Abs(*material.BaseConversionRate-10) > 1e-9 ||
		material.OverallCTR != 9.9 || material.OverallConversionRate != 19.9 {
		t.Fatalf("material = %#v", material)
	}
}

func TestMaterialFromReportRowPrefersOfficialBasicRateFields(t *testing.T) {
	row := sdk.UniPromotionReportRow{
		Dimensions: map[string]any{
			"material_id": "7654214461952425993",
		},
		Metrics: map[string]any{
			"basic_stat_cost_for_roi2_v2":         100.0,
			"basic_live_show_count_for_roi2_v2":   999,
			"basic_live_watch_count_for_roi2_v2":  1,
			"basic_live_cvr_rate_for_roi2_v2":     6.75,
			"basic_live_convert_rate_for_roi2_v2": 11.25,
		},
	}
	material := materialFromReportRow(row, 1)
	if material.BaseCTR == nil || material.BaseConversionRate == nil ||
		*material.BaseCTR != 6.75 || *material.BaseConversionRate != 11.25 {
		t.Fatalf("material = %#v", material)
	}
}

func TestMaterialFromReportRowUnwrapsOfficialValues(t *testing.T) {
	row := sdk.UniPromotionReportRow{
		Dimensions: map[string]any{
			"material_id": map[string]any{
				"Value":    "7654214461952425993",
				"ValueStr": "7654214461952425993",
			},
			"roi2_material_video_name": map[string]any{
				"Value": "唐 6.17 11.mp4",
			},
		},
		Metrics: map[string]any{
			"stat_cost_for_roi2": map[string]any{
				"Value":         41.46,
				"ValueStr":      "41.46",
				"ShortValueStr": "41.46",
			},
			"live_show_count_for_roi2_v2": map[string]any{
				"Value": float64(1000),
			},
			"live_watch_count_for_roi2_v2": map[string]any{
				"Value": float64(70),
			},
		},
	}
	material := materialFromReportRow(row, 1)
	if material.MaterialID != "7654214461952425993" ||
		material.Title != "唐 6.17 11.mp4" ||
		material.Cost != 41.46 ||
		math.Abs(material.CTR-7) > 1e-9 {
		t.Fatalf("material = %#v", material)
	}
}

func TestMaterialFromReportRowKeepsLargeMaterialIDPrecision(t *testing.T) {
	row := sdk.UniPromotionReportRow{
		Dimensions: map[string]any{
			"material_id": json.Number("7654214461952425993"),
		},
		Metrics: map[string]any{
			"stat_cost_for_roi2": json.Number("416.25"),
		},
	}
	material := materialFromReportRow(row, 1)
	if material.MaterialID != "7654214461952425993" || material.Cost != 416.25 {
		t.Fatalf("material = %#v", material)
	}
}

func TestMaterialThresholdBoundaryKeepsSingleThresholdEligible(t *testing.T) {
	config := DefaultRuleConfig()
	tests := []struct {
		name     string
		material MaterialMetrics
		eligible bool
		dual     bool
	}{
		{
			name:     "ctr only at exact boundary",
			material: MaterialMetrics{CTR: 7, ConversionRate: 1},
			eligible: true,
		},
		{
			name:     "conversion only just inside boundary",
			material: MaterialMetrics{CTR: 1, ConversionRate: 7.9999999999},
			eligible: true,
		},
		{
			name:     "ctr only above boundary",
			material: MaterialMetrics{CTR: 7.0000000001, ConversionRate: 1},
			eligible: true,
		},
		{
			name:     "conversion only above boundary",
			material: MaterialMetrics{CTR: 1, ConversionRate: 8.0000000001},
			eligible: true,
		},
		{
			name:     "dual exact boundaries",
			material: MaterialMetrics{CTR: 7, ConversionRate: 8},
			eligible: true,
			dual:     true,
		},
		{
			name:     "below both boundaries",
			material: MaterialMetrics{CTR: 6.999999, ConversionRate: 7.999999},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := materialThresholdPassed(test.material, config); actual != test.eligible {
				t.Fatalf("eligible = %v, want %v", actual, test.eligible)
			}
			dual := len(dualThresholdMaterials([]MaterialMetrics{test.material}, config)) == 1
			if dual != test.dual {
				t.Fatalf("dual = %v, want %v", dual, test.dual)
			}
		})
	}
}

func TestMaterialChaseWriteRequiresDedicatedSwitchAndConfirmation(t *testing.T) {
	tools := &Tools{}
	if err := tools.requireMaterialChaseWrite(map[string]any{"confirm": true}); err == nil {
		t.Fatal("write switch disabled: expected refusal")
	}
	tools.materialChaseWriteEnabled = true
	if err := tools.requireMaterialChaseWrite(map[string]any{}); err == nil {
		t.Fatal("missing confirm=true: expected refusal")
	}
	if err := tools.requireMaterialChaseWrite(map[string]any{"confirm": "true"}); err == nil {
		t.Fatal("non-boolean confirm: expected refusal")
	}
	if err := tools.requireMaterialChaseWrite(map[string]any{"confirm": true}); err != nil {
		t.Fatalf("valid write guard refused: %v", err)
	}
}

func TestControlTaskMaterialIDComparisonIgnoresOrder(t *testing.T) {
	materials := []sdk.UniPromotionControlTaskMaterial{
		{MaterialId: 7660659586093924358},
		{MaterialId: 7662517346247393343},
	}
	if !sameIDSet(materials, []string{"7662517346247393343", "7660659586093924358"}) {
		t.Fatal("reordered material IDs should match")
	}
	if sameIDSet(materials, []string{"7662517346247393343"}) {
		t.Fatal("material ID subset should not match")
	}
}

func TestReportPageSizeUsesOfficialAllowedValues(t *testing.T) {
	tests := map[int]int{
		1:   10,
		10:  10,
		11:  20,
		21:  50,
		51:  100,
		101: 200,
		999: 200,
	}
	for input, expected := range tests {
		if actual := reportPageSize(input); actual != expected {
			t.Errorf("reportPageSize(%d) = %d, want %d", input, actual, expected)
		}
	}
}

type materialDateTestClient struct {
	sdk.QianchuanClient
	request sdk.QianchuanVideoGetReq
	found   []sdk.QianchuanVideo
}

func (c *materialDateTestClient) QianchuanVideoGet(
	_ context.Context,
	req sdk.QianchuanVideoGetReq,
) (*sdk.QianchuanVideoGetRes, error) {
	c.request = req
	return &sdk.QianchuanVideoGetRes{
		Code:    0,
		Message: "OK",
		Data: struct {
			List     []sdk.QianchuanVideo `json:"list"`
			PageInfo sdk.PageInfo         `json:"page_info"`
		}{
			List: c.found,
		},
	}, nil
}

func TestFillMaterialCreationDatesUsesOfficialVideoQuery(t *testing.T) {
	client := &materialDateTestClient{
		found: []sdk.QianchuanVideo{
			{
				MaterialID: 7654214461952425993,
				CreateTime: "2026-09-08 17:33:23",
			},
		},
	}
	tools := &Tools{service: &service.QianchuanService{Client: client}}
	materials := []MaterialMetrics{
		{MaterialID: "7654214461952425993"},
		{MaterialID: "7660659586093924358"},
		{MaterialID: "not-a-number"},
	}

	source, warnings := tools.fillMaterialCreationDates(
		context.Background(),
		"test-token",
		1855810750424170,
		materials,
	)

	if source != "qianchuan_video_get" {
		t.Fatalf("source = %q", source)
	}
	if len(client.request.MaterialIDs) != 2 ||
		client.request.MaterialIDs[0] != 7654214461952425993 ||
		client.request.MaterialIDs[1] != 7660659586093924358 {
		t.Fatalf("material IDs sent = %#v", client.request.MaterialIDs)
	}
	if materials[0].UploadedAt != "2026-09-08 17:33:23" ||
		materials[0].CreationDate != "2026-09-08" {
		t.Fatalf("filled material = %#v", materials[0])
	}
	if len(warnings) != 2 ||
		warnings[0] != "已跳过 1 条无有效素材ID的报表聚合行" ||
		warnings[1] != "素材创建时间仅补齐 1/2 条" {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestPlanMaterialsPaginationKeepsProtectedTop3(t *testing.T) {
	materials := make([]MaterialMetrics, 0, 25)
	for index := 1; index <= 25; index++ {
		materials = append(materials, MaterialMetrics{
			MaterialID:      fmt.Sprintf("material-%d", index),
			Cost:            float64(100 - index),
			CostRank:        index,
			ProtectedTop3:   index <= 3,
			EligibleReplace: index > 3,
		})
	}
	result := materialPage(materials, 2, 10)
	page, ok := result["page_info"].(sdk.PageInfo)
	if !ok {
		t.Fatalf("page_info type = %T", result["page_info"])
	}
	if page.Page != 2 || page.PageSize != 10 || page.TotalNumber != 25 || page.TotalPage != 3 {
		t.Fatalf("page_info = %#v", page)
	}
	rows, ok := result["materials"].([]MaterialMetrics)
	if !ok {
		t.Fatalf("materials type = %T", result["materials"])
	}
	if len(rows) != 10 || rows[0].CostRank != 11 {
		t.Fatalf("page rows = %#v", rows)
	}
	protected, ok := result["protected_top3"].([]MaterialMetrics)
	if !ok {
		t.Fatalf("protected type = %T", result["protected_top3"])
	}
	if len(protected) != 3 || protected[0].CostRank != 1 || protected[2].CostRank != 3 {
		t.Fatalf("protected = %#v", protected)
	}
}

func TestSelectLivingRoomPrefersCurrentLiveRoom(t *testing.T) {
	rooms := []map[string]any{
		{"room_id": "7709635958206991154", "room_status": "FINISH", "start_time": "2026-11-20 08:00:00"},
		{"room_id": "7710006340290513690", "room_status": "LIVING", "start_time": "2026-11-20 19:00:00"},
	}
	room, warnings := selectLivingRoom(rooms)
	if len(warnings) != 0 || room == nil {
		t.Fatalf("room = %#v warnings = %#v", room, warnings)
	}
	if actual := liveRoomIDFromMap(room); actual != "7710006340290513690" {
		t.Fatalf("room_id = %q", actual)
	}
}

func TestNormalizeLiveReportAmountsConvertsThousandthCent(t *testing.T) {
	values := normalizeLiveReportAmountsMap(map[string]any{
		"stat_cost":                float64(2500000),
		"luban_live_pay_order_gmv": float64(5000000),
		"live_pay_order_gmv_alias": float64(9000000),
		"total_live_pay_order_gpm": float64(123.4),
	})
	if values["stat_cost"] != float64(25) ||
		values["luban_live_pay_order_gmv"] != float64(50) ||
		values["live_pay_order_gmv_alias"] != float64(90) ||
		values["total_live_pay_order_gpm"] != float64(123.4) {
		t.Fatalf("normalized live values = %#v", values)
	}
}

func TestPageInfoAcceptsOfficialTotalNum(t *testing.T) {
	var page sdk.PageInfo
	err := json.Unmarshal([]byte(`{"page":1,"page_size":100,"total_page":4,"total_num":361}`), &page)
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalNumber != 361 || page.TotalPage != 4 {
		t.Fatalf("page = %#v", page)
	}
}

func TestAPIErrorIncludesRequestID(t *testing.T) {
	err := apiError("获取素材报表失败", 40000, "下游依赖服务相关错误", "req-123")
	if err.Error() != "获取素材报表失败[40000]: 下游依赖服务相关错误 (request_id=req-123)" {
		t.Fatalf("api error = %q", err.Error())
	}
}
