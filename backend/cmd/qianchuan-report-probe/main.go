package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/tokenmanager"
	"github.com/CriarBrand/qianchuan-backend/pkg/tokenstore"
	"github.com/joho/godotenv"
)

type probeOutput struct {
	AccountID       string                           `json:"account_id"`
	DataTopic       string                           `json:"data_topic"`
	ConfigRequestID string                           `json:"config_request_id,omitempty"`
	QueryLimit      sdk.UniPromotionReportQueryLimit `json:"query_limit"`
	Dimensions      []dimensionOutput                `json:"dimensions"`
	Metrics         []sdk.UniPromotionReportMetric   `json:"metrics"`
	Data            *dataProbeOutput                 `json:"data,omitempty"`
	PlanMaterials   *planMaterialProbeOutput         `json:"plan_materials,omitempty"`
}

type dataProbeOutput struct {
	RequestURL  string          `json:"request_url"`
	HTTPStatus  int             `json:"http_status"`
	RequestID   string          `json:"request_id,omitempty"`
	Code        int64           `json:"code"`
	Message     string          `json:"message,omitempty"`
	RawResponse json.RawMessage `json:"raw_response"`
}

type planMaterialProbeOutput struct {
	RequestURL  string          `json:"request_url"`
	HTTPStatus  int             `json:"http_status"`
	RequestID   string          `json:"request_id,omitempty"`
	Code        int64           `json:"code"`
	Message     string          `json:"message,omitempty"`
	RawResponse json.RawMessage `json:"raw_response"`
}

type dimensionOutput struct {
	Field            string                                   `json:"field"`
	Name             string                                   `json:"name"`
	Filterable       bool                                     `json:"filterable"`
	FilterOnly       bool                                     `json:"filter_only"`
	IsRequired       bool                                     `json:"is_required"`
	ExclusionDims    []string                                 `json:"exclusion_dims,omitempty"`
	ExclusionMetrics []string                                 `json:"exclusion_metrics,omitempty"`
	FilterRequired   bool                                     `json:"filter_required"`
	FilterOperator   int64                                    `json:"filter_operator,omitempty"`
	FilterValues     []string                                 `json:"filter_values,omitempty"`
	FilterOptions    []sdk.UniPromotionReportFilterRangeValue `json:"filter_options,omitempty"`
}

func main() {
	envFile := flag.String("env", "", "optional dotenv file")
	tenantDir := flag.String("tenant-dir", "/opt/go-qianchuan/data/tenants", "tenant token directory")
	tenantID := flag.String("tenant", "owner", "tenant id")
	advertiserID := flag.String("advertiser-id", "", "optional advertiser id override")
	dataTopic := flag.String("data-topic", "SITE_PROMOTION_POST_DATA_VIDEO", "official report data topic")
	awemeID := flag.String("aweme-id", "", "required when probing report data")
	probeData := flag.Bool("probe-data", false, "also call the official data endpoint")
	probePlanMaterials := flag.Bool("probe-plan-materials", false, "also call the official plan-material endpoint")
	startTime := flag.String("start-time", "", "data endpoint start time")
	endTime := flag.String("end-time", "", "data endpoint end time")
	planID := flag.String("plan-id", "", "required when probing plan materials")
	materialType := flag.String("material-type", "VIDEO", "plan-material type: VIDEO or LIVE_ROOM")
	materialSelectType := flag.String("material-select-type", "ALL", "plan-material selection type")
	metricsArg := flag.String("metrics", "stat_cost_for_roi2", "comma-separated metric fields")
	dimensionsArg := flag.String("dimensions", "material_id,roi2_material_video_name,roi2_material_video_type", "comma-separated dimension fields")
	smartBidValue := flag.String("smart-bid", "0", "aggregate_smart_bid_type filter code")
	platformValue := flag.String("platform", "1", "ecp_app_id filter code")
	videoTypeValue := flag.String("video-type", "1", "roi2_material_video_type filter code")
	pageSize := flag.Int("page-size", 10, "official page size: 10, 20, 50, 100 or 200")
	orderField := flag.String("order-field", "stat_cost_for_roi2", "official report order field")
	orderType := flag.Int("order-type", 2, "official report order type: 1 asc, 2 desc")
	legacy := flag.Bool("legacy", false, "probe legacy report material endpoint instead")
	flag.Parse()

	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			log.Fatalf("load env: %v", err)
		}
	}
	appID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("APP_ID")), 10, 64)
	if err != nil || appID <= 0 {
		log.Fatal("APP_ID is required")
	}
	appSecret := strings.TrimSpace(os.Getenv("APP_SECRET"))
	if appSecret == "" {
		log.Fatal("APP_SECRET is required")
	}
	encryptionKey := strings.TrimSpace(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if encryptionKey == "" {
		log.Fatal("TOKEN_ENCRYPTION_KEY is required")
	}
	account, err := strconv.ParseInt(strings.TrimSpace(*advertiserID), 10, 64)
	if err != nil || account <= 0 {
		log.Fatal("--advertiser-id is required")
	}

	store, err := tokenstore.New(filepath.Join(*tenantDir, *tenantID, "tokens.json"), encryptionKey)
	if err != nil {
		log.Fatalf("open tenant token store: %v", err)
	}
	client := sdk.NewOceanengineClient(appID, appSecret)
	manager, err := tokenmanager.New(client, store)
	if err != nil {
		log.Fatalf("create token manager: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	token, err := manager.AccessToken(ctx, account)
	if err != nil {
		log.Fatalf("load access token: %v", err)
	}
	response, err := client.UniPromotionReportConfig(ctx, sdk.UniPromotionReportConfigReq{
		AccessToken:  token,
		AdvertiserId: uint64(account),
		DataTopics:   []string{*dataTopic},
	})
	if err != nil {
		log.Fatalf("get official report config: %v", err)
	}
	if response.Code != 0 {
		log.Fatalf("official report config failed: code=%d message=%s request_id=%s",
			response.Code, response.Message, response.RequestID)
	}

	var topic *sdk.UniPromotionReportTopicConfig
	for index := range response.Data.CustomConfigDatas {
		if response.Data.CustomConfigDatas[index].DataTopic == *dataTopic {
			topic = &response.Data.CustomConfigDatas[index]
			break
		}
	}
	if topic == nil {
		log.Fatalf("official report config omitted %s", *dataTopic)
	}
	output := probeOutput{
		AccountID:       strconv.FormatInt(account, 10),
		DataTopic:       *dataTopic,
		ConfigRequestID: response.RequestID,
		QueryLimit:      topic.QueryLimit,
		Dimensions:      make([]dimensionOutput, 0, len(topic.Dimensions)),
		Metrics:         topic.Metrics,
	}
	for _, dimension := range topic.Dimensions {
		item := dimensionOutput{
			Field:            dimension.Field,
			Name:             dimension.Name,
			Filterable:       dimension.Filterable,
			FilterOnly:       dimension.FilterOnly,
			IsRequired:       dimension.IsRequired,
			ExclusionDims:    dimension.ExclusionDims,
			ExclusionMetrics: dimension.ExclusionMetric,
		}
		if dimension.FilterConfig != nil {
			item.FilterRequired = dimension.FilterConfig.FilterRequired
			item.FilterOperator = dimension.FilterConfig.Operator
			for _, value := range dimension.FilterConfig.RangeValues {
				item.FilterValues = append(item.FilterValues, value.Value)
			}
			item.FilterOptions = dimension.FilterConfig.RangeValues
		}
		output.Dimensions = append(output.Dimensions, item)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		log.Fatalf("encode output: %v", err)
	}
	if *probeData {
		if *awemeID == "" || *startTime == "" || *endTime == "" {
			log.Fatal("--aweme-id, --start-time and --end-time are required with --probe-data")
		}
		dimensions := splitCSV(*dimensionsArg)
		metrics := splitCSV(*metricsArg)
		values := url.Values{}
		values.Set("advertiser_id", strconv.FormatInt(account, 10))
		values.Set("data_topic", *dataTopic)
		values.Set("dimensions", marshalStringSlice(dimensions))
		values.Set("metrics", marshalStringSlice(metrics))
		values.Set("filters", fmt.Sprintf(
			`[{"field":"anchor_id","operator":7,"values":["%s"]},{"field":"aggregate_smart_bid_type","operator":7,"values":["%s"]},{"field":"ecp_app_id","operator":7,"values":["%s"]},{"field":"roi2_material_video_type","operator":7,"values":["%s"]}]`,
			*awemeID, *smartBidValue, *platformValue, *videoTypeValue,
		))
		values.Set("start_time", *startTime)
		values.Set("end_time", *endTime)
		orderBy, _ := json.Marshal([]map[string]any{{"type": *orderType, "field": *orderField}})
		values.Set("order_by", string(orderBy))
		values.Set("page", "1")
		values.Set("page_size", strconv.Itoa(*pageSize))
		endpoint := "https://api.oceanengine.com/open_api/v1.0/qianchuan/report/uni_promotion/data/get/?"
		if *legacy {
			values = url.Values{}
			values.Set("advertiser_id", strconv.FormatInt(account, 10))
			values.Set("start_date", strings.Split(*startTime, " ")[0])
			values.Set("end_date", strings.Split(*endTime, " ")[0])
			values.Set("fields", marshalStringSlice([]string{
				"stat_cost",
				"show_cnt",
				"click_cnt",
				"ctr",
				"convert_cnt",
				"conversion_rate",
				"pay_order_amount",
				"pay_order_roi",
			}))
			values.Set("filtering", fmt.Sprintf(
				`{"marketing_goal":"LIVE_PROM_GOODS","material_type":"VIDEO","aweme_ids":["%s"]}`,
				*awemeID,
			))
			values.Set("page", "1")
			values.Set("page_size", "10")
			endpoint = "https://api.oceanengine.com/open_api/v1.0/qianchuan/report/material/get/?"
		}
		requestURL := endpoint + values.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			log.Fatalf("create data probe request: %v", err)
		}
		request.Header.Set("Access-Token", token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			log.Fatalf("call data probe: %v", err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
		if err != nil {
			log.Fatalf("read data probe response: %v", err)
		}
		dataResult := &dataProbeOutput{
			RequestURL:  requestURL,
			HTTPStatus:  response.StatusCode,
			RawResponse: append(json.RawMessage(nil), body...),
		}
		var envelope struct {
			Code      int64  `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(body, &envelope)
		dataResult.Code = envelope.Code
		dataResult.Message = envelope.Message
		dataResult.RequestID = envelope.RequestID
		output.Data = dataResult
		if err := encoder.Encode(output); err != nil {
			log.Fatalf("encode data probe output: %v", err)
		}
	}
	if *probePlanMaterials {
		if *planID == "" || *startTime == "" || *endTime == "" {
			log.Fatal("--plan-id, --start-time and --end-time are required with --probe-plan-materials")
		}
		filtering, _ := json.Marshal(map[string]string{
			"material_type":        *materialType,
			"material_select_type": *materialSelectType,
			"material_status":      "ALL",
			"start_date":           strings.Split(*startTime, " ")[0],
			"end_date":             strings.Split(*endTime, " ")[0],
		})
		values := url.Values{}
		values.Set("advertiser_id", strconv.FormatInt(account, 10))
		values.Set("ad_id", *planID)
		values.Set("filtering", string(filtering))
		values.Set("fields", marshalStringSlice([]string{
			"stat_cost_for_roi2",
			"live_show_count_for_roi2_v2",
			"live_watch_count_for_roi2_v2",
			"live_cvr_rate_for_roi2_v2",
			"live_convert_rate_for_roi2_v2",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
		}))
		values.Set("order_field", "stat_cost_for_roi2")
		values.Set("order_type", "DESC")
		values.Set("page", "1")
		values.Set("page_size", "10")
		requestURL := "https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_promotion/ad/material/get/?" + values.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			log.Fatalf("create plan material probe request: %v", err)
		}
		request.Header.Set("Access-Token", token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			log.Fatalf("call plan material probe: %v", err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
		if err != nil {
			log.Fatalf("read plan material probe response: %v", err)
		}
		planResult := &planMaterialProbeOutput{
			RequestURL:  requestURL,
			HTTPStatus:  response.StatusCode,
			RawResponse: append(json.RawMessage(nil), body...),
		}
		var envelope struct {
			Code      int64  `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal(body, &envelope)
		planResult.Code = envelope.Code
		planResult.Message = envelope.Message
		planResult.RequestID = envelope.RequestID
		output.PlanMaterials = planResult
		if err := encoder.Encode(output); err != nil {
			log.Fatalf("encode plan material probe output: %v", err)
		}
	}
	fmt.Fprintln(os.Stderr, "official report config probe completed without exposing credentials")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func marshalStringSlice(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}
