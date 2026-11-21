package sdk

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ===========================
// 项目内部 DTO 类型定义
// 与底层 SDK 解耦，便于切换 SDK 实现
// ===========================

// ===== Auth & Session Types =====

type OauthAccessTokenReq struct {
	AuthCode string `json:"auth_code"`
}

type OauthAccessTokenRes struct {
	Code    int64                    `json:"code"`
	Message string                   `json:"message"`
	Data    *OauthAccessTokenResData `json:"data,omitempty"`
}

type OauthAccessTokenResData struct {
	AccessToken           string  `json:"access_token"`
	RefreshToken          string  `json:"refresh_token"`
	ExpiresIn             int64   `json:"expires_in"`
	RefreshTokenExpiresIn int64   `json:"refresh_token_expires_in"`
	AdvertiserId          int64   `json:"advertiser_id"`
	AdvertiserIds         []int64 `json:"advertiser_ids"`
}

type OauthRefreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}

type OauthRefreshTokenRes struct {
	Code    int64                    `json:"code"`
	Message string                   `json:"message"`
	Data    *OauthAccessTokenResData `json:"data,omitempty"`
}

type UserInfoReq struct {
	AccessToken string `json:"access_token"`
}

type UserInfoRes struct {
	Code    int64         `json:"code"`
	Message string        `json:"message"`
	Data    *UserInfoData `json:"data,omitempty"`
}

type UserInfoData struct {
	Id          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// ===== Advertiser Budget Types =====

type AdvertiserBudgetGetReq struct {
	AccessToken  string
	AdvertiserId int64
}

type AdvertiserBudgetGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Budget     int64  `json:"budget"`
		BudgetMode string `json:"budget_mode"`
	} `json:"data"`
}

type AdvertiserBudgetUpdateReq struct {
	AccessToken  string
	AdvertiserId int64
	Budget       int64
}

type AdvertiserBudgetUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// ===== Advertiser & Account Types =====

type AdvertiserListReq struct {
	AccessToken string `json:"access_token"`
	AppId       int64  `json:"app_id"`
	Secret      string `json:"secret"`
}

type AdvertiserListRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []AdvertiserListItem `json:"list"`
	} `json:"data"`
}

type AdvertiserListItem struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	AdvertiserName string `json:"advertiser_name"`
	IsValid        bool   `json:"is_valid"`
	AccountRole    string `json:"account_role"`
}

type AdvertiserInfoReq struct {
	AccessToken   string   `json:"access_token"`
	AdvertiserIds []int64  `json:"advertiser_ids"`
	Fields        []string `json:"fields,omitempty"`
}

type AdvertiserInfoRes struct {
	Code    int64                   `json:"code"`
	Message string                  `json:"message"`
	Data    []AdvertiserInfoResData `json:"data"`
}

type AdvertiserInfoResData struct {
	AdvertiserId       int64  `json:"advertiser_id"`
	AdvertiserName     string `json:"advertiser_name"`
	Company            string `json:"company"`
	FirstIndustryName  string `json:"first_industry_name"`
	SecondIndustryName string `json:"second_industry_name"`
}

type ShopAdvertiserListReq struct {
	AccessToken string `json:"access_token"`
	ShopId      int64  `json:"shop_id"`
	Page        int64  `json:"page"`
	PageSize    int64  `json:"page_size"`
}

type ShopAdvertiserListRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []ShopAdvertiserItem `json:"list"`
		PageInfo PageInfo             `json:"page_info"`
	} `json:"data"`
}

type ShopAdvertiserItem struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	AdvertiserName string `json:"advertiser_name"`
}

type AgentAdvertiserListReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
}

type AgentAdvertiserListRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []AgentAdvertiserItem `json:"list"`
		PageInfo PageInfo              `json:"page_info"`
	} `json:"data"`
}

type AgentAdvertiserItem struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	AdvertiserName string `json:"advertiser_name"`
	Company        string `json:"company"`
}

type CustomerCenterAdvertiserListReq struct {
	AccessToken    string `json:"access_token"`
	CustomerCenter int64  `json:"customer_center_id"`
	Page           int64  `json:"page"`
	PageSize       int64  `json:"page_size"`
}

type CustomerCenterAdvertiserListRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []CustomerCenterAdvertiserItem `json:"list"`
		PageInfo PageInfo                       `json:"page_info"`
	} `json:"data"`
}

type CustomerCenterAdvertiserItem struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	AdvertiserName string `json:"advertiser_name"`
	AdvertiserType string `json:"advertiser_type"`
	AccountName    string `json:"account_name"`
	AccountType    string `json:"account_type"`
}

// ===== Aweme Authorized Types =====

type AwemeAuthorizedGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
}

type AwemeAuthorizedGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []AwemeAuthorizedGetResDetail `json:"list"`
		PageInfo PageInfo                      `json:"page_info"`
	} `json:"data"`
}

type AwemeAuthorizedGetResDetail struct {
	AwemeId         int64    `json:"aweme_id"`
	AwemeName       string   `json:"aweme_name"`
	AwemeShowId     string   `json:"aweme_show_id"`
	AwemeAvatar     string   `json:"aweme_avatar"`
	BindType        []string `json:"bind_type"`
	AccountTypeList []string `json:"account_type_list"`
}

// ===== Product Types =====

type ProductAvailableGetReq struct {
	AccessToken  string                     `json:"access_token"`
	AdvertiserId int64                      `json:"advertiser_id"`
	Page         int64                      `json:"page"`
	PageSize     int64                      `json:"page_size"`
	Filtering    *ProductAvailableGetFilter `json:"filtering,omitempty"`
}

type ProductAvailableGetFilter struct {
	ProductIds  []int64 `json:"product_ids,omitempty"`
	ProductName string  `json:"product_name,omitempty"`
}

type ProductAvailableGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []ProductAvailableGetResDetail `json:"list"`
		PageInfo PageInfo                       `json:"page_info"`
	} `json:"data"`
}

type ProductAvailableGetResDetail struct {
	ProductId     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	DiscountPrice float64 `json:"discount_price"`
	MarketPrice   float64 `json:"market_price"`
	Img           string  `json:"img"`
	ProductRate   float64 `json:"product_rate"`
	SaleTime      string  `json:"sale_time"`
	CategoryName  string  `json:"category_name"`
}

// ===== Campaign Types =====

type CampaignListGetReq struct {
	AccessToken  string                `json:"access_token"`
	AdvertiserId int64                 `json:"advertiser_id"`
	Page         int64                 `json:"page"`
	PageSize     int64                 `json:"page_size"`
	Filter       CampaignListGetFilter `json:"filter"`
}

type CampaignListGetFilter struct {
	Ids           []int64 `json:"ids,omitempty"`
	Name          string  `json:"name,omitempty"`
	MarketingGoal string  `json:"marketing_goal,omitempty"`
	Status        string  `json:"status,omitempty"`
}

type CampaignListGetRes struct {
	Code    int64             `json:"code"`
	Message string            `json:"message"`
	Data    *CampaignListData `json:"data,omitempty"`
}

type CampaignListData struct {
	List     []Campaign `json:"list"`
	PageInfo PageInfo   `json:"page_info"`
}

type Campaign struct {
	Id            int64   `json:"id"`
	Name          string  `json:"name"`
	Budget        float64 `json:"budget"`
	BudgetMode    string  `json:"budget_mode"`
	MarketingGoal string  `json:"marketing_goal"`
	Status        string  `json:"status"`
	CreateDate    string  `json:"create_date"`
}

type CampaignCreateReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         CampaignCreateBody `json:"body"`
}

type CampaignCreateBody struct {
	AdvertiserId  int64   `json:"advertiser_id"`
	CampaignName  string  `json:"campaign_name"`
	Budget        float64 `json:"budget,omitempty"`
	BudgetMode    string  `json:"budget_mode,omitempty"`
	MarketingGoal string  `json:"marketing_goal"`
}

type CampaignCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CampaignId int64 `json:"campaign_id"`
	} `json:"data"`
}

type CampaignUpdateReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         CampaignUpdateBody `json:"body"`
}

type CampaignUpdateBody struct {
	AdvertiserId int64   `json:"advertiser_id"`
	CampaignId   int64   `json:"campaign_id"`
	CampaignName string  `json:"campaign_name,omitempty"`
	Budget       float64 `json:"budget,omitempty"`
	BudgetMode   string  `json:"budget_mode,omitempty"`
}

type CampaignUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CampaignId int64 `json:"campaign_id"`
	} `json:"data"`
}

type BatchCampaignStatusUpdateReq struct {
	AccessToken  string                        `json:"access_token"`
	AdvertiserId int64                         `json:"advertiser_id"`
	Body         BatchCampaignStatusUpdateBody `json:"body"`
}

type BatchCampaignStatusUpdateBody struct {
	AdvertiserId int64   `json:"advertiser_id"`
	CampaignIds  []int64 `json:"campaign_ids"`
	OptStatus    string  `json:"opt_status"`
}

type BatchCampaignStatusUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CampaignIds []int64                     `json:"campaign_ids"`
		Errors      []CampaignStatusUpdateError `json:"errors,omitempty"`
	} `json:"data"`
}

type CampaignStatusUpdateError struct {
	CampaignId int64  `json:"campaign_id"`
	ErrorCode  int64  `json:"error_code"`
	ErrorMsg   string `json:"error_msg"`
}

// ===== Ad Types =====

type AdListGetReq struct {
	AccessToken      string             `json:"access_token"`
	AdvertiserId     int64              `json:"advertiser_id"`
	RequestAwemeInfo int64              `json:"request_aweme_info,omitempty"`
	Page             int64              `json:"page"`
	PageSize         int64              `json:"page_size"`
	Filtering        AdListGetFiltering `json:"filtering"`
}

type AdListGetFiltering struct {
	Ids               []int64 `json:"ids,omitempty"`
	AdName            string  `json:"ad_name,omitempty"`
	Status            string  `json:"status,omitempty"`
	CampaignScene     string  `json:"campaign_scene,omitempty"`
	MarketingScene    string  `json:"marketing_scene,omitempty"`
	PromotionWay      string  `json:"promotion_way,omitempty"`
	MarketingGoal     string  `json:"marketing_goal,omitempty"`
	CampaignId        int64   `json:"campaign_id,omitempty"`
	AdCreateStartDate string  `json:"ad_create_start_date,omitempty"`
	AdCreateEndDate   string  `json:"ad_create_end_date,omitempty"`
	AdModifyTime      string  `json:"ad_modify_time,omitempty"`
	AwemeId           int64   `json:"aweme_id,omitempty"`
	AutoManageFilter  string  `json:"auto_manage_filter,omitempty"`
}

type AdListGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    *AdListData `json:"data,omitempty"`
}

type AdListData struct {
	List     []interface{} `json:"list"`
	PageInfo PageInfo      `json:"page_info"`
	FailList []int64       `json:"fail_list,omitempty"`
}

type AdDetailGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AdId         int64  `json:"ad_id"`
}

type AdDetailGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type AdCreateReq struct {
	AccessToken  string       `json:"access_token"`
	AdvertiserId int64        `json:"advertiser_id"`
	Body         AdCreateBody `json:"body"`
}

type AdCreateBody struct {
	AdvertiserId    int64       `json:"advertiser_id"`
	Name            string      `json:"name"`
	CampaignId      int64       `json:"campaign_id,omitempty"`
	DeliverySetting interface{} `json:"delivery_setting,omitempty"`
	Audience        interface{} `json:"audience,omitempty"`
}

type AdCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdId int64 `json:"ad_id"`
	} `json:"data"`
}

type AdUpdateReq struct {
	AccessToken  string       `json:"access_token"`
	AdvertiserId int64        `json:"advertiser_id"`
	Body         AdUpdateBody `json:"body"`
}

type AdUpdateBody struct {
	AdvertiserId    int64       `json:"advertiser_id"`
	AdId            int64       `json:"ad_id"`
	Name            string      `json:"name,omitempty"`
	DeliverySetting interface{} `json:"delivery_setting,omitempty"`
	Audience        interface{} `json:"audience,omitempty"`
}

type AdUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdId int64 `json:"ad_id"`
	} `json:"data"`
}

type AdStatusUpdateReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         AdStatusUpdateBody `json:"body"`
}

type AdStatusUpdateBody struct {
	AdvertiserId int64   `json:"advertiser_id"`
	AdIds        []int64 `json:"ad_ids"`
	OptStatus    string  `json:"opt_status"`
}

type AdStatusUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdIds  []int64         `json:"ad_ids"`
		Errors []AdStatusError `json:"errors,omitempty"`
	} `json:"data"`
}

type AdStatusError struct {
	AdId      int64  `json:"ad_id"`
	ErrorCode int64  `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

type AdBudgetUpdateReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         AdBudgetUpdateBody `json:"body"`
}

type AdBudgetUpdateBody struct {
	AdvertiserId int64          `json:"advertiser_id"`
	Data         []AdBudgetItem `json:"data"`
}

type AdBudgetItem struct {
	AdId   int64   `json:"ad_id"`
	Budget float64 `json:"budget"`
}

type AdBudgetUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdIds  []int64         `json:"ad_ids"`
		Errors []AdStatusError `json:"errors,omitempty"`
	} `json:"data"`
}

type AdBidUpdateReq struct {
	AccessToken  string          `json:"access_token"`
	AdvertiserId int64           `json:"advertiser_id"`
	Body         AdBidUpdateBody `json:"body"`
}

type AdBidUpdateBody struct {
	AdvertiserId int64       `json:"advertiser_id"`
	Data         []AdBidItem `json:"data"`
}

type AdBidItem struct {
	AdId int64   `json:"ad_id"`
	Bid  float64 `json:"bid"`
}

type AdBidUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdIds  []int64         `json:"ad_ids"`
		Errors []AdStatusError `json:"errors,omitempty"`
	} `json:"data"`
}

// ===== Creative Types =====

type CreativeGetReq struct {
	AccessToken  string                   `json:"access_token"`
	AdvertiserId int64                    `json:"advertiser_id"`
	Page         int64                    `json:"page"`
	PageSize     int64                    `json:"page_size"`
	Filtering    *CreativeGetReqFiltering `json:"filtering,omitempty"`
}

type CreativeGetReqFiltering struct {
	MarketingGoal         string  `json:"marketing_goal,omitempty"`
	Status                string  `json:"status,omitempty"`
	AdIds                 []int64 `json:"ad_ids,omitempty"`
	CreativeId            int64   `json:"creative_id,omitempty"`
	CreativeMaterialMode  string  `json:"creative_material_mode,omitempty"`
	CampaignId            int64   `json:"campaign_id,omitempty"`
	CreativeCreateEndDate string  `json:"creative_create_end_date,omitempty"`
	CreativeModifyTime    string  `json:"creative_modify_time,omitempty"`
}

type CreativeGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type CreativeRejectReasonReq struct {
	AccessToken  string  `json:"access_token"`
	AdvertiserId int64   `json:"advertiser_id"`
	CreativeIds  []int64 `json:"creative_ids"`
}

type CreativeRejectReasonRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

// ===== File / Material Types =====

type FileImageAdReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         FileImageAdReqBody `json:"body"`
}

type FileImageAdReqBody struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	UploadType     string `json:"upload_type"`
	ImageSignature string `json:"image_signature,omitempty"`
	ImageFile      []byte `json:"-"`
	ImageUrl       string `json:"image_url,omitempty"`
	Filename       string `json:"filename,omitempty"`
}

type FileImageAdRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ImageId   string `json:"image_id"`
		ImageUrl  string `json:"image_url"`
		Size      int64  `json:"size"`
		Width     int64  `json:"width"`
		Height    int64  `json:"height"`
		Format    string `json:"format"`
		Signature string `json:"signature"`
	} `json:"data"`
}

type FileVideoAdReq struct {
	AccessToken  string             `json:"access_token"`
	AdvertiserId int64              `json:"advertiser_id"`
	Body         FileVideoAdReqBody `json:"body"`
}

type FileVideoAdReqBody struct {
	AdvertiserId   int64  `json:"advertiser_id"`
	VideoSignature string `json:"video_signature,omitempty"`
	VideoFile      []byte `json:"-"`
	VideoUrl       string `json:"video_url,omitempty"`
	Filename       string `json:"filename,omitempty"`
}

type FileVideoAdRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		VideoId  string  `json:"video_id"`
		Size     int64   `json:"size"`
		Width    int64   `json:"width"`
		Height   int64   `json:"height"`
		Duration float64 `json:"duration"`
	} `json:"data"`
}

type FileImageGetReq struct {
	AccessToken  string                    `json:"access_token"`
	AdvertiserId int64                     `json:"advertiser_id"`
	Page         int64                     `json:"page"`
	PageSize     int64                     `json:"page_size"`
	Filtering    *FileImageGetReqFiltering `json:"filtering,omitempty"`
}

type FileImageGetReqFiltering struct {
	ImageIds    []string  `json:"image_ids,omitempty"`
	MaterialIds []int64   `json:"material_ids,omitempty"`
	Signatures  []string  `json:"signatures,omitempty"`
	Width       int64     `json:"width,omitempty"`
	Height      int64     `json:"height,omitempty"`
	Ratio       []float64 `json:"ratio,omitempty"`
	StartTime   string    `json:"start_time,omitempty"`
	EndTime     string    `json:"end_time,omitempty"`
}

type FileImageGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type FileVideoGetReq struct {
	AccessToken  string                    `json:"access_token"`
	AdvertiserId int64                     `json:"advertiser_id"`
	Page         int64                     `json:"page"`
	PageSize     int64                     `json:"page_size"`
	Filtering    *FileVideoGetReqFiltering `json:"filtering,omitempty"`
}

type FileVideoGetReqFiltering struct {
	Width       int64     `json:"width,omitempty"`
	Height      int64     `json:"height,omitempty"`
	Ratio       []float64 `json:"ratio,omitempty"`
	VideoIds    []string  `json:"video_ids,omitempty"`
	MaterialIds []int64   `json:"material_ids,omitempty"`
	Signatures  []string  `json:"signatures,omitempty"`
	StartTime   string    `json:"start_time,omitempty"`
	EndTime     string    `json:"end_time,omitempty"`
}

type FileVideoGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

// ===== Report Types =====

type AdvertiserReportReq struct {
	AccessToken  string                     `json:"access_token"`
	AdvertiserId int64                      `json:"advertiser_id"`
	StartDate    string                     `json:"start_date"`
	EndDate      string                     `json:"end_date"`
	Fields       []string                   `json:"fields,omitempty"`
	Filtering    *AdvertiserReportFiltering `json:"filtering,omitempty"`
}

type AdvertiserReportFiltering struct {
	MarketingGoal string `json:"marketing_goal,omitempty"`
}

type AdvertiserReportRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ReportAdGetReq struct {
	AccessToken  string                `json:"access_token"`
	AdvertiserId int64                 `json:"advertiser_id"`
	StartDate    string                `json:"start_date"`
	EndDate      string                `json:"end_date"`
	Page         int64                 `json:"page"`
	PageSize     int64                 `json:"page_size"`
	Fields       []string              `json:"fields,omitempty"`
	Filtering    *ReportAdGetFiltering `json:"filtering,omitempty"`
	OrderField   string                `json:"order_field,omitempty"`
	OrderType    string                `json:"order_type,omitempty"`
}

type ReportAdGetFiltering struct {
	AdIds         []int64 `json:"ad_ids,omitempty"`
	MarketingGoal string  `json:"marketing_goal,omitempty"`
}

type ReportAdGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type ReportCreativeGetReq struct {
	AccessToken  string                      `json:"access_token"`
	AdvertiserId int64                       `json:"advertiser_id"`
	StartDate    string                      `json:"start_date"`
	EndDate      string                      `json:"end_date"`
	Page         int64                       `json:"page"`
	PageSize     int64                       `json:"page_size"`
	Fields       []string                    `json:"fields,omitempty"`
	Filtering    *ReportCreativeGetFiltering `json:"filtering,omitempty"`
}

type ReportCreativeGetFiltering struct {
	CreativeIds   []int64 `json:"creative_ids,omitempty"`
	MarketingGoal string  `json:"marketing_goal,omitempty"`
}

type ReportCreativeGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type ReportMaterialGetReq struct {
	AccessToken     string                      `json:"-"`
	AdvertiserId    int64                       `json:"advertiser_id"`
	StartDate       string                      `json:"start_date"`
	EndDate         string                      `json:"end_date"`
	Page            int64                       `json:"page"`
	PageSize        int64                       `json:"page_size"`
	Fields          []string                    `json:"fields,omitempty"`
	Filtering       *ReportMaterialGetFiltering `json:"filtering,omitempty"`
	TimeGranularity string                      `json:"time_granularity,omitempty"`
	OrderField      string                      `json:"order_field,omitempty"`
	OrderType       string                      `json:"order_type,omitempty"`
}

type ReportMaterialGetFiltering struct {
	AdIds         []int64  `json:"ad_ids,omitempty"`
	MaterialId    []int64  `json:"material_id,omitempty"`
	MarketingGoal string   `json:"marketing_goal,omitempty"`
	MaterialType  string   `json:"material_type,omitempty"`
	AnalysisType  []string `json:"analysis_type,omitempty"`
}

type ReportMaterialGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

// ===== Tools Types =====

type ToolsIndustryGetReq struct {
	AccessToken string `json:"access_token"`
	Level       int64  `json:"level,omitempty"`
	Type        string `json:"type,omitempty"`
}

type ToolsIndustryGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsInterestActionInterestCategoryReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
}

type ToolsInterestActionInterestCategoryRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsInterestActionInterestKeywordReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	QueryWords   string `json:"query_words"`
}

type ToolsInterestActionInterestKeywordRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsInterestActionActionCategoryReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	ActionScene  []string `json:"action_scene"`
	ActionDays   int64    `json:"action_days"`
}

type ToolsInterestActionActionCategoryRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsInterestActionActionKeywordReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	QueryWords   string   `json:"query_words"`
	ActionScene  []string `json:"action_scene"`
	ActionDays   int64    `json:"action_days"`
}

type ToolsInterestActionActionKeywordRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsAwemeMultiLevelCategoryGetReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	Behaviors    []string `json:"behaviors,omitempty"`
}

type ToolsAwemeMultiLevelCategoryGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsAwemeAuthorInfoGetReq struct {
	AccessToken  string  `json:"access_token"`
	AdvertiserId int64   `json:"advertiser_id"`
	AwemeIds     []int64 `json:"aweme_ids"`
}

type ToolsAwemeAuthorInfoGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type ToolsCreativeWordSelectReq struct {
	AccessToken     string   `json:"access_token"`
	AdvertiserId    int64    `json:"advertiser_id"`
	CreativeWordIds []string `json:"creative_word_ids,omitempty"`
}

type ToolsCreativeWordSelectRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type DmpAudiencesGetReq struct {
	AccessToken         string `json:"access_token"`
	AdvertiserId        int64  `json:"advertiser_id"`
	RetargetingTagsType int64  `json:"retargeting_tags_type,omitempty"`
	Offset              int64  `json:"offset"`
	Limit               int64  `json:"limit"`
}

type DmpAudiencesGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List       []interface{} `json:"retargeting_tags"`
		TotalCount int64         `json:"total_count"`
	} `json:"data"`
}

// ===== Finance Types =====

type WalletGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
}

type WalletGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TotalBalance      float64 `json:"total_balance"`
		GrantBalance      float64 `json:"grant_balance"`
		CashBalance       float64 `json:"cash_balance"`
		ValidGrantBalance float64 `json:"valid_grant_balance"`
		ValidCashBalance  float64 `json:"valid_cash_balance"`
	} `json:"data"`
}

type BalanceGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
}

type BalanceGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Balance      float64 `json:"balance"`
		ValidBalance float64 `json:"valid_balance"`
		Cash         float64 `json:"cash"`
		Grant        float64 `json:"grant"`
	} `json:"data"`
}

type DetailGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
}

type DetailGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type FundTransferSeqCreateReq struct {
	AccessToken        string `json:"access_token"`
	AdvertiserId       int64  `json:"advertiser_id"`
	TargetAdvertiserId int64  `json:"target_advertiser_id"`
	TransferType       string `json:"transfer_type"`
}

type FundTransferSeqCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TransferSeq string `json:"transfer_seq"`
	} `json:"data"`
}

type FundTransferSeqCommitReq struct {
	AccessToken  string  `json:"access_token"`
	AdvertiserId int64   `json:"advertiser_id"`
	TransferSeq  string  `json:"transfer_seq"`
	Amount       float64 `json:"amount"`
}

type FundTransferSeqCommitRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TransferId string `json:"transfer_id"`
	} `json:"data"`
}

type RefundTransferSeqCreateReq struct {
	AccessToken        string `json:"access_token"`
	AdvertiserId       int64  `json:"advertiser_id"`
	TargetAdvertiserId int64  `json:"target_advertiser_id"`
	TransferType       string `json:"transfer_type"`
}

type RefundTransferSeqCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TransferSeq string `json:"transfer_seq"`
	} `json:"data"`
}

type RefundTransferSeqCommitReq struct {
	AccessToken  string  `json:"access_token"`
	AdvertiserId int64   `json:"advertiser_id"`
	TransferSeq  string  `json:"transfer_seq"`
	Amount       float64 `json:"amount"`
}

type RefundTransferSeqCommitRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TransferId string `json:"transfer_id"`
	} `json:"data"`
}

// ===== Live Types =====

type LiveGetReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	AwemeId      int64    `json:"aweme_id"`
	StartTime    string   `json:"start_time"`
	EndTime      string   `json:"end_time"`
	Fields       []string `json:"fields,omitempty"`
}

type LiveGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LiveRoomGetReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	AwemeId      int64    `json:"aweme_id"`
	DateTime     string   `json:"date_time"`
	RoomStatus   string   `json:"room_status,omitempty"`
	AdStatus     string   `json:"ad_status,omitempty"`
	Fields       []string `json:"fields,omitempty"`
	Page         int64    `json:"page"`
	PageSize     int64    `json:"page_size"`
}

type LiveRoomGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Rooms    []map[string]json.RawMessage `json:"room_list"`
		PageInfo PageInfo                     `json:"page_info"`
	} `json:"data"`
}

type LiveRoomDetailGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	RoomId       int64  `json:"room_id"`
}

type LiveRoomDetailGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LiveRoomFlowPerformanceGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	RoomId       int64  `json:"room_id"`
}

type LiveRoomFlowPerformanceGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LiveRoomUserGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	RoomId       int64  `json:"room_id"`
}

type LiveRoomUserGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LiveRoomProductListGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	RoomId       int64  `json:"room_id"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
}

type LiveRoomProductListGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

// ===== Aweme (随心推) Types =====

type AwemeOrderGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
	StartTime    string `json:"start_time,omitempty"`
	EndTime      string `json:"end_time,omitempty"`
}

type AwemeOrderGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type AwemeOrderDetailGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	OrderId      int64  `json:"order_id"`
}

type AwemeOrderDetailGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type AwemeOrderCreateReq struct {
	AccessToken  string      `json:"access_token"`
	AdvertiserId int64       `json:"advertiser_id"`
	Body         interface{} `json:"body"`
}

type AwemeOrderCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OrderId int64 `json:"order_id"`
	} `json:"data"`
}

type AwemeOrderTerminateReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	OrderId      int64  `json:"order_id"`
}

type AwemeOrderTerminateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		OrderId int64 `json:"order_id"`
		Result  bool  `json:"result"`
	} `json:"data"`
}

type AwemeVideoGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AwemeId      int64  `json:"aweme_id"`
	Page         int64  `json:"page"`
	PageSize     int64  `json:"page_size"`
}

type AwemeVideoGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []interface{} `json:"list"`
		PageInfo PageInfo      `json:"page_info"`
	} `json:"data"`
}

type AwemeOrderBudgetAddReq struct {
	AccessToken  string  `json:"access_token"`
	AdvertiserId int64   `json:"advertiser_id"`
	OrderId      int64   `json:"order_id"`
	AddBudget    float64 `json:"add_budget"`
}

type AwemeOrderBudgetAddRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type AwemeSuggestBidReq struct {
	AccessToken    string `json:"access_token"`
	AdvertiserId   int64  `json:"advertiser_id"`
	ExternalAction string `json:"external_action"`
	ProductId      int64  `json:"product_id,omitempty"`
}

type AwemeSuggestBidRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		SuggestBid float64 `json:"suggest_bid"`
	} `json:"data"`
}

type AwemeEstimateProfitReq struct {
	AccessToken   string  `json:"access_token"`
	AdvertiserId  int64   `json:"advertiser_id"`
	Budget        float64 `json:"budget"`
	DeliveryRange string  `json:"delivery_range"`
}

type AwemeEstimateProfitRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type AwemeOrderQuotaGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
}

type AwemeOrderQuotaGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ===== Keywords Types =====

type KeywordPackageGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AdId         int64  `json:"ad_id"`
}

type KeywordPackageGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type RecommendKeywordsGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AdId         int64  `json:"ad_id"`
}

type RecommendKeywordsGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type KeywordCheckReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	Keywords     []string `json:"keywords"`
}

type KeywordCheckRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type PrivatewordsGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AdId         int64  `json:"ad_id"`
}

type PrivatewordsGetRes struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type KeywordsUpdateReq struct {
	AccessToken  string              `json:"access_token"`
	AdvertiserId int64               `json:"advertiser_id"`
	AdId         int64               `json:"ad_id"`
	Keywords     []KeywordUpdateInfo `json:"keywords"`
}

type KeywordUpdateInfo struct {
	Word      string `json:"word"`
	MatchType string `json:"match_type"`
}

type KeywordsUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type KeywordsGetReq struct {
	AccessToken  string `json:"access_token"`
	AdvertiserId int64  `json:"advertiser_id"`
	AdId         int64  `json:"ad_id"`
}

type KeywordsGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

type PrivatewordsUpdateReq struct {
	AccessToken  string   `json:"access_token"`
	AdvertiserId int64    `json:"advertiser_id"`
	AdId         int64    `json:"ad_id"`
	PhraseWords  []string `json:"phrase_words,omitempty"`
	PreciseWords []string `json:"precise_words,omitempty"`
}

type PrivatewordsUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// ===== UniPromotion (全域推广) Types =====

type UniPromotion struct {
	AdId          uint64     `json:"ad_id"`
	AdName        string     `json:"ad_name"`
	AdvertiserId  uint64     `json:"advertiser_id"`
	MarketingGoal string     `json:"marketing_goal"`
	AdlabScene    AdlabScene `json:"adlab_scene,omitempty"`
	Status        string     `json:"status"`
	OptStatus     string     `json:"opt_status"`
	Budget        float64    `json:"budget"`
	Roi2Goal      float64    `json:"roi2_goal"`
	DeepBidType   string     `json:"deep_bid_type"`
	AwemeId       uint64     `json:"aweme_id"`
	AwemeName     string     `json:"aweme_name"`
	ProductId     uint64     `json:"product_id"`
	ProductName   string     `json:"product_name"`
	ProductInfo   []struct {
		ProductID   uint64 `json:"product_id"`
		ProductName string `json:"product_name"`
	} `json:"product_info"`
	StatsInfo struct {
		StatCost           float64 `json:"stat_cost"`
		CostForOverallROI2 float64 `json:"stat_cost_for_overall_roi2"`
		ROI2               float64 `json:"total_prepay_and_pay_order_roi2"`
		PayOrderGMV        float64 `json:"total_pay_order_gmv_for_roi2"`
		PayOrderCount      float64 `json:"total_pay_order_count_for_roi2"`
		PayOrderCost       float64 `json:"total_cost_per_pay_order_for_roi2"`
	} `json:"stats_info"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	CreateTime    string  `json:"create_time"`
	ModifyTime    string  `json:"modify_time"`
	SmartBidType  string  `json:"smart_bid_type,omitempty"`
	StatCost      float64 `json:"stat_cost"`
	PayOrderCount float64 `json:"total_pay_order_count_for_roi2"`
	PayOrderGMV   float64 `json:"total_pay_order_gmv_for_roi2"`
	PayOrderROI   float64 `json:"total_prepay_and_pay_order_roi2"`
}

type UniPromotionListReq struct {
	AccessToken   string                     `json:"-"`
	AdvertiserId  uint64                     `json:"advertiser_id"`
	StartTime     string                     `json:"start_time,omitempty"`
	EndTime       string                     `json:"end_time,omitempty"`
	MarketingGoal string                     `json:"marketing_goal,omitempty"`
	Fields        []string                   `json:"fields,omitempty"`
	OrderField    string                     `json:"order_field,omitempty"`
	OrderType     string                     `json:"order_type,omitempty"`
	Page          int                        `json:"page,omitempty"`
	PageSize      int                        `json:"page_size,omitempty"`
	AdlabScene    string                     `json:"adlab_scene,omitempty"`
	Filtering     *UniPromotionListFiltering `json:"filtering,omitempty"`
}

type UniPromotionListFiltering struct {
	Status        string   `json:"status,omitempty"`
	MarketingGoal string   `json:"marketing_goal,omitempty"`
	AdIds         []uint64 `json:"ad_ids,omitempty"`
	AdName        string   `json:"ad_name,omitempty"`
	SmartBidType  string   `json:"smart_bid_type,omitempty"`
}

type UniPromotionListRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []UniPromotion `json:"list"`
		PageInfo PageInfo       `json:"page_info"`
	} `json:"data"`
}

type UniPromotionDetailReq struct {
	AccessToken  string `json:"-"`
	AdvertiserId uint64 `json:"advertiser_id"`
	AdId         uint64 `json:"ad_id"`
}

type UniPromotionDetailRes struct {
	Code      int64               `json:"code"`
	Message   string              `json:"message"`
	RequestID string              `json:"request_id,omitempty"`
	Data      *UniPromotionDetail `json:"data"`
}

type UniPromotionDetail struct {
	AdId                          uint64                                 `json:"ad_id"`
	AdvertiserId                  uint64                                 `json:"advertiser_id,omitempty"`
	AdName                        string                                 `json:"ad_name"`
	MarketingGoal                 string                                 `json:"marketing_goal,omitempty"`
	Status                        string                                 `json:"status,omitempty"`
	OptStatus                     string                                 `json:"opt_status,omitempty"`
	Budget                        float64                                `json:"budget"`
	Roi2Goal                      float64                                `json:"roi2_goal"`
	SmartBidType                  string                                 `json:"smart_bid_type,omitempty"`
	DeepBidType                   string                                 `json:"deep_bid_type,omitempty"`
	CreateTime                    string                                 `json:"create_time,omitempty"`
	ModifyTime                    string                                 `json:"modify_time,omitempty"`
	AwemeId                       uint64                                 `json:"aweme_id,omitempty"`
	AwemeName                     string                                 `json:"aweme_name,omitempty"`
	StartTime                     string                                 `json:"start_time,omitempty"`
	EndTime                       string                                 `json:"end_time,omitempty"`
	StatCost                      float64                                `json:"stat_cost,omitempty"`
	PayOrderCount                 float64                                `json:"total_pay_order_count_for_roi2,omitempty"`
	PayOrderGMV                   float64                                `json:"total_pay_order_gmv_for_roi2,omitempty"`
	PayOrderROI                   float64                                `json:"total_prepay_and_pay_order_roi2,omitempty"`
	Name                          string                                 `json:"name,omitempty"`
	AdlabScene                    AdlabScene                             `json:"adlab_scene,omitempty"`
	DeliverySetting               *UniPromotionDeliverySetting           `json:"delivery_setting,omitempty"`
	ProductInfos                  []UniPromotionProductInfo              `json:"product_infos,omitempty"`
	CreativeSetting               map[string]any                         `json:"creative_setting,omitempty"`
	RoomInfo                      []UniPromotionRoomInfo                 `json:"room_info,omitempty"`
	ProgrammaticCreativeMediaList *UniPromotionProgrammaticCreativeMedia `json:"programmatic_creative_media_list,omitempty"`
	MultiProductCreativeList      []UniPromotionMultiProductCreative     `json:"multi_product_creative_list,omitempty"`
	AwemeInfos                    []UniPromotionAwemeInfo                `json:"aweme_infos,omitempty"`
	LabAdType                     string                                 `json:"lab_ad_type,omitempty"`
	AutoExtendEnabled             int                                    `json:"auto_extend_enabled,omitempty"`
}

type UniPromotionProductInfo struct {
	ProductID   uint64 `json:"product_id,omitempty"`
	ChannelType string `json:"channel_type,omitempty"`
	ChannelID   uint64 `json:"channel_id,omitempty"`
}

// AdlabScene accepts both official response forms: the list API returns
// OVERALL_PROJECT/UNI_PROJECT while the detail API documents numeric 1/0.
type AdlabScene string

const (
	AdlabSceneUni     AdlabScene = "UNI_PROJECT"
	AdlabSceneOverall AdlabScene = "OVERALL_PROJECT"
)

func (s *AdlabScene) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" {
		*s = ""
		return nil
	}
	if value[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*s = AdlabScene(strings.TrimSpace(text))
		return nil
	}
	var number int64
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("decode adlab_scene: %w", err)
	}
	switch number {
	case 1:
		*s = AdlabSceneOverall
	case 0:
		*s = AdlabSceneUni
	default:
		*s = AdlabScene(strconv.FormatInt(number, 10))
	}
	return nil
}

type UniPromotionDeliverySetting struct {
	ExternalAction     string  `json:"external_action,omitempty"`
	SmartBidType       string  `json:"smart_bid_type,omitempty"`
	DeepExternalAction string  `json:"deep_external_action,omitempty"`
	DeepBidType        string  `json:"deep_bid_type,omitempty"`
	PricingType        string  `json:"pricing_type,omitempty"`
	Roi2Goal           float64 `json:"roi2_goal,omitempty"`
	BudgetMode         string  `json:"budget_mode,omitempty"`
	Budget             float64 `json:"budget,omitempty"`
	LiveScheduleType   string  `json:"live_schedule_type,omitempty"`
	StartTime          string  `json:"start_time,omitempty"`
	EndTime            string  `json:"end_time,omitempty"`
	QcpxMode           string  `json:"qcpx_mode,omitempty"`
}

type UniPromotionProgrammaticCreativeMedia struct {
	VideoMaterial      []UniPromotionCreativeVideoMaterial `json:"video_material,omitempty"`
	TitleMaterial      []UniPromotionCreativeTitleMaterial `json:"title_material,omitempty"`
	BlockVideoMaterial []UniPromotionCreativeBlockMaterial `json:"block_video_material,omitempty"`
}

type UniPromotionMultiProductCreative struct {
	ProductID        uint64                                   `json:"product_id,omitempty"`
	AwemeUID         uint64                                   `json:"aweme_uid,omitempty"`
	CreativeType     string                                   `json:"creative_type,omitempty"`
	HideInAweme      bool                                     `json:"hide_in_aweme,omitempty"`
	VideoMaterial    []UniPromotionCreativeVideoMaterial      `json:"video_material,omitempty"`
	ImageMaterial    []UniPromotionCreativeImageMaterial      `json:"image_material,omitempty"`
	TitleMaterial    []UniPromotionMultiCreativeTitleMaterial `json:"title_material,omitempty"`
	CreativeCard     *UniPromotionCreativeCard                `json:"creative_card,omitempty"`
	BlockMaterial    []UniPromotionCreativeBlockMaterial      `json:"block_material,omitempty"`
	CarouselMaterial []UniPromotionCreativeCarouselMaterial   `json:"carousel_material,omitempty"`
}

type UniPromotionCreativeVideoMaterial struct {
	ImageMode    string `json:"image_mode,omitempty"`
	VideoID      string `json:"video_id,omitempty"`
	VideoCoverID string `json:"video_cover_id,omitempty"`
	AwemeItemID  uint64 `json:"aweme_item_id,omitempty"`
	StarTraffic  int64  `json:"star_traffic,omitempty"`
}

type UniPromotionCreativeTitleMaterial struct {
	Title string `json:"title,omitempty"`
}

type UniPromotionCreativeBlockMaterial struct {
	AwemeItemID uint64 `json:"aweme_item_id,omitempty"`
}

type UniPromotionCreativeImageMaterial struct {
	ImageMode string   `json:"image_mode,omitempty"`
	ImageIDs  []string `json:"image_ids,omitempty"`
}

type UniPromotionMultiCreativeTitleMaterial struct {
	Title     string `json:"title,omitempty"`
	TitleType string `json:"title_type,omitempty"`
}

type UniPromotionCreativeCard struct {
	ProductImageID             string   `json:"product_image_id,omitempty"`
	PromotionCardTitle         string   `json:"promotion_card_title,omitempty"`
	PromotionCardSellingPoints []string `json:"promotion_card_selling_points,omitempty"`
	PromotionCardImageID       string   `json:"promotion_card_image_id,omitempty"`
	PromotionCardActionButton  string   `json:"promotion_card_action_button,omitempty"`
}

type UniPromotionCreativeCarouselMaterial struct {
	AwemeCarouselID uint64 `json:"aweme_carousel_id,omitempty"`
	CarouselID      uint64 `json:"carousel_id,omitempty"`
}

type UniPromotionRoomInfo struct {
	AnchorID     uint64 `json:"anchor_id,omitempty"`
	AnchorName   string `json:"anchor_name,omitempty"`
	AnchorAvatar string `json:"anchor_avatar,omitempty"`
}

type UniPromotionAwemeInfo struct {
	AwemeUID  uint64 `json:"aweme_uid,omitempty"`
	AwemeName string `json:"aweme_name,omitempty"`
	UniqueID  string `json:"unique_id,omitempty"`
}

type UniPromotionCreateReq struct {
	AccessToken   string                 `json:"-"`
	AdvertiserId  uint64                 `json:"advertiser_id"`
	MarketingGoal string                 `json:"marketing_goal"`
	AwemeId       uint64                 `json:"aweme_id"`
	ProductId     uint64                 `json:"product_id,omitempty"`
	Budget        float64                `json:"budget"`
	Roi2Goal      float64                `json:"roi2_goal,omitempty"`
	DeepBidType   string                 `json:"deep_bid_type,omitempty"`
	StartTime     string                 `json:"start_time,omitempty"`
	EndTime       string                 `json:"end_time,omitempty"`
	VideoMaterial []UniPromotionMaterial `json:"video_material,omitempty"`
}

type UniPromotionMaterial struct {
	VideoId      string `json:"video_id,omitempty"`
	VideoCoverId string `json:"video_cover_id,omitempty"`
	AwemeItemId  uint64 `json:"aweme_item_id,omitempty"`
	ImageId      string `json:"image_id,omitempty"`
	ImageMode    string `json:"image_mode,omitempty"`
}

type UniPromotionCreateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdId uint64 `json:"ad_id"`
	} `json:"data"`
}

type UniPromotionUpdateReq struct {
	AccessToken   string                 `json:"-"`
	AdvertiserId  uint64                 `json:"advertiser_id"`
	AdId          uint64                 `json:"ad_id"`
	Budget        float64                `json:"budget,omitempty"`
	Roi2Goal      float64                `json:"roi2_goal,omitempty"`
	StartTime     string                 `json:"start_time,omitempty"`
	EndTime       string                 `json:"end_time,omitempty"`
	VideoMaterial []UniPromotionMaterial `json:"video_material,omitempty"`
}

type UniPromotionUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AdId uint64 `json:"ad_id"`
	} `json:"data"`
}

type UniPromotionStatusUpdateReq struct {
	AccessToken  string   `json:"-"`
	AdvertiserId uint64   `json:"advertiser_id"`
	AdIds        []uint64 `json:"ad_ids"`
	OptStatus    string   `json:"opt_status"`
}

type UniPromotionStatusUpdateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		Results []UniPromotionStatusResult `json:"results,omitempty"`
		AdIds   []uint64                   `json:"ad_ids,omitempty"`
		Errors  []UniPromotionOpError      `json:"errors,omitempty"`
	} `json:"data"`
}

type UniPromotionStatusResult struct {
	AdId         uint64 `json:"ad_id"`
	Success      bool   `json:"success"`
	Flag         bool   `json:"flag"`
	ErrorCode    int64  `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type UniPromotionOpError struct {
	AdId         uint64 `json:"ad_id"`
	ErrorCode    int64  `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type UniPromotionBudgetUpdateReq struct {
	AccessToken  string                         `json:"-"`
	AdvertiserId uint64                         `json:"advertiser_id"`
	Data         []UniPromotionBudgetUpdateItem `json:"update_budget_infos"`
}

type UniPromotionBudgetUpdateItem struct {
	AdId   uint64  `json:"ad_id"`
	Budget float64 `json:"budget"`
}

type UniPromotionBudgetUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Results []UniPromotionBudgetUpdateResult `json:"results"`
	} `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}

type UniPromotionBudgetUpdateResult struct {
	AdId         uint64 `json:"ad_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type UniPromotionOpResult struct {
	AdId         uint64 `json:"ad_id"`
	Success      bool   `json:"success"`
	ErrorCode    int64  `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type UniPromotionRoiGoalUpdateReq struct {
	AccessToken  string                          `json:"-"`
	AdvertiserId uint64                          `json:"advertiser_id"`
	Data         []UniPromotionRoiGoalUpdateItem `json:"data"`
}

type UniPromotionRoiGoalUpdateItem struct {
	AdId     uint64  `json:"ad_id"`
	Roi2Goal float64 `json:"roi2_goal"`
}

type UniPromotionRoiGoalUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Results []UniPromotionOpResult `json:"results"`
	} `json:"data"`
}

type UniPromotionScheduleUpdateReq struct {
	AccessToken  string                           `json:"-"`
	AdvertiserId uint64                           `json:"advertiser_id"`
	Data         []UniPromotionScheduleUpdateItem `json:"data"`
}

type UniPromotionScheduleUpdateItem struct {
	AdId         uint64 `json:"ad_id"`
	ScheduleType string `json:"schedule_type,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	EndTime      string `json:"end_time,omitempty"`
}

type UniPromotionScheduleUpdateRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Results []UniPromotionOpResult `json:"results"`
	} `json:"data"`
}

type UniPromotionMaterialGetReq struct {
	AccessToken        string   `json:"-"`
	AdvertiserId       uint64   `json:"advertiser_id"`
	AdId               uint64   `json:"ad_id"`
	MaterialType       string   `json:"material_type,omitempty"`
	VideoType          string   `json:"video_type,omitempty"`
	Status             string   `json:"material_status,omitempty"`
	MaterialSelectType string   `json:"material_select_type,omitempty"`
	AnalysisType       []string `json:"analysis_type,omitempty"`
	Page               int      `json:"page,omitempty"`
	PageSize           int      `json:"page_size,omitempty"`
	Fields             []string `json:"fields,omitempty"`
	OrderField         string   `json:"order_field,omitempty"`
	OrderType          string   `json:"order_type,omitempty"`
	StartDate          string   `json:"start_date,omitempty"`
	EndDate            string   `json:"end_date,omitempty"`
}

type UniPromotionMaterialGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Materials []UniPromotionMaterialInfo `json:"ad_material_infos"`
		PageInfo  PageInfo                   `json:"page_info"`
	} `json:"data"`
}

type UniPromotionMaterialInfo struct {
	MaterialId                     uint64   `json:"material_id"`
	VideoId                        string   `json:"video_id,omitempty"`
	VideoCoverId                   string   `json:"video_cover_id,omitempty"`
	AwemeItemId                    uint64   `json:"aweme_item_id,omitempty"`
	ImageId                        string   `json:"image_id,omitempty"`
	ImageMode                      string   `json:"image_mode,omitempty"`
	MaterialName                   string   `json:"material_name,omitempty"`
	MaterialType                   string   `json:"material_type,omitempty"`
	MaterialStatus                 string   `json:"material_status,omitempty"`
	AuditStatus                    string   `json:"audit_status,omitempty"`
	IsDeleted                      bool     `json:"is_delete,omitempty"`
	DeliveryNotReason              []string `json:"delivery_not_reason,omitempty"`
	CoverImage                     string   `json:"cover_image,omitempty"`
	ProductId                      uint64   `json:"product_id,omitempty"`
	ProductName                    string   `json:"product_name,omitempty"`
	StatCost                       float64  `json:"stat_cost,omitempty"`
	ShowCnt                        int64    `json:"show_cnt,omitempty"`
	ClickCnt                       int64    `json:"click_cnt,omitempty"`
	ConvertCnt                     int64    `json:"convert_cnt,omitempty"`
	Ctr                            float64  `json:"ctr,omitempty"`
	ConversionRate                 float64  `json:"conversion_rate,omitempty"`
	PayOrderAmount                 float64  `json:"pay_order_amount,omitempty"`
	Gpm                            float64  `json:"gpm,omitempty"`
	Roi2                           float64  `json:"roi2,omitempty"`
	LiveShowCount                  int64    `json:"live_show_count,omitempty"`
	LiveWatchCount                 int64    `json:"live_watch_count,omitempty"`
	LiveCtr                        float64  `json:"live_ctr,omitempty"`
	LiveConversionRate             float64  `json:"live_conversion_rate,omitempty"`
	LiveShowCountExcludeVideo      int64    `json:"live_show_count_exclude_video,omitempty"`
	LiveWatchCountExcludeVideo     int64    `json:"live_watch_count_exclude_video,omitempty"`
	LiveCtrExcludeVideo            float64  `json:"live_ctr_exclude_video,omitempty"`
	LiveConversionRateExcludeVideo float64  `json:"live_conversion_rate_exclude_video,omitempty"`
}

type UniPromotionMaterialAddReq struct {
	AccessToken         string                                   `json:"-"`
	AdvertiserId        uint64                                   `json:"advertiser_id"`
	AdId                uint64                                   `json:"ad_id"`
	VideoMaterial       []UniPromotionMaterial                   `json:"video_material"`
	ProductCreativeList []UniPromotionProductCreativeMaterialAdd `json:"-"`
}

type UniPromotionProductCreativeMaterialAdd struct {
	ProductID     uint64                                   `json:"product_id,omitempty"`
	AwemeUID      uint64                                   `json:"aweme_uid,omitempty"`
	VideoMaterial []UniPromotionMaterial                   `json:"video_material,omitempty"`
	ImageMaterial []UniPromotionImageMaterial              `json:"image_material,omitempty"`
	TitleMaterial []UniPromotionMultiCreativeTitleMaterial `json:"title_material,omitempty"`
}

type UniPromotionImageMaterial struct {
	ImageMode string   `json:"image_mode"`
	ImageIds  []string `json:"image_ids"`
}

type UniPromotionMaterialAddRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type UniPromotionMaterialDeleteReq struct {
	AccessToken  string   `json:"-"`
	AdvertiserId uint64   `json:"advertiser_id"`
	AdId         uint64   `json:"ad_id"`
	MaterialType string   `json:"material_type,omitempty"`
	MaterialIds  []uint64 `json:"material_ids,omitempty"`
	Titles       []string `json:"titles,omitempty"`
}

type UniPromotionMaterialDeleteRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type UniPromotionAuthorizedGetReq struct {
	AccessToken  string `json:"-"`
	AdvertiserId uint64 `json:"advertiser_id"`
	Page         int    `json:"page,omitempty"`
	PageSize     int    `json:"page_size,omitempty"`
}

type UniPromotionAuthorizedGetRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List     []UniPromotionAweme `json:"list"`
		PageInfo PageInfo            `json:"page_info"`
	} `json:"data"`
}

type UniPromotionAweme struct {
	AwemeId     uint64 `json:"aweme_id"`
	AwemeName   string `json:"aweme_name"`
	AwemeShowId string `json:"aweme_show_id"`
	AwemeAvatar string `json:"aweme_avatar"`
}

type UniPromotionAuthInitReq struct {
	AccessToken  string `json:"-"`
	AdvertiserId uint64 `json:"advertiser_id"`
	AwemeId      uint64 `json:"aweme_id"`
}

type UniPromotionAuthInitRes struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

type UniPromotionReportConfigReq struct {
	AccessToken  string   `json:"-"`
	AdvertiserId uint64   `json:"advertiser_id"`
	DataTopics   []string `json:"data_topics"`
	DataPeriod   string   `json:"data_period,omitempty"`
}

type UniPromotionReportConfigRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		CustomConfigDatas []UniPromotionReportTopicConfig `json:"custom_config_datas"`
	} `json:"data"`
}

type UniPromotionReportTopicConfig struct {
	DataTopic  string                        `json:"data_topic"`
	Dimensions []UniPromotionReportDimension `json:"dimensions"`
	Metrics    []UniPromotionReportMetric    `json:"metrics"`
	QueryLimit UniPromotionReportQueryLimit  `json:"query_limit"`
}

func (c UniPromotionReportTopicConfig) MaxDimensionLimit() int {
	limit, err := strconv.Atoi(c.QueryLimit.MaxDimensionLimit)
	if err != nil || limit <= 0 {
		return 0
	}
	return limit
}

type UniPromotionReportDimension struct {
	Field           string                          `json:"field"`
	Name            string                          `json:"name"`
	Description     string                          `json:"description"`
	Sortable        bool                            `json:"sortable"`
	Filterable      bool                            `json:"filterable"`
	FilterOnly      bool                            `json:"filter_only"`
	FilterConfig    *UniPromotionReportFilterConfig `json:"filter_config,omitempty"`
	ExclusionDims   []string                        `json:"exclusion_dims"`
	ExclusionMetric []string                        `json:"exclusion_metrics"`
	IsRequired      bool                            `json:"is_required"`
}

type UniPromotionReportFilterConfig struct {
	Type           int64                                `json:"type"`
	Operator       int64                                `json:"operator"`
	ValueLimit     int64                                `json:"value_limit"`
	FilterRequired bool                                 `json:"filter_required"`
	RangeValues    []UniPromotionReportFilterRangeValue `json:"range_values"`
}

type UniPromotionReportFilterRangeValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type UniPromotionReportMetric struct {
	Field           string   `json:"field"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Sortable        bool     `json:"sortable"`
	Unit            int64    `json:"unit"`
	ExclusionDims   []string `json:"exclusion_dims"`
	ExclusionMetric []string `json:"exclusion_metrics"`
}

type UniPromotionReportQueryLimit struct {
	MaxDimensionLimit string `json:"max_dimension_limit"`
	MaxDay            string `json:"max_day"`
	EarliestDay       string `json:"earliest_day"`
	MaxDayByDay       string `json:"max_day_by_day"`
	MaxDayByHour      string `json:"max_day_by_hour"`
}

type UniPromotionReportDataReq struct {
	AccessToken  string                     `json:"-"`
	AdvertiserId uint64                     `json:"advertiser_id"`
	DataTopic    string                     `json:"data_topic"`
	DataPeriod   string                     `json:"data_period,omitempty"`
	Dimensions   []string                   `json:"dimensions"`
	Metrics      []string                   `json:"metrics"`
	Filters      []UniPromotionReportFilter `json:"filters"`
	StartTime    string                     `json:"start_time"`
	EndTime      string                     `json:"end_time"`
	OrderBy      []UniPromotionReportOrder  `json:"order_by"`
	Page         int                        `json:"page,omitempty"`
	PageSize     int                        `json:"page_size,omitempty"`
}

type UniPromotionReportFilter struct {
	Field    string   `json:"field"`
	Operator int64    `json:"operator"`
	Values   []string `json:"values"`
}

type UniPromotionReportOrder struct {
	Type  int64  `json:"type"`
	Field string `json:"field"`
}

type UniPromotionReportDataRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		Rows     []UniPromotionReportRow `json:"rows"`
		PageInfo PageInfo                `json:"page_info"`
	} `json:"data"`
}

type UniPromotionReportRow struct {
	Dimensions map[string]any `json:"dimensions"`
	Metrics    map[string]any `json:"metrics"`
}

type UniPromotionControlTaskCreateReq struct {
	AccessToken    string   `json:"-"`
	AdvertiserId   uint64   `json:"advertiser_id"`
	AdId           uint64   `json:"ad_id"`
	Name           string   `json:"name,omitempty"`
	Scene          string   `json:"scene"`
	Budget         float64  `json:"budget"`
	Duration       float64  `json:"duration,omitempty"`
	MaterialIds    []uint64 `json:"material_ids,omitempty"`
	ExternalAction string   `json:"external_action,omitempty"`
	SmartBidType   string   `json:"smart_bid_type,omitempty"`
	Bid            float64  `json:"bid,omitempty"`
	MaterialType   string   `json:"material_type,omitempty"`
}

type UniPromotionControlTaskCreateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		Id uint64 `json:"id"`
	} `json:"data"`
}

type UniPromotionControlTaskListReq struct {
	AccessToken   string                                `json:"-"`
	AdvertiserId  uint64                                `json:"advertiser_id"`
	MarketingGoal string                                `json:"marketing_goal"`
	AdId          uint64                                `json:"ad_id"`
	StartTime     string                                `json:"start_time"`
	EndTime       string                                `json:"end_time"`
	Fields        []string                              `json:"fields,omitempty"`
	Scene         string                                `json:"scene"`
	Filtering     *UniPromotionControlTaskListFiltering `json:"filtering,omitempty"`
	OrderType     string                                `json:"order_type,omitempty"`
	OrderField    string                                `json:"order_field,omitempty"`
	Page          int                                   `json:"page,omitempty"`
	PageSize      int                                   `json:"page_size,omitempty"`
}

type UniPromotionControlTaskListFiltering struct {
	SearchKeyword   string `json:"search_keyword,omitempty"`
	TaskStatus      string `json:"task_status,omitempty"`
	HavingCost      string `json:"having_cost,omitempty"`
	CreateStartTime string `json:"create_start_time,omitempty"`
	CreateEndTime   string `json:"create_end_time,omitempty"`
	SmartBidType    string `json:"smart_bid_type,omitempty"`
	ExternalAction  string `json:"external_action,omitempty"`
}

type UniPromotionControlTask struct {
	Id                     uint64                            `json:"id"`
	Name                   string                            `json:"name"`
	TaskStatus             string                            `json:"task_status"`
	Budget                 float64                           `json:"budget"`
	Duration               float64                           `json:"duration"`
	CreateTime             string                            `json:"create_time"`
	Bid                    float64                           `json:"bid"`
	ExternalActionName     string                            `json:"external_action_name"`
	DeepExternalActionName string                            `json:"deep_external_action_name"`
	SmartBidType           string                            `json:"smart_bid_type"`
	Roi2Goal               float64                           `json:"roi2_goal"`
	MaterialList           []UniPromotionControlTaskMaterial `json:"material_list"`
	Metrics                map[string]any                    `json:"metrics"`
}

type UniPromotionControlTaskMaterial struct {
	MaterialId    uint64 `json:"material_id"`
	AwemeItemId   uint64 `json:"aweme_item_id"`
	VideoDuration int64  `json:"video_duration"`
	Title         string `json:"title"`
}

type UniPromotionControlTaskListRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		TaskList []UniPromotionControlTask `json:"task_list"`
		PageInfo PageInfo                  `json:"page_info"`
	} `json:"data"`
}

type UniPromotionControlTaskStatusUpdateReq struct {
	AccessToken  string   `json:"-"`
	AdvertiserId uint64   `json:"advertiser_id"`
	TaskIds      []uint64 `json:"task_ids"`
	OptType      string   `json:"opt_type"`
}

type UniPromotionControlTaskStatusUpdateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		TaskIds []uint64                                   `json:"task_ids,omitempty"`
		Errors  []UniPromotionControlTaskStatusUpdateError `json:"errors,omitempty"`
	} `json:"data"`
}

type UniPromotionControlTaskStatusUpdateError struct {
	TaskID       uint64 `json:"task_id"`
	ErrorMessage string `json:"error_message"`
}

type UniPromotionControlTaskUpdateReq struct {
	AccessToken  string   `json:"-"`
	AdvertiserId uint64   `json:"advertiser_id"`
	TaskID       uint64   `json:"task_id"`
	Name         string   `json:"name"`
	Budget       float64  `json:"budget"`
	Roi2Goal     float64  `json:"roi2_goal,omitempty"`
	MaterialIDs  []uint64 `json:"material_ids,omitempty"`
	Duration     float64  `json:"duration,omitempty"`
	Bid          float64  `json:"bid,omitempty"`
}

type UniPromotionControlTaskUpdateRes struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      struct {
		FailedInfos []uint64 `json:"failed_infos,omitempty"`
	} `json:"data"`
}

// ===== Common Types =====

type PageInfo struct {
	Page        int   `json:"page"`
	PageSize    int   `json:"page_size"`
	TotalNumber int64 `json:"total_number"`
	TotalPage   int   `json:"total_page"`
}

func (p *PageInfo) UnmarshalJSON(data []byte) error {
	var raw struct {
		Page        int         `json:"page"`
		PageSize    int         `json:"page_size"`
		TotalNumber json.Number `json:"total_number"`
		TotalNum    json.Number `json:"total_num"`
		TotalPage   int         `json:"total_page"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Page = raw.Page
	p.PageSize = raw.PageSize
	p.TotalPage = raw.TotalPage
	p.TotalNumber = 0
	for _, value := range []json.Number{raw.TotalNumber, raw.TotalNum} {
		if value == "" {
			continue
		}
		parsed, err := value.Int64()
		if err == nil {
			p.TotalNumber = parsed
			break
		}
	}
	return nil
}
