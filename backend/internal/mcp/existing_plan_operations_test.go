package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
)

func TestExistingPlanOperationRequiresDedicatedWriteSwitchAndConfirmation(t *testing.T) {
	tools := &Tools{
		service:                  &service.QianchuanService{Client: planCRUDNoopClient{}},
		planCRUDTestWriteEnabled: false,
	}
	args := map[string]any{
		"ad_id":        "1880000000000201",
		"name":         existingPlanOperationTestPrefix + "20261121-120000",
		"budget":       31.0,
		"boost_budget": 1.0,
		"add_image_ids": []any{
			"tos-cn-i-sd07hgqsbj/existing-image",
		},
		"add_titles": []any{
			"红卫羊脂皂五盒装",
		},
		"confirm": true,
	}
	if _, err := tools.testExistingProductPlanOperations(context.Background(), args); err == nil {
		t.Fatal("operation proceeded while dedicated CRUD test switch was disabled")
	}
	tools.planCRUDTestWriteEnabled = true
	delete(args, "confirm")
	if _, err := tools.testExistingProductPlanOperations(context.Background(), args); err == nil {
		t.Fatal("operation proceeded without confirmation")
	}
}

func TestExistingPlanOperationRejectsActivePlan(t *testing.T) {
	for _, plan := range []*sdk.UniPromotionDetail{
		nil,
		{MarketingGoal: "LIVE_PROM_GOODS", Status: "DISABLE", OptStatus: "DISABLE"},
		{MarketingGoal: "VIDEO_PROM_GOODS", Status: "ENABLE", OptStatus: "DISABLE"},
		{MarketingGoal: "VIDEO_PROM_GOODS", Status: "DISABLE", OptStatus: "ENABLE"},
	} {
		if err := validateExistingPlanOperationTarget(plan); err == nil {
			t.Fatalf("unsafe target accepted: %#v", plan)
		}
	}
	if err := validateExistingPlanOperationTarget(&sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		Status:        "DISABLE",
		OptStatus:     "DISABLE",
	}); err != nil {
		t.Fatalf("paused product plan was rejected: %v", err)
	}
}

func TestExistingPlanCreativeMaterialTypeUsesActualPlanContent(t *testing.T) {
	imagePlan := &sdk.UniPromotionDetail{
		MultiProductCreativeList: []sdk.UniPromotionMultiProductCreative{{
			ImageMaterial: []sdk.UniPromotionCreativeImageMaterial{{
				ImageMode: "SQUARE",
				ImageIDs:  []string{"tos-cn-i-sd07hgqsbj/existing-image"},
			}},
		}},
	}
	if got, err := existingPlanCreativeMaterialType(imagePlan); err != nil || got != "IMAGE" {
		t.Fatalf("image plan material type = %q, %v; want IMAGE", got, err)
	}
	videoPlan := &sdk.UniPromotionDetail{
		MultiProductCreativeList: []sdk.UniPromotionMultiProductCreative{{
			VideoMaterial: []sdk.UniPromotionCreativeVideoMaterial{{
				VideoID: "v0200fg10000",
			}},
		}},
	}
	if got, err := existingPlanCreativeMaterialType(videoPlan); err != nil || got != "VIDEO" {
		t.Fatalf("video plan material type = %q, %v; want VIDEO", got, err)
	}
	if _, err := existingPlanCreativeMaterialType(&sdk.UniPromotionDetail{}); err == nil {
		t.Fatal("plan without creative material was accepted")
	}
}

func TestIsolatedOverallTestPlanAllowsImageFallback(t *testing.T) {
	valid := &sdk.UniPromotionDetail{
		MarketingGoal: "VIDEO_PROM_GOODS",
		AdlabScene:    sdk.AdlabSceneOverall,
		Name:          overallPlanCRUDTestPrefix + "20261121-174919",
	}
	if !isolatedOverallTestPlan(valid) {
		t.Fatal("isolated overall test plan was not recognized")
	}
	for _, plan := range []*sdk.UniPromotionDetail{
		nil,
		{
			MarketingGoal: "VIDEO_PROM_GOODS",
			Name:          overallPlanCRUDTestPrefix + "20261121-174919",
		},
		{
			MarketingGoal: "VIDEO_PROM_GOODS",
			AdlabScene:    sdk.AdlabSceneUni,
			Name:          overallPlanCRUDTestPrefix + "20261121-174919",
		},
		{
			MarketingGoal: "VIDEO_PROM_GOODS",
			AdlabScene:    sdk.AdlabSceneOverall,
			Name:          "production-plan",
		},
	} {
		if isolatedOverallTestPlan(plan) {
			t.Fatalf("unsafe plan accepted for image fallback: %#v", plan)
		}
	}
}

func TestExistingPlanOperationRejectsUnboundAccountAndInvalidBoostBudget(t *testing.T) {
	tools := &Tools{
		service:                  &service.QianchuanService{Client: planCRUDNoopClient{}},
		planCRUDTestWriteEnabled: true,
	}
	base := map[string]any{
		"ad_id":        "1880000000000201",
		"name":         existingPlanOperationTestPrefix + "20261121-120000",
		"budget":       31.0,
		"boost_budget": 1.0,
		"add_image_ids": []any{
			"tos-cn-i-sd07hgqsbj/existing-image",
		},
		"add_titles": []any{
			"红卫羊脂皂五盒装",
		},
		"confirm": true,
	}
	if _, err := tools.testExistingProductPlanOperations(context.Background(), base); err == nil ||
		!strings.Contains(err.Error(), "account_id is not bound") {
		t.Fatalf("invalid account was not rejected before network access: %v", err)
	}
	invalidBoost := cloneMap(base)
	invalidBoost["boost_budget"] = 31.01
	if _, err := tools.testExistingProductPlanOperations(context.Background(), invalidBoost); err == nil ||
		!strings.Contains(err.Error(), "cannot exceed budget") {
		t.Fatalf("boost budget above plan budget was not rejected: %v", err)
	}
}

func TestExistingPlanMaterialDiffAndRestoration(t *testing.T) {
	if !containsAny([]string{"101", "102"}, []string{"102"}) {
		t.Fatal("existing material overlap was not detected")
	}
	if containsAny([]string{"101", "102"}, []string{"103"}) {
		t.Fatal("non-overlapping material was reported as existing")
	}
	if !sameStrings([]string{"103", "101", "102"}, []string{"101", "102", "103"}) {
		t.Fatal("material restoration comparison ignored order")
	}
}

func TestProductCardTitleMaterialsUsePlatformCommodityCardType(t *testing.T) {
	titles := productCardTitleMaterials([]string{"红卫羊脂皂五盒装", "温和洁净手工香皂"})
	if len(titles) != 2 {
		t.Fatalf("title count = %d, want 2", len(titles))
	}
	for _, title := range titles {
		if title.TitleType != "COMMODITY_CARD" || strings.TrimSpace(title.Title) == "" {
			t.Fatalf("invalid product-card title: %#v", title)
		}
	}
}

func TestExistingPlanMaterialDiffSeparatesImagesAndTitles(t *testing.T) {
	images := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 101, ImageId: "existing-image"},
		{MaterialId: 102, ImageId: "new-image"},
	}
	titles := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 201, MaterialName: "existing-title"},
		{MaterialId: 202, MaterialName: "new-title"},
	}
	if got := existingPlanMaterialIDs(images); !sameStrings(got, []string{"101", "102"}) {
		t.Fatalf("image material IDs = %v", got)
	}
	if got := existingPlanImageIDs(images); !sameStrings(got, []string{"existing-image", "new-image"}) {
		t.Fatalf("image IDs = %v", got)
	}
	if got := existingPlanTitleTexts(titles); !sameStrings(got, []string{"existing-title", "new-title"}) {
		t.Fatalf("title texts = %v", got)
	}
	if got := existingPlanNewMaterialIDs(
		existingPlanMaterialIDs(images),
		[]string{"101"},
	); !sameStrings(got, []string{"102"}) {
		t.Fatalf("new image material IDs = %v", got)
	}
	if got := existingPlanNewMaterialIDs(
		existingPlanMaterialIDs(titles),
		[]string{"201"},
	); !sameStrings(got, []string{"202"}) {
		t.Fatalf("new title material IDs = %v", got)
	}
	if !containsAllStrings(
		existingPlanImageIDs(images),
		[]string{"existing-image", "new-image"},
	) || containsAllStrings(
		existingPlanImageIDs(images),
		[]string{"missing-image"},
	) {
		t.Fatal("image readback containment is incorrect")
	}
}

func TestDeletedMaterialsAreExcludedFromActiveReadback(t *testing.T) {
	images := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 101, ImageId: "existing-image"},
		{MaterialId: 102, ImageId: "deleted-image", IsDeleted: true},
		{MaterialId: 103, ImageId: "deleted-status-image", MaterialStatus: "DELETED"},
	}
	titles := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 201, MaterialName: "existing-title"},
		{MaterialId: 202, MaterialName: "deleted-title", IsDeleted: true},
	}
	if got := existingPlanActiveMaterialIDs(images); !sameStrings(got, []string{"101"}) {
		t.Fatalf("active image material IDs = %v", got)
	}
	if got := existingPlanImageIDs(images); !sameStrings(got, []string{"existing-image"}) {
		t.Fatalf("active image IDs = %v", got)
	}
	if got := existingPlanTitleTexts(titles); !sameStrings(got, []string{"existing-title"}) {
		t.Fatalf("active title texts = %v", got)
	}
	if got := existingPlanAllTitleTexts(titles); !sameStrings(got, []string{"existing-title", "deleted-title"}) {
		t.Fatalf("all title texts = %v", got)
	}
}

func TestCleanupOnlyArgValidation(t *testing.T) {
	enabled, err := cleanupOnlyArg(map[string]any{"cleanup_only": true})
	if err != nil || !enabled {
		t.Fatalf("cleanup_only=true parsed as %v, %v", enabled, err)
	}
	enabled, err = cleanupOnlyArg(map[string]any{})
	if err != nil || enabled {
		t.Fatalf("missing cleanup_only parsed as %v, %v", enabled, err)
	}
	if _, err := cleanupOnlyArg(map[string]any{"cleanup_only": "true"}); err == nil {
		t.Fatal("non-boolean cleanup_only was accepted")
	}
}

func TestExistingPlanCreativeBindingsUsePlanDetail(t *testing.T) {
	plan := &sdk.UniPromotionDetail{
		MultiProductCreativeList: []sdk.UniPromotionMultiProductCreative{{
			ImageMaterial: []sdk.UniPromotionCreativeImageMaterial{{
				ImageIDs: []string{"existing-image", "new-image", "existing-image"},
			}},
			TitleMaterial: []sdk.UniPromotionMultiCreativeTitleMaterial{
				{Title: "existing-title"},
				{Title: "new-title"},
				{Title: "existing-title"},
			},
		}},
	}
	images, titles := existingPlanCreativeBindings(plan)
	if !sameStrings(images, []string{"existing-image", "new-image"}) {
		t.Fatalf("creative image IDs = %v", images)
	}
	if !sameStrings(titles, []string{"existing-title", "new-title"}) {
		t.Fatalf("creative title texts = %v", titles)
	}
	if !sameStrings(
		existingPlanCreativeMaterialIDs(plan),
		[]string{"existing-image", "new-image"},
	) {
		t.Fatal("creative image material IDs do not match image IDs")
	}
}

type budgetUpdateTestClient struct {
	sdk.QianchuanClient
	response *sdk.UniPromotionBudgetUpdateRes
}

func (c budgetUpdateTestClient) UniPromotionBudgetUpdate(
	context.Context,
	sdk.UniPromotionBudgetUpdateReq,
) (*sdk.UniPromotionBudgetUpdateRes, error) {
	return c.response, nil
}

func TestExistingPlanBudgetRequiresSuccessResult(t *testing.T) {
	const adID uint64 = 1882460844769367
	tests := []struct {
		name    string
		results []sdk.UniPromotionBudgetUpdateResult
		wantErr string
	}{
		{
			name:    "success",
			results: []sdk.UniPromotionBudgetUpdateResult{{AdId: adID, Status: "SUCCESS"}},
		},
		{
			name: "failed includes API error",
			results: []sdk.UniPromotionBudgetUpdateResult{{
				AdId:         adID,
				Status:       "FAILED",
				ErrorMessage: "budget below minimum",
			}},
			wantErr: "status=FAILED error=budget below minimum",
		},
		{
			name:    "missing result",
			wantErr: "response omitted ad_id",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := &sdk.UniPromotionBudgetUpdateRes{Code: 0}
			response.Data.Results = test.results
			tools := &Tools{
				service: &service.QianchuanService{
					Client: budgetUpdateTestClient{response: response},
				},
			}
			err := tools.updateExistingPlanBudget(
				context.Background(),
				"test-token",
				1855810750424170,
				adID,
				301,
			)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("successful budget result was rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("budget error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestExistingPlanRestoredStateRequiresPauseBudgetAndOriginalBindings(t *testing.T) {
	const budget = 300.0
	valid := &sdk.UniPromotionDetail{
		Status:    "DISABLE",
		OptStatus: "DISABLE",
		Budget:    budget,
	}
	if !existingPlanRestoredStateMatches(
		valid,
		budget,
		"DISABLE",
		"DISABLE",
		[]string{"image-1"},
		[]string{"title-1"},
		[]string{"image-1"},
		[]string{"title-1"},
	) {
		t.Fatal("fully restored plan state was rejected")
	}
	for _, test := range []struct {
		name       string
		plan       *sdk.UniPromotionDetail
		budget     float64
		status     string
		optStatus  string
		imageIDs   []string
		titleTexts []string
	}{
		{name: "nil plan", budget: budget, status: "DISABLE", optStatus: "DISABLE"},
		{name: "budget", plan: valid, budget: budget + 1, status: "DISABLE", optStatus: "DISABLE", imageIDs: []string{"image-1"}, titleTexts: []string{"title-1"}},
		{name: "status", plan: &sdk.UniPromotionDetail{Status: "ENABLE", OptStatus: "DISABLE", Budget: budget}, budget: budget, status: "DISABLE", optStatus: "DISABLE", imageIDs: []string{"image-1"}, titleTexts: []string{"title-1"}},
		{name: "opt status", plan: &sdk.UniPromotionDetail{Status: "DISABLE", OptStatus: "ENABLE", Budget: budget}, budget: budget, status: "DISABLE", optStatus: "DISABLE", imageIDs: []string{"image-1"}, titleTexts: []string{"title-1"}},
		{name: "image", plan: valid, budget: budget, status: "DISABLE", optStatus: "DISABLE", imageIDs: []string{"image-1", "test-image"}, titleTexts: []string{"title-1"}},
		{name: "title", plan: valid, budget: budget, status: "DISABLE", optStatus: "DISABLE", imageIDs: []string{"image-1"}, titleTexts: []string{"title-1", "test-title"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if existingPlanRestoredStateMatches(
				test.plan,
				test.budget,
				test.status,
				test.optStatus,
				test.imageIDs,
				test.titleTexts,
				[]string{"image-1"},
				[]string{"title-1"},
			) {
				t.Fatal("incomplete restoration was accepted")
			}
		})
	}
}

func TestExistingPlanDeleteTargetsUseMaterialIDsForNewImageAndTitle(t *testing.T) {
	before := &existingPlanSnapshot{
		ImageIDs:   []string{"existing-image"},
		TitleTexts: []string{"existing-title"},
	}
	afterAdd := &existingPlanSnapshot{
		ImageIDs:   []string{"existing-image", "new-image"},
		TitleTexts: []string{"existing-title", "new-title"},
		MaterialImageIDMap: map[string]string{
			"existing-image": "101",
			"new-image":      "102",
		},
		MaterialTitleMap: map[string]string{
			"existing-title": "201",
			"new-title":      "202",
		},
	}
	if got := existingPlanAddedImageMaterialIDs(afterAdd, before); !sameStrings(got, []string{"102"}) {
		t.Fatalf("new image material IDs = %v", got)
	}
	if got := existingPlanTitleMaterialIDs(afterAdd, []string{"new-title"}); !sameStrings(got, []string{"202"}) {
		t.Fatalf("new title material IDs = %v", got)
	}
	if got := existingPlanAddedImageMaterialIDs(afterAdd, before); containsString(got, "101") {
		t.Fatalf("existing image material ID leaked into delete targets: %v", got)
	}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

var _ = errors.New
