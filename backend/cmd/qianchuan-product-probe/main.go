package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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

type planOutput struct {
	AdID             string                         `json:"ad_id"`
	Name             string                         `json:"name"`
	Status           string                         `json:"status"`
	OptStatus        string                         `json:"opt_status"`
	Budget           float64                        `json:"budget"`
	AdlabScene       string                         `json:"adlab_scene"`
	ProductID        string                         `json:"product_id,omitempty"`
	AwemeUID         string                         `json:"aweme_uid,omitempty"`
	CreativeType     string                         `json:"creative_type,omitempty"`
	MaterialCount    int                            `json:"material_count,omitempty"`
	MaterialIDs      []string                       `json:"material_ids,omitempty"`
	VideoIDs         []string                       `json:"video_ids,omitempty"`
	ImageIDs         []string                       `json:"image_ids,omitempty"`
	MaterialReadback json.RawMessage                `json:"material_readback,omitempty"`
	Detail           *sdk.UniPromotionDetail        `json:"detail,omitempty"`
	Error            string                         `json:"error,omitempty"`
	Materials        []sdk.UniPromotionMaterialInfo `json:"-"`
}

func main() {
	envFile := flag.String("env", "", "optional dotenv file")
	tenantDir := flag.String("tenant-dir", "/opt/go-qianchuan/data/tenants", "tenant token directory")
	tenantID := flag.String("tenant", "owner", "tenant id")
	advertiserID := flag.String("advertiser-id", "", "advertiser id")
	scene := flag.String("scene", "OVERALL_PROJECT", "OVERALL_PROJECT or UNI_PROJECT")
	planID := flag.String("plan-id", "", "optional plan id; when omitted, list paused candidate plans")
	pageSize := flag.Int("page-size", 100, "list page size, max 100")
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
	*scene = strings.ToUpper(strings.TrimSpace(*scene))
	if *scene != string(sdk.AdlabSceneOverall) && *scene != string(sdk.AdlabSceneUni) {
		log.Fatal("--scene must be OVERALL_PROJECT or UNI_PROJECT")
	}
	if *pageSize < 1 || *pageSize > 100 {
		log.Fatal("--page-size must be between 1 and 100")
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

	var plans []planOutput
	if strings.TrimSpace(*planID) != "" {
		adID, err := strconv.ParseUint(strings.TrimSpace(*planID), 10, 64)
		if err != nil || adID == 0 {
			log.Fatal("--plan-id must be a positive numeric id")
		}
		plan, err := inspectPlan(ctx, client, token, uint64(account), adID)
		if err != nil {
			log.Fatalf("inspect plan: %v", err)
		}
		plans = append(plans, plan)
	} else {
		now := time.Now().In(time.FixedZone("CST", 8*3600))
		response, err := client.UniPromotionList(ctx, sdk.UniPromotionListReq{
			AccessToken:   token,
			AdvertiserId:  uint64(account),
			StartTime:     now.AddDate(0, 0, -1).Format("2006-01-02 15:04:05"),
			EndTime:       now.Format("2006-01-02 15:04:05"),
			MarketingGoal: "VIDEO_PROM_GOODS",
			AdlabScene:    *scene,
			Fields: []string{
				"stat_cost_for_overall_roi2",
				"total_pay_order_gmv_for_roi2",
				"total_pay_order_count_for_roi2",
				"total_prepay_and_pay_order_roi2",
			},
			OrderField: "stat_cost_for_overall_roi2",
			OrderType:  "DESC",
			Page:       1,
			PageSize:   *pageSize,
			Filtering: &sdk.UniPromotionListFiltering{
				Status: "ALL",
			},
		})
		if err != nil {
			log.Fatalf("list product plans: %v", err)
		}
		if response.Code != 0 {
			log.Fatalf(
				"list product plans: code=%d message=%s",
				response.Code,
				response.Message,
			)
		}
		for _, plan := range response.Data.List {
			if !strings.EqualFold(plan.Status, "DISABLE") ||
				!strings.EqualFold(plan.OptStatus, "DISABLE") {
				continue
			}
			item, err := inspectPlan(ctx, client, token, uint64(account), plan.AdId)
			if err != nil {
				plans = append(plans, planOutput{
					AdID:       strconv.FormatUint(plan.AdId, 10),
					Name:       plan.AdName,
					Status:     plan.Status,
					OptStatus:  plan.OptStatus,
					Budget:     plan.Budget,
					AdlabScene: string(plan.AdlabScene),
					Error:      err.Error(),
				})
				continue
			}
			plans = append(plans, item)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(map[string]any{
		"account_id": strconv.FormatInt(account, 10),
		"scene":      *scene,
		"plans":      plans,
	}); err != nil {
		log.Fatalf("encode output: %v", err)
	}
}

func inspectPlan(
	ctx context.Context,
	client sdk.QianchuanClient,
	token string,
	advertiserID uint64,
	adID uint64,
) (planOutput, error) {
	response, err := client.UniPromotionDetail(ctx, sdk.UniPromotionDetailReq{
		AccessToken:  token,
		AdvertiserId: advertiserID,
		AdId:         adID,
	})
	if err != nil {
		return planOutput{}, fmt.Errorf("detail request failed: %w", err)
	}
	if response.Code != 0 || response.Data == nil {
		return planOutput{}, fmt.Errorf(
			"detail failed: code=%d message=%s request_id=%s",
			response.Code,
			response.Message,
			response.RequestID,
		)
	}
	detail := response.Data
	output := planOutput{
		AdID:       strconv.FormatUint(adID, 10),
		Name:       detail.Name,
		Status:     detail.Status,
		OptStatus:  detail.OptStatus,
		Budget:     detail.Budget,
		AdlabScene: string(detail.AdlabScene),
		Detail:     detail,
	}
	productID, awemeUID := planIdentity(detail)
	if productID > 0 {
		output.ProductID = strconv.FormatUint(productID, 10)
	}
	if awemeUID > 0 {
		output.AwemeUID = strconv.FormatUint(awemeUID, 10)
	}
	creativeType, err := planCreativeType(detail)
	if err != nil {
		output.Error = err.Error()
		return output, nil
	}
	output.CreativeType = creativeType
	materials, err := readPlanMaterials(ctx, client, token, advertiserID, adID, creativeType)
	if err != nil {
		output.Error = err.Error()
		return output, nil
	}
	output.Materials = materials
	output.MaterialCount = len(materials)
	for _, material := range materials {
		if material.MaterialId > 0 {
			output.MaterialIDs = append(output.MaterialIDs, strconv.FormatUint(material.MaterialId, 10))
		}
		if material.VideoId != "" {
			output.VideoIDs = append(output.VideoIDs, material.VideoId)
		}
		if material.ImageId != "" {
			output.ImageIDs = append(output.ImageIDs, material.ImageId)
		}
	}
	raw, _ := json.Marshal(materials)
	output.MaterialReadback = raw
	return output, nil
}

func planIdentity(detail *sdk.UniPromotionDetail) (uint64, uint64) {
	var productID uint64
	if len(detail.ProductInfos) > 0 {
		productID = detail.ProductInfos[0].ProductID
	}
	if productID == 0 && len(detail.MultiProductCreativeList) > 0 {
		productID = detail.MultiProductCreativeList[0].ProductID
	}
	awemeUID := detail.AwemeId
	if awemeUID == 0 && len(detail.AwemeInfos) > 0 {
		awemeUID = detail.AwemeInfos[0].AwemeUID
	}
	if awemeUID == 0 && len(detail.MultiProductCreativeList) > 0 {
		awemeUID = detail.MultiProductCreativeList[0].AwemeUID
	}
	return productID, awemeUID
}

func planCreativeType(detail *sdk.UniPromotionDetail) (string, error) {
	if len(detail.MultiProductCreativeList) > 0 {
		creative := detail.MultiProductCreativeList[0]
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
	if detail.ProgrammaticCreativeMediaList != nil {
		switch {
		case len(detail.ProgrammaticCreativeMediaList.VideoMaterial) > 0:
			return "VIDEO", nil
		case len(detail.ProgrammaticCreativeMediaList.TitleMaterial) > 0:
			return "TITLE", nil
		}
	}
	return "", fmt.Errorf("cannot determine creative material type")
}

func readPlanMaterials(
	ctx context.Context,
	client sdk.QianchuanClient,
	token string,
	advertiserID uint64,
	adID uint64,
	materialType string,
) ([]sdk.UniPromotionMaterialInfo, error) {
	var result []sdk.UniPromotionMaterialInfo
	for page := 1; page <= 100; page++ {
		response, err := client.UniPromotionMaterialGet(ctx, sdk.UniPromotionMaterialGetReq{
			AccessToken:        token,
			AdvertiserId:       advertiserID,
			AdId:               adID,
			MaterialType:       materialType,
			Status:             "ALL",
			MaterialSelectType: "ALL",
			Page:               page,
			PageSize:           100,
		})
		if err != nil {
			return nil, fmt.Errorf("material list page %d failed: %w", page, err)
		}
		if response.Code != 0 {
			return nil, fmt.Errorf(
				"material list failed: code=%d message=%s",
				response.Code,
				response.Message,
			)
		}
		result = append(result, response.Data.Materials...)
		if len(response.Data.Materials) < 100 ||
			response.Data.PageInfo.TotalPage <= page {
			return result, nil
		}
	}
	return result, nil
}
