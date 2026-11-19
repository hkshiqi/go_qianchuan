package unipromotion

// Material 全域推广计划素材
type Material struct {
	// MaterialInfo 素材基础信息
	MaterialInfo *MaterialInfo `json:"material_info,omitempty"`
	// IsDelete 是否已删除
	IsDelete bool `json:"is_delete,omitempty"`
	// MaterialStatus 素材状态
	MaterialStatus string `json:"material_status,omitempty"`
	// DeliveryNotReason 不可投放原因
	DeliveryNotReason []string `json:"delivery_not_reason,omitempty"`
	// AuditStatus 审核状态
	AuditStatus string `json:"audit_status,omitempty"`
	// MaterialSelectType 素材选择类型
	MaterialSelectType string `json:"material_select_type,omitempty"`
	// ProductIDList 乘方多号场景商品 ID 列表
	ProductIDList []uint64 `json:"product_id_list,omitempty"`
	// AwemeIDList 乘方多号场景抖音号 ID 列表
	AwemeIDList []uint64 `json:"aweme_id_list,omitempty"`
	// ProductInfo 关联商品
	ProductInfo []MaterialProductInfo `json:"product_info,omitempty"`
	// StatsInfo 直播全域素材指标（官方 *_for_roi2_v2 字段）
	StatsInfo *LiveMaterialStats `json:"stats_info,omitempty"`
}

// LiveMaterialStats 直播全域素材指标，单位与官方接口一致。
type LiveMaterialStats struct {
	ProductShowCountForRoi2    int64   `json:"product_show_count_for_roi2,omitempty"`
	ProductClickCountForRoi2   int64   `json:"product_click_count_for_roi2,omitempty"`
	ProductCvrRateForRoi2      float64 `json:"product_cvr_rate_for_roi2,omitempty"`
	ProductConvertRateForRoi2  float64 `json:"product_convert_rate_for_roi2,omitempty"`
	StatCostForRoi2            float64 `json:"stat_cost_for_roi2,omitempty"`
	TotalPrepayAndPayOrderRoi2 float64 `json:"total_prepay_and_pay_order_roi2,omitempty"`
	TotalPayOrderGmvForRoi2    float64 `json:"total_pay_order_gmv_for_roi2,omitempty"`
	TotalPayOrderCountForRoi2  int64   `json:"total_pay_order_count_for_roi2,omitempty"`
	TotalCostPerPayOrderForRoi2 float64 `json:"total_cost_per_pay_order_for_roi2,omitempty"`
	TotalPayOrderCouponAmountForRoi2 float64 `json:"total_pay_order_coupon_amount_for_roi2,omitempty"`
	TotalUnfinishedEstimateOrderGmvForRoi2 float64 `json:"total_unfinished_estimate_order_gmv_for_roi2,omitempty"`
	TotalEcomPlatformSubsidyAmountForRoi2 float64 `json:"total_ecom_platform_subsidy_amount_for_roi2,omitempty"`
	TotalPayOrderGmvIncludeCouponForRoi2 float64 `json:"total_pay_order_gmv_include_coupon_for_roi2,omitempty"`
	TotalCostPerPayOrderSettleForRoi2_1H float64 `json:"total_cost_per_pay_order_settle_for_roi2_1h,omitempty"`
	TotalOrderSettleCountForRoi2_1H int64 `json:"total_order_settle_count_for_roi2_1h,omitempty"`
	TotalOrderSettleAmountForRoi2_1H float64 `json:"total_order_settle_amount_for_roi2_1h,omitempty"`
	TotalPrepayAndPaySettleRoi2_1H float64 `json:"total_prepay_and_pay_settle_roi2_1h,omitempty"`
	TotalOrderRealSettleAmountForRoi2_1H float64 `json:"total_order_real_settle_amount_for_roi2_1h,omitempty"`
	LiveShowCountForRoi2V2     int64   `json:"live_show_count_for_roi2_v2,omitempty"`
	LiveWatchCountForRoi2V2    int64   `json:"live_watch_count_for_roi2_v2,omitempty"`
	LiveCvrRateForRoi2V2       float64 `json:"live_cvr_rate_for_roi2_v2,omitempty"`
	LiveConvertRateForRoi2V2   float64 `json:"live_convert_rate_for_roi2_v2,omitempty"`
	LiveShowCountExcludeVideoForRoi2 int64 `json:"live_show_count_exclude_video_for_roi2,omitempty"`
	LiveWatchCountExcludeVideoForRoi2 int64 `json:"live_watch_count_exclude_video_for_roi2,omitempty"`
	LiveCvrRateExcludeVideoForRoi2 float64 `json:"live_cvr_rate_exclude_video_for_roi2,omitempty"`
	LiveConvertRateExcludeVideoForRoi2 float64 `json:"live_convert_rate_exclude_video_for_roi2,omitempty"`
}

// MaterialInfo 素材基础信息
type MaterialInfo struct {
	// MaterialType 素材类型
	MaterialType string `json:"material_type,omitempty"`
	// VideoMaterial 视频素材信息
	VideoMaterial *VideoMaterial `json:"video_material,omitempty"`
	// TitleMaterial 标题素材信息
	TitleMaterial *TitleMaterial `json:"title_material,omitempty"`
	// ImageMaterial 图片素材信息
	ImageMaterial *ImageMaterial `json:"image_material,omitempty"`
	// RoomMaterial 直播间画面素材信息
	RoomMaterial *RoomMaterial `json:"room_material,omitempty"`
	// CarouselMaterial 图文素材信息
	CarouselMaterial *CarouselMaterial `json:"carousel_material,omitempty"`
}

// MaterialImage 素材图片
type MaterialImage struct {
	Height   int64  `json:"height,omitempty"`
	Width    int64  `json:"width,omitempty"`
	ID       string `json:"id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// ImageMaterial 图片素材
type ImageMaterial struct {
	MaterialID uint64 `json:"material_id,omitempty"`
	ImageID    string `json:"image_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}

// RoomMaterial 直播间画面素材
type RoomMaterial struct {
	MaterialID   uint64   `json:"material_id,omitempty"`
	AwemeID      uint64   `json:"aweme_id,omitempty"`
	AwemeName    string   `json:"aweme_name,omitempty"`
	AwemeAvatar  []string `json:"aweme_avatar,omitempty"`
}

// CarouselMaterial 图文素材
type CarouselMaterial struct {
	MaterialID  uint64 `json:"material_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MusicURL    string `json:"music_url,omitempty"`
}

// MaterialProductInfo 素材关联商品
type MaterialProductInfo struct {
	ProductID   uint64 `json:"product_id,omitempty"`
	ProductName string `json:"product_name,omitempty"`
	ProductImage string `json:"product_image,omitempty"`
}
