package sdk

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeUniPromotionDetailAcceptsObjectCreativeMedia(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"message": "OK",
		"data": {
			"ad_id": 1856283025550586,
			"name": "主力直播计划",
			"marketing_goal": "LIVE_PROM_GOODS",
			"status": "DELIVERY_OK",
			"delivery_setting": {
				"budget": 20000,
				"roi2_goal": 1.98,
				"smart_bid_type": "SMART_BID_CUSTOM",
				"start_time": "2026-11-20 00:00:00"
			},
			"programmatic_creative_media_list": {
				"video_material": [
					{
						"image_mode": "VIDEO_VERTICAL",
						"video_id": "v0200fg10000",
						"video_cover_id": "cover-1",
						"aweme_item_id": 7654214461952425993
					}
				],
				"title_material": [
					{"title": "测试标题"}
				],
				"block_video_material": [
					{"aweme_item_id": 7662517359383560233}
				]
			}
		}
	}`)
	result, err := decodeUniPromotionDetail(body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data == nil {
		t.Fatal("detail is nil")
	}
	if result.Data.AdId != 1856283025550586 ||
		result.Data.AdName != "主力直播计划" ||
		result.Data.Budget != 20000 ||
		result.Data.Roi2Goal != 1.98 ||
		result.Data.DeliverySetting == nil ||
		result.Data.DeliverySetting.Budget != 20000 ||
		result.Data.DeliverySetting.Roi2Goal != 1.98 {
		t.Fatalf("detail = %#v", result.Data)
	}
	media := result.Data.ProgrammaticCreativeMediaList
	if media == nil || len(media.VideoMaterial) != 1 ||
		media.VideoMaterial[0].VideoID != "v0200fg10000" ||
		media.VideoMaterial[0].AwemeItemID != 7654214461952425993 ||
		len(media.TitleMaterial) != 1 ||
		len(media.BlockVideoMaterial) != 1 {
		t.Fatalf("creative media = %#v", media)
	}
}

func TestDecodeUniPromotionDetailPreservesFlattenedProductCard(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"message": "OK",
		"data": {
			"ad_id": 1882474755521636,
			"name": "Codex-CRUD-PRODUCT-CARD-TEST-20261121-213029",
			"marketing_goal": "VIDEO_PROM_GOODS",
			"status": "DISABLE",
			"multi_product_creative_list": [{
				"product_id": 3826314764871664464,
				"creative_type": "PROGRAMMATIC_CREATIVE",
				"image_material": [{
					"image_mode": "SQUARE",
					"image_ids": ["tos-cn-i-sd07hgqsbj/test-image"]
				}],
				"title_material": [{
					"title": "红卫温和洁净润肤手工香皂",
					"title_type": "COMMODITY_CARD"
				}],
				"product_image_id": "tos-cn-i-sd07hgqsbj/test-image",
				"promotion_card_title": "红卫温和洁净润肤手工香皂",
				"promotion_card_selling_points": ["温和清洁不紧绷"],
				"qcpx_mode": "QCPX_MODE_OFF"
			}]
		}
	}`)
	result, err := decodeUniPromotionDetail(body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data == nil || len(result.Data.MultiProductCreativeList) != 1 {
		t.Fatalf("detail = %#v", result.Data)
	}
	card := result.Data.MultiProductCreativeList[0].CreativeCard
	if card == nil {
		t.Fatal("flattened creative_card was dropped")
	}
	if card.PromotionCardTitle != "红卫温和洁净润肤手工香皂" ||
		card.PromotionCardImageID != "tos-cn-i-sd07hgqsbj/test-image" ||
		strings.Join(card.PromotionCardSellingPoints, "|") != "温和清洁不紧绷" {
		t.Fatalf("creative card = %#v", card)
	}
}

func TestAdlabSceneAcceptsDetailNumberAndListString(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want AdlabScene
	}{
		{raw: `1`, want: AdlabSceneOverall},
		{raw: `0`, want: AdlabSceneUni},
		{raw: `"OVERALL_PROJECT"`, want: AdlabSceneOverall},
		{raw: `"UNI_PROJECT"`, want: AdlabSceneUni},
		{raw: `null`, want: ""},
	} {
		var got AdlabScene
		if err := json.Unmarshal([]byte(test.raw), &got); err != nil {
			t.Fatalf("unmarshal %s: %v", test.raw, err)
		}
		if got != test.want {
			t.Fatalf("adlab_scene %s = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestValidateUniPromotionProductPlanCreateReq(t *testing.T) {
	valid := UniPromotionProductPlanCreateReq{
		AccessToken:        "token",
		AdvertiserId:       1855810750424170,
		Name:               "Codex-CRUD-PRODUCT-CARD-TEST-20261120-120000",
		AwemeId:            63307887035,
		ProductId:          3828195166146134185,
		Budget:             30,
		StartDate:          "2026-11-21",
		EndDate:            "2026-11-21",
		DeepExternalAction: "AD_CONVERT_TYPE_LIVE_PURE_PAY_ROI",
		ProductCard:        true,
		ProductImage:       "tos-cn-i-sd07hgqsbj/fa1793f2e57b4c87ba5c57706ab3d272",
		ProductTitle:       "红卫羊脂皂旗舰店纯商品卡测试",
		SellingPoints:      []string{"温和清洁不紧绷"},
		PromotionCardImage: "tos-cn-i-sd07hgqsbj/fa1793f2e57b4c87ba5c57706ab3d272",
	}
	if err := validateUniPromotionProductPlanCreateReq(valid); err != nil {
		t.Fatalf("valid product-card request was rejected: %v", err)
	}
	cases := map[string]func(*UniPromotionProductPlanCreateReq){
		"missing image": func(req *UniPromotionProductPlanCreateReq) {
			req.ProductImage = ""
		},
		"budget below platform minimum": func(req *UniPromotionProductPlanCreateReq) {
			req.Budget = 29.99
		},
		"wrong optimization target": func(req *UniPromotionProductPlanCreateReq) {
			req.DeepExternalAction = "AD_CONVERT_TYPE_LIVE_PAY_ROI"
		},
		"short title": func(req *UniPromotionProductPlanCreateReq) {
			req.ProductTitle = "太短"
		},
		"missing selling points": func(req *UniPromotionProductPlanCreateReq) {
			req.SellingPoints = nil
		},
		"missing promotion card image": func(req *UniPromotionProductPlanCreateReq) {
			req.PromotionCardImage = ""
		},
		"selling point too short": func(req *UniPromotionProductPlanCreateReq) {
			req.SellingPoints = []string{"羊脂皂"}
		},
		"selling point too long": func(req *UniPromotionProductPlanCreateReq) {
			req.SellingPoints = []string{"这一条投放卡片卖点文字明显超过了平台允许的最大长度"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			if err := validateUniPromotionProductPlanCreateReq(req); err == nil {
				t.Fatal("invalid product-card request was accepted")
			}
		})
	}
}

func TestValidateOverallProductPlanCreateReq(t *testing.T) {
	tomorrow := time.Now().In(shanghaiTimezone()).AddDate(0, 0, 1).Format("2006-01-02")
	valid := OverallProductPlanCreateReq{
		AccessToken:  "redacted",
		AdvertiserId: 1855810750424170,
		Name:         "overall-plan",
		ProductId:    3828195166146134185,
		AwemeId:      2362231432675704,
		Budget:       OverallProductPlanMinimumBudget,
		Roi2Goal:     1.90,
		StartDate:    tomorrow,
		EndDate:      tomorrow,
	}
	if err := validateOverallProductPlanCreateReq(valid); err != nil {
		t.Fatalf("valid request was rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*OverallProductPlanCreateReq)
	}{
		{name: "missing token", mutate: func(req *OverallProductPlanCreateReq) { req.AccessToken = "" }},
		{name: "missing advertiser", mutate: func(req *OverallProductPlanCreateReq) { req.AdvertiserId = 0 }},
		{name: "missing name", mutate: func(req *OverallProductPlanCreateReq) { req.Name = "" }},
		{name: "missing product", mutate: func(req *OverallProductPlanCreateReq) { req.ProductId = 0 }},
		{name: "missing aweme", mutate: func(req *OverallProductPlanCreateReq) { req.AwemeId = 0 }},
		{name: "budget below minimum", mutate: func(req *OverallProductPlanCreateReq) { req.Budget = 299.99 }},
		{name: "roi zero", mutate: func(req *OverallProductPlanCreateReq) { req.Roi2Goal = 0 }},
		{name: "roi above maximum", mutate: func(req *OverallProductPlanCreateReq) { req.Roi2Goal = 100.01 }},
		{name: "start today", mutate: func(req *OverallProductPlanCreateReq) {
			req.StartDate = time.Now().In(shanghaiTimezone()).Format("2006-01-02")
			req.EndDate = req.StartDate
		}},
		{name: "end before start", mutate: func(req *OverallProductPlanCreateReq) {
			req.EndDate = time.Now().In(shanghaiTimezone()).AddDate(0, 0, 1).Format("2006-01-02")
			req.StartDate = time.Now().In(shanghaiTimezone()).AddDate(0, 0, 2).Format("2006-01-02")
		}},
		{name: "invalid start format", mutate: func(req *OverallProductPlanCreateReq) {
			req.StartDate = "11/21/2026"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := valid
			test.mutate(&req)
			if err := validateOverallProductPlanCreateReq(req); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}
