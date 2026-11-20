package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const OverallProductPlanMinimumBudget = 300.0

type OverallAwemeListReq struct {
	AccessToken  string
	AdvertiserId uint64
	ProductId    uint64
	Page         int
	PageSize     int
	CanApplyOnly bool
}

type OverallAweme struct {
	ID                   uint64   `json:"id"`
	Name                 string   `json:"name"`
	ShowID               string   `json:"show_id"`
	Avatar               string   `json:"avatar"`
	AuthTypes            []string `json:"auth_types"`
	HasAuthorized        bool     `json:"has_authorized"`
	CanControl           bool     `json:"can_control_uniprom"`
	CanApply             bool     `json:"can_apply_uniprom"`
	HasShopPermission    bool     `json:"has_shop_permission"`
	IsAwemeC             bool     `json:"is_aweme_c"`
	Disabled             bool     `json:"is_product_uni_prom_disabled"`
	DisableReasons       []string `json:"product_disable_reasons"`
	MultiAwemeAuthStatus string   `json:"multi_aweme_uni_prom_auth_status"`
	CanDeliverMall       bool     `json:"can_deliver_mall"`
}

type OverallAwemeListRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		BindAwemeUserInfos []struct {
			AwemeUserInfo struct {
				AwemeID     uint64 `json:"aweme_id"`
				AwemeName   string `json:"aweme_name"`
				AwemeShowID string `json:"aweme_show_id"`
				AwemeAvatar string `json:"aweme_avatar"`
			} `json:"aweme_user_info"`
			AuthTypes              []string `json:"auth_types"`
			HasAuthorized          bool     `json:"has_authorized"`
			CanControlUniProm      bool     `json:"can_control_uniprom"`
			CanApplyUniProm        bool     `json:"can_apply_uniprom"`
			HasShopPermission      bool     `json:"has_shop_permission"`
			IsAwemeC               bool     `json:"is_aweme_c"`
			ProductUniPromDisabled bool     `json:"is_product_uni_prom_disabled"`
			ProductDisableReasons  []string `json:"product_disable_reasons"`
			MultiAwemeAuthStatus   string   `json:"multi_aweme_uni_prom_auth_status"`
			ProductUniPromInfo     struct {
				IsAllowMall bool `json:"is_allow_mall"`
			} `json:"product_uni_prom_info"`
		} `json:"bind_aweme_user_infos"`
		Pagination PageInfo `json:"pagination"`
	} `json:"data"`
}

type OverallProductPlanCreateReq struct {
	AccessToken  string
	AdvertiserId uint64
	Name         string
	ProductId    uint64
	AwemeId      uint64
	Budget       float64
	Roi2Goal     float64
	StartDate    string
	EndDate      string
}

type OverallProductPlanCreateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		AdId uint64 `json:"ad_id"`
	} `json:"data"`
}

func (c *OceanengineClient) OverallAwemeList(
	ctx context.Context,
	req OverallAwemeListReq,
) (*OverallAwemeListRes, error) {
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}
	filtering := map[string]any{
		"filter_can_apply": req.CanApplyOnly,
	}
	if req.ProductId > 0 {
		filtering["product_id"] = req.ProductId
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	pageParams := map[string]int{"page": page, "page_size": pageSize}
	encodedFilter, _ := json.Marshal(filtering)
	encodedPage, _ := json.Marshal(pageParams)
	values := url.Values{}
	values.Set("advertiser_id", strconv.FormatUint(req.AdvertiserId, 10))
	values.Set("filter", string(encodedFilter))
	values.Set("page_params", string(encodedPage))
	endpoint := "https://api.oceanengine.com/open_api/v1.0/qianchuan/overall_aweme/list/?" +
		values.Encode()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Access-Token", req.AccessToken)
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("overall aweme list HTTP %d: %s", response.StatusCode, string(body))
	}
	var result OverallAwemeListRes
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode overall aweme list: %w", err)
	}
	return &result, nil
}

func (c *OceanengineClient) OverallProductPlanCreate(
	ctx context.Context,
	req OverallProductPlanCreateReq,
) (*OverallProductPlanCreateRes, error) {
	if err := validateOverallProductPlanCreateReq(req); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"advertiser_id":  req.AdvertiserId,
		"name":           req.Name,
		"marketing_goal": "VIDEO_PROM_GOODS",
		"product_ids":    []uint64{req.ProductId},
		"delivery_setting": map[string]any{
			"smart_bid_type":       "SMART_BID_CUSTOM",
			"roi2_goal":            req.Roi2Goal,
			"qcpx_mode":            "QCPX_MODE_OFF",
			"deep_external_action": "AD_CONVERT_TYPE_LIVE_PURE_PAY_ROI",
			"budget":               req.Budget,
			"video_schedule_type":  "SCHEDULE_START_END",
			"start_time":           req.StartDate,
			"end_time":             req.EndDate,
		},
		"multi_product_creative_list": []map[string]any{{
			"product_id":    req.ProductId,
			"aweme_uid":     req.AwemeId,
			"creative_type": "PROGRAMMATIC_CREATIVE",
			"hide_in_aweme": true,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result OverallProductPlanCreateRes
	if err := c.postPlanCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/overall_video/create/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateOverallProductPlanCreateReq(req OverallProductPlanCreateReq) error {
	if strings.TrimSpace(req.AccessToken) == "" {
		return errors.New("overall product plan access token is required")
	}
	if req.AdvertiserId == 0 {
		return errors.New("overall product plan advertiser_id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("overall product plan name is required")
	}
	if req.ProductId == 0 {
		return errors.New("overall product plan product_id is required")
	}
	if req.AwemeId == 0 {
		return errors.New("overall product plan aweme_id is required")
	}
	if req.Budget < OverallProductPlanMinimumBudget {
		return fmt.Errorf(
			"overall product plan budget must be at least %.0f CNY",
			OverallProductPlanMinimumBudget,
		)
	}
	if req.Roi2Goal <= 0 || req.Roi2Goal > 100 {
		return errors.New("overall product plan roi2_goal must be greater than 0 and at most 100")
	}
	startDate, err := time.ParseInLocation("2006-01-02", req.StartDate, shanghaiTimezone())
	if err != nil {
		return errors.New("overall product plan start_date must use YYYY-MM-DD")
	}
	endDate, err := time.ParseInLocation("2006-01-02", req.EndDate, shanghaiTimezone())
	if err != nil {
		return errors.New("overall product plan end_date must use YYYY-MM-DD")
	}
	now := time.Now().In(shanghaiTimezone())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, shanghaiTimezone())
	if !startDate.After(today) {
		return errors.New("overall product plan start_date must be tomorrow or later")
	}
	if endDate.Before(startDate) {
		return errors.New("overall product plan end_date cannot be earlier than start_date")
	}
	return nil
}

func shanghaiTimezone() *time.Location {
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func (c *OceanengineClient) OverallProductPlanNameUpdate(
	ctx context.Context,
	req OverallProductPlanNameUpdateReq,
) (*UniPromotionNameUpdateRes, error) {
	payload := map[string]any{
		"advertiser_id": req.AdvertiserId,
		"ad_id":         req.AdId,
		"name":          req.Name,
		"multi_product_creative_list": []map[string]any{{
			"product_id":    req.ProductId,
			"aweme_uid":     req.AwemeId,
			"creative_type": "PROGRAMMATIC_CREATIVE",
			"hide_in_aweme": true,
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result UniPromotionNameUpdateRes
	if err := c.postPlanCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/overall_video/update/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}
