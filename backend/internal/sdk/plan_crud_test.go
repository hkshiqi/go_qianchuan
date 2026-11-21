package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func TestUniPromotionProductAvailableGetOfficialFilters(t *testing.T) {
	var capturedURL string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedURL = r.URL.String()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"OK","data":{"product_list":[],"page_info":{"page":1,"page_size":10,"total_page":0,"total_number":0}}}`))
	}))
	defer server.Close()

	client := &OceanengineClient{throttle: &apiThrottle{}}
	_, err := client.uniPromotionProductAvailableGet(
		context.Background(),
		UniPromotionProductAvailableGetReq{
			AccessToken:            "test-token",
			AdvertiserId:           1855810750424170,
			AwemeId:                63307887035,
			Page:                   1,
			PageSize:               10,
			OverallMarketing:       true,
			FilterNotDelivery:      true,
			CreateROI2LimitProduct: true,
		},
		server.URL+"/open_api/v1.0/qianchuan/uni_promotion/product/get/",
	)
	if err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	parsedURL, err := url.Parse(capturedURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsedURL.Query()
	if got := query.Get("is_overall_marketing"); got != "true" {
		t.Fatalf("is_overall_marketing = %q, want true", got)
	}
	filter := query.Get("filtering")
	if filter == "" {
		t.Fatalf("missing filtering parameter in %q", capturedURL)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(filter), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["filter_not_delivery_product"] != true {
		t.Fatalf("filter_not_delivery_product missing from filtering: %#v", parsed)
	}
	if parsed["create_roi2_limit_product"] != true {
		t.Fatalf("create_roi2_limit_product missing from filtering: %#v", parsed)
	}
}
