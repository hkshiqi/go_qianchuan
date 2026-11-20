package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
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
