package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestQianchuanVideoGetSendsIntegerMaterialIDs(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"OK","data":{"list":[],"page_info":{"page":1,"page_size":100,"total_page":0,"total_number":0}}}`))
	}))
	defer server.Close()

	client := &OceanengineClient{throttle: &apiThrottle{}}
	_, err := client.qianchuanVideoGet(
		context.Background(),
		QianchuanVideoGetReq{
			AccessToken:  "test-token",
			AdvertiserId: 1855810750424170,
			MaterialIDs:  []int64{7654214461952425993, 7660659586093924358},
			Page:         1,
			PageSize:     100,
		},
		server.URL+"/open_api/v1.0/qianchuan/video/get/",
	)
	if err != nil {
		t.Fatal(err)
	}

	parsedURL, err := url.Parse(capturedURL)
	if err != nil {
		t.Fatal(err)
	}
	var filtering struct {
		MaterialIDs []json.Number `json:"material_ids"`
	}
	decoder := json.NewDecoder(strings.NewReader(parsedURL.Query().Get("filtering")))
	decoder.UseNumber()
	if err := decoder.Decode(&filtering); err != nil {
		t.Fatal(err)
	}
	if len(filtering.MaterialIDs) != 2 ||
		filtering.MaterialIDs[0].String() != "7654214461952425993" ||
		filtering.MaterialIDs[1].String() != "7660659586093924358" {
		t.Fatalf("filtering material_ids = %#v", filtering.MaterialIDs)
	}
}
