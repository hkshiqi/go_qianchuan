package handler

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/gin-gonic/gin"
)

// MCPHandler exposes a small, token-authenticated API for the local
// agent. Credentials remain server-side and are never returned.
type MCPHandler struct {
	service      *service.QianchuanService
	apiKey       string
	writeEnabled bool
}

func NewMCPHandler(qianchuanService *service.QianchuanService, apiKey string, writeEnabled bool) *MCPHandler {
	return &MCPHandler{
		service:      qianchuanService,
		apiKey:       apiKey,
		writeEnabled: writeEnabled,
	}
}

// RequireAPIKey validates X-MCP-Key in constant time.
func (h *MCPHandler) RequireAPIKey(c *gin.Context) {
	provided := strings.TrimSpace(c.GetHeader("X-MCP-Key"))
	if h.apiKey == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(h.apiKey)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, util.ApiResponse{
			Code:    util.CodeUnauthorized,
			Message: "MCP 鉴权失败",
		})
		return
	}
	c.Next()
}

func (h *MCPHandler) Status(c *gin.Context) {
	util.Success(c, gin.H{
		"advertiser_id":  h.service.DefaultAdvertiserID,
		"aweme_id":       h.service.DefaultAwemeID,
		"live_room_id":   h.service.DefaultLiveRoomID,
		"read_enabled":   true,
		"write_enabled":  h.writeEnabled,
	})
}

func (h *MCPHandler) Plans(c *gin.Context) {
	token, advertiserID, ok := h.token(c)
	if !ok {
		return
	}
	now := time.Now()
	startTime := c.DefaultQuery("start_time", now.AddDate(0, -6, 0).Format("2006-01-02 15:04:05"))
	endTime := c.DefaultQuery("end_time", now.Format("2006-01-02 15:04:05"))
	page := positiveInt(c.DefaultQuery("page", "1"), 1)
	pageSize := pageSizeForPlans(c.DefaultQuery("page_size", "20"))
	fields := parseStringList(c.Query("fields"))
	if len(fields) == 0 {
		fields = []string{
			"stat_cost",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
		}
	}
	response, err := h.service.Client.UniPromotionList(c.Request.Context(), sdk.UniPromotionListReq{
		AccessToken:   token,
		AdvertiserId:  uint64(advertiserID),
		StartTime:     startTime,
		EndTime:       endTime,
		MarketingGoal: c.DefaultQuery("marketing_goal", "LIVE_PROM_GOODS"),
		Fields:        fields,
		OrderField:    c.DefaultQuery("order_field", "stat_cost"),
		OrderType:     strings.ToUpper(c.DefaultQuery("order_type", "DESC")),
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		util.ServerError(c, "获取计划列表失败: "+err.Error())
		return
	}
	if response.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取计划列表失败[%d]: %s", response.Code, response.Message))
		return
	}
	util.Success(c, gin.H{
		"list":      response.Data.List,
		"page_info": response.Data.PageInfo,
		"window": gin.H{
			"start_time": startTime,
			"end_time":   endTime,
		},
	})
}

func (h *MCPHandler) PlanDetail(c *gin.Context) {
	token, advertiserID, ok := h.token(c)
	if !ok {
		return
	}
	adID, err := parseUint(c.Param("ad_id"))
	if err != nil {
		util.BadRequest(c, "ad_id 参数错误")
		return
	}
	response, err := h.service.Client.UniPromotionDetail(c.Request.Context(), sdk.UniPromotionDetailReq{
		AccessToken:  token,
		AdvertiserId: uint64(advertiserID),
		AdId:         adID,
	})
	if err != nil {
		util.ServerError(c, "获取计划详情失败: "+err.Error())
		return
	}
	if response.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取计划详情失败[%d]: %s", response.Code, response.Message))
		return
	}
	util.Success(c, response.Data)
}

func (h *MCPHandler) PlanMaterials(c *gin.Context) {
	token, advertiserID, ok := h.token(c)
	if !ok {
		return
	}
	adID, err := parseUint(c.Param("ad_id"))
	if err != nil {
		util.BadRequest(c, "ad_id 参数错误")
		return
	}
	materialType := strings.ToUpper(c.DefaultQuery("material_type", "VIDEO"))
	page := positiveInt(c.DefaultQuery("page", "1"), 1)
	pageSize := pageSizeForMaterials(c.DefaultQuery("page_size", "20"))
	fields := parseStringList(c.Query("fields"))
	if len(fields) == 0 {
		fields = []string{
			"stat_cost_for_roi2",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
			"live_show_count_for_roi2_v2",
			"live_watch_count_for_roi2_v2",
			"live_cvr_rate_for_roi2_v2",
			"live_convert_rate_for_roi2_v2",
			"live_show_count_exclude_video_for_roi2",
			"live_watch_count_exclude_video_for_roi2",
			"live_cvr_rate_exclude_video_for_roi2",
			"live_convert_rate_exclude_video_for_roi2",
		}
	}
	response, err := h.service.Client.UniPromotionMaterialGet(
		c.Request.Context(),
		sdk.UniPromotionMaterialGetReq{
			AccessToken:        token,
			AdvertiserId:       uint64(advertiserID),
			AdId:               adID,
			MaterialType:       materialType,
			VideoType:          c.Query("video_type"),
			Status:             c.Query("material_status"),
			MaterialSelectType: c.Query("material_select_type"),
			AnalysisType:       parseStringList(c.Query("analysis_type")),
			StartDate:          c.Query("start_date"),
			EndDate:            c.Query("end_date"),
			Fields:             fields,
			OrderField:         c.DefaultQuery("order_field", "stat_cost_for_roi2"),
			OrderType:          strings.ToUpper(c.DefaultQuery("order_type", "DESC")),
			Page:               page,
			PageSize:           pageSize,
		},
	)
	if err != nil {
		util.ServerError(c, "获取计划素材失败: "+err.Error())
		return
	}
	if response.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取计划素材失败[%d]: %s", response.Code, response.Message))
		return
	}
	today := time.Now().Format("2006-01-02")
	top3, err := topSpendingMaterialIDs(
		c.Request.Context(),
		h.service.Client,
		token,
		uint64(advertiserID),
		adID,
		materialType,
		today,
		today,
	)
	if err != nil {
		util.ServerError(c, "识别消耗前3素材失败: "+err.Error())
		return
	}
	util.Success(c, gin.H{
		"materials":        response.Data.Materials,
		"page_info":        response.Data.PageInfo,
		"protected_top3":   top3,
		"protection_rule":  "服务端按当日 stat_cost 降序，前三名素材不可删除",
	})
}

func (h *MCPHandler) MaterialReport(c *gin.Context) {
	token, advertiserID, ok := h.token(c)
	if !ok {
		return
	}
	var req struct {
		StartDate string                          `json:"start_date" binding:"required"`
		EndDate   string                          `json:"end_date" binding:"required"`
		Fields    []string                        `json:"fields"`
		Page      int64                           `json:"page"`
		PageSize  int64                           `json:"page_size"`
		Filtering *sdk.ReportMaterialGetFiltering `json:"filtering"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 1000 {
		req.PageSize = 100
	}
	if len(req.Fields) == 0 {
		req.Fields = []string{
			"stat_cost",
			"show_cnt",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"conversion_rate",
			"total_pay_order_gmv_for_roi2",
			"total_prepay_and_pay_order_roi2",
		}
	}
	response, err := h.service.Client.ReportMaterialGet(c.Request.Context(), sdk.ReportMaterialGetReq{
		AccessToken:  token,
		AdvertiserId: int64(advertiserID),
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Fields:       req.Fields,
		Page:         req.Page,
		PageSize:     req.PageSize,
		OrderField:   "stat_cost",
		OrderType:    "DESC",
		Filtering:    req.Filtering,
	})
	if err != nil {
		util.ServerError(c, "获取素材报表失败: "+err.Error())
		return
	}
	if response.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取素材报表失败[%d]: %s", response.Code, response.Message))
		return
	}
	util.Success(c, gin.H{
		"list":      response.Data.List,
		"page_info": response.Data.PageInfo,
	})
}

func (h *MCPHandler) LiveOverview(c *gin.Context) {
	token, advertiserID, ok := h.token(c)
	if !ok {
		return
	}
	now := time.Now()
	startTime := c.DefaultQuery("start_time", now.Format("2006-01-02")+" 00:00:00")
	endTime := c.DefaultQuery("end_time", now.Format("2006-01-02")+" 23:59:59")
	fields := parseStringList(c.Query("fields"))
	if len(fields) == 0 {
		fields = []string{
			"stat_cost",
			"click_cnt",
			"ctr",
			"convert_cnt",
			"convert_rate",
			"today_live_pay_order_gpm",
			"luban_live_pay_order_gpm",
			"live_pay_order_gmv_alias",
			"luban_live_pay_order_gmv",
			"live_pay_order_gmv_roi",
			"total_live_watch_cnt",
			"live_click_product_count_alias",
		}
	}

	// The SDK's current LiveGet wrapper does not expose time bounds.
	// Return a clear contract marker instead of silently using a stale day.
	if startTime != now.Format("2006-01-02")+" 00:00:00" ||
		endTime != now.Format("2006-01-02")+" 23:59:59" {
		util.BadRequest(c, "当前只读接口仅支持当天直播数据")
		return
	}
	response, err := h.service.Client.LiveGet(c.Request.Context(), sdk.LiveGetReq{
		AccessToken:  token,
		AdvertiserId: int64(advertiserID),
		AwemeId:      h.service.DefaultAwemeID,
		Fields:       fields,
	})
	if err != nil {
		util.ServerError(c, "获取直播数据失败: "+err.Error())
		return
	}
	if response.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取直播数据失败[%d]: %s", response.Code, response.Message))
		return
	}
	util.Success(c, gin.H{
		"advertiser_id": advertiserID,
		"aweme_id":      h.service.DefaultAwemeID,
		"live_room_id":  h.service.DefaultLiveRoomID,
		"data":          response.Data,
		"refreshed_at":  now.Unix(),
	})
}

func (h *MCPHandler) WriteDisabled(c *gin.Context) {
	c.JSON(http.StatusForbidden, util.ApiResponse{
		Code:    util.CodeForbidden,
		Message: "投放写操作已停用",
		Data: gin.H{
			"write_enabled": false,
		},
	})
}

func (h *MCPHandler) token(c *gin.Context) (string, int64, bool) {
	token, advertiserID, err := h.service.AccessTokenFor(c.Request.Context(), h.service.DefaultAdvertiserID)
	if err != nil {
		util.Unauthorized(c, err.Error())
		return "", 0, false
	}
	return token, advertiserID, true
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func pageSizeForPlans(value string) int {
	size := positiveInt(value, 20)
	switch size {
	case 10, 20, 50, 100:
		return size
	default:
		return 20
	}
}

func pageSizeForMaterials(value string) int {
	size := positiveInt(value, 20)
	switch size {
	case 10, 20, 50, 100:
		return size
	default:
		return 20
	}
}

func parseUint(value string) (uint64, error) {
	return strconv.ParseUint(value, 10, 64)
}
