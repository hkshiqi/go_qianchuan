package handler

import (
	"log"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/CriarBrand/qianchuan-backend/pkg/session"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	service *service.QianchuanService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(service *service.QianchuanService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

// OAuthExchange OAuth code换取session
// @Summary      OAuth授权登录
// @Description  通过OAuth授权码exchange获取AccessToken并创建会话
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      object{code=string,state=string}  true  "OAuth授权码和State"
// @Success      200      {object}  map[string]interface{}  "登录成功"
// @Failure      400      {object}  map[string]interface{}  "参数错误"
// @Failure      403      {object}  map[string]interface{}  "State验证失败"
// @Failure      500      {object}  map[string]interface{}  "内部错误"
// @Router       /oauth/exchange [post]
func (h *AuthHandler) OAuthExchange(c *gin.Context) {
	log.Printf("[OAuthExchange] starting OAuth exchange")

	var req struct {
		Code  string `json:"code" binding:"required"`
		State string `json:"state"` // State参数用于CSRF防护
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ [OAuthExchange] Bad request: %v", err)
		util.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 调用SDK换取AccessToken
	log.Printf("[OAuthExchange] exchanging authorization code")
	tokenResp, err := h.service.Client.OauthAccessToken(c.Request.Context(), sdk.OauthAccessTokenReq{
		AuthCode: req.Code,
	})
	if err != nil {
		log.Printf("[OAuthExchange] OAuth exchange failed: %v", err)
		util.ServerError(c, "OAuth授权失败: "+err.Error())
		return
	}

	// 检查响应码
	if tokenResp.Code != 0 {
		log.Printf("[OAuthExchange] OAuth API error: code=%d, message=%s", tokenResp.Code, tokenResp.Message)
		util.ServerError(c, "OAuth授权失败: "+tokenResp.Message)
		return
	}

	// 检查 Data 是否为 nil
	if tokenResp.Data == nil {
		log.Printf("[OAuthExchange] OAuth response data is nil")
		util.ServerError(c, "OAuth授权失败: 响应数据为空")
		return
	}

	log.Printf("[OAuthExchange] token exchange succeeded; expires_in=%d", tokenResp.Data.ExpiresIn)

	// 尝试获取广告主列表（用于获取advertiser_id）
	log.Printf("[OAuthExchange] fetching advertiser list")
	var advertiserId int64 = 0

	// 优先使用 Token响应中的 AdvertiserId
	if tokenResp.Data.AdvertiserId != 0 {
		advertiserId = tokenResp.Data.AdvertiserId
		log.Printf("[OAuthExchange] using advertiser_id from token response: %d", advertiserId)
	} else {
		// 如果 token 响应中没有，尝试调用 AdvertiserList API
		advertiserResp, err := h.service.Client.AdvertiserList(c.Request.Context(), sdk.AdvertiserListReq{
			AccessToken: tokenResp.Data.AccessToken,
			AppId:       h.service.AppId,
			Secret:      h.service.AppSecret,
		})

		if advertiserResp != nil {
			log.Printf("[OAuthExchange] advertiser list response: code=%d, count=%d", advertiserResp.Code, len(advertiserResp.Data.List))
		}

		// 如果 API 调用失败，记录警告但不阻止登录
		if err != nil {
			log.Printf("[OAuthExchange] get advertiser list failed (non-critical): %v", err)
		} else {
			log.Printf("[OAuthExchange] advertiser list fetched")

			// 如果有广告主，使用第一个
			if len(advertiserResp.Data.List) > 0 {
				advertiserId = advertiserResp.Data.List[0].AdvertiserId
				log.Printf("[OAuthExchange] using advertiser_id from list: %d", advertiserId)
			}
		}
	}

	if advertiserId == 0 {
		advertiserId = h.service.DefaultAdvertiserID
	}
	// 无论是否获取到 advertiserId，都允许登录
	if advertiserId == 0 {
		log.Printf("[OAuthExchange] no advertiser_id available; session has no advertiser context")
	}

	// 创建会话
	log.Printf("[OAuthExchange] creating session")
	userSession := session.NewSessionFromTokenResponse(&session.TokenResponse{
		AccessToken:           tokenResp.Data.AccessToken,
		RefreshToken:          tokenResp.Data.RefreshToken,
		ExpiresIn:             tokenResp.Data.ExpiresIn,
		RefreshTokenExpiresIn: tokenResp.Data.RefreshTokenExpiresIn,
	}, advertiserId)
	if h.service.TokenManager != nil && advertiserId > 0 {
		if err := h.service.TokenManager.SaveToken(advertiserId, userSession); err != nil {
			log.Printf("[OAuthExchange] persist token failed: %v", err)
			util.ServerError(c, "保存授权凭证失败")
			return
		}
	}

	// 保存到Session
	sess := sessions.Default(c)
	sess.Set("user", userSession)
	if err := sess.Save(); err != nil {
		log.Printf("[OAuthExchange] save session failed: %v", err)
		util.ServerError(c, "保存会话失败")
		return
	}
	log.Printf("[OAuthExchange] session saved; OAuth exchange completed")

	util.Success(c, gin.H{"success": true})
}

// GetUserInfo 获取用户信息
// @Summary      获取当前用户信息
// @Description  获取已登录用户的详细信息
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Success      200  {object}  map[string]interface{}  "用户信息"
// @Failure      401  {object}  map[string]interface{}  "未登录"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /user/info [get]
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	log.Printf("[GetUserInfo] fetching user info")
	sess := sessions.Default(c)
	sessionData := sess.Get("user")

	if sessionData == nil {
		log.Printf("[GetUserInfo] no session data found")
		util.Unauthorized(c, "未登录")
		return
	}
	log.Printf("[GetUserInfo] session data found")

	userSession, ok := sessionData.(*session.UserSession)
	if !ok {
		log.Printf("[GetUserInfo] session data format error: type=%T", sessionData)
		util.ServerError(c, "会话数据格式错误")
		return
	}
	log.Printf("[GetUserInfo] session parsed; advertiser_id=%d", userSession.AdvertiserID)

	// 调用SDK获取用户信息
	log.Printf("[GetUserInfo] fetching user info from API")
	userInfoResp, err := h.service.Client.UserInfo(c.Request.Context(), sdk.UserInfoReq{
		AccessToken: userSession.AccessToken,
	})

	if err != nil {
		log.Printf("[GetUserInfo] get user info failed; returning basic info: %v", err)
		// 如果失败，返回基本信息
		util.Success(c, gin.H{
			"id":    userSession.AdvertiserID,
			"name":  "广告主",
			"email": "",
		})
		return
	}

	log.Printf("[GetUserInfo] got user info from API")
	// 返回真实用户信息
	util.Success(c, gin.H{
		"id":    userInfoResp.Data.Id,
		"name":  userInfoResp.Data.DisplayName,
		"email": userInfoResp.Data.Email,
	})
}

// Logout 登出
// @Summary      用户登出
// @Description  清除用户会话
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Security     CSRFToken
// @Success      200  {object}  map[string]interface{}  "登出成功"
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	sess.Save()

	util.SuccessWithMessage(c, nil, "登出成功")
}

// RefreshSession 刷新会话
// @Summary      刷新会话Token
// @Description  使用RefreshToken刷新AccessToken
// @Tags         认证
// @Accept       json
// @Produce      json
// @Security     SessionAuth
// @Success      200  {object}  map[string]interface{}  "刷新成功"
// @Failure      401  {object}  map[string]interface{}  "会话过期"
// @Failure      500  {object}  map[string]interface{}  "内部错误"
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshSession(c *gin.Context) {
	sess := sessions.Default(c)
	sessionData := sess.Get("user")

	if sessionData == nil {
		util.Unauthorized(c, "未登录")
		return
	}

	userSession, ok := sessionData.(*session.UserSession)
	if !ok {
		util.ServerError(c, "会话数据格式错误")
		return
	}

	// 如果RefreshToken也过期了，需要重新登录
	if userSession.IsRefreshExpired() {
		util.Unauthorized(c, "会话已过期，请重新登录")
		return
	}

	var newSession *session.UserSession
	if h.service.TokenManager != nil {
		var err error
		newSession, err = h.service.TokenManager.Refresh(c.Request.Context(), userSession.AdvertiserID)
		if err != nil {
			log.Printf("Refresh token failed: %v", err)
			util.ServerError(c, "刷新会话失败")
			return
		}
	} else {
		refreshResp, err := h.service.Client.OauthRefreshToken(c.Request.Context(), sdk.OauthRefreshTokenReq{
			RefreshToken: userSession.RefreshToken,
		})
		if err != nil {
			log.Printf("Refresh token failed: %v", err)
			util.ServerError(c, "刷新会话失败")
			return
		}
		newSession = session.NewSessionFromTokenResponse(&session.TokenResponse{
			AccessToken:           refreshResp.Data.AccessToken,
			RefreshToken:          refreshResp.Data.RefreshToken,
			ExpiresIn:             refreshResp.Data.ExpiresIn,
			RefreshTokenExpiresIn: refreshResp.Data.RefreshTokenExpiresIn,
		}, userSession.AdvertiserID)
	}

	sess.Set("user", newSession)
	if err := sess.Save(); err != nil {
		log.Printf("Save session failed: %v", err)
		util.ServerError(c, "保存会话失败")
		return
	}

	util.Success(c, gin.H{
		"expires_at": newSession.ExpiresAt,
	})
}
