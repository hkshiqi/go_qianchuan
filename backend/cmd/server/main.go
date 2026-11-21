package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/CriarBrand/qianchuan-backend/docs"
	"github.com/CriarBrand/qianchuan-backend/internal/authstate"
	"github.com/CriarBrand/qianchuan-backend/internal/handler"
	"github.com/CriarBrand/qianchuan-backend/internal/mcp"
	"github.com/CriarBrand/qianchuan-backend/internal/middleware"
	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/CriarBrand/qianchuan-backend/internal/tokenmanager"
	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
	"github.com/CriarBrand/qianchuan-backend/pkg/tokenstore"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	_ = godotenv.Load()

	port := envString("PORT", "18081")
	publicPathPrefix := normalizePublicPathPrefix(envString("PUBLIC_PATH_PREFIX", ""))
	appID, err := strconv.ParseInt(envString("APP_ID", ""), 10, 64)
	if err != nil || appID <= 0 {
		log.Fatal("APP_ID must be a valid positive integer")
	}
	appSecret := envString("APP_SECRET", "")
	if appSecret == "" {
		log.Fatal("APP_SECRET is required")
	}
	if envString("APP_ACCESS_USERNAME", "") == "" || len(envString("APP_ACCESS_PASSWORD_SHA256", "")) != 64 {
		log.Fatal("APP_ACCESS_USERNAME and a 64-character APP_ACCESS_PASSWORD_SHA256 are required")
	}

	gin.SetMode(envString("GIN_MODE", gin.ReleaseMode))
	router := buildRouter(appID, appSecret, publicPathPrefix)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("qianchuan backend listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func buildRouter(appID int64, appSecret string, publicPathPrefix string) *gin.Engine {
	publicPathPrefix = normalizePublicPathPrefix(publicPathPrefix)
	client := sdk.NewOceanengineClient(appID, appSecret)
	client.SetMinRequestInterval(time.Duration(envInt64("QIANCHUAN_MIN_REQUEST_INTERVAL_MS", 250)) * time.Millisecond)
	qianchuanService := service.NewQianchuanService(client, appID, appSecret)
	tokenStorePath := envString("TOKEN_STORE_PATH", "data/tokens.json")
	tokenEncryptionKey := envString("TOKEN_ENCRYPTION_KEY", "")
	tokenStore, err := tokenstore.New(tokenStorePath, tokenEncryptionKey)
	if err != nil {
		log.Fatalf("initialize token store: %v", err)
	}
	tokenManager, err := tokenmanager.New(client, tokenStore)
	if err != nil {
		log.Fatalf("initialize token manager: %v", err)
	}
	qianchuanService.TokenManager = tokenManager
	qianchuanService.DefaultAdvertiserID = envInt64("QIANCHUAN_DEFAULT_ADVERTISER_ID", 0)
	qianchuanService.DefaultAwemeID = envInt64("QIANCHUAN_DEFAULT_AWEME_ID", 0)
	qianchuanService.DefaultLiveRoomID = envInt64("QIANCHUAN_DEFAULT_LIVE_ROOM_ID", 0)
	operationsWriteEnabled := envBool("OPERATIONS_WRITE_ENABLED", false)
	materialChaseWriteEnabled := envBool("MATERIAL_CHASE_WRITE_ENABLED", false)
	planCRUDTestWriteEnabled := envBool("PLAN_CRUD_TEST_WRITE_ENABLED", false)
	tenants, err := tenantstore.New(
		envString("TENANT_STORE_PATH", "data/tenants.json"),
		envString("TENANT_ENCRYPTION_KEY", ""),
	)
	if err != nil {
		log.Fatalf("initialize tenant store: %v", err)
	}
	authStates := authstate.New(10 * time.Minute)
	oauthAuthorizationURL := envString("OAUTH_PUBLIC_ORIGIN", "https://dkl4ii67.zjz-service.cn") +
		publicPathPrefix + "/oauth/authorize"
	tenantFactory, err := mcp.NewTenantFactory(mcp.TenantFactoryOptions{
		Client:                    client,
		AppID:                     appID,
		AppSecret:                 appSecret,
		TokenRoot:                 mcp.TenantTokenRoot(tokenStorePath),
		TokenEncryptionKey:        tokenEncryptionKey,
		WriteEnabled:              operationsWriteEnabled,
		MaterialChaseWriteEnabled: materialChaseWriteEnabled,
		PlanCRUDTestWriteEnabled:  planCRUDTestWriteEnabled,
		OAuthAuthorizationURL:     oauthAuthorizationURL,
		IssueAuthState:            authStates.Issue,
		IssueAccountAuthState:     authStates.IssueForAccount,
	})
	if err != nil {
		log.Fatalf("initialize tenant runtime factory: %v", err)
	}

	authHandler := handler.NewAuthHandler(qianchuanService)
	accessHandler := handler.NewAccessHandler()
	advertiserHandler := handler.NewAdvertiserHandler(qianchuanService)
	campaignHandler := handler.NewCampaignHandler(qianchuanService)
	adHandler := handler.NewAdHandler(qianchuanService)
	creativeHandler := handler.NewCreativeHandler(qianchuanService)
	fileHandler := handler.NewFileHandler(qianchuanService)
	reportHandler := handler.NewReportHandler(qianchuanService)
	liveHandler := handler.NewLiveHandler(qianchuanService)
	toolsHandler := handler.NewToolsHandler(qianchuanService)
	keywordHandler := handler.NewKeywordHandler(qianchuanService)
	financeHandler := handler.NewFinanceHandler(qianchuanService)
	awemeHandler := handler.NewAwemeHandler(qianchuanService)
	uniPromotionHandler := handler.NewUniPromotionHandler(qianchuanService)
	activityHandler := handler.NewActivityHandler()
	webhookHandler := handler.NewWebhookHandler()
	oauthHandler := handler.NewOAuthHandler(
		client,
		appID,
		envString(
			"OAUTH_REDIRECT_URI",
			"https://dkl4ii67.zjz-service.cn"+publicPathPrefix+"/auth/callback",
		),
		tenants,
		authStates,
		tenantFactory,
	)
	mcpServer := mcp.NewServer(tenants, tenantFactory)

	router := gin.New()
	router.Use(gin.Recovery(), middleware.Trace(), middleware.Logger(), middleware.Metrics())
	router.Use(middleware.CORS())

	secret := []byte(envString("COOKIE_SECRET", ""))
	if len(secret) < 32 {
		sum := sha256.Sum256([]byte(envString("APP_ID", "qianchuan") + ":" + envString("APP_SECRET", "local")))
		secret = sum[:]
	}
	encryptionKey := sha256.Sum256(append(append([]byte{}, secret...), []byte("qianchuan-session-encryption")...))
	store := memstore.NewStore(secret, encryptionKey[:])
	store.Options(sessionOptions())
	router.Use(sessions.Sessions(envString("SESSION_NAME", "qianchuan_session"), store))

	registerPublicRoutes := func(root *gin.RouterGroup) {
		root.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Unix()}) })
		root.GET("/healthz", middleware.HealthCheckHandler)
		root.GET("/ready", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ready", "timestamp": time.Now().Unix()}) })
		root.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"version": "1.0.0", "service": "qianchuan-live-operations-mcp"})
		})
		root.GET("/oauth/authorize", oauthHandler.AuthorizationURL)
		root.GET("/auth/callback", oauthHandler.Callback)
		root.POST("/mcp", gin.WrapH(mcpServer))
		root.OPTIONS("/mcp", func(c *gin.Context) {
			c.Header("Allow", "POST, OPTIONS")
			c.Status(http.StatusNoContent)
		})
		root.GET("/metrics", middleware.ExportMetricsHandler)
		root.Any("/webhook/callback", func(c *gin.Context) {
			if c.Request.Method == http.MethodGet {
				webhookHandler.HandleChallenge(c)
				return
			}
			webhookHandler.HandleWebhookEvent(c)
		})
	}

	root := router.Group(publicPathPrefix)
	registerPublicRoutes(root)
	if publicPathPrefix != "" {
		// A dedicated host proxies root paths here, while the shared host keeps
		// the established prefix.
		registerPublicRoutes(&router.RouterGroup)
	}

	docs.SwaggerInfo.BasePath = publicPathPrefix + "/api"
	root.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	if publicPathPrefix != "" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	rootAPI := root.Group("/api")
	registerAPIRoutes(rootAPI, publicPathPrefix, operationsWriteEnabled, qianchuanService,
		authHandler, accessHandler, advertiserHandler, campaignHandler, adHandler,
		creativeHandler, fileHandler, reportHandler, liveHandler, toolsHandler,
		keywordHandler, financeHandler, awemeHandler, uniPromotionHandler, activityHandler)
	if publicPathPrefix != "" {
		registerAPIRoutes(router.Group("/api"), "", operationsWriteEnabled, qianchuanService,
			authHandler, accessHandler, advertiserHandler, campaignHandler, adHandler,
			creativeHandler, fileHandler, reportHandler, liveHandler, toolsHandler,
			keywordHandler, financeHandler, awemeHandler, uniPromotionHandler, activityHandler)
	}

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, publicPathPrefix+"/api/") ||
			(publicPathPrefix != "" && strings.HasPrefix(c.Request.URL.Path, "/api/")) {
			util.NotFound(c, "接口不存在")
			return
		}
		c.String(http.StatusNotFound, "not found")
	})

	return router
}

func registerAPIRoutes(
	api *gin.RouterGroup,
	publicPathPrefix string,
	operationsWriteEnabled bool,
	qianchuanService *service.QianchuanService,
	authHandler *handler.AuthHandler,
	accessHandler *handler.AccessHandler,
	advertiserHandler *handler.AdvertiserHandler,
	campaignHandler *handler.CampaignHandler,
	adHandler *handler.AdHandler,
	creativeHandler *handler.CreativeHandler,
	fileHandler *handler.FileHandler,
	reportHandler *handler.ReportHandler,
	liveHandler *handler.LiveHandler,
	toolsHandler *handler.ToolsHandler,
	keywordHandler *handler.KeywordHandler,
	financeHandler *handler.FinanceHandler,
	awemeHandler *handler.AwemeHandler,
	uniPromotionHandler *handler.UniPromotionHandler,
	activityHandler *handler.ActivityHandler,
) {
	api.GET("/access/status", accessHandler.Status)
	api.POST("/access/login", accessHandler.Login)
	api.POST("/access/logout", accessHandler.Logout)
	api.POST("/auth/refresh", middleware.AccessRequired(), authHandler.RefreshSession)

	protected := api.Group("")
	protected.Use(middleware.AccessRequired(), middleware.AuthRequired(), middleware.AutoRefreshToken(qianchuanService), middleware.CSRFProtection())
	if !operationsWriteEnabled {
		protected.Use(middleware.WriteGuard(false, operationReadOnlyPaths(publicPathPrefix)...))
	}

	protected.GET("/user/info", authHandler.GetUserInfo)
	protected.POST("/auth/logout", authHandler.Logout)

	protected.GET("/advertiser/list", advertiserHandler.List)
	protected.GET("/advertiser/info", advertiserHandler.Info)
	protected.POST("/advertiser/update", advertiserHandler.Update)

	protected.GET("/qianchuan/advertiser/info", advertiserHandler.Info)
	protected.GET("/qianchuan/advertiser/public-info", advertiserHandler.Info)
	protected.GET("/qianchuan/advertiser/budget/get", advertiserHandler.GetBudget)
	protected.POST("/qianchuan/advertiser/budget/update", advertiserHandler.UpdateBudget)
	protected.GET("/qianchuan/advertiser/aweme/authorized", advertiserHandler.GetAuthorizedAwemeList)
	protected.GET("/qianchuan/advertiser/aweme/auth-list", advertiserHandler.GetAwemeAuthList)
	protected.GET("/qianchuan/shop/advertiser/list", advertiserHandler.GetShopAdvertiserList)
	protected.GET("/qianchuan/agent/advertiser/select", advertiserHandler.GetAgentAdvertiserList)

	protected.GET("/qianchuan/campaign/list", campaignHandler.List)
	protected.GET("/qianchuan/campaign/get", campaignHandler.Get)
	protected.POST("/qianchuan/campaign/create", campaignHandler.Create)
	protected.POST("/qianchuan/campaign/update", campaignHandler.Update)
	protected.POST("/qianchuan/campaign/status/update", campaignHandler.UpdateStatus)

	protected.GET("/qianchuan/ad/list", adHandler.List)
	protected.GET("/qianchuan/ad/get", adHandler.Get)
	protected.POST("/qianchuan/ad/create", adHandler.Create)
	protected.POST("/qianchuan/ad/update", adHandler.Update)
	protected.POST("/qianchuan/ad/status/update", adHandler.UpdateStatus)
	protected.POST("/qianchuan/ad/budget/update", adHandler.UpdateBudget)
	protected.POST("/qianchuan/ad/bid/update", adHandler.UpdateBid)
	protected.POST("/qianchuan/ad/region/update", adHandler.UpdateRegion)
	protected.POST("/qianchuan/ad/schedule/date/update", adHandler.UpdateScheduleDate)
	protected.POST("/qianchuan/ad/schedule/time/update", adHandler.UpdateScheduleTime)
	protected.POST("/qianchuan/ad/schedule/fixed-range/update", adHandler.UpdateScheduleFixedRange)
	protected.GET("/qianchuan/ad/low-quality", adHandler.GetLowQualityAds)
	protected.POST("/qianchuan/ad/suggest-bid", adHandler.SuggestBid)
	protected.POST("/qianchuan/ad/suggest-budget", adHandler.SuggestBudget)
	protected.POST("/qianchuan/ad/suggest-roi-goal", adHandler.SuggestRoiGoal)
	protected.POST("/qianchuan/ad/estimate-effect", adHandler.EstimateEffect)
	protected.GET("/qianchuan/ad/reject-reason", creativeHandler.RejectReason)

	protected.GET("/qianchuan/creative/list", creativeHandler.List)
	protected.GET("/qianchuan/creative/get", creativeHandler.Get)
	protected.POST("/qianchuan/creative/create", creativeHandler.Create)
	protected.POST("/qianchuan/creative/status/update", creativeHandler.UpdateStatus)

	protected.POST("/qianchuan/file/image/upload", fileHandler.UploadImage)
	protected.POST("/qianchuan/file/video/upload", fileHandler.UploadVideo)
	protected.GET("/qianchuan/file/image/get", fileHandler.GetImageList)
	protected.GET("/qianchuan/file/video/get", fileHandler.GetVideoList)
	protected.GET("/qianchuan/file/material/title/get", fileHandler.GetMaterialTitleList)

	protected.POST("/qianchuan/report/advertiser/get", reportHandler.GetAdvertiserReport)
	protected.POST("/qianchuan/report/campaign/get", reportHandler.GetCampaignReport)
	protected.POST("/qianchuan/report/ad/get", reportHandler.GetAdReport)
	protected.POST("/qianchuan/report/creative/get", reportHandler.GetCreativeReport)
	protected.POST("/qianchuan/report/material/get", reportHandler.GetMaterialReport)
	protected.POST("/qianchuan/report/search-word/get", reportHandler.GetSearchWordReport)
	protected.POST("/qianchuan/report/video-user-lose/get", reportHandler.GetVideoUserLoseReport)
	protected.GET("/qianchuan/report/custom/config", reportHandler.GetCustomReportConfig)
	protected.GET("/qianchuan/report/custom/get", reportHandler.GetCustomReport)

	protected.GET("/qianchuan/report/live/get", liveHandler.GetLiveStats)
	protected.GET("/qianchuan/live/room/get", liveHandler.GetLiveRooms)
	protected.GET("/qianchuan/live/room/detail/get", liveHandler.GetLiveRoomDetail)
	protected.GET("/qianchuan/live/room/flow-performance/get", liveHandler.GetLiveRoomFlowPerformance)
	protected.GET("/qianchuan/live/room/user/get", liveHandler.GetLiveRoomUserInsight)
	protected.GET("/qianchuan/live/room/product-list/get", liveHandler.GetLiveRoomProducts)

	protected.GET("/qianchuan/tools/industry", toolsHandler.GetIndustry)
	protected.GET("/qianchuan/tools/interest/category", toolsHandler.GetInterestCategory)
	protected.POST("/qianchuan/tools/interest/keyword", toolsHandler.GetInterestKeyword)
	protected.POST("/qianchuan/tools/action/category", toolsHandler.GetActionCategory)
	protected.POST("/qianchuan/tools/action/keyword", toolsHandler.GetActionKeyword)
	protected.POST("/qianchuan/tools/aweme/category", toolsHandler.GetAwemeCategory)
	protected.POST("/qianchuan/tools/aweme/author", toolsHandler.GetAwemeAuthorInfo)
	protected.POST("/qianchuan/tools/creative/word", toolsHandler.GetCreativeWord)
	protected.GET("/qianchuan/tools/audience/list", toolsHandler.GetAudienceList)

	protected.GET("/qianchuan/keyword/package/list", keywordHandler.GetKeywordPackage)
	protected.GET("/qianchuan/keyword/recommend", keywordHandler.GetRecommendKeywords)
	protected.POST("/qianchuan/keyword/recommend", keywordHandler.GetRecommendKeywords)
	protected.POST("/qianchuan/keyword/compliance/check", keywordHandler.CheckKeywords)
	protected.GET("/qianchuan/keyword/negative/list", keywordHandler.GetNegativeKeywords)
	protected.POST("/qianchuan/keyword/negative/update", keywordHandler.UpdateNegativeKeywords)

	protected.GET("/qianchuan/finance/wallet/get", financeHandler.GetWallet)
	protected.GET("/qianchuan/advertiser/balance/get", financeHandler.GetBalance)
	protected.GET("/qianchuan/finance/detail/get", financeHandler.GetFinanceDetail)
	protected.POST("/qianchuan/agent/fund/transfer-seq/create", financeHandler.CreateTransferSeq)
	protected.POST("/qianchuan/agent/fund/transfer-seq/commit", financeHandler.CommitTransferSeq)
	protected.POST("/qianchuan/agent/refund/transfer-seq/create", financeHandler.CreateRefundSeq)
	protected.POST("/qianchuan/agent/refund/transfer-seq/commit", financeHandler.CommitRefundSeq)

	protected.GET("/qianchuan/aweme/order/get", awemeHandler.GetOrderList)
	protected.GET("/qianchuan/aweme/order/detail/get", awemeHandler.GetOrderDetail)
	protected.POST("/qianchuan/aweme/order/create", awemeHandler.CreateOrder)
	protected.POST("/qianchuan/aweme/order/terminate", awemeHandler.TerminateOrder)
	protected.GET("/qianchuan/aweme/video/get", awemeHandler.GetVideoList)
	protected.POST("/qianchuan/aweme/order/budget/add", awemeHandler.AddBudget)
	protected.POST("/qianchuan/aweme/suggest/bid", awemeHandler.GetSuggestBid)
	protected.POST("/qianchuan/aweme/estimate", awemeHandler.GetEstimate)
	protected.GET("/qianchuan/aweme/quota", awemeHandler.GetQuota)

	protected.GET("/qianchuan/activity/list", activityHandler.List)

	registerUniPromotionRoutes(protected, uniPromotionHandler)
	registerUnsupportedRoutes(protected)
}

func normalizePublicPathPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	return "/" + strings.Trim(value, "/")
}

func registerUniPromotionRoutes(group *gin.RouterGroup, h *handler.UniPromotionHandler) {
	group.GET("/qianchuan/uni-promotion/list", h.List)
	group.GET("/qianchuan/uni-promotion/detail", h.GetDetail)
	group.POST("/qianchuan/uni-promotion/create", h.Create)
	group.POST("/qianchuan/uni-promotion/update", h.Update)
	group.POST("/qianchuan/uni-promotion/status/update", h.UpdateStatus)
	group.POST("/qianchuan/uni-promotion/ad/name/update", h.Update)
	group.POST("/qianchuan/uni-promotion/ad/budget/update", h.UpdateBudget)
	group.POST("/qianchuan/uni-promotion/ad/roi-goal/update", h.UpdateROIGoal)
	group.POST("/qianchuan/uni-promotion/ad/schedule-date/update", h.UpdateScheduleTime)
	group.GET("/qianchuan/uni-promotion/material/get", h.GetMaterials)
	group.POST("/qianchuan/uni-promotion/material/delete", h.UpdateMaterial)
	group.POST("/qianchuan/uni-promotion/material/add-existing", h.AddExistingMaterial)
	group.GET("/qianchuan/uni-promotion/aweme/authorized", h.GetBudgetSchedule)
	group.POST("/qianchuan/uni-promotion/auth-init", h.UpdateAuth)
}

var operationReadOnlyPOSTPaths = []string{
	"/api/auth/logout",
	"/api/qianchuan/report/advertiser/get",
	"/api/qianchuan/report/campaign/get",
	"/api/qianchuan/report/ad/get",
	"/api/qianchuan/report/creative/get",
	"/api/qianchuan/report/material/get",
	"/api/qianchuan/report/search-word/get",
	"/api/qianchuan/report/video-user-lose/get",
	"/api/qianchuan/tools/interest/keyword",
	"/api/qianchuan/tools/action/category",
	"/api/qianchuan/tools/action/keyword",
	"/api/qianchuan/tools/aweme/category",
	"/api/qianchuan/tools/aweme/author",
	"/api/qianchuan/tools/creative/word",
	"/api/qianchuan/keyword/recommend",
	"/api/qianchuan/keyword/compliance/check",
	"/api/qianchuan/ad/suggest-bid",
	"/api/qianchuan/ad/suggest-budget",
	"/api/qianchuan/ad/suggest-roi-goal",
	"/api/qianchuan/ad/estimate-effect",
	"/api/qianchuan/aweme/suggest/bid",
	"/api/qianchuan/aweme/estimate",
}

func operationReadOnlyPaths(prefix string) []string {
	result := make([]string, 0, len(operationReadOnlyPOSTPaths))
	for _, item := range operationReadOnlyPOSTPaths {
		result = append(result, prefix+item)
	}
	return result
}

func registerUnsupportedRoutes(group *gin.RouterGroup) {
	unsupported := func(feature string) gin.HandlerFunc {
		return func(c *gin.Context) {
			util.NotImplemented(c, feature+"暂未实现", "当前上游版本没有对应的后端处理器，页面会保留功能入口但不执行平台写操作。")
		}
	}

	for _, route := range []struct {
		method  string
		path    string
		feature string
	}{
		{http.MethodGet, "/qianchuan/agent/info", "代理商信息"},
		{http.MethodGet, "/qianchuan/shop/get", "店铺信息"},
		{http.MethodGet, "/qianchuan/advertiser/type", "广告主类型"},
		{http.MethodPost, "/qianchuan/tools/aweme/auth", "抖音号授权"},
		{http.MethodPost, "/qianchuan/tools/shop/auth", "店铺授权"},
		{http.MethodGet, "/qianchuan/ad/learning-status", "学习期状态"},
		{http.MethodGet, "/qianchuan/ad/compensate-status", "成本保障状态"},
		{http.MethodGet, "/qianchuan/ad/quota", "计划配额"},
		{http.MethodPost, "/qianchuan/creative/update", "创意更新"},
		{http.MethodGet, "/qianchuan/file/video/aweme/get", "抖音号视频素材"},
		{http.MethodGet, "/qianchuan/file/video/original/get", "原创视频素材"},
		{http.MethodGet, "/qianchuan/file/video/ineffective/get", "低效视频素材"},
		{http.MethodPost, "/qianchuan/file/image/delete", "图片删除"},
		{http.MethodPost, "/qianchuan/file/video/delete", "视频删除"},
		{http.MethodGet, "/qianchuan/carousel/get", "图文素材"},
		{http.MethodGet, "/qianchuan/carousel/aweme/get", "抖音号图文素材"},
		{http.MethodPost, "/qianchuan/material/suggestion/get", "素材建议"},
		{http.MethodPost, "/qianchuan/material/ad/get", "计划素材"},
		{http.MethodPost, "/qianchuan/keyword/package/create", "词包创建"},
		{http.MethodPost, "/qianchuan/keyword/package/update", "词包更新"},
		{http.MethodGet, "/qianchuan/keyword/system/recommend", "系统推荐词"},
		{http.MethodGet, "/qianchuan/product/analyse/list", "商品分析"},
		{http.MethodPost, "/qianchuan/product/analyse/compare-stats", "商品对比分析"},
		{http.MethodGet, "/qianchuan/product/analyse/compare-creative", "商品创意对比"},
		{http.MethodGet, "/qianchuan/product/analyse/summary", "商品分析汇总"},
		{http.MethodPost, "/qianchuan/dmp/audience/create", "人群包创建"},
		{http.MethodPost, "/qianchuan/dmp/audience/update", "人群包更新"},
		{http.MethodPost, "/qianchuan/dmp/audience/delete", "人群包删除"},
	} {
		switch route.method {
		case http.MethodGet:
			group.GET(route.path, unsupported(route.feature))
		case http.MethodPost:
			group.POST(route.path, unsupported(route.feature))
		}
	}
}

func sessionOptions() sessions.Options {
	maxAge, _ := strconv.Atoi(envString("SESSION_TTL", "86400"))
	return sessions.Options{
		Path:     "/",
		Domain:   envString("COOKIE_DOMAIN", ""),
		MaxAge:   maxAge,
		Secure:   envBool("COOKIE_SECURE", false),
		HttpOnly: envBool("COOKIE_HTTP_ONLY", true),
		SameSite: parseSameSite(envString("COOKIE_SAME_SITE", "lax")),
	}
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Fatalf("%s must be a valid integer", key)
	}
	return parsed
}
