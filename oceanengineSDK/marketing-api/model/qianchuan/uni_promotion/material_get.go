package unipromotion

import (
	"strconv"

	"github.com/bububa/oceanengine/marketing-api/model"
	"github.com/bububa/oceanengine/marketing-api/util"
)

// MaterialGetRequest 获取全域推广计划下素材 API Request
type MaterialGetRequest struct {
	// Filtering 过滤条件
	Filtering *MaterialGetFilter `json:"filtering,omitempty"`
	// AdvertiserID 千川广告账户ID
	AdvertiserID uint64 `json:"advertiser_id,omitempty"`
	// AdID 计划id
	AdID uint64 `json:"ad_id,omitempty"`
	// Fields 指定返回字段
	Fields []string `json:"fields,omitempty"`
	// OrderField 排序字段
	OrderField string `json:"order_field,omitempty"`
	// OrderType 排序方向
	OrderType string `json:"order_type,omitempty"`
	// Page 页码，默认值：1
	Page int `json:"page,omitempty"`
	// PageSize 页面大小，允许值：10, 20, 50, 100，默认值：10
	PageSize int `json:"page_size,omitempty"`
}

// MaterialGetFilter 过滤条件
type MaterialGetFilter struct {
	// MaterialType 素材类型，可选值:
	// IMAGE 图片，图文
	// LIVE_ROOM 直播间画面
	// TITLE 标题
	// VIDEO 视频
	MaterialType string `json:"material_type,omitempty"`
	// VideoType  视频类型，仅material_type=VIDEO时支持，可选值:
	// ALL 全部
	// CUSTOM 自选投放视频
	// AUTO 智能优选视频
	VideoType string `json:"video_type,omitempty"`
	// MaterialStatus  投放状态，仅material_type=VIDEO/TITLE时支持，可选值:
	// DELIVERY_OK 投放中，默认
	// DELETED 已删除
	// ALL 全部
	MaterialStatus string `json:"material_status,omitempty"`
	// AnalysisType 素材评估，仅material_type=VIDEO时支持
	// 首发素材 FIRST_PUBLISH_MATERIAL
	// 优质素材 HIGH_QUALITY_MATERIAL
	// 低质素材 LOW_QUALITY_MATERIAL
	// 低效素材 INEFFICIENT_MATERIAL
	// 搬运素材 CARRY_MATERIAL
	// 同质化素材 SIMILAR_MATERIAL
	AnalysisType []string `json:"analysis_type,omitempty"`
	// MaterialSelectType 素材选择方式
	MaterialSelectType string `json:"material_select_type,omitempty"`
	// SearchKeyword 搜索关键词，支持根据视频mid进行搜索
	// 注意：仅material_type=VIDEO时支持
	SearchKeyword string `json:"search_keyword,omitempty"`
	// ProductKeyword 商品关键词
	ProductKeyword string `json:"product_keyword,omitempty"`
	// ProductID 商品ID
	ProductID uint64 `json:"product_id,omitempty"`
	// StartDate 统计起始日期
	StartDate string `json:"start_date,omitempty"`
	// EndDate 统计截止日期
	EndDate string `json:"end_date,omitempty"`
}

// Encode implements GetRequest interface
func (r MaterialGetRequest) Encode() string {
	values := util.GetUrlValues()
	values.Set("advertiser_id", strconv.FormatUint(r.AdvertiserID, 10))
	values.Set("ad_id", strconv.FormatUint(r.AdID, 10))
	if r.Filtering != nil {
		values.Set("filtering", string(util.JSONMarshal(r.Filtering)))
	}
	if len(r.Fields) > 0 {
		values.Set("fields", string(util.JSONMarshal(r.Fields)))
	}
	if r.OrderField != "" {
		values.Set("order_field", r.OrderField)
	}
	if r.OrderType != "" {
		values.Set("order_type", r.OrderType)
	}
	if r.Page > 0 {
		values.Set("page", strconv.Itoa(r.Page))
	}
	if r.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(r.PageSize))
	}
	ret := values.Encode()
	util.PutUrlValues(values)
	return ret
}

// MaterialGetResponse 获取全域推广计划下素材 API Response
type MaterialGetResponse struct {
	Data *MaterialGetResult `json:"data,omitempty"`
	model.BaseResponse
}

type MaterialGetResult struct {
	// PageInfo 分页信息
	PageInfo *model.PageInfo `json:"page_info,omitempty"`
	// AdMaterialInfos 素材列表
	AdMaterialInfos []Material `json:"ad_material_infos,omitempty"`
}
