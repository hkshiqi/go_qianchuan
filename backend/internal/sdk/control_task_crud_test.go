package sdk

import (
	"encoding/json"
	"testing"
)

func TestBuildProductSmartBoostPayloadOmitsLiveOnlyFields(t *testing.T) {
	body, err := buildControlTaskCreatePayload(UniPromotionControlTaskCreateReq{
		AdvertiserId: 1855810750424170,
		AdId:         1880000000000201,
		Name:         "Codex-CRUD-EXISTING-PLAN-TEST-20261121-120000",
		Scene:        "SMART_BOOST",
		Budget:       1,
		Duration:     0.5,
	})
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	for _, key := range []string{
		"external_action",
		"smart_bid_type",
		"bid",
		"material_ids",
		"material_type",
	} {
		if _, exists := payload[key]; exists {
			t.Fatalf("product SMART_BOOST payload unexpectedly contains %q: %s", key, body)
		}
	}
	for _, key := range []string{"advertiser_id", "ad_id", "name", "scene", "budget", "duration"} {
		if _, exists := payload[key]; !exists {
			t.Fatalf("product SMART_BOOST payload omitted %q: %s", key, body)
		}
	}
}

func TestBuildControlTaskCreateValidatesSceneRequirements(t *testing.T) {
	tests := []UniPromotionControlTaskCreateReq{
		{AdId: 1, Scene: "SMART_BOOST", Budget: 1, Duration: 0.5},
		{AdvertiserId: 1, Scene: "SMART_BOOST", Budget: 1, Duration: 0.5},
		{AdvertiserId: 1, AdId: 1, Scene: "UNKNOWN", Budget: 1, Duration: 0.5},
		{AdvertiserId: 1, AdId: 1, Scene: "SMART_BOOST", Duration: 0.5},
		{AdvertiserId: 1, AdId: 1, Scene: "SMART_BOOST", Budget: 1, Duration: 0.25},
		{AdvertiserId: 1, AdId: 1, Scene: "SMART_BOOST", Budget: 1, Duration: 24.5},
		{AdvertiserId: 1, AdId: 1, Scene: "MATERIAL_ADD_BUDGET", Budget: 10, Duration: 1},
	}
	for _, req := range tests {
		if _, err := buildControlTaskCreatePayload(req); err == nil {
			t.Fatalf("invalid control-task payload accepted: %#v", req)
		}
	}
}
