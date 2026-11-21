package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUniPromotionBudgetUpdateUsesOfficialRequestAndResultContract(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Access-Token"); got != "test-token" {
			t.Fatalf("Access-Token = %q, want test-token", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"OK","data":{"results":[{"ad_id":1882460844769367,"status":"SUCCESS"}]}}`))
	}))
	defer server.Close()

	client := &OceanengineClient{throttle: &apiThrottle{}}
	response, err := client.uniPromotionBudgetUpdateAt(
		context.Background(),
		UniPromotionBudgetUpdateReq{
			AccessToken:  "test-token",
			AdvertiserId: 1855810750424170,
			Data: []UniPromotionBudgetUpdateItem{{
				AdId:   1882460844769367,
				Budget: 301,
			}},
		},
		server.URL+"/open_api/v1.0/qianchuan/uni_promotion/ad/budget/update/",
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || len(response.Data.Results) != 1 ||
		response.Data.Results[0].Status != "SUCCESS" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if requestBody["advertiser_id"] != float64(1855810750424170) {
		t.Fatalf("advertiser_id missing from payload: %#v", requestBody)
	}
	infos, ok := requestBody["update_budget_infos"].([]any)
	if !ok || len(infos) != 1 {
		t.Fatalf("update_budget_infos = %#v", requestBody["update_budget_infos"])
	}
	item, ok := infos[0].(map[string]any)
	if !ok || item["ad_id"] != float64(1882460844769367) || item["budget"] != float64(301) {
		t.Fatalf("unexpected budget item: %#v", infos[0])
	}
}
