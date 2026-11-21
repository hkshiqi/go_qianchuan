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
	Plan                   *sdk.UniPromotionDetail
	MaterialType           string
	MaterialIDs            []string
	MaterialImageIDMap     map[string]string
	MaterialTitleMap       map[string]string
	DeleteImageMaterialIDs []string
	DeleteTitleMaterialIDs []string
	DeleteTitleTexts       []string
	ImageIDs               []string
	TitleMaterialIDs       []string
	TitleTexts             []string
	ActiveImageIDs         []string
	ActiveTitleIDs         []string
	ActiveTitleTexts       []string
	ProductID              uint64
	AwemeUID               uint64
	TaskStartTime          string
	TaskEndTime            string
}

type existingPlanAddedMaterials struct {
	ImageMaterialIDs         []string
	TitleMaterialIDs         []string
	AddedImageIDs            []string
	AddedTitleTexts          []string
	ExcludedImageMaterialIDs []string
	ExcludedTitleMaterialIDs []string
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
	boostBudget, ok := floatArg(args, "boost_budget")
	if !ok || boostBudget <= 0 || boostBudget > budget {
		return nil, errors.New("boost_budget must be greater than zero and cannot exceed budget")
	}
	addImageIDs, err := optionalCreativeIDListArg(args, "add_image_ids")
	if err != nil {
		return nil, err
	}
	if len(addImageIDs) == 0 {
		return nil, errors.New("add_image_ids must contain at least one existing square image")
	}
	if len(addImageIDs) > 20 {
		return nil, errors.New("add_image_ids cannot contain more than 20 IDs")
	}
	addTitles, err := optionalCreativeTitleListArg(args, "add_titles")
	if err != nil {
		return nil, err
	}
	if len(addTitles) == 0 {
		return nil, errors.New("add_titles must contain at least one existing product-card title")
	}
	if len(addTitles) > 20 {
		return nil, errors.New("add_titles cannot contain more than 20 titles")
	}
	cleanupOnly, err := cleanupOnlyArg(args)
	if err != nil {
		return nil, err
	}
	account, err := t.resolveAccount(args)
	if err != nil {
		return nil, err
	}
	token, advertiserID, err := t.tokenFor(ctx, account)
	if err != nil {
		return nil, err
	}

	before, err := t.readExistingPlanSnapshot(ctx, token, advertiserID, adID, "IMAGE")
	if err != nil {
		return nil, err
	}
	if err := validateExistingPlanOperationTarget(before.Plan); err != nil {
		return nil, err
	}
	if before.MaterialType != "IMAGE" {
		return nil, fmt.Errorf("plan creative material_type=%q; image add test requires IMAGE", before.MaterialType)
	}
	for _, imageID := range addImageIDs {
		if containsString(before.ImageIDs, imageID) {
			if cleanupOnly {
				continue
			}
			return nil, fmt.Errorf("test image %s is already attached to the plan", imageID)
		}
	}
	for _, title := range addTitles {
		if containsString(before.TitleTexts, title) {
			if cleanupOnly {
				continue
			}
			return nil, fmt.Errorf("test title %q is already attached to the plan", title)
		}
	}
	if before.ProductID == 0 || before.AwemeUID == 0 {
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
		"account":                     t.account(account),
		"ad_id":                       formatUintID(adID),
		"name":                        name,
		"material_type":               before.MaterialType,
		"original_budget":             originalBudget,
		"original_status":             before.Plan.Status,
		"original_opt_status":         before.Plan.OptStatus,
		"original_material_ids":       before.MaterialIDs,
		"original_title_material_ids": before.TitleMaterialIDs,
		"requested_budget":            budget,
		"boost_budget":                boostBudget,
		"add_image_ids":               addImageIDs,
		"add_titles":                  addTitles,
		"cleanup_only":                cleanupOnly,
		"steps":                       []map[string]any{},
	}
	var boostTaskID uint64
	var addedImageIDs []string
	var addedTitleIDs []string
	budgetChanged := false
	planResumed := false
	materialsAdded := false
	restorePlanStatus := func() error {
		if !planResumed {
			return nil
		}
		response, err := t.service.Client.UniPromotionStatusUpdate(
			ctx,
			sdk.UniPromotionStatusUpdateReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AdIds:        []uint64{adID},
				OptStatus:    before.Plan.OptStatus,
			},
		)
		if err != nil {
			return err
		}
		if !uniPromotionStatusUpdateSucceeded(response, adID) {
			return apiError(
				"restore test plan status failed",
				responseCode(response),
				responseMessage(response),
				responseRequestID(response),
			)
		}
		planResumed = false
		return nil
	}
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
		var boostStopErr error
		if boostTaskID != 0 {
			boostStopErr = stopExistingPlanBoost(ctx, t, token, advertiserID, boostTaskID)
		}
		statusErr := restorePlanStatus()
		if cleanupOnly {
			return waitExistingPlanCleanup(
				ctx,
				t,
				token,
				advertiserID,
				adID,
				before,
				addImageIDs,
				addTitles,
				budget,
				result,
				primaryErr,
			)
		}
		imageMaterialIDs := before.DeleteImageMaterialIDs
		titleMaterialIDs := before.DeleteTitleMaterialIDs
		deleteTitles := before.DeleteTitleTexts
		if materialsAdded &&
			len(imageMaterialIDs) == 0 && len(titleMaterialIDs) == 0 {
			discoveredImages, discoveredTitles, discoverErr :=
				t.discoverExistingPlanAddedMaterialIDs(
					ctx,
					token,
					advertiserID,
					adID,
					before,
				)
			if discoverErr == nil {
				imageMaterialIDs = discoveredImages
				titleMaterialIDs = discoveredTitles
				deleteTitles = addTitles
			}
		}
		if materialsAdded &&
			(len(imageMaterialIDs) > 0 || len(titleMaterialIDs) > 0) {
			_ = t.deleteExistingPlanMaterials(
				ctx,
				token,
				advertiserID,
				adID,
				imageMaterialIDs,
				titleMaterialIDs,
				deleteTitles,
			)
		}
		restoreErr := restoreBudget()
		evidence := map[string]any{
			"rolled_back_image_material_ids": addedImageIDs,
			"rolled_back_title_material_ids": addedTitleIDs,
			"rolled_back_titles":             addTitles,
			"boost_stop_error":               nil,
			"status_restore_error":           nil,
			"budget_restore_error":           nil,
		}
		if boostStopErr != nil {
			evidence["boost_stop_error"] = boostStopErr.Error()
		}
		if statusErr != nil {
			evidence["status_restore_error"] = statusErr.Error()
		}
		if restoreErr != nil {
			evidence["budget_restore_error"] = restoreErr.Error()
		}
		result["rollback"] = evidence
		result["error"] = primaryErr.Error()
		if statusErr != nil || boostStopErr != nil {
			return result, fmt.Errorf(
				"%w; rollback status=%v boost_stop=%v",
				primaryErr,
				statusErr,
				boostStopErr,
			)
		}
		return result, primaryErr
	}

	if cleanupOnly {
		if err := t.deleteExistingPlanMaterials(
			ctx,
			token,
			advertiserID,
			adID,
			before.ActiveImageIDs,
			before.ActiveTitleIDs,
			before.ActiveTitleTexts,
		); err != nil {
			return rollback(fmt.Errorf("cleanup existing plan materials failed: %w", err))
		}
		cleaned, err := t.waitExistingPlanMaterialsCleared(
			ctx,
			token,
			advertiserID,
			adID,
		)
		if err != nil {
			return rollback(err)
		}
		result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
			"action":   "cleanup_existing_plan_materials",
			"verified": true,
			"state":    cleaned,
		})
		result["restored"] = cleaned
		result["verified"] = true
		return result, nil
	}

	if err := t.updateExistingPlanBudget(ctx, token, advertiserID, adID, budget); err != nil {
		return rollback(fmt.Errorf("budget update failed: %w", err))
	}
	budgetChanged = true
	afterBudget, err := t.waitExistingPlanBudget(
		ctx,
		token,
		advertiserID,
		adID,
		budget,
	)
	if err != nil {
		return rollback(fmt.Errorf("verify plan budget update failed: %w", err))
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
	beforeAdd, err := t.readExistingPlanSnapshot(
		ctx,
		token,
		advertiserID,
		adID,
		before.MaterialType,
	)
	if err != nil {
		return rollback(fmt.Errorf("read material state before add failed: %w", err))
	}
	before.MaterialIDs = beforeAdd.MaterialIDs
	before.MaterialImageIDMap = beforeAdd.MaterialImageIDMap
	before.MaterialTitleMap = beforeAdd.MaterialTitleMap
	before.ImageIDs = beforeAdd.ImageIDs
	before.TitleMaterialIDs = beforeAdd.TitleMaterialIDs
	before.TitleTexts = beforeAdd.TitleTexts
	before.ActiveImageIDs = beforeAdd.ActiveImageIDs
	before.ActiveTitleIDs = beforeAdd.ActiveTitleIDs
	before.ActiveTitleTexts = beforeAdd.ActiveTitleTexts
	result["original_material_ids"] = before.MaterialIDs
	result["original_title_material_ids"] = before.TitleMaterialIDs

	addResponse, err := t.service.Client.UniPromotionMaterialAdd(
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
					TitleMaterial: productCardTitleMaterials(addTitles),
				},
			},
		},
	)
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
	materialsAdded = true
	added, err := t.verifyExistingPlanAddedMaterialsFromDetail(
		ctx,
		token,
		advertiserID,
		adID,
		before,
		addImageIDs,
		addTitles,
	)
	addedImageIDs = added.ImageMaterialIDs
	addedTitleIDs = added.TitleMaterialIDs
	if err != nil {
		return rollback(err)
	}
	addedIDs := uniqueStrings(append(
		append([]string(nil), added.ImageMaterialIDs...),
		added.TitleMaterialIDs...,
	))
	deleteTitles := uniqueStrings(addTitles)
	beforeDelete, err := t.readExistingPlanSnapshot(
		ctx,
		token,
		advertiserID,
		adID,
		before.MaterialType,
	)
	if err != nil {
		return rollback(fmt.Errorf("read material state before delete failed: %w", err))
	}
	deleteImageMaterialIDs := existingPlanAddedImageMaterialIDs(
		beforeDelete,
		before,
	)
	deleteTitleMaterialIDs := existingPlanTitleMaterialIDs(
		beforeDelete,
		addTitles,
	)
	before.DeleteImageMaterialIDs = deleteImageMaterialIDs
	before.DeleteTitleMaterialIDs = deleteTitleMaterialIDs
	before.DeleteTitleTexts = deleteTitles
	if len(deleteImageMaterialIDs) == 0 ||
		len(deleteTitleMaterialIDs) == 0 {
		return rollback(fmt.Errorf(
			"platform list did not resolve all newly added material IDs: image_ids=%v title_material_ids=%v",
			deleteImageMaterialIDs,
			deleteTitleMaterialIDs,
		))
	}
	result["delete_preflight"] = map[string]any{
		"detail_image_ids":          beforeDelete.ImageIDs,
		"detail_title_texts":        beforeDelete.TitleTexts,
		"detail_image_material_ids": beforeDelete.MaterialIDs,
		"detail_title_material_ids": deleteTitleMaterialIDs,
		"delete_image_material_ids": deleteImageMaterialIDs,
		"delete_title_material_ids": deleteTitleMaterialIDs,
		"delete_titles":             deleteTitles,
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":             "add_materials",
		"image_material_ids": addedImageIDs,
		"title_material_ids": addedTitleIDs,
		"added_ids":          addedIDs,
		"verified":           true,
	})

	if err := t.deleteExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		deleteImageMaterialIDs,
		deleteTitleMaterialIDs,
		deleteTitles,
	); err != nil {
		return rollback(fmt.Errorf("delete test-added materials failed: %w", err))
	}
	afterDelete, err := t.waitExistingPlanCreativeBindings(
		ctx,
		token,
		advertiserID,
		adID,
		before,
		addedIDs,
		deleteTitles,
	)
	if err != nil {
		return rollback(fmt.Errorf("read plan after material delete failed: %w", err))
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":              "delete_test_added_materials",
		"image_material_ids":  addedImageIDs,
		"title_material_ids":  addedTitleIDs,
		"added_ids":           addedIDs,
		"titles":              addTitles,
		"verified":            true,
		"detail_material_ids": afterDelete.MaterialIDs,
		"detail_image_ids":    afterDelete.ImageIDs,
		"detail_title_texts":  afterDelete.TitleTexts,
	})
	addedImageIDs = nil
	addedTitleIDs = nil

	enableResponse, err := t.service.Client.UniPromotionStatusUpdate(
		ctx,
		sdk.UniPromotionStatusUpdateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdIds:        []uint64{adID},
			OptStatus:    "ENABLE",
		},
	)
	if err != nil {
		return rollback(fmt.Errorf("enable isolated test plan for one-click boost failed: %w", err))
	}
	if !uniPromotionStatusUpdateSucceeded(enableResponse, adID) {
		return rollback(apiError(
			"enable isolated test plan for one-click boost failed",
			responseCode(enableResponse),
			responseMessage(enableResponse),
			responseRequestID(enableResponse),
		))
	}
	planResumed = true
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "enable_isolated_test_plan",
		"verified": true,
		"status":   "ENABLE",
	})

	createResponse, err := t.service.Client.UniPromotionControlTaskCreate(
		ctx,
		sdk.UniPromotionControlTaskCreateReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
			Name:         name,
			Scene:        "SMART_BOOST",
			Budget:       boostBudget,
			Duration:     0.5,
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

	if err := restorePlanStatus(); err != nil {
		return result, fmt.Errorf("restore original plan status failed: %w", err)
	}
	result["steps"] = append(result["steps"].([]map[string]any), map[string]any{
		"action":   "restore_plan_status",
		"expected": before.Plan.OptStatus,
		"verified": true,
	})

	if err := restoreBudget(); err != nil {
		return result, fmt.Errorf("restore original budget failed: %w", err)
	}
	budgetChanged = false
	restored, err := t.waitExistingPlanRestored(
		ctx,
		token,
		advertiserID,
		adID,
		originalBudget,
		before.Plan.Status,
		before.Plan.OptStatus,
		before.MaterialIDs,
		before.TitleMaterialIDs,
		before.ImageIDs,
		before.TitleTexts,
	)
	if err != nil {
		return result, fmt.Errorf("verify plan restoration failed: %w", err)
	}
	result["restored"] = map[string]any{
		"budget":             restored.Plan.Budget,
		"material_ids":       restored.MaterialIDs,
		"title_material_ids": restored.TitleMaterialIDs,
		"title_texts":        restored.TitleTexts,
		"status":             restored.Plan.Status,
		"opt_status":         restored.Plan.OptStatus,
	}
	result["verified"] = true
	return result, nil
}

func (t *Tools) readExistingPlanSnapshot(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	fallbackMaterialType string,
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
		if fallbackMaterialType == "" ||
			!isolatedOverallTestPlan(response.Data) {
			return nil, err
		}
		materialType = fallbackMaterialType
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
	titleMaterials, err := t.readExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		"TITLE",
	)
	if err != nil {
		return nil, err
	}
	productID, awemeUID := existingPlanCreativeIdentity(response.Data)
	startTime, endTime := businessDayRange(time.Now().In(shanghaiLocation))
	return &existingPlanSnapshot{
		Plan:               response.Data,
		MaterialType:       materialType,
		MaterialIDs:        existingPlanMaterialIDs(materials),
		MaterialImageIDMap: existingPlanMaterialImageIDMap(materials),
		MaterialTitleMap:   existingPlanMaterialTitleMap(titleMaterials),
		ImageIDs:           existingPlanImageIDs(materials),
		TitleMaterialIDs:   existingPlanMaterialIDs(titleMaterials),
		TitleTexts:         existingPlanTitleTexts(titleMaterials),
		ActiveImageIDs:     existingPlanActiveMaterialIDs(materials),
		ActiveTitleIDs:     existingPlanActiveMaterialIDs(titleMaterials),
		ActiveTitleTexts:   existingPlanTitleTexts(titleMaterials),
		ProductID:          productID,
		AwemeUID:           awemeUID,
		TaskStartTime:      startTime,
		TaskEndTime:        endTime,
	}, nil
}

func (t *Tools) readExistingPlanDetail(
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
	return &existingPlanSnapshot{
		Plan: response.Data,
	}, nil
}

func (t *Tools) waitExistingPlanBudget(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	expectedBudget float64,
) (*existingPlanSnapshot, error) {
	var last *existingPlanSnapshot
	for attempt := 0; attempt < 15; attempt++ {
		snapshot, err := t.readExistingPlanDetail(ctx, token, advertiserID, adID)
		if err == nil && snapshot != nil {
			last = snapshot
			if almostEqual(snapshot.Plan.Budget, expectedBudget, 0.000001) {
				return snapshot, nil
			}
		}
		if attempt < 14 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if last == nil {
		return nil, errors.New("plan detail did not become readable after budget update")
	}
	return nil, fmt.Errorf(
		"budget readback mismatch: expected %.2f, got %.2f",
		expectedBudget,
		last.Plan.Budget,
	)
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

func isolatedOverallTestPlan(plan *sdk.UniPromotionDetail) bool {
	return plan != nil &&
		plan.MarketingGoal == "VIDEO_PROM_GOODS" &&
		plan.AdlabScene == sdk.AdlabSceneOverall &&
		validOverallPlanCRUDTestName(plan.Name)
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

func (t *Tools) verifyExistingPlanAddedMaterialsFromDetail(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	before *existingPlanSnapshot,
	imageIDs []string,
	titles []string,
) (existingPlanAddedMaterials, error) {
	var last *existingPlanSnapshot
	for attempt := 0; attempt < 15; attempt++ {
		snapshot, err := t.readExistingPlanDetail(ctx, token, advertiserID, adID)
		if err == nil && snapshot != nil {
			last = snapshot
			afterImageIDs, afterTitleTexts := existingPlanCreativeBindings(snapshot.Plan)
			if containsAllStrings(afterImageIDs, imageIDs) &&
				containsAllStrings(afterTitleTexts, titles) {
				return existingPlanAddedMaterials{
					ImageMaterialIDs: uniqueStrings(append(
						append([]string(nil), before.MaterialIDs...),
						afterImageIDs...,
					)),
					TitleMaterialIDs: uniqueStrings(append(
						append([]string(nil), before.TitleMaterialIDs...),
						afterTitleTexts...,
					)),
					AddedImageIDs:   uniqueStrings(append([]string(nil), imageIDs...)),
					AddedTitleTexts: uniqueStrings(append([]string(nil), titles...)),
				}, nil
			}
		}
		if attempt < 14 {
			select {
			case <-ctx.Done():
				return existingPlanAddedMaterials{}, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if last == nil {
		return existingPlanAddedMaterials{}, errors.New("plan detail did not become readable after material add")
	}
	imageBindings, titleTexts := existingPlanCreativeBindings(last.Plan)
	return existingPlanAddedMaterials{}, fmt.Errorf(
		"plan detail did not show requested materials: image_ids=%v title_texts=%v",
		imageBindings,
		titleTexts,
	)
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
	for _, item := range response.Data.Results {
		if item.AdId == adID && strings.EqualFold(item.Status, "SUCCESS") {
			return nil
		}
	}
	for _, item := range response.Data.Results {
		if item.AdId == adID {
			return fmt.Errorf(
				"update plan budget rejected: status=%s error=%s",
				item.Status,
				item.ErrorMessage,
			)
		}
	}
	return fmt.Errorf("update plan budget response omitted ad_id %s", formatUintID(adID))
}

func (t *Tools) discoverExistingPlanAddedMaterialIDs(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	before *existingPlanSnapshot,
) ([]string, []string, error) {
	response, err := t.service.Client.UniPromotionDetail(
		ctx,
		sdk.UniPromotionDetailReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
		},
	)
	if err != nil {
		return nil, nil, err
	}
	if response.Code != 0 || response.Data == nil {
		return nil, nil, apiError(
			"read plan detail for material rollback failed",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	imageIDs, titleTexts := existingPlanCreativeBindings(response.Data)
	return existingPlanNewMaterialIDs(
			imageIDs,
			before.MaterialIDs,
		),
		existingPlanNewMaterialIDs(
			titleTexts,
			before.TitleMaterialIDs,
		),
		nil
}

func existingPlanMaterialIDs(materials []sdk.UniPromotionMaterialInfo) []string {
	ids := make([]string, 0, len(materials))
	for _, material := range materials {
		if material.MaterialId > 0 {
			ids = append(ids, formatUintID(material.MaterialId))
		}
	}
	return uniqueStrings(ids)
}

func existingPlanMaterialImageIDMap(
	materials []sdk.UniPromotionMaterialInfo,
) map[string]string {
	result := make(map[string]string)
	for _, material := range materials {
		if material.ImageId == "" || material.MaterialId == 0 ||
			existingPlanMaterialDeleted(material) {
			continue
		}
		result[material.ImageId] = formatUintID(material.MaterialId)
	}
	return result
}

func existingPlanMaterialTitleMap(
	materials []sdk.UniPromotionMaterialInfo,
) map[string]string {
	result := make(map[string]string)
	for _, material := range materials {
		title := strings.TrimSpace(material.MaterialName)
		if title == "" || material.MaterialId == 0 ||
			existingPlanMaterialDeleted(material) {
			continue
		}
		result[title] = formatUintID(material.MaterialId)
	}
	return result
}

func existingPlanActiveMaterialIDs(materials []sdk.UniPromotionMaterialInfo) []string {
	ids := make([]string, 0, len(materials))
	for _, material := range materials {
		if material.MaterialId > 0 && !existingPlanMaterialDeleted(material) {
			ids = append(ids, formatUintID(material.MaterialId))
		}
	}
	return uniqueStrings(ids)
}

func existingPlanMaterialDeleted(material sdk.UniPromotionMaterialInfo) bool {
	return material.IsDeleted ||
		strings.EqualFold(strings.TrimSpace(material.MaterialStatus), "DELETED")
}

func existingPlanImageIDs(materials []sdk.UniPromotionMaterialInfo) []string {
	ids := make([]string, 0, len(materials))
	for _, material := range materials {
		if material.ImageId != "" && !existingPlanMaterialDeleted(material) {
			ids = append(ids, material.ImageId)
		}
	}
	return uniqueStrings(ids)
}

func existingPlanTitleTexts(materials []sdk.UniPromotionMaterialInfo) []string {
	titles := make([]string, 0, len(materials))
	for _, material := range materials {
		title := strings.TrimSpace(material.MaterialName)
		if title != "" && !existingPlanMaterialDeleted(material) {
			titles = append(titles, title)
		}
	}
	return uniqueStrings(titles)
}

func existingPlanAllTitleTexts(materials []sdk.UniPromotionMaterialInfo) []string {
	titles := make([]string, 0, len(materials))
	for _, material := range materials {
		title := strings.TrimSpace(material.MaterialName)
		if title != "" {
			titles = append(titles, title)
		}
	}
	return uniqueStrings(titles)
}

func existingPlanNewMaterialIDs(after []string, before []string) []string {
	beforeSet := make(map[string]bool, len(before))
	for _, id := range before {
		beforeSet[id] = true
	}
	added := make([]string, 0)
	for _, id := range after {
		if id != "" && !beforeSet[id] {
			added = append(added, id)
		}
	}
	return uniqueStrings(added)
}

func containsAllStrings(values []string, targets []string) bool {
	for _, target := range targets {
		if !containsString(values, target) {
			return false
		}
	}
	return true
}

func cleanupOnlyArg(args map[string]any) (bool, error) {
	value, ok := args["cleanup_only"]
	if !ok || value == nil {
		return false, nil
	}
	result, ok := value.(bool)
	if !ok {
		return false, errors.New("cleanup_only must be a boolean")
	}
	return result, nil
}

func waitExistingPlanCleanup(
	ctx context.Context,
	t *Tools,
	token string,
	advertiserID int64,
	adID uint64,
	before *existingPlanSnapshot,
	imageIDs []string,
	titles []string,
	targetBudget float64,
	result map[string]any,
	primaryErr error,
) (any, error) {
	_ = t.deleteExistingPlanMaterials(
		ctx,
		token,
		advertiserID,
		adID,
		before.ActiveImageIDs,
		before.ActiveTitleIDs,
		before.ActiveTitleTexts,
	)
	state, waitErr := t.waitExistingPlanMaterialsCleared(
		ctx,
		token,
		advertiserID,
		adID,
	)
	restoreErr := t.updateExistingPlanBudget(
		ctx,
		token,
		advertiserID,
		adID,
		before.Plan.Budget,
	)
	evidence := map[string]any{
		"cleanup_only":         true,
		"requested_images":     imageIDs,
		"requested_titles":     titles,
		"requested_budget":     targetBudget,
		"budget_restore_error": nil,
		"cleanup_error":        nil,
	}
	if waitErr != nil {
		evidence["cleanup_error"] = waitErr.Error()
	}
	if restoreErr != nil {
		evidence["budget_restore_error"] = restoreErr.Error()
	}
	result["rollback"] = evidence
	result["error"] = primaryErr.Error()
	if waitErr != nil || restoreErr != nil {
		return result, fmt.Errorf(
			"cleanup-only rollback failed: cleanup=%v budget=%v",
			waitErr,
			restoreErr,
		)
	}
	result["state"] = state
	return result, primaryErr
}

func (t *Tools) waitExistingPlanCreativeBindings(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	original *existingPlanSnapshot,
	removedMaterialIDs []string,
	removedTitles []string,
) (*existingPlanSnapshot, error) {
	for attempt := 0; attempt < 15; attempt++ {
		response, err := t.service.Client.UniPromotionDetail(
			ctx,
			sdk.UniPromotionDetailReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AdId:         adID,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("read plan detail after material delete failed: %w", err)
		}
		if response.Code != 0 || response.Data == nil {
			return nil, apiError(
				"read plan detail after material delete failed",
				response.Code,
				response.Message,
				response.RequestID,
			)
		}
		imageIDs, titleTexts := existingPlanCreativeBindings(response.Data)
		if len(removedMaterialIDs) > 0 && containsAny(titleTexts, removedTitles) {
			goto wait
		}
		if !sameStrings(imageIDs, original.ImageIDs) ||
			!sameStrings(titleTexts, original.TitleTexts) {
			goto wait
		}
		return &existingPlanSnapshot{
			Plan:         response.Data,
			MaterialType: original.MaterialType,
			MaterialIDs:  existingPlanCreativeMaterialIDs(response.Data),
			ImageIDs:     imageIDs,
			TitleTexts:   titleTexts,
			ProductID:    original.ProductID,
			AwemeUID:     original.AwemeUID,
		}, nil
	wait:
		if attempt < 14 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil, fmt.Errorf(
		"plan detail still contains non-original creative bindings: image_ids=%v title_texts=%v",
		original.ImageIDs,
		original.TitleTexts,
	)
}

func (t *Tools) waitExistingPlanRestored(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	expectedBudget float64,
	expectedStatus string,
	expectedOptStatus string,
	expectedMaterialIDs []string,
	expectedTitleMaterialIDs []string,
	expectedImageIDs []string,
	expectedTitleTexts []string,
) (*existingPlanSnapshot, error) {
	var lastPlan *sdk.UniPromotionDetail
	var lastImageIDs []string
	var lastTitleTexts []string
	for attempt := 0; attempt < 15; attempt++ {
		response, err := t.service.Client.UniPromotionDetail(
			ctx,
			sdk.UniPromotionDetailReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AdId:         adID,
			},
		)
		if err == nil && response.Code == 0 && response.Data != nil {
			lastPlan = response.Data
			lastImageIDs, lastTitleTexts = existingPlanCreativeBindings(response.Data)
			if existingPlanRestoredStateMatches(
				response.Data,
				expectedBudget,
				expectedStatus,
				expectedOptStatus,
				lastImageIDs,
				lastTitleTexts,
				expectedImageIDs,
				expectedTitleTexts,
			) {
				return &existingPlanSnapshot{
					Plan:             response.Data,
					MaterialType:     "IMAGE",
					MaterialIDs:      uniqueStrings(expectedMaterialIDs),
					ImageIDs:         lastImageIDs,
					TitleMaterialIDs: uniqueStrings(expectedTitleMaterialIDs),
					TitleTexts:       lastTitleTexts,
				}, nil
			}
		}
		if attempt < 14 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if lastPlan == nil {
		return nil, errors.New("plan detail did not become readable after restoration")
	}
	return nil, fmt.Errorf(
		"plan restoration not confirmed: status=%q opt_status=%q budget=%.2f image_ids=%v title_texts=%v",
		lastPlan.Status,
		lastPlan.OptStatus,
		lastPlan.Budget,
		lastImageIDs,
		lastTitleTexts,
	)
}

func existingPlanRestoredStateMatches(
	plan *sdk.UniPromotionDetail,
	expectedBudget float64,
	expectedStatus string,
	expectedOptStatus string,
	imageIDs []string,
	titleTexts []string,
	expectedImageIDs []string,
	expectedTitleTexts []string,
) bool {
	return plan != nil &&
		almostEqual(plan.Budget, expectedBudget, 0.000001) &&
		strings.EqualFold(plan.Status, expectedStatus) &&
		strings.EqualFold(plan.OptStatus, expectedOptStatus) &&
		sameStrings(imageIDs, expectedImageIDs) &&
		sameStrings(titleTexts, expectedTitleTexts)
}

func existingPlanCreativeBindings(
	plan *sdk.UniPromotionDetail,
) ([]string, []string) {
	if plan == nil {
		return nil, nil
	}
	imageIDs := make([]string, 0)
	titleTexts := make([]string, 0)
	for _, creative := range plan.MultiProductCreativeList {
		for _, image := range creative.ImageMaterial {
			imageIDs = append(imageIDs, image.ImageIDs...)
		}
		for _, title := range creative.TitleMaterial {
			if strings.TrimSpace(title.Title) != "" {
				titleTexts = append(titleTexts, strings.TrimSpace(title.Title))
			}
		}
	}
	if media := plan.ProgrammaticCreativeMediaList; media != nil {
		for _, title := range media.TitleMaterial {
			if strings.TrimSpace(title.Title) != "" {
				titleTexts = append(titleTexts, strings.TrimSpace(title.Title))
			}
		}
	}
	return uniqueStrings(imageIDs), uniqueStrings(titleTexts)
}

func existingPlanCreativeMaterialIDs(
	plan *sdk.UniPromotionDetail,
) []string {
	if plan == nil {
		return nil
	}
	ids := make([]string, 0)
	for _, creative := range plan.MultiProductCreativeList {
		for _, image := range creative.ImageMaterial {
			ids = append(ids, image.ImageIDs...)
		}
	}
	return uniqueStrings(ids)
}

func existingPlanMaterialIDsForImages(
	snapshot *existingPlanSnapshot,
	imageIDs []string,
) []string {
	if snapshot == nil {
		return nil
	}
	targets := make(map[string]bool, len(imageIDs))
	for _, imageID := range imageIDs {
		targets[imageID] = true
	}
	ids := make([]string, 0, len(snapshot.MaterialImageIDMap))
	for imageID, materialID := range snapshot.MaterialImageIDMap {
		if targets[imageID] && materialID != "" {
			ids = append(ids, materialID)
		}
	}
	return uniqueStrings(ids)
}

func existingPlanAddedImageMaterialIDs(
	afterAdd *existingPlanSnapshot,
	before *existingPlanSnapshot,
) []string {
	if afterAdd == nil || before == nil {
		return nil
	}
	imageIDs := make([]string, 0)
	for _, imageID := range afterAdd.ImageIDs {
		if !containsString(before.ImageIDs, imageID) {
			imageIDs = append(imageIDs, imageID)
		}
	}
	return existingPlanMaterialIDsForImageIDs(
		afterAdd,
		imageIDs,
	)
}

func existingPlanTitleMaterialIDs(
	snapshot *existingPlanSnapshot,
	titles []string,
) []string {
	if snapshot == nil {
		return nil
	}
	ids := make([]string, 0, len(titles))
	for _, title := range titles {
		if materialID := snapshot.MaterialTitleMap[title]; materialID != "" {
			ids = append(ids, materialID)
		}
	}
	return uniqueStrings(ids)
}

func existingPlanMaterialIDsForImageIDs(
	snapshot *existingPlanSnapshot,
	imageIDs []string,
) []string {
	if snapshot == nil {
		return nil
	}
	ids := make([]string, 0, len(imageIDs))
	for _, imageID := range imageIDs {
		if materialID := snapshot.MaterialImageIDMap[imageID]; materialID != "" {
			ids = append(ids, materialID)
		}
	}
	return uniqueStrings(ids)
}

func (t *Tools) waitExistingPlanMaterialsCleared(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
) (map[string]any, error) {
	var lastImageIDs []string
	var lastTitleTexts []string
	for attempt := 0; attempt < 20; attempt++ {
		response, err := t.service.Client.UniPromotionDetail(
			ctx,
			sdk.UniPromotionDetailReq{
				AccessToken:  token,
				AdvertiserId: uint64(advertiserID),
				AdId:         adID,
			},
		)
		if err != nil {
			return nil, err
		}
		if response.Code != 0 || response.Data == nil {
			return nil, apiError(
				"read plan detail after material cleanup failed",
				response.Code,
				response.Message,
				response.RequestID,
			)
		}
		lastImageIDs, lastTitleTexts = existingPlanCreativeBindings(response.Data)
		if len(lastImageIDs) == 0 && len(lastTitleTexts) == 0 {
			return map[string]any{
				"image_ids":   []string{},
				"title_texts": []string{},
				"status":      "cleared",
			}, nil
		}
		if attempt < 19 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
	return nil, fmt.Errorf(
		"materials remain active after cleanup: image_ids=%v title_texts=%v",
		lastImageIDs,
		lastTitleTexts,
	)
}

func (t *Tools) deleteExistingPlanMaterials(
	ctx context.Context,
	token string,
	advertiserID int64,
	adID uint64,
	imageMaterialIDs []string,
	titleMaterialIDs []string,
	titles []string,
) error {
	titles = uniqueStrings(titles)
	imageMaterialIDs = uniqueStrings(imageMaterialIDs)
	if len(imageMaterialIDs) == 0 && len(titles) == 0 {
		return nil
	}
	numericImageMaterialIDs, err := numericIDList(imageMaterialIDs)
	if err != nil {
		return fmt.Errorf("image material IDs must be numeric: %w", err)
	}
	response, err := t.service.Client.UniPromotionMaterialDelete(
		ctx,
		sdk.UniPromotionMaterialDeleteReq{
			AccessToken:  token,
			AdvertiserId: uint64(advertiserID),
			AdId:         adID,
			MaterialIds:  numericImageMaterialIDs,
			Titles:       titles,
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

func optionalCreativeTitleListArg(args map[string]any, key string) ([]string, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}
	var items []any
	switch typed := value.(type) {
	case []any:
		items = typed
	case []string:
		items = make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
	default:
		return nil, fmt.Errorf("%s must be an array", key)
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

func productCardTitleMaterials(titles []string) []sdk.UniPromotionMultiCreativeTitleMaterial {
	result := make([]sdk.UniPromotionMultiCreativeTitleMaterial, 0, len(titles))
	for _, title := range titles {
		result = append(result, sdk.UniPromotionMultiCreativeTitleMaterial{
			Title:     title,
			TitleType: "COMMODITY_CARD",
		})
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
	case *sdk.UniPromotionStatusUpdateRes:
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
	case *sdk.UniPromotionStatusUpdateRes:
		if response != nil {
			return response.Message
		}
	}
	return ""
}

func responseRequestID(value any) string {
	switch response := value.(type) {
	case *sdk.UniPromotionStatusUpdateRes:
		if response != nil {
			return response.RequestID
		}
	}
	return ""
}
