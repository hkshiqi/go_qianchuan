package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
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
	AccessToken            string
	AdvertiserId           uint64
	AwemeId                uint64
	Page                   int
	PageSize               int
	ProductIDs             []uint64
	ProductName            string
	CreateROI2LimitProduct bool
	OverallMarketing       bool
	FilterNotDelivery      bool
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
	AccessToken        string
	AdvertiserId       uint64
	Name               string
	AwemeId            uint64
	ProductId          uint64
	Budget             float64
	StartDate          string
	EndDate            string
	DeepExternalAction string
	ProductCard        bool
	ProductImage       string
	ProductTitle       string
	SellingPoints      []string
	PromotionCardImage string
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

type QianchuanImageGetReq struct {
	AccessToken  string
	AdvertiserId uint64
	ImageMode    string
	ImageIDs     []string
	Page         int
	PageSize     int
}

type QianchuanImage struct {
	ID         string `json:"id"`
	Size       int64  `json:"size"`
	Width      int64  `json:"width"`
	Height     int64  `json:"height"`
	URL        string `json:"url"`
	Signature  string `json:"signature"`
	MaterialID uint64 `json:"material_id"`
	Filename   string `json:"filename"`
	ImageMode  string `json:"image_mode"`
	CreateTime string `json:"create_time"`
	Source     string `json:"source"`
}

type QianchuanImageGetRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		List     []QianchuanImage `json:"list"`
		PageInfo PageInfo         `json:"page_info"`
	} `json:"data"`
}

type QianchuanVideoGetReq struct {
	AccessToken  string
	AdvertiserId uint64
	MaterialIDs  []int64
	Page         int
	PageSize     int
}

type QianchuanVideo struct {
	ID         string  `json:"id"`
	VideoID    string  `json:"video_id"`
	MaterialID uint64  `json:"material_id"`
	Duration   float64 `json:"duration"`
	CreateTime string  `json:"create_time"`
	Filename   string  `json:"filename"`
	ImageMode  string  `json:"image_mode"`
	Source     string  `json:"source"`
}

type QianchuanVideoGetRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		List     []QianchuanVideo `json:"list"`
		PageInfo PageInfo         `json:"page_info"`
	} `json:"data"`
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
	return c.uniPromotionProductAvailableGet(
		ctx,
		req,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_promotion/product/get/",
	)
}

func (c *OceanengineClient) uniPromotionProductAvailableGet(
	ctx context.Context,
	req UniPromotionProductAvailableGetReq,
	endpointBase string,
) (*UniPromotionProductAvailableGetRes, error) {
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("advertiser_id", strconv.FormatUint(req.AdvertiserId, 10))
	if req.AwemeId > 0 {
		values.Set("aweme_id", strconv.FormatUint(req.AwemeId, 10))
	}
	if req.OverallMarketing {
		values.Set("is_overall_marketing", "true")
	}
	filtering := map[string]any{"tab": "ALL"}
	if len(req.ProductIDs) > 0 {
		filtering["product_ids"] = req.ProductIDs
	}
	if req.ProductName != "" {
		filtering["product_name"] = req.ProductName
	}
	if req.CreateROI2LimitProduct {
		filtering["create_roi2_limit_product"] = true
	}
	if req.FilterNotDelivery {
		filtering["filter_not_delivery_product"] = true
	}
	encodedFiltering, _ := json.Marshal(filtering)
	values.Set("filtering", string(encodedFiltering))
	if req.Page > 0 {
		values.Set("page", strconv.Itoa(req.Page))
	}
	if req.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(req.PageSize))
	}
	endpoint := endpointBase + "?" + values.Encode()
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
	if err := validateUniPromotionProductPlanCreateReq(req); err != nil {
		return nil, err
	}
	promotionCardTitle := "红卫好物"
	if req.ProductCard {
		promotionCardTitle = req.ProductTitle
	}
	creative := map[string]any{
		"product_id":    req.ProductId,
		"creative_type": "PROGRAMMATIC_CREATIVE",
		"hide_in_aweme": true,
		"creative_card": map[string]any{
			"promotion_card_title":          promotionCardTitle,
			"promotion_card_action_button":  "查看详情",
			"promotion_card_selling_points": req.SellingPoints,
			"promotion_card_image_id":       req.PromotionCardImage,
		},
	}
	if req.ProductCard {
		creative["image_material"] = []map[string]any{{
			"image_mode": "SQUARE",
			"image_ids":  []string{req.ProductImage},
		}}
		creative["title_material"] = []map[string]any{{
			"title":      req.ProductTitle,
			"title_type": "COMMODITY_CARD",
		}}
	}
	payload := map[string]any{
		"advertiser_id":  req.AdvertiserId,
		"name":           req.Name,
		"marketing_goal": "VIDEO_PROM_GOODS",
		"product_ids":    []uint64{req.ProductId},
		"delivery_setting": map[string]any{
			"smart_bid_type":       "SMART_BID_CONSERVATIVE",
			"qcpx_mode":            "QCPX_MODE_OFF",
			"deep_external_action": req.DeepExternalAction,
			"budget":               req.Budget,
			"video_schedule_type":  "SCHEDULE_START_END",
			"start_time":           req.StartDate,
			"end_time":             req.EndDate,
		},
		"creative_setting": map[string]any{
			"smart_select_material": false,
			"hide_in_aweme":         true,
		},
		"multi_product_creative_list": []map[string]any{creative},
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

func validateUniPromotionProductPlanCreateReq(req UniPromotionProductPlanCreateReq) error {
	if req.AdvertiserId == 0 {
		return errors.New("advertiser_id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if req.ProductId == 0 {
		return errors.New("product_id is required")
	}
	if req.Budget < 30 {
		return errors.New("budget must be at least 30 CNY")
	}
	if req.StartDate == "" || req.EndDate == "" {
		return errors.New("start_date and end_date are required")
	}
	if req.ProductCard {
		if req.DeepExternalAction != "AD_CONVERT_TYPE_LIVE_PURE_PAY_ROI" {
			return errors.New(
				"deep_external_action must be AD_CONVERT_TYPE_LIVE_PURE_PAY_ROI for a pure product-card plan",
			)
		}
		if strings.TrimSpace(req.ProductImage) == "" {
			return errors.New("product_image is required for a pure product-card plan")
		}
		if utf8.RuneCountInString(strings.TrimSpace(req.ProductTitle)) < 10 {
			return errors.New("product_title must contain at least 10 characters for a pure product-card plan")
		}
		if len(req.SellingPoints) == 0 {
			return errors.New("selling_points is required for a pure product-card plan")
		}
		if strings.TrimSpace(req.PromotionCardImage) == "" {
			return errors.New("promotion_card_image is required for a pure product-card plan")
		}
		for _, sellingPoint := range req.SellingPoints {
			width := platformDisplayWidth(strings.TrimSpace(sellingPoint))
			if width < 11 || width > 18 {
				return errors.New("each selling point must contain between 11 and 18 display characters")
			}
		}
	}
	return nil
}

func platformDisplayWidth(text string) int {
	width := 0
	for _, r := range text {
		if r > 0x7f {
			width += 2
		} else {
			width++
		}
	}
	return width
}

func (c *OceanengineClient) QianchuanImageGet(
	ctx context.Context,
	req QianchuanImageGetReq,
) (*QianchuanImageGetRes, error) {
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	mode := req.ImageMode
	if mode == "" {
		mode = "SQUARE"
	}
	filter := map[string]any{"image_mode": []string{mode}}
	if len(req.ImageIDs) > 0 {
		filter["image_ids"] = req.ImageIDs
	}
	encodedFilter, _ := json.Marshal(filter)
	values := url.Values{}
	values.Set("advertiser_id", strconv.FormatUint(req.AdvertiserId, 10))
	values.Set("filtering", string(encodedFilter))
	values.Set("page", strconv.Itoa(page))
	values.Set("page_size", strconv.Itoa(pageSize))
	endpoint := "https://api.oceanengine.com/open_api/v1.0/qianchuan/image/get/?" + values.Encode()
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
		return nil, fmt.Errorf("qianchuan image get HTTP %d: %s", response.StatusCode, string(body))
	}
	var result QianchuanImageGetRes
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode qianchuan image list: %w", err)
	}
	return &result, nil
}

func (c *OceanengineClient) QianchuanVideoGet(
	ctx context.Context,
	req QianchuanVideoGetReq,
) (*QianchuanVideoGetRes, error) {
	return c.qianchuanVideoGet(
		ctx,
		req,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/video/get/",
	)
}

func (c *OceanengineClient) qianchuanVideoGet(
	ctx context.Context,
	req QianchuanVideoGetReq,
	endpointBase string,
) (*QianchuanVideoGetRes, error) {
	if err := c.throttle.wait(ctx); err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 100 {
		pageSize = 100
	}
	filter := map[string]any{}
	if len(req.MaterialIDs) > 0 {
		filter["material_ids"] = req.MaterialIDs
	}
	encodedFilter, _ := json.Marshal(filter)
	values := url.Values{}
	values.Set("advertiser_id", strconv.FormatUint(req.AdvertiserId, 10))
	values.Set("filtering", string(encodedFilter))
	values.Set("page", strconv.Itoa(page))
	values.Set("page_size", strconv.Itoa(pageSize))
	endpoint := endpointBase + "?" + values.Encode()
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
		return nil, fmt.Errorf("qianchuan video get HTTP %d: %s", response.StatusCode, string(body))
	}
	var result QianchuanVideoGetRes
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode qianchuan video list: %w", err)
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
	return c.postPlanCRUDJSONWithEndpoint(ctx, accessToken, endpoint, body, target)
}

func (c *OceanengineClient) postPlanCRUDJSONWithEndpoint(
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
