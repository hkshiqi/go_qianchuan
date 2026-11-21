package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
)

type planCRUDNoopClient struct {
	sdk.QianchuanClient
}

func (planCRUDNoopClient) UniPromotionDetail(
	context.Context,
	sdk.UniPromotionDetailReq,
) (*sdk.UniPromotionDetailRes, error) {
	return nil, errors.New("unexpected network read")
}

func newPlanCRUDGuardTools(writeEnabled bool) *Tools {
	return &Tools{
		service:                  &service.QianchuanService{Client: planCRUDNoopClient{}},
		planCRUDTestWriteEnabled: writeEnabled,
	}
}

func TestPlanCRUDRequiresDedicatedWriteSwitchAndConfirmation(t *testing.T) {
	tools := newPlanCRUDGuardTools(false)
	args := map[string]any{
		"ad_id":    "1880000000000001",
		"new_name": planCRUDTestPrefix + "20261120-120000-UPDATED",
		"confirm":  true,
	}
	if _, err := tools.renameTestProductPlan(context.Background(), args); err == nil {
		t.Fatal("rename proceeded while the dedicated CRUD test switch was disabled")
	}
	tools.planCRUDTestWriteEnabled = true
	delete(args, "confirm")
	if _, err := tools.renameTestProductPlan(context.Background(), args); err == nil {
		t.Fatal("rename proceeded without confirmation")
	}
}

func TestPlanCRUDOnlyAcceptsTestNamePrefix(t *testing.T) {
	for _, name := range []string{
		"",
		"production plan",
		planCRUDTestPrefix,
		"x" + planCRUDTestPrefix + "20261120",
	} {
		if validPlanCRUDTestName(name) {
			t.Fatalf("test name accepted: %q", name)
		}
	}
	if !validPlanCRUDTestName(planCRUDTestPrefix + "20261120-120000") {
		t.Fatal("valid generated test name was rejected")
	}
}

func TestPlanCRUDRejectsDeleteOfLivePlan(t *testing.T) {
	tools := newPlanCRUDGuardTools(true)
	_, err := tools.deleteTestProductPlan(context.Background(), map[string]any{
		"ad_id":   "1880000000000002",
		"confirm": true,
	})
	if err == nil {
		t.Fatal("delete attempted to read/delete a non-test live plan")
	}
}

func TestPlanCRUDIdentityRequiresProductPlanAndPrefix(t *testing.T) {
	for _, plan := range []*sdk.UniPromotionDetail{
		nil,
		{MarketingGoal: "LIVE_PROM_GOODS", Name: planCRUDTestPrefix + "live"},
		{MarketingGoal: "VIDEO_PROM_GOODS", Name: "existing product plan"},
	} {
		if testPlanIdentityValid(plan) {
			t.Fatalf("non-test plan identity accepted: %#v", plan)
		}
	}
	if !testPlanIdentityValid(&sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		Name:          planCRUDTestPrefix + "product",
	}) {
		t.Fatal("valid test product plan identity was rejected")
	}
}

func TestPlanCRUDProductPageSizeMatchesOfficialEnum(t *testing.T) {
	cases := map[int]int{0: 10, 1: 10, 10: 10, 11: 20, 20: 20, 21: 30, 90: 30}
	for input, want := range cases {
		if got := closestProductPageSize(input); got != want {
			t.Fatalf("closestProductPageSize(%d) = %d, want %d", input, got, want)
		}
	}
}

func TestProductPlanOccupancyDeduplicatesProductIDs(t *testing.T) {
	withProductInfo := sdk.UniPromotion{AdId: 5}
	withProductInfo.ProductInfo = append(withProductInfo.ProductInfo, struct {
		ProductID   uint64 `json:"product_id"`
		ProductName string `json:"product_name"`
	}{
		ProductID:   3826400000000000001,
		ProductName: "红卫商品全域占用测试",
	})
	plans := []sdk.UniPromotion{
		{AdId: 1, ProductId: 3828195166146134185, ProductName: "红卫羊脂皂五盒装"},
		{AdId: 2, ProductId: 3828195166146134185, ProductName: "红卫羊脂皂五盒装"},
		{AdId: 3, ProductId: 3826314764871664464, ProductName: "红卫温和洁净手工香皂"},
		{AdId: 4},
		withProductInfo,
	}
	occupied, productPlans := productPlanOccupancy(plans)
	want := []string{
		"3828195166146134185",
		"3826314764871664464",
		"3826400000000000001",
	}
	if !sameStrings(occupied, want) {
		t.Fatalf("occupied products = %#v, want %#v", occupied, want)
	}
	if len(productPlans) != len(plans) {
		t.Fatalf("product plans = %d, want %d", len(productPlans), len(plans))
	}
	if productPlans[0]["product_name"] != "红卫羊脂皂五盒装" {
		t.Fatalf("product name missing from plan map: %#v", productPlans[0])
	}
	if productPlans[4]["product_name"] != "红卫商品全域占用测试" {
		t.Fatalf("product_info name missing from plan map: %#v", productPlans[4])
	}
}

func TestProductPlanListQueryUsesOverallSceneAndDeletedStatus(t *testing.T) {
	var captured sdk.UniPromotionListReq
	tools := &Tools{
		service: &service.QianchuanService{
			Client: planCRUDCaptureListClient{
				capture: &captured,
			},
		},
		accessToken: "test-access-token",
		accounts: []tenantstore.AccountBinding{{
			AdvertiserID: 1855810750424170,
			AwemeID:      63307887035,
			LiveRoomID:   7710006340290513690,
		}},
	}
	_, err := tools.listProductPlans(context.Background(), map[string]any{
		"account_id": "1855810750424170",
		"page":       1,
		"page_size":  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.MarketingGoal != "VIDEO_PROM_GOODS" {
		t.Fatalf("marketing_goal = %q", captured.MarketingGoal)
	}
	if captured.AdlabScene != string(sdk.AdlabSceneOverall) {
		t.Fatalf("adlab_scene = %q", captured.AdlabScene)
	}
	if captured.Filtering == nil || captured.Filtering.Status != "ALL_INCLUDE_DELETED" {
		t.Fatalf("status filter = %#v", captured.Filtering)
	}
}

func TestAvailableProductsOmitsAwemeInOverallMarketingMode(t *testing.T) {
	var captured sdk.UniPromotionProductAvailableGetReq
	tools := &Tools{
		service: &service.QianchuanService{
			Client: planCRUDCaptureProductClient{capture: &captured},
		},
		accessToken: "test-access-token",
		accounts: []tenantstore.AccountBinding{{
			AdvertiserID: 1855810750424170,
			AwemeID:      63307887035,
		}},
	}
	_, err := tools.listAvailableProducts(context.Background(), map[string]any{
		"account_id":                  "1855810750424170",
		"is_overall_marketing":        true,
		"filter_not_delivery_product": true,
		"page":                        1,
		"page_size":                   30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured.AwemeId != 0 {
		t.Fatalf("aweme_id = %d, want omitted in overall-marketing mode", captured.AwemeId)
	}
	if !captured.OverallMarketing || !captured.FilterNotDelivery {
		t.Fatalf("official overall-marketing filters were not forwarded: %#v", captured)
	}
	if captured.CreateROI2LimitProduct {
		t.Fatal("unsupported create_roi2_limit_product filter was forwarded in overall-marketing mode")
	}
}

func TestAvailableProductsRejectsUnsupportedOverallMarketingFilterCombination(t *testing.T) {
	tools := &Tools{
		service:     &service.QianchuanService{Client: planCRUDNoopClient{}},
		accessToken: "test-access-token",
		accounts: []tenantstore.AccountBinding{{
			AdvertiserID: 1855810750424170,
			AwemeID:      63307887035,
		}},
	}
	_, err := tools.listAvailableProducts(context.Background(), map[string]any{
		"account_id":                "1855810750424170",
		"is_overall_marketing":      true,
		"create_roi2_limit_product": true,
	})
	if err == nil {
		t.Fatal("unsupported overall-marketing filter combination was accepted")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestControlTaskMatchesIgnoresMaterialOrder(t *testing.T) {
	task := sdk.UniPromotionControlTask{
		Id:         1882434954198116,
		Name:       "Codex-CRUD-CHASE-TEST-order",
		Budget:     210,
		Bid:        14.32,
		TaskStatus: "DISABLE",
		MaterialList: []sdk.UniPromotionControlTaskMaterial{
			{MaterialId: 7662739677875601471},
			{MaterialId: 7662517359383560233},
			{MaterialId: 7654213262646247451},
			{MaterialId: 7622118464573095945},
		},
	}
	if !controlTaskMatches(
		task,
		task.Id,
		task.Name,
		task.Budget,
		task.Bid,
		[]string{
			"7662517359383560233",
			"7662739677875601471",
			"7622118464573095945",
			"7654213262646247451",
		},
		"DISABLE",
	) {
		t.Fatal("material order should not invalidate control-task readback")
	}
}

type planCRUDCaptureProductClient struct {
	sdk.QianchuanClient
	capture *sdk.UniPromotionProductAvailableGetReq
}

func (c planCRUDCaptureProductClient) UniPromotionProductAvailableGet(
	_ context.Context,
	req sdk.UniPromotionProductAvailableGetReq,
) (*sdk.UniPromotionProductAvailableGetRes, error) {
	*c.capture = req
	return &sdk.UniPromotionProductAvailableGetRes{}, nil
}

type planCRUDCaptureListClient struct {
	sdk.QianchuanClient
	capture *sdk.UniPromotionListReq
}

func (c planCRUDCaptureListClient) UniPromotionList(
	_ context.Context,
	req sdk.UniPromotionListReq,
) (*sdk.UniPromotionListRes, error) {
	*c.capture = req
	return &sdk.UniPromotionListRes{}, nil
}

func validProductCardFixture() *sdk.UniPromotionDetail {
	return &sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		Name:          productCardCRUDTestPrefix + "20261120-120000",
		Status:        "DISABLE",
		OptStatus:     "DISABLE",
		ProductInfos: []sdk.UniPromotionProductInfo{{
			ProductID: 7654321,
		}},
		MultiProductCreativeList: []sdk.UniPromotionMultiProductCreative{{
			ProductID:    7654321,
			CreativeType: "PROGRAMMATIC_CREATIVE",
			ImageMaterial: []sdk.UniPromotionCreativeImageMaterial{{
				ImageMode: "SQUARE",
				ImageIDs:  []string{"999001"},
			}},
			TitleMaterial: []sdk.UniPromotionMultiCreativeTitleMaterial{{
				TitleType: "COMMODITY_CARD",
				Title:     "红卫羊脂皂旗舰店纯商品卡测试",
			}},
			CreativeCard: &sdk.UniPromotionCreativeCard{
				PromotionCardTitle:         "红卫羊脂皂旗舰店纯商品卡测试",
				PromotionCardSellingPoints: []string{"温和清洁不紧绷"},
				PromotionCardImageID:       "999001",
			},
		}},
	}
}

func TestProductCardTestIdentityValidatesPureCardStructure(t *testing.T) {
	valid := validProductCardFixture()
	if !productCardTestIdentityValid(valid) {
		t.Fatal("valid pure product-card fixture was rejected")
	}
	otherTitle := validProductCardFixture()
	otherTitle.MultiProductCreativeList[0].TitleMaterial[0].Title = "温和洁净手工香皂"
	otherTitle.MultiProductCreativeList[0].CreativeCard.PromotionCardTitle = "温和洁净手工香皂"
	if !productCardTestIdentityValid(otherTitle) {
		t.Fatal("valid pure product-card fixture with another product title was rejected")
	}
	verified, evidence := productCardTestPlanIdentity(
		valid,
		7654321,
		"999001",
		"红卫羊脂皂旗舰店纯商品卡测试",
		[]string{"温和清洁不紧绷"},
	)
	if !verified {
		t.Fatalf("valid pure product-card identity was rejected: %#v", evidence)
	}
	if verified, _ := productCardTestPlanIdentity(
		valid,
		7654321,
		"999002",
		"红卫羊脂皂旗舰店纯商品卡测试",
		[]string{"温和清洁不紧绷"},
	); verified {
		t.Fatal("identity check accepted an unexpected expected image ID")
	}
	if verified, _ := productCardTestPlanIdentity(
		valid,
		7654321,
		"999001",
		"其它商品卡标题",
		[]string{"温和清洁不紧绷"},
	); verified {
		t.Fatal("identity check accepted an unexpected expected title")
	}
	if verified, _ := productCardTestPlanIdentity(
		valid,
		7654321,
		"999001",
		"红卫羊脂皂旗舰店纯商品卡测试",
		[]string{"国货老牌品质放心"},
	); verified {
		t.Fatal("identity check accepted unexpected selling points")
	}

	cases := map[string]func(*sdk.UniPromotionDetail){
		"wrong product": func(plan *sdk.UniPromotionDetail) {
			plan.ProductInfos[0].ProductID = 7654322
		},
		"wrong creative product": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].ProductID = 7654322
		},
		"non-programmatic creative": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].CreativeType = "CUSTOM_CREATIVE"
		},
		"video material": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].VideoMaterial = []sdk.UniPromotionCreativeVideoMaterial{{
				VideoID: "123",
			}}
		},
		"carousel material": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].CarouselMaterial = []sdk.UniPromotionCreativeCarouselMaterial{{
				CarouselID: 123,
			}}
		},
		"non-square image": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].ImageMaterial[0].ImageMode = "VERTICAL"
		},
		"missing image": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].ImageMaterial = nil
		},
		"wrong title type": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].TitleMaterial[0].TitleType = "VIDEO"
		},
		"missing title": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].TitleMaterial = nil
		},
		"missing selling points": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].CreativeCard = nil
		},
		"missing promotion card image": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].CreativeCard.PromotionCardImageID = ""
		},
		"promotion card title missing product": func(plan *sdk.UniPromotionDetail) {
			plan.MultiProductCreativeList[0].CreativeCard.PromotionCardTitle = "红卫好物"
		},
		"wrong name prefix": func(plan *sdk.UniPromotionDetail) {
			plan.Name = planCRUDTestPrefix + "wrong-prefix"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			plan := validProductCardFixture()
			mutate(plan)
			if productCardTestIdentityValid(plan) {
				t.Fatalf("invalid pure product-card structure was accepted: %#v", plan)
			}
		})
	}
}

func TestProductCardImageInputAcceptsOfficialStringID(t *testing.T) {
	const imageID = "tos-cn-i-sd07hgqsbj/fa1793f2e57b4c87ba5c57706ab3d272"
	if err := validateProductCardImageInput(imageID); err != nil {
		t.Fatalf("official string image ID was rejected: %v", err)
	}
	if err := validateProductCardImageInput("   "); err == nil {
		t.Fatal("blank image ID was accepted")
	}
	if !productCardImageMatches(imageID, sdk.QianchuanImage{
		ID:        imageID,
		ImageMode: "SQUARE",
	}) {
		t.Fatal("exact official SQUARE image record was rejected")
	}
	if productCardImageMatches(imageID, sdk.QianchuanImage{
		ID:        imageID,
		ImageMode: "VERTICAL",
	}) {
		t.Fatal("non-SQUARE image record was accepted")
	}
	if productCardImageMatches(imageID, sdk.QianchuanImage{
		ID:        "7654214461952425993",
		ImageMode: "SQUARE",
	}) {
		t.Fatal("material report ID was accepted as an image ID")
	}
}

func TestMaterialChaseSelectionProtectsTop3AndThresholdsOthers(t *testing.T) {
	config := DefaultRuleConfig()
	top3 := []MaterialMetrics{
		{MaterialID: "101", CostRank: 1, ProtectedTop3: true, CTR: 1, ConversionRate: 1},
		{MaterialID: "102", CostRank: 2, ProtectedTop3: true, CTR: 1, ConversionRate: 1},
		{MaterialID: "103", CostRank: 3, ProtectedTop3: true, CTR: 1, ConversionRate: 1},
	}
	library := append([]MaterialMetrics(nil),
		top3...)
	library = append(library,
		MaterialMetrics{MaterialID: "201", CTR: 7, ConversionRate: 0},
		MaterialMetrics{MaterialID: "202", CTR: 0, ConversionRate: 8},
		MaterialMetrics{MaterialID: "203", CTR: 6.99, ConversionRate: 7.99},
	)

	selected, err := selectMaterialChaseMaterials(
		[]string{"101", "102", "103", "201", "202"},
		top3,
		library,
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 5 {
		t.Fatalf("selected %d materials, want 5", len(selected))
	}
	if _, err := selectMaterialChaseMaterials(
		[]string{"101", "102", "201"},
		top3,
		library,
		config,
	); err == nil {
		t.Fatal("chase selection accepted a set missing protected spend rank 3")
	}
	if _, err := selectMaterialChaseMaterials(
		[]string{"101", "102", "103", "203"},
		top3,
		library,
		config,
	); err == nil {
		t.Fatal("chase selection accepted a non-top-3 material below both thresholds")
	}
	if _, err := selectMaterialChaseMaterials(
		[]string{"101", "101", "102", "103"},
		top3,
		library,
		config,
	); err == nil {
		t.Fatal("chase selection accepted duplicate material IDs")
	}
}

func TestMetricDateBounds(t *testing.T) {
	if got, err := materialChaseMetricDate(today(), "", false); err != nil || got != today() {
		t.Fatalf("regular material chase metric date = %q, %v; want %q", got, err, today())
	}
	yesterday := time.Now().In(shanghaiLocation).AddDate(0, 0, -1).Format("2006-01-02")
	if got, err := materialChaseMetricDate("", yesterday, true); err != nil || got != yesterday {
		t.Fatalf("test material chase metric date = %q, %v; want %q", got, err, yesterday)
	}
	if err := validateMetricDate(yesterday); err != nil {
		t.Fatalf("yesterday was rejected: %v", err)
	}
	if err := validateMetricDate(time.Now().In(shanghaiLocation).AddDate(0, 0, 1).Format("2006-01-02")); err == nil {
		t.Fatal("future metric date was accepted")
	}
	if err := validateMetricDate("2026/11/20"); err == nil {
		t.Fatal("invalid metric date was accepted")
	}
}

func TestControlTaskStatusVerificationAndDeletionSemantics(t *testing.T) {
	if !controlTaskStatusMatches("DISABLE", "DISABLE") {
		t.Fatal("paused task was not accepted")
	}
	if controlTaskStatusMatches("PROCESSING", "DISABLE") {
		t.Fatal("processing task was accepted as paused")
	}
	for _, status := range []string{"DELETE", "DELETED"} {
		if !controlTaskStatusMatches(status, "DELETED") {
			t.Fatalf("explicit delete status %q was rejected", status)
		}
	}
	if !errors.Is(
		fmt.Errorf("%w: task missing", errMaterialChaseTestTaskNotFound),
		errMaterialChaseTestTaskNotFound,
	) {
		t.Fatal("wrapped not-found error lost its sentinel")
	}
}

func TestControlTaskStatusUpdateRequiresExplicitTargetConfirmation(t *testing.T) {
	const taskID uint64 = 1880000000000101
	cases := []struct {
		name     string
		response *sdk.UniPromotionControlTaskStatusUpdateRes
		want     bool
	}{
		{name: "nil", response: nil},
		{
			name:     "top-level failure",
			response: &sdk.UniPromotionControlTaskStatusUpdateRes{Code: 40000},
		},
		{
			name: "different task",
			response: &sdk.UniPromotionControlTaskStatusUpdateRes{
				Code: 0,
				Data: struct {
					TaskIds []uint64                                       `json:"task_ids,omitempty"`
					Errors  []sdk.UniPromotionControlTaskStatusUpdateError `json:"errors,omitempty"`
				}{TaskIds: []uint64{taskID + 1}},
			},
		},
		{
			name: "matching task",
			want: true,
			response: &sdk.UniPromotionControlTaskStatusUpdateRes{
				Code: 0,
				Data: struct {
					TaskIds []uint64                                       `json:"task_ids,omitempty"`
					Errors  []sdk.UniPromotionControlTaskStatusUpdateError `json:"errors,omitempty"`
				}{TaskIds: []uint64{taskID}},
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := controlTaskStatusUpdateSucceeded(test.response, taskID); got != test.want {
				t.Fatalf("status success = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOverallPlanCRUDTestROIGoalDefaultsAndValidates(t *testing.T) {
	got, err := overallPlanCRUDTestROIGoal(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got != 1.90 {
		t.Fatalf("default roi2_goal = %v, want 1.90", got)
	}
	got, err = overallPlanCRUDTestROIGoal(map[string]any{"roi2_goal": 2.15})
	if err != nil || got != 2.15 {
		t.Fatalf("explicit roi2_goal = %v, err = %v", got, err)
	}
	for _, value := range []any{0, -1, 100.01, "1.9"} {
		if _, err := overallPlanCRUDTestROIGoal(map[string]any{"roi2_goal": value}); err == nil {
			t.Fatalf("invalid roi2_goal accepted: %#v", value)
		}
	}
}

func TestOverallPlanCRUDBudgetHonorsPlatformMinimum(t *testing.T) {
	if !almostEqual(300, sdk.OverallProductPlanMinimumBudget, 0.000001) {
		t.Fatal("the official overall-plan minimum budget was not accepted")
	}
	for _, value := range []float64{0, 0.01, 299.99, 300.01} {
		if almostEqual(value, sdk.OverallProductPlanMinimumBudget, 0.000001) {
			t.Fatalf("non-minimum test budget accepted: %v", value)
		}
	}
}

func TestOverallPlanCRUDOnlyAcceptsOverallTestIdentity(t *testing.T) {
	name := overallPlanCRUDTestPrefix + "20261120-130000"
	valid := &sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		AdlabScene:    sdk.AdlabSceneOverall,
		Name:          name,
	}
	if !overallTestPlanIdentityValid(valid) {
		t.Fatal("valid overall test plan identity was rejected")
	}
	for _, plan := range []*sdk.UniPromotionDetail{
		nil,
		{MarketingGoal: "VIDEO_PROM_GOODS", AdlabScene: sdk.AdlabSceneUni, Name: name},
		{MarketingGoal: "VIDEO_PROM_GOODS", Name: name},
		{MarketingGoal: "LIVE_PROM_GOODS", AdlabScene: sdk.AdlabSceneOverall, Name: name},
		{MarketingGoal: "VIDEO_PROM_GOODS", AdlabScene: sdk.AdlabSceneOverall, Name: planCRUDTestPrefix + "product"},
	} {
		if overallTestPlanIdentityValid(plan) {
			t.Fatalf("non-overall-test plan accepted: %#v", plan)
		}
	}
}

func TestOverallPlanCRUDRequiresPausedReadback(t *testing.T) {
	name := overallPlanCRUDTestPrefix + "20261120-130000"
	valid := &sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		AdlabScene:    sdk.AdlabSceneOverall,
		Name:          name,
		Status:        "DISABLE",
		OptStatus:     "DISABLE",
	}
	if !overallTestPlanPausedAndValid(valid) {
		t.Fatal("paused overall test plan was rejected")
	}
	for _, plan := range []*sdk.UniPromotionDetail{
		nil,
		{MarketingGoal: "VIDEO_PROM_GOODS", AdlabScene: sdk.AdlabSceneOverall, Name: name},
		{MarketingGoal: "VIDEO_PROM_GOODS", AdlabScene: sdk.AdlabSceneOverall, Name: name, Status: "ENABLE"},
		{MarketingGoal: "VIDEO_PROM_GOODS", AdlabScene: sdk.AdlabSceneOverall, Name: name, Status: "DISABLE", OptStatus: "ENABLE"},
	} {
		if overallTestPlanPausedAndValid(plan) {
			t.Fatalf("unpaused or invalid overall test plan accepted: %#v", plan)
		}
	}
}

func TestOverallPlanCreativeIdentityUsesDetailIDs(t *testing.T) {
	plan := &sdk.UniPromotionDetail{
		ProductInfos: []sdk.UniPromotionProductInfo{{ProductID: 7654321}},
		AwemeInfos:   []sdk.UniPromotionAwemeInfo{{AwemeUID: 63307887035}},
	}
	productID, awemeID, err := overallPlanCreativeIdentity(plan)
	if err != nil {
		t.Fatal(err)
	}
	if productID != 7654321 || awemeID != 63307887035 {
		t.Fatalf("identity = %d/%d", productID, awemeID)
	}
	for _, invalid := range []*sdk.UniPromotionDetail{
		nil,
		{AwemeInfos: []sdk.UniPromotionAwemeInfo{{AwemeUID: 63307887035}}},
		{ProductInfos: []sdk.UniPromotionProductInfo{{ProductID: 7654321}}},
	} {
		if _, _, err := overallPlanCreativeIdentity(invalid); err == nil {
			t.Fatalf("invalid plan identity accepted: %#v", invalid)
		}
	}
}

func TestUniPromotionStatusUpdateRequiresTargetSuccess(t *testing.T) {
	const adID uint64 = 1880000000000099
	cases := []struct {
		name     string
		response *sdk.UniPromotionStatusUpdateRes
		want     bool
	}{
		{name: "nil", response: nil},
		{name: "top-level error", response: &sdk.UniPromotionStatusUpdateRes{Code: 40000}},
		{name: "result failure", response: &sdk.UniPromotionStatusUpdateRes{
			Data: struct {
				Results []sdk.UniPromotionStatusResult `json:"results,omitempty"`
				AdIds   []uint64                       `json:"ad_ids,omitempty"`
				Errors  []sdk.UniPromotionOpError      `json:"errors,omitempty"`
			}{Results: []sdk.UniPromotionStatusResult{{AdId: adID}}},
		}},
		{name: "official flag success", want: true, response: &sdk.UniPromotionStatusUpdateRes{
			Data: struct {
				Results []sdk.UniPromotionStatusResult `json:"results,omitempty"`
				AdIds   []uint64                       `json:"ad_ids,omitempty"`
				Errors  []sdk.UniPromotionOpError      `json:"errors,omitempty"`
			}{Results: []sdk.UniPromotionStatusResult{{AdId: adID, Flag: true}}},
		}},
		{name: "success with error", response: &sdk.UniPromotionStatusUpdateRes{
			Data: struct {
				Results []sdk.UniPromotionStatusResult `json:"results,omitempty"`
				AdIds   []uint64                       `json:"ad_ids,omitempty"`
				Errors  []sdk.UniPromotionOpError      `json:"errors,omitempty"`
			}{Results: []sdk.UniPromotionStatusResult{{
				AdId:         adID,
				Flag:         true,
				ErrorCode:    40000,
				ErrorMessage: "failed",
			}}},
		}},
		{name: "other ad success", response: &sdk.UniPromotionStatusUpdateRes{
			Data: struct {
				Results []sdk.UniPromotionStatusResult `json:"results,omitempty"`
				AdIds   []uint64                       `json:"ad_ids,omitempty"`
				Errors  []sdk.UniPromotionOpError      `json:"errors,omitempty"`
			}{AdIds: []uint64{adID + 1}},
		}},
		{name: "target success", want: true, response: &sdk.UniPromotionStatusUpdateRes{
			Data: struct {
				Results []sdk.UniPromotionStatusResult `json:"results,omitempty"`
				AdIds   []uint64                       `json:"ad_ids,omitempty"`
				Errors  []sdk.UniPromotionOpError      `json:"errors,omitempty"`
			}{AdIds: []uint64{adID}},
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := uniPromotionStatusUpdateSucceeded(test.response, adID); got != test.want {
				t.Fatalf("status success = %v, want %v", got, test.want)
			}
		})
	}
}
