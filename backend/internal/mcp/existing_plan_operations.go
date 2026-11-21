package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
)

const existingPlanOperationTestPrefix = "Codex-CRUD-EXISTING-PLAN-TEST-"

type existingPlanSnapshot struct {
	Plan          *sdk.UniPromotionDetail
	MaterialType  string
	MaterialIDs   []string
	VideoIDs      []string
	ImageIDs      []string
	ProductID     uint64
	AwemeUID      uint64
	TaskStartTime string
	TaskEndTime   string
}

func (t *Tools) testExistingProductPlanOperations(
	ctx context.Context,
	args map[string]any,
) (any, error) {
	if err := t.requirePlanCRUDTestWrite(args); err != nil {
		return nil, err
	}
	adID, err := requiredID(args, "ad_id")
	if err != nil {
		return nil, err
	}
	name := stringArg(
		args,
		"name",
		existingPlanOperationTestPrefix+time.Now().In(shanghaiLocation).Format("20060102-150405"),
	)
	if !strings.HasPrefix(strings.TrimSpace(name), existingPlanOperationTestPrefix) {
		return nil, fmt.Errorf("name must start with %s", existingPlanOperationTestPrefix)
	}
	if len([]rune(strings.TrimSpace(name))) > 50 {
		return nil, errors.New("name cannot contain more than 50 characters")
	}
	budget, ok := floatArg(args, "budget")
	if !ok || budget <= 0 {
		return nil, errors.New("budget must be greater than zero")
	}
	externalAction := strings.ToUpper(strings.TrimSpace(stringArg(
		args,
		"external_action",
		"AD_CONVERT_TYPE_LIVE_SUCCESSORDER_PAY",
	)))
	if externalAction != "AD_CONVERT_TYPE_LIVE_SUCCESSORDER_PAY" &&
		externalAction != "AD_CONVERT_TYPE_LIVE_HEAT" {
		return nil, errors.New("external_action must be AD_CONVERT_TYPE_LIVE_SUCCESSORDER_PAY or AD_CONVERT_TYPE_LIVE_HEAT")
	}
	boostBudget, ok := floatArg(args, "boost_budget")
	if !ok || boostBudget <= 0 || boostBudget > budget {
		return nil, errors.New("boost_budget must be greater than zero and cannot exceed budget")
	}
	addVideoIDs, err := optionalCreativeIDListArg(args, "add_video_ids")
	if err != nil {
		return nil, err
	}
	addImageIDs, err := optionalCreativeIDListArg(args, "add_image_ids")
	if err != nil {
		return nil, err
	}
	if len(addVideoIDs) == 0 && len(addImageIDs) == 0 {
		return nil, errors.New("at least one of add_video_ids or add_image_ids is required")
	}
	if len(addVideoIDs) > 0 && len(addImageIDs) > 0 {
		return nil, errors.New("add_video_ids and add_image_ids cannot be combined in one test")
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}

	before, err := t.readExistingPlanSnapshot(ctx, token, advertiserID, adID)
	if err != nil {
		return nil, err
	}
	if err := validateExistingPlanOperationTarget(before.Plan); err != nil {
		return nil, err
	}
	if len(addVideoIDs) > 0 && before.MaterialType != "VIDEO" {
		return nil, fmt.Errorf("plan creative material_type=%q; video add test requires VIDEO", before.MaterialType)
	}
	if len(addImageIDs) > 0 && before.MaterialType != "IMAGE" {
		return nil, fmt.Errorf("plan creative material_type=%q; image add test requires IMAGE", before.MaterialType)
	}
	if len(addImageIDs) > 0 && (before.ProductID == 0 || before.AwemeUID == 0) {
		return nil, errors.New("image add test requires product_id and aweme_uid from plan detail")
	}
	originalBudget := before.Plan.Budget
	if originalBudget <= 0 {
		return nil, errors.New("existing plan budget must be positive")
	}
	if originalBudget == budget {
		return nil, errors.New("test budget must differ from the current plan budget")
	}

	result := map[string]any{
		"account":               t.account(account),
		"ad_id":                 formatUintID(adID),
		"name":                  name,
		"material_type":         before.MaterialType,
		"original_budget":       originalBudget,
		"original_status":       before.Plan.Status,
		"original_opt_status":   before.Plan.OptStatus,
		"original_material_ids": before.MaterialIDs,
		"requested_budget":      budget,
		"boost_budget":          boostBudget,
		"external_action":       externalAction,
		"steps":                 []map[string]any{},
	}
	var boostTaskID uint64
	var addedIDs []string
	budgetChanged := false
	restoreBudget := func() error {
		if !budgetChanged {
			return nil
		}
		return t.updateExistingPlanBudget(
			ctx,
			token,
			advertiserID,
			adID,
			originalBudget,
		)
	}
	rollback := func(primaryErr error) (any, error) {
		if boostTaskID != 0 {
			stopExistingPlanBoost(ctx, t, token, advertiserID, boostTaskID)
		}
		if len(addedIDs) > 0 {
			_ = t.deleteExistingPlanMaterials(
				ctx,
				token,
				advertiserID,
				adID,
				before.MaterialType,
				addedIDs,
			)
		}
		restoreErr := restoreBudget()
		evidence := map[string]any{
			"rolled_back_materials": addedIDs,
			"budget_restore_error":  nil,
		}
		if restoreErr != nil {
			evidence["budget_restore_error"] = restoreErr.Error()
		}
		result["rollback"] = evidence
		result["error"] = primaryErr.Error()
		return result, primaryErr
	}

	if err := t.updateExistingPlanBudget(ctx, token, advertiserID, adID, budget); err != nil {
		return rollback(fmt.Errorf("budget update failed: %w", err))
	}
	budgetChanged = true
	afterBudget, err := t.readExistingPlanSnapshot(ctx, token, advertiserID, adID)
	if err != nil {
		return rollback(fmt.Errorf("read plan after budget update failed: %w", err))
	}
	if !almostEqual(afterBudget.Plan.Budget, budget, 0.000001) {
		return rollback(fmt.Errorf(
			"budget readback mismatch: expected %.2f, got %.2f",
			budget,
			afterBudget.Plan.Budget,
		))
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "update_budget",
		"expected": budget,
		"actual":   afterBudget.Plan.Budget,
		"verified": true,
	})

	var addResponse *sdk.UniPromotionMaterialAddRes
	if len(addVideoIDs) > 0 {
		addResponse, err = t.service.Client.UniPromotionMaterialAdd(
			ctx,
			sdk.UniPromotionMaterialAddReq{
				AccessToken:   token,
				AdvertiserId:  uint64(advertiserID),
				AdId:          adID,
				VideoMaterial: videoMaterialPayload(addVideoIDs),
			},
		)
	} else {
		addResponse, err = t.service.Client.UniPromotionMaterialAdd(
			ctx,
			sdk.UniPromotionMaterialAddReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AdId:         adID,
				ProductCreativeList: []sdk.UniPromotionProductCreativeMaterialAdd{
					{
						ProductID: before.ProductID,
						AwemeUID:  before.AwemeUID,
						ImageMaterial: []sdk.UniPromotionImageMaterial{
							{ImageMode: "SQUARE", ImageIds: addImageIDs},
						},
					},
				},
			},
		)
	}
	if err != nil {
		return rollback(fmt.Errorf("add materials failed: %w", err))
	}
	if addResponse == nil || addResponse.Code != 0 {
		return rollback(apiError(
			"add materials failed",
			responseCode(addResponse),
			responseMessage(addResponse),
		))
	}
	addedIDs, err = t.verifyExistingPlanAddedMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		before.MaterialType,
		before.MaterialIDs,
		addVideoIDs,
		addImageIDs,
	)
	if err != nil {
		return rollback(err)
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "add_materials",
		"ids":      addedIDs,
		"verified": true,
	})

	if err := t.deleteExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		before.MaterialType,
		addedIDs,
	); err != nil {
		return rollback(fmt.Errorf("delete test-added materials failed: %w", err))
	}
	afterDelete, err := t.readExistingPlanSnapshot(ctx, token, advertiserID, adID)
	if err != nil {
		return rollback(fmt.Errorf("read plan after material delete failed: %w", err))
	}
	if containsAny(afterDelete.MaterialIDs, addedIDs) {
		return rollback(fmt.Errorf("material delete readback still contains test IDs %v", addedIDs))
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "delete_test_added_materials",
		"ids":      addedIDs,
		"verified": true,
	})
	addedIDs = nil

	createResponse, err := t.service.Client.UniPromotionControlTaskCreate(
		ctx,
		sdk.UniPromotionControlTaskCreateReq{
			AccessToken:    token,
			AdvertiserId:   uint64(advertiserID),
			AdId:           adID,
			Name:           name,
			Scene:          "SMART_BOOST",
			Budget:         boostBudget,
			Duration:       0.5,
			ExternalAction: externalAction,
		},
	)
	if err != nil {
		return rollback(fmt.Errorf("create one-click boost failed: %w", err))
	}
	if createResponse == nil || createResponse.Code != 0 || createResponse.Data.Id == 0 {
		return rollback(apiError(
			"create one-click boost failed",
			responseCode(createResponse),
			responseMessage(createResponse),
		))
	}
	boostTaskID = createResponse.Data.Id
	boostTask, err := t.readExistingPlanBoostTask(
		ctx,
		token,
		advertiserID,
		adID,
		boostTaskID,
		before.TaskStartTime,
		before.TaskEndTime,
	)
	if err != nil {
		return rollback(err)
	}
	if err := stopExistingPlanBoost(ctx, t, token, advertiserID, boostTaskID); err != nil {
		return rollback(fmt.Errorf("stop one-click boost after creation failed: %w", err))
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "create_and_stop_one_click_boost",
		"task_id":  formatUintID(boostTaskID),
		"verified": true,
		"task":     boostTask,
	})
	boostTaskID = 0

	if err := restoreBudget(); err != nil {
		return result, fmt.Errorf("restore original budget failed: %w", err)
	}
	budgetChanged = false
	if err := t.verifyExistingPlanMaterialRestoration(
		ctx,
		token,
		advertiserID,
		adID,
		before.MaterialType,
		before.MaterialIDs,
	); err != nil {
		return result, fmt.Errorf("verify material restoration failed: %w", err)
	}
	restored, err := t.readExistingPlanSnapshot(ctx, token, advertiserID, adID)
	if err != nil {
		return result, fmt.Errorf("read plan after restoration failed: %w", err)
	}
	if !almostEqual(restored.Plan.Budget, originalBudget, 0.000001) {
		return result, fmt.Errorf(
			"restored budget mismatch: expected %.2f, got %.2f",
			originalBudget,
			restored.Plan.Budget,
		)
	}
	if !sameStrings(restored.MaterialIDs, before.MaterialIDs) {
		return result, fmt.Errorf("restored material set does not match original")
	}
	result["restored"] = map[string]any{
		"budget":       restored.Plan.Budget,
		"material_ids": restored.MaterialIDs,
		"status":       restored.Plan.Status,
		"opt_status":   restored.Plan.OptStatus,
	}
	result["verified"] = true
	return result, nil
}

func (t *Tools) readExistingPlanSnapshot(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
) (*existingPlanSnapshot, error) {
	response, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("read existing plan detail failed: %w", err)
	}
	if response.Code != 0 || response.Data == nil {
		return nil, apiError(
			"read existing plan detail failed",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	materialType, err := existingPlanCreativeMaterialType(response.Data)
	if err != nil {
		return nil, err
	}
	materials, err := t.readExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		materialType,
	)
	if err != nil {
		return nil, err
	}
	materialIDs := make([]string, 0, len(materials))
	videoIDs := make([]string, 0, len(materials))
	imageIDs := make([]string, 0, len(materials))
	for _, material := range materials {
		if material.MaterialId > 0 {
			materialIDs = append(materialIDs, formatUintID(material.MaterialId))
		}
		if material.VideoId != "" {
			videoIDs = append(videoIDs, material.VideoId)
		}
		if material.ImageId != "" {
			imageIDs = append(imageIDs, material.ImageId)
		}
	}
	productID, awemeUID := existingPlanCreativeIdentity(response.Data)
	startTime, endTime := businessDayRange(time.Now().In(shanghaiLocation))
	return &existingPlanSnapshot{
		Plan:          response.Data,
		MaterialType:  materialType,
		MaterialIDs:   uniqueStrings(materialIDs),
		VideoIDs:      uniqueStrings(videoIDs),
		ImageIDs:      uniqueStrings(imageIDs),
		ProductID:     productID,
		AwemeUID:      awemeUID,
		TaskStartTime: startTime,
		TaskEndTime:   endTime,
	}, nil
}

func existingPlanCreativeMaterialType(plan *sdk.UniPromotionDetail) (string, error) {
	if plan == nil {
		return "", errors.New("plan detail is empty")
	}
	if len(plan.MultiProductCreativeList) > 0 {
		creative := plan.MultiProductCreativeList[0]
		switch {
		case len(creative.ImageMaterial) > 0:
			return "IMAGE", nil
		case len(creative.VideoMaterial) > 0:
			return "VIDEO", nil
		case len(creative.CarouselMaterial) > 0:
			return "CAROUSEL", nil
		case len(creative.TitleMaterial) > 0:
			return "TITLE", nil
		}
	}
	if plan.ProgrammaticCreativeMediaList != nil {
		switch {
		case len(plan.ProgrammaticCreativeMediaList.VideoMaterial) > 0:
			return "VIDEO", nil
		case len(plan.ProgrammaticCreativeMediaList.TitleMaterial) > 0:
			return "TITLE", nil
		}
	}
	return "", errors.New("cannot determine creative material type; plan must contain an existing creative material list")
}

func validateExistingPlanOperationTarget(plan *sdk.UniPromotionDetail) error {
	if plan == nil {
		return errors.New("plan detail is empty")
	}
	if plan.MarketingGoal != "VIDEO_PROM_GOODS" {
		return fmt.Errorf("plan marketing_goal=%q, required VIDEO_PROM_GOODS", plan.MarketingGoal)
	}
	if !strings.EqualFold(plan.Status, "DISABLE") || !strings.EqualFold(plan.OptStatus, "DISABLE") {
		return fmt.Errorf(
			"refusing to test non-paused plan: status=%q opt_status=%q",
			plan.Status,
			plan.OptStatus,
		)
	}
	return nil
}

func existingPlanCreativeIdentity(plan *sdk.UniPromotionDetail) (uint64, uint64) {
	if plan == nil {
		return 0, 0
	}
	var productID uint64
	if len(plan.ProductInfos) > 0 {
		productID = plan.ProductInfos[0].ProductID
	}
	if productID == 0 && len(plan.MultiProductCreativeList) > 0 {
		productID = plan.MultiProductCreativeList[0].ProductID
	}
	awemeUID := plan.AwemeId
	if awemeUID == 0 && len(plan.AwemeInfos) > 0 {
		awemeUID = plan.AwemeInfos[0].AwemeUID
	}
	if awemeUID == 0 && len(plan.MultiProductCreativeList) > 0 {
		awemeUID = plan.MultiProductCreativeList[0].AwemeUID
	}
	return productID, awemeUID
}

func (t *Tools) readExistingPlanMaterials(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	materialType string,
) ([]sdk.UniPromotionMaterialInfo, error) {
	var materials []sdk.UniPromotionMaterialInfo
	for page := 1; page <= 100; page++ {
		response, err := t.service.Client.UniPromotionMaterialGet(
			ctx,
			sdk.UniPromotionMaterialGetReq{
				AccessToken:        token,
				AdvertiserId:       uint64(advertiserID),
				AdId:               adID,
				MaterialType:       materialType,
				Status:             "ALL",
				MaterialSelectType: "ALL",
				Page:               page,
				PageSize:           100,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("read existing plan materials page %d failed: %w", page, err)
		}
		if response.Code != 0 {
			return nil, apiError(
				"read existing plan materials failed",
				response.Code,
				response.Message,
			)
		}
		materials = append(materials, response.Data.Materials...)
		if len(response.Data.Materials) < 100 ||
			response.Data.PageInfo.TotalPage <= page {
			break
		}
	}
	return materials, nil
}

func (t *Tools) verifyExistingPlanAddedMaterials(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	materialType string,
	beforeIDs []string,
	videoIDs []string,
	imageIDs []string,
) ([]string, error) {
	var after []sdk.UniPromotionMaterialInfo
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		after, err = t.readExistingPlanMaterials(
			ctx,
			token,
			advertiserID,
			adID,
			materialType,
		)
		if err == nil && len(after) > len(beforeIDs) {
			break
		}
		if attempt < 9 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if err != nil {
		return nil, err
	}
	beforeSet := make(map[string]bool, len(beforeIDs))
	for _, id := range beforeIDs {
		beforeSet[id] = true
	}
	var added []string
	afterVideos := make(map[string]bool)
	afterImages := make(map[string]bool)
	for _, material := range after {
		id := formatUintID(material.MaterialId)
		if id != "" && !beforeSet[id] {
			added = append(added, id)
		}
		if material.VideoId != "" {
			afterVideos[material.VideoId] = true
		}
		if material.ImageId != "" {
			afterImages[material.ImageId] = true
		}
	}
	if len(added) == 0 {
		return nil, errors.New("official material list did not show any newly attached material")
	}
	for _, id := range videoIDs {
		if !afterVideos[id] {
			return nil, fmt.Errorf("new video %s not found in official material readback", id)
		}
	}
	for _, id := range imageIDs {
		if !afterImages[id] {
			return nil, fmt.Errorf("new image %s not found in official material readback", id)
		}
	}
	return added, nil
}

func (t *Tools) verifyExistingPlanMaterialRestoration(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	materialType string,
	expectedIDs []string,
) error {
	after, err := t.readExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		materialType,
	)
	if err != nil {
		return err
	}
	afterIDs := make([]string, 0, len(after))
	for _, material := range after {
		afterIDs = append(afterIDs, formatUintID(material.MaterialId))
	}
	if !sameStrings(uniqueStrings(afterIDs), uniqueStrings(expectedIDs)) {
		return fmt.Errorf(
			"material set not restored: expected %v, got %v",
			expectedIDs,
			afterIDs,
		)
	}
	return nil
}

func (t *Tools) updateExistingPlanBudget(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	budget float64,
) error {
	response, err := t.service.Client.UniPromotionBudgetUpdate(
		ctx,
		sdk.UniPromotionBudgetUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			Data: []sdk.UniPromotionBudgetUpdateItem{{
				AdId:   adID,
				Budget: budget,
			}},
		},
	)
	if err != nil {
		return err
	}
	if response == nil || response.Code != 0 {
		return apiError(
			"update plan budget failed",
			responseCode(response),
			responseMessage(response),
		)
	}
	return nil
}

func (t *Tools) deleteExistingPlanMaterials(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	materialType string,
	materialIDs []string,
) error {
	response, err := t.service.Client.UniPromotionMaterialDelete(
		ctx,
		sdk.UniPromotionMaterialDeleteReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
			MaterialType: materialType,
			MaterialIds:  mustNumericIDs(materialIDs),
		},
	)
	if err != nil {
		return err
	}
	if response == nil || response.Code != 0 {
		return apiError(
			"delete plan materials failed",
			responseCode(response),
			responseMessage(response),
		)
	}
	return nil
}

func (t *Tools) readExistingPlanBoostTask(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	taskID uint64,
	startTime string,
	endTime string,
) (*sdk.UniPromotionControlTask, error) {
	for attempt := 0; attempt < 10; attempt++ {
		response, err := t.service.Client.UniPromotionControlTaskList(
			ctx,
			sdk.UniPromotionControlTaskListReq{
				AccessToken:   token,
				AdvertiserId:  uint64(advertiserID),
				MarketingGoal: "VIDEO_PROM_GOODS",
				AdId:          adID,
				StartTime:     startTime,
				EndTime:       endTime,
				Scene:         "SMART_BOOST",
				Filtering: &sdk.UniPromotionControlTaskListFiltering{
					SearchKeyword: formatUintID(taskID),
				},
				Page:     1,
				PageSize: 100,
			},
		)
		if err == nil && response != nil && response.Code == 0 {
			for index := range response.Data.TaskList {
				task := &response.Data.TaskList[index]
				if task.Id == taskID {
					return task, nil
				}
			}
		}
		if attempt < 9 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil, fmt.Errorf("one-click boost task %s did not appear in official list readback", formatUintID(taskID))
}

func stopExistingPlanBoost(
	ctx context.Context,
	t *Tools,
	token string,
	advertiserID int64,
	taskID uint64,
) error {
	response, err := t.service.Client.UniPromotionControlTaskStatusUpdate(
		ctx,
		sdk.UniPromotionControlTaskStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			TaskIds:      []uint64{taskID},
			OptType:      "DELETE",
		},
	)
	if err != nil {
		return err
	}
	if !controlTaskStatusUpdateSucceeded(response, taskID) {
		return apiError(
			"delete one-click boost failed",
			responseCode(response),
			responseMessage(response),
		)
	}
	return nil
}

func containsAny(values []string, candidates []string) bool {
	for _, candidate := range candidates {
		for _, value := range values {
			if value == candidate {
				return true
			}
		}
	}
	return false
}

func optionalCreativeIDListArg(args map[string]any, key string) ([]string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		if typed, typedOK := value.([]string); typedOK {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			return nil, fmt.Errorf("%s must be an array", key)
		}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text == "" {
			return nil, fmt.Errorf("%s cannot contain blank values", key)
		}
		result = append(result, text)
	}
	return uniqueStrings(result), nil
}

func videoMaterialPayload(videoIDs []string) []sdk.UniPromotionMaterial {
	result := make([]sdk.UniPromotionMaterial, 0, len(videoIDs))
	for _, id := range videoIDs {
		result = append(result, sdk.UniPromotionMaterial{VideoId: id})
	}
	return result
}

func responseCode(value any) int64 {
	switch response := value.(type) {
	case *sdk.UniPromotionMaterialAddRes:
		if response != nil {
			return response.Code
		}
	case *sdk.UniPromotionControlTaskCreateRes:
		if response != nil {
			return response.Code
		}
	}
	return 0
}

func responseMessage(value any) string {
	switch response := value.(type) {
	case *sdk.UniPromotionMaterialAddRes:
		if response != nil {
			return response.Message
		}
	case *sdk.UniPromotionControlTaskCreateRes:
		if response != nil {
			return response.Message
		}
	}
	return ""
}
