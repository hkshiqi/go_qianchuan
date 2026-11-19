package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/middleware"
	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/gin-gonic/gin"
)

// UniPromotionHandler 全域推广Handler
type UniPromotionHandler struct {
	service *service.QianchuanService
}

// NewUniPromotionHandler 创建全域推广Handler
func NewUniPromotionHandler(service *service.QianchuanService) *UniPromotionHandler {
	return &UniPromotionHandler{service: service}
}

// List 获取全域推广列表
// GET /qianchuan/uni_promotion/list
func (h *UniPromotionHandler) List(c *gin.Context) {
	userSession, _ := middleware.GetUserSession(c)
	advertiserIdStr := c.Query("advertiser_id")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	advertiserId, _ := strconv.ParseUint(advertiserIdStr, 10, 64)
	if advertiserId == 0 {
		advertiserId = uint64(userSession.AdvertiserID)
	}

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if page <= 0 {
		page = 1
	}
	switch pageSize {
	case 10, 20, 50, 100:
	default:
		pageSize = 20
	}

	now := time.Now()
	startTime := c.DefaultQuery("start_time", now.AddDate(0, -6, 0).Format("2006-01-02 15:04:05"))
	endTime := c.DefaultQuery("end_time", now.Format("2006-01-02 15:04:05"))
	marketingGoal := c.DefaultQuery("marketing_goal", "LIVE_PROM_GOODS")
	orderField := c.DefaultQuery("order_field", "stat_cost")
	orderType := strings.ToUpper(c.DefaultQuery("order_type", "DESC"))
	fields := parseStringList(c.Query("fields"))
	if len(fields) == 0 {
		fields = []string{
			"stat_cost",
			"total_pay_order_gmv_for_roi2",
			"total_pay_order_count_for_roi2",
			"total_prepay_and_pay_order_roi2",
		}
	}

	log.Printf("UniPromotion List: advertiser_id=%d, page=%d, page_size=%d", advertiserId, page, pageSize)

	resp, err := h.service.Client.UniPromotionList(c.Request.Context(), sdk.UniPromotionListReq{
		AccessToken:   userSession.AccessToken,
		AdvertiserId:  advertiserId,
		StartTime:     startTime,
		EndTime:       endTime,
		MarketingGoal: marketingGoal,
		Fields:        fields,
		OrderField:    orderField,
		OrderType:     orderType,
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		util.ServerError(c, "获取全域推广列表失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取全域推广列表失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{
		"list":      resp.Data.List,
		"page_info": resp.Data.PageInfo,
	})
}

// GetDetail 获取全域推广详情
// GET /qianchuan/uni_promotion/detail
func (h *UniPromotionHandler) GetDetail(c *gin.Context) {
	userSession, _ := middleware.GetUserSession(c)
	advertiserIdStr := c.Query("advertiser_id")
	adIdStr := c.Query("ad_id")

	advertiserId, _ := strconv.ParseUint(advertiserIdStr, 10, 64)
	if advertiserId == 0 {
		advertiserId = uint64(userSession.AdvertiserID)
	}
	adId, err := strconv.ParseUint(adIdStr, 10, 64)
	if err != nil || adId == 0 {
		util.BadRequest(c, "ad_id 参数必填")
		return
	}

	log.Printf("UniPromotion Detail: advertiser_id=%d, ad_id=%d", advertiserId, adId)

	resp, err := h.service.Client.UniPromotionDetail(c.Request.Context(), sdk.UniPromotionDetailReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: advertiserId,
		AdId:         adId,
	})
	if err != nil {
		util.ServerError(c, "获取全域推广详情失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取全域推广详情失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, resp.Data)
}

// Create 创建全域推广
// POST /qianchuan/uni_promotion/create
func (h *UniPromotionHandler) Create(c *gin.Context) {
	var req struct {
		AdvertiserId uint64  `json:"advertiser_id"`
		AwemeId      uint64  `json:"aweme_id" binding:"required"`
		Budget       float64 `json:"budget" binding:"required"`
		Roi2Goal     float64 `json:"roi2_goal"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion Create: advertiser_id=%d, aweme_id=%d, budget=%f",
		req.AdvertiserId, req.AwemeId, req.Budget)

	resp, err := h.service.Client.UniPromotionCreate(c.Request.Context(), sdk.UniPromotionCreateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		AwemeId:      req.AwemeId,
		Budget:       req.Budget,
		Roi2Goal:     req.Roi2Goal,
	})
	if err != nil {
		util.ServerError(c, "创建全域推广失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("创建全域推广失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"ad_id": resp.Data.AdId})
}

// Update 更新全域推广
// POST /qianchuan/uni_promotion/update
func (h *UniPromotionHandler) Update(c *gin.Context) {
	var req struct {
		AdvertiserId uint64  `json:"advertiser_id"`
		AdId         uint64  `json:"ad_id" binding:"required"`
		Budget       float64 `json:"budget"`
		Roi2Goal     float64 `json:"roi2_goal"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion Update: advertiser_id=%d, ad_id=%d", req.AdvertiserId, req.AdId)

	resp, err := h.service.Client.UniPromotionUpdate(c.Request.Context(), sdk.UniPromotionUpdateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		AdId:         req.AdId,
		Budget:       req.Budget,
		Roi2Goal:     req.Roi2Goal,
	})
	if err != nil {
		util.ServerError(c, "更新全域推广失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("更新全域推广失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"ad_id": resp.Data.AdId})
}

// UpdateStatus 更新全域推广状态
// POST /qianchuan/uni_promotion/status/update
func (h *UniPromotionHandler) UpdateStatus(c *gin.Context) {
	var req struct {
		AdvertiserId uint64   `json:"advertiser_id"`
		AdIds        []uint64 `json:"ad_ids" binding:"required"`
		OptStatus    string   `json:"opt_status" binding:"required"` // ENABLE / DISABLE
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion StatusUpdate: advertiser_id=%d, ad_ids=%v, opt_status=%s",
		req.AdvertiserId, req.AdIds, req.OptStatus)

	resp, err := h.service.Client.UniPromotionStatusUpdate(c.Request.Context(), sdk.UniPromotionStatusUpdateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		AdIds:        req.AdIds,
		OptStatus:    req.OptStatus,
	})
	if err != nil {
		util.ServerError(c, "更新全域推广状态失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("更新全域推广状态失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{
		"ad_ids": resp.Data.AdIds,
		"errors": resp.Data.Errors,
	})
}

// UpdateMaterial 删除全域推广计划下素材
// POST /qianchuan/uni-promotion/material/delete
func (h *UniPromotionHandler) UpdateMaterial(c *gin.Context) {
	var req struct {
		AdvertiserId uint64   `json:"advertiser_id"`
		AdId         uint64   `json:"ad_id" binding:"required"`
		MaterialIds  []uint64 `json:"material_ids" binding:"required"`
		Titles       []string `json:"titles"`
		StartDate    string   `json:"start_date"`
		EndDate      string   `json:"end_date"`
		MaterialType string   `json:"material_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if len(req.MaterialIds) == 0 {
		util.BadRequest(c, "material_ids 不能为空，禁止仅按标题删除素材")
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}
	// The protection window is server-controlled. Client-supplied dates must
	// not be able to hide top-spending materials from the delete guard.
	req.StartDate = time.Now().Format("2006-01-02")
	req.EndDate = req.StartDate
	if req.MaterialType == "" {
		req.MaterialType = "VIDEO"
	}

	protected, err := topSpendingMaterialIDs(
		c.Request.Context(),
		h.service.Client,
		userSession.AccessToken,
		req.AdvertiserId,
		req.AdId,
		req.MaterialType,
		req.StartDate,
		req.EndDate,
	)
	if err != nil {
		util.ServerError(c, "无法验证素材消耗排名，已拒绝删除: "+err.Error())
		return
	}
	var blocked []uint64
	for _, materialID := range req.MaterialIds {
		if protected[materialID] {
			blocked = append(blocked, materialID)
		}
	}
	if len(blocked) > 0 {
		c.JSON(http.StatusConflict, util.ApiResponse{
			Code:    util.CodeConflict,
			Message: "消耗最高的前3素材必须保留，删除请求已拒绝",
			Data: gin.H{
				"blocked_material_ids": blocked,
				"window": gin.H{
					"start_date": req.StartDate,
					"end_date":   req.EndDate,
				},
			},
		})
		return
	}

	log.Printf("UniPromotion MaterialDelete: advertiser_id=%d, ad_id=%d", req.AdvertiserId, req.AdId)

	resp, err := h.service.Client.UniPromotionMaterialDelete(c.Request.Context(), sdk.UniPromotionMaterialDeleteReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		AdId:         req.AdId,
		MaterialIds:  req.MaterialIds,
		Titles:       req.Titles,
		MaterialType: req.MaterialType,
	})
	if err != nil {
		util.ServerError(c, "删除全域推广素材失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("删除全域推广素材失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"message": "删除成功"})
}

// AddExistingMaterial adds material-library entries to a plan.
// POST /qianchuan/uni-promotion/material/add-existing
func (h *UniPromotionHandler) AddExistingMaterial(c *gin.Context) {
	var req struct {
		AdvertiserId  uint64                   `json:"advertiser_id"`
		AdId          uint64                   `json:"ad_id" binding:"required"`
		VideoMaterial []sdk.UniPromotionMaterial `json:"video_material" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	resp, err := h.service.Client.UniPromotionMaterialAdd(
		c.Request.Context(),
		sdk.UniPromotionMaterialAddReq{
			AccessToken:   userSession.AccessToken,
			AdvertiserId:  req.AdvertiserId,
			AdId:          req.AdId,
			VideoMaterial: req.VideoMaterial,
		},
	)
	if err != nil {
		util.ServerError(c, "添加现有素材失败: "+err.Error())
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("添加现有素材失败[%d]: %s", resp.Code, resp.Message))
		return
	}
	util.Success(c, gin.H{"message": "添加成功"})
}

// UpdateAuth 全域授权初始化
// POST /qianchuan/uni_promotion/auth/init
func (h *UniPromotionHandler) UpdateAuth(c *gin.Context) {
	var req struct {
		AdvertiserId uint64 `json:"advertiser_id"`
		AwemeId      uint64 `json:"aweme_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion AuthInit: advertiser_id=%d", req.AdvertiserId)

	resp, err := h.service.Client.UniPromotionAuthInit(c.Request.Context(), sdk.UniPromotionAuthInitReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		AwemeId:      req.AwemeId,
	})
	if err != nil {
		util.ServerError(c, "全域授权初始化失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("全域授权初始化失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"message": "初始化成功"})
}

// UpdateBudget 更新全域推广预算
// POST /qianchuan/uni_promotion/budget/update
func (h *UniPromotionHandler) UpdateBudget(c *gin.Context) {
	var req struct {
		AdvertiserId uint64 `json:"advertiser_id"`
		Data         []struct {
			AdId   uint64  `json:"ad_id"`
			Budget float64 `json:"budget"`
		} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion BudgetUpdate: advertiser_id=%d, data_count=%d", req.AdvertiserId, len(req.Data))

	var updateItems []sdk.UniPromotionBudgetUpdateItem
	for _, item := range req.Data {
		updateItems = append(updateItems, sdk.UniPromotionBudgetUpdateItem{
			AdId:   item.AdId,
			Budget: item.Budget,
		})
	}

	resp, err := h.service.Client.UniPromotionBudgetUpdate(c.Request.Context(), sdk.UniPromotionBudgetUpdateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		Data:         updateItems,
	})
	if err != nil {
		util.ServerError(c, "更新全域推广预算失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("更新全域推广预算失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"results": resp.Data.Results})
}

// UpdateROIGoal 更新全域推广ROI目标
// POST /qianchuan/uni_promotion/roi_goal/update
func (h *UniPromotionHandler) UpdateROIGoal(c *gin.Context) {
	var req struct {
		AdvertiserId uint64 `json:"advertiser_id"`
		Data         []struct {
			AdId     uint64  `json:"ad_id"`
			Roi2Goal float64 `json:"roi2_goal"`
		} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion RoiGoalUpdate: advertiser_id=%d, data_count=%d", req.AdvertiserId, len(req.Data))

	var updateItems []sdk.UniPromotionRoiGoalUpdateItem
	for _, item := range req.Data {
		updateItems = append(updateItems, sdk.UniPromotionRoiGoalUpdateItem{
			AdId:     item.AdId,
			Roi2Goal: item.Roi2Goal,
		})
	}

	resp, err := h.service.Client.UniPromotionRoiGoalUpdate(c.Request.Context(), sdk.UniPromotionRoiGoalUpdateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		Data:         updateItems,
	})
	if err != nil {
		util.ServerError(c, "更新全域推广ROI目标失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("更新全域推广ROI目标失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"results": resp.Data.Results})
}

// UpdateScheduleTime 更新全域推广投放时间
// POST /qianchuan/uni_promotion/schedule/update
func (h *UniPromotionHandler) UpdateScheduleTime(c *gin.Context) {
	var req struct {
		AdvertiserId uint64 `json:"advertiser_id"`
		Data         []struct {
			AdId      uint64 `json:"ad_id"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
		} `json:"data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userSession, _ := middleware.GetUserSession(c)
	if req.AdvertiserId == 0 {
		req.AdvertiserId = uint64(userSession.AdvertiserID)
	}

	log.Printf("UniPromotion ScheduleUpdate: advertiser_id=%d, data_count=%d", req.AdvertiserId, len(req.Data))

	var updateItems []sdk.UniPromotionScheduleUpdateItem
	for _, item := range req.Data {
		updateItems = append(updateItems, sdk.UniPromotionScheduleUpdateItem{
			AdId:      item.AdId,
			StartTime: item.StartTime,
			EndTime:   item.EndTime,
		})
	}

	resp, err := h.service.Client.UniPromotionScheduleUpdate(c.Request.Context(), sdk.UniPromotionScheduleUpdateReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: req.AdvertiserId,
		Data:         updateItems,
	})
	if err != nil {
		util.ServerError(c, "更新全域推广投放时间失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("更新全域推广投放时间失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{"results": resp.Data.Results})
}

// GetBudgetSchedule 获取可投全域推广抖音号列表
// GET /qianchuan/uni_promotion/authorized/get
func (h *UniPromotionHandler) GetBudgetSchedule(c *gin.Context) {
	userSession, _ := middleware.GetUserSession(c)
	advertiserIdStr := c.Query("advertiser_id")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	advertiserId, _ := strconv.ParseUint(advertiserIdStr, 10, 64)
	if advertiserId == 0 {
		advertiserId = uint64(userSession.AdvertiserID)
	}

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	log.Printf("UniPromotion AuthorizedGet: advertiser_id=%d, page=%d, page_size=%d", advertiserId, page, pageSize)

	resp, err := h.service.Client.UniPromotionAuthorizedGet(c.Request.Context(), sdk.UniPromotionAuthorizedGetReq{
		AccessToken:  userSession.AccessToken,
		AdvertiserId: advertiserId,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		util.ServerError(c, "获取可投全域推广抖音号列表失败: "+err.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取可投全域推广抖音号列表失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	util.Success(c, gin.H{
		"list":      resp.Data.List,
		"page_info": resp.Data.PageInfo,
	})
}

// GetMaterials 获取全域推广素材
// GET /qianchuan/uni_promotion/material/get
func (h *UniPromotionHandler) GetMaterials(c *gin.Context) {
	userSession, _ := middleware.GetUserSession(c)
	advertiserIdStr := c.Query("advertiser_id")
	adIdStr := c.Query("ad_id")

	advertiserId, _ := strconv.ParseUint(advertiserIdStr, 10, 64)
	if advertiserId == 0 {
		advertiserId = uint64(userSession.AdvertiserID)
	}
	adId, err := strconv.ParseUint(adIdStr, 10, 64)
	if err != nil || adId == 0 {
		util.BadRequest(c, "ad_id 参数必填")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	switch pageSize {
	case 10, 20, 50, 100:
	default:
		pageSize = 20
	}
	materialType := strings.ToUpper(c.DefaultQuery("material_type", "VIDEO"))
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

	log.Printf("UniPromotion MaterialGet: advertiser_id=%d, ad_id=%d", advertiserId, adId)

	resp, errSdk := h.service.Client.UniPromotionMaterialGet(c.Request.Context(), sdk.UniPromotionMaterialGetReq{
		AccessToken:        userSession.AccessToken,
		AdvertiserId:       advertiserId,
		AdId:               adId,
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
	})
	if errSdk != nil {
		util.ServerError(c, "获取全域推广素材失败: "+errSdk.Error())
		return
	}
	if resp.Code == 501 {
		util.NotImplemented(c, resp.Message, "请使用 oceanengine 构建版本以启用全域推广功能")
		return
	}
	if resp.Code != 0 {
		util.BadRequest(c, fmt.Sprintf("获取全域推广素材失败[%d]: %s", resp.Code, resp.Message))
		return
	}

	protected, err := topSpendingMaterialIDs(
		c.Request.Context(),
		h.service.Client,
		userSession.AccessToken,
		advertiserId,
		adId,
		materialType,
		time.Now().Format("2006-01-02"),
		time.Now().Format("2006-01-02"),
	)
	if err != nil {
		util.ServerError(c, "识别消耗前3素材失败: "+err.Error())
		return
	}
	util.Success(c, gin.H{
		"materials":       resp.Data.Materials,
		"page_info":       resp.Data.PageInfo,
		"protected_top3":  protected,
		"protection_rule": "当日消耗按 stat_cost_for_roi2 降序的前3名素材必须保留",
	})
}

func parseStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "[]" {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		var result []string
		if err := json.Unmarshal([]byte(value), &result); err == nil {
			return result
		}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
