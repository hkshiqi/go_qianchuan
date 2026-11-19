package mcp

import (
	"math"
	"testing"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
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
			{Field: "roi2_material_status", FilterOnly: true},
		},
		Metrics: []sdk.UniPromotionReportMetric{
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
	if len(metrics) != 4 || metrics[0] != "stat_cost_for_roi2" ||
		metrics[1] != "total_pay_order_gmv_include_coupon_for_roi2" ||
		metrics[2] != "total_prepay_and_pay_order_roi2" ||
		metrics[3] != "total_ecpm_for_roi2" {
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
		},
		Metrics: map[string]any{
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
	if material.MaterialID != "7654214461952425993" || material.Cost != 387.05 ||
		material.CTR != 0.98 || material.ConversionRate != 4.5 ||
		material.PayOrderCount != 4 || material.PayOrderGMV != 120 ||
		material.GPM != 12 {
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

func TestAPIErrorIncludesRequestID(t *testing.T) {
	err := apiError("获取素材报表失败", 40000, "下游依赖服务相关错误", "req-123")
	if err.Error() != "获取素材报表失败[40000]: 下游依赖服务相关错误 (request_id=req-123)" {
		t.Fatalf("api error = %q", err.Error())
	}
}
