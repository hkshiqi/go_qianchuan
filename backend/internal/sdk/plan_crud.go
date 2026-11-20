package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type UniPromotionProduct struct {
	ID             uint64   `json:"id"`
	Name           string   `json:"name"`
	Img            string   `json:"img"`
	Tag            []string `json:"tag,omitempty"`
	CategoryName   string   `json:"category_name,omitempty"`
	SellNum        int64    `json:"sell_num,omitempty"`
	StockNum       int64    `json:"stock_num,omitempty"`
	AuditTime      string   `json:"audit_time,omitempty"`
	GrayReason     []string `json:"gray_reason,omitempty"`
	ChannelID      uint64   `json:"channel_id,omitempty"`
	ChannelType    string   `json:"channel_type,omitempty"`
	SquareImageURL []string `json:"square_image_url,omitempty"`
}

type UniPromotionProductAvailableGetReq struct {
	AccessToken  string
	AdvertiserId uint64
	AwemeId      uint64
	Page         int
	PageSize     int
	ProductIDs   []uint64
	ProductName  string
}

type UniPromotionProductAvailableGetRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		ProductList []UniPromotionProduct `json:"product_list"`
		PageInfo    PageInfo              `json:"page_info"`
	} `json:"data"`
}

type UniPromotionProductPlanCreateReq struct {
	AccessToken  string
	AdvertiserId uint64
	Name         string
	AwemeId      uint64
	ProductId    uint64
	Budget       float64
	StartDate    string
	EndDate      string
}

type UniPromotionProductPlanCreateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		AdId uint64 `json:"ad_id"`
	} `json:"data"`
}

type UniPromotionNameUpdateReq struct {
	AccessToken  string
	AdvertiserId uint64
	AdId         uint64
	Name         string
}

type UniPromotionNameUpdateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type OverallProductPlanNameUpdateReq struct {
	AccessToken  string
	AdvertiserId uint64
	AdId         uint64
	Name         string
	ProductId    uint64
	AwemeId      uint64
}

func (c *OceanengineClient) UniPromotionProductAvailableGet(
	ctx context.Context,
	req UniPromotionProductAvailableGetReq,
) (*UniPromotionProductAvailableGetRes, error) {
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("advertiser_id", strconv.FormatUint(req.AdvertiserId, 10))
	if req.AwemeId > 0 {
		values.Set("aweme_id", strconv.FormatUint(req.AwemeId, 10))
	}
	filtering := map[string]any{"tab": "ALL"}
	if len(req.ProductIDs) > 0 {
		filtering["product_ids"] = req.ProductIDs
	}
	if req.ProductName != "" {
		filtering["product_name"] = req.ProductName
	}
	encodedFiltering, _ := json.Marshal(filtering)
	values.Set("filtering", string(encodedFiltering))
	if req.Page > 0 {
		values.Set("page", strconv.Itoa(req.Page))
	}
	if req.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(req.PageSize))
	}
	endpoint := "https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_promotion/product/get/?" +
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
		return nil, fmt.Errorf("uni promotion product list HTTP %d: %s", response.StatusCode, string(body))
	}
	var result UniPromotionProductAvailableGetRes
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode uni promotion product list: %w", err)
	}
	return &result, nil
}

func (c *OceanengineClient) UniPromotionProductPlanCreate(
	ctx context.Context,
	req UniPromotionProductPlanCreateReq,
) (*UniPromotionProductPlanCreateRes, error) {
	payload := map[string]any{
		"advertiser_id":  req.AdvertiserId,
		"name":           req.Name,
		"marketing_goal": "VIDEO_PROM_GOODS",
		"product_ids":    []uint64{req.ProductId},
		"delivery_setting": map[string]any{
			"smart_bid_type":      "SMART_BID_CONSERVATIVE",
			"qcpx_mode":           "QCPX_MODE_OFF",
			"budget":              req.Budget,
			"video_schedule_type": "SCHEDULE_START_END",
			"start_time":          req.StartDate,
			"end_time":            req.EndDate,
		},
		"creative_setting": map[string]any{
			"smart_select_material": false,
			"hide_in_aweme":         true,
		},
		"multi_product_creative_list": []map[string]any{{
			"product_id":    req.ProductId,
			"creative_type": "PROGRAMMATIC_CREATIVE",
			"hide_in_aweme": true,
			"creative_card": map[string]any{
				"promotion_card_title":         "红卫好物",
				"promotion_card_action_button": "查看详情",
			},
		}},
	}
	if req.AwemeId > 0 {
		payload["aweme_id"] = req.AwemeId
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result UniPromotionProductPlanCreateRes
	if err := c.postPlanCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_aweme/ad/create/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OceanengineClient) UniPromotionNameUpdate(
	ctx context.Context,
	req UniPromotionNameUpdateReq,
) (*UniPromotionNameUpdateRes, error) {
	return c.updatePlanName(ctx, req, "uni_promotion/ad/name/update")
}

func (c *OceanengineClient) updatePlanName(
	ctx context.Context,
	req UniPromotionNameUpdateReq,
	resource string,
) (*UniPromotionNameUpdateRes, error) {
	body, err := json.Marshal(map[string]any{
		"advertiser_id": req.AdvertiserId,
		"ad_id":         req.AdId,
		"name":          req.Name,
	})
	if err != nil {
		return nil, err
	}
	var result UniPromotionNameUpdateRes
	if err := c.postPlanCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/"+resource+"/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OceanengineClient) postPlanCRUDJSON(
	ctx context.Context,
	accessToken string,
	endpoint string,
	body []byte,
	target any,
) error {
	if err := c.throttle.wait(ctx); err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("Access-Token", accessToken)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("qianchuan plan CRUD HTTP %d: %s", response.StatusCode, string(responseBody))
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode qianchuan plan CRUD response: %w", err)
	}
	return nil
}
