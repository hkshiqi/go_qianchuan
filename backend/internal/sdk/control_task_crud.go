package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *OceanengineClient) UniPromotionControlTaskStatusUpdate(
	ctx context.Context,
	req UniPromotionControlTaskStatusUpdateReq,
) (*UniPromotionControlTaskStatusUpdateRes, error) {
	if req.AdvertiserId == 0 {
		return nil, errors.New("advertiser_id is required")
	}
	if len(req.TaskIds) == 0 || len(req.TaskIds) > 10 {
		return nil, errors.New("task_ids must contain between 1 and 10 IDs")
	}
	for _, taskID := range req.TaskIds {
		if taskID == 0 {
			return nil, errors.New("task_ids cannot contain zero")
		}
	}
	switch req.OptType {
	case "PAUSE", "ENABLE", "DISABLE", "DELETE":
	default:
		return nil, fmt.Errorf("unsupported opt_type %q", req.OptType)
	}
	body, err := json.Marshal(map[string]any{
		"advertiser_id": req.AdvertiserId,
		"task_ids":      req.TaskIds,
		"opt_type":      req.OptType,
	})
	if err != nil {
		return nil, err
	}
	var result UniPromotionControlTaskStatusUpdateRes
	if err := c.postControlTaskCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_promotion/ad/control_task/status/update/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *OceanengineClient) UniPromotionControlTaskUpdate(
	ctx context.Context,
	req UniPromotionControlTaskUpdateReq,
) (*UniPromotionControlTaskUpdateRes, error) {
	if req.AdvertiserId == 0 || req.TaskID == 0 {
		return nil, errors.New("advertiser_id and task_id are required")
	}
	if len(req.Name) < 1 || len(req.Name) > 50 {
		return nil, errors.New("name must contain between 1 and 50 characters")
	}
	if req.Budget <= 0 {
		return nil, errors.New("budget must be greater than zero")
	}
	if len(req.MaterialIDs) > 20 {
		return nil, errors.New("material_ids cannot contain more than 20 IDs")
	}
	payload := map[string]any{
		"advertiser_id": req.AdvertiserId,
		"task_id":       req.TaskID,
		"name":          req.Name,
		"budget":        req.Budget,
	}
	if req.Roi2Goal > 0 {
		payload["roi2_goal"] = req.Roi2Goal
	}
	if len(req.MaterialIDs) > 0 {
		payload["material_ids"] = req.MaterialIDs
	}
	if req.Duration > 0 {
		payload["duration"] = req.Duration
	}
	if req.Bid > 0 {
		payload["bid"] = req.Bid
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var result UniPromotionControlTaskUpdateRes
	if err := c.postControlTaskCRUDJSON(
		ctx,
		req.AccessToken,
		"https://api.oceanengine.com/open_api/v1.0/qianchuan/uni_promotion/ad/control_task/update/",
		body,
		&result,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func buildControlTaskCreatePayload(req UniPromotionControlTaskCreateReq) ([]byte, error) {
	if req.AdvertiserId == 0 || req.AdId == 0 {
		return nil, errors.New("advertiser_id and ad_id are required")
	}
	if req.Scene != "SMART_BOOST" && req.Scene != "MATERIAL_ADD_BUDGET" {
		return nil, fmt.Errorf("unsupported scene %q", req.Scene)
	}
	if req.Budget <= 0 {
		return nil, errors.New("budget must be greater than zero")
	}
	if req.Scene == "SMART_BOOST" && (req.Duration < 0.5 || req.Duration > 24 || req.Duration*2 != float64(int(req.Duration*2))) {
		return nil, errors.New("SMART_BOOST duration must be 0.5-24 hours in 0.5-hour steps")
	}
	if req.Scene == "MATERIAL_ADD_BUDGET" && len(req.MaterialIds) == 0 {
		return nil, errors.New("material_ids are required for MATERIAL_ADD_BUDGET")
	}
	if len(req.MaterialIds) > 20 {
		return nil, errors.New("material_ids cannot contain more than 20 IDs")
	}
	payload := map[string]any{
		"advertiser_id": req.AdvertiserId,
		"ad_id":         req.AdId,
		"scene":         req.Scene,
		"budget":        req.Budget,
	}
	if strings.TrimSpace(req.Name) != "" {
		payload["name"] = req.Name
	}
	if req.Duration > 0 {
		payload["duration"] = req.Duration
	}
	if len(req.MaterialIds) > 0 {
		payload["material_ids"] = req.MaterialIds
	}
	if req.ExternalAction != "" {
		payload["external_action"] = req.ExternalAction
	}
	if req.SmartBidType != "" {
		payload["smart_bid_type"] = req.SmartBidType
	}
	if req.Bid > 0 {
		payload["bid"] = req.Bid
	}
	if req.MaterialType != "" {
		payload["material_type"] = req.MaterialType
	}
	return json.Marshal(payload)
}

func (c *OceanengineClient) postControlTaskCRUDJSON(
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
		return fmt.Errorf("qianchuan control task CRUD HTTP %d: %s", response.StatusCode, string(responseBody))
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode qianchuan control task CRUD response: %w", err)
	}
	return nil
}
