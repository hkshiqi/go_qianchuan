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

func TestExistingPlanOperationRejectsMixedAndInvalidBoostBudget(t *testing.T) {
	tools := &Tools{
		service:                  &service.QianchuanService{Client: planCRUDNoopClient{}},
		planCRUDTestWriteEnabled: true,
	}
	base := map[string]any{
		"ad_id":        "1880000000000201",
		"name":         existingPlanOperationTestPrefix + "20261121-120000",
		"budget":       31.0,
		"boost_budget": 1.0,
		"confirm":      true,
	}
	mixed := cloneMap(base)
	mixed["add_video_ids"] = []any{"7000000000000001"}
	mixed["add_image_ids"] = []any{"tos-cn-i-sd07hgqsbj/existing-image"}
	if _, err := tools.testExistingProductPlanOperations(context.Background(), mixed); err == nil ||
		!strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("mixed video/image add was not rejected: %v", err)
	}
	invalidBoost := cloneMap(base)
	invalidBoost["boost_budget"] = 31.01
	invalidBoost["add_image_ids"] = []any{"tos-cn-i-sd07hgqsbj/existing-image"}
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

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

var _ = errors.New
