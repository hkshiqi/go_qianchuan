package sdk

import (
	"encoding/json"
	"testing"
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
