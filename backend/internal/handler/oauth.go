package handler

import (
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/CriarBrand/qianchuan-backend/internal/authstate"
	"github.com/CriarBrand/qianchuan-backend/internal/mcp"
	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
	"github.com/CriarBrand/qianchuan-backend/pkg/tenantstore"
	"github.com/gin-gonic/gin"
)

type OAuthHandler struct {
	client      sdk.QianchuanClient
	appID       int64
	redirectURI string
	tenants     *tenantstore.Store
	states      *authstate.Store
	tokens      *mcp.TenantFactory
}

func NewOAuthHandler(
	client sdk.QianchuanClient,
	appID int64,
	redirectURI string,
	tenants *tenantstore.Store,
	states *authstate.Store,
	tokens *mcp.TenantFactory,
) *OAuthHandler {
	return &OAuthHandler{
		client:      client,
		appID:       appID,
		redirectURI: redirectURI,
		tenants:     tenants,
		states:      states,
		tokens:      tokens,
	}
}

func (h *OAuthHandler) AuthorizationURL(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		h.renderResult(c, http.StatusBadRequest, "授权失败", "缺少租户授权 state")
		return
	}
	if _, err := h.states.Peek(state); err != nil {
		h.renderResult(c, http.StatusBadRequest, "授权失败", "授权链接已失效，请回到 MCP 重新获取")
		return
	}
	target := fmt.Sprintf(
		"https://qianchuan.jinritemai.com/openapi/qc/audit/oauth.html?app_id=%d&state=%s&material_auth=1&redirect_uri=%s",
		h.appID,
		url.QueryEscape(state),
		url.QueryEscape(h.redirectURI),
	)
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, target)
}

func (h *OAuthHandler) Callback(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	tenantID, accountID, err := h.states.PeekAccount(state)
	if err != nil {
		h.renderResult(c, http.StatusBadRequest, "授权失败", "state 无效、过期或已使用")
		return
	}
	fail := func(status int, title, detail string) {
		h.states.Discard(state)
		h.renderResult(c, status, title, detail)
	}
	tenant, err := h.tenants.Get(tenantID)
	if err != nil {
		fail(http.StatusForbidden, "授权失败", "租户不存在")
		return
	}
	code := strings.TrimSpace(c.Query("auth_code"))
	if code == "" {
		code = strings.TrimSpace(c.Query("code"))
	}
	if code == "" {
		h.renderResult(c, http.StatusBadRequest, "授权失败", "回调缺少 auth_code")
		return
	}
	if _, err := h.states.Consume(state); err != nil {
		h.renderResult(c, http.StatusBadRequest, "授权失败", "state 无效、过期或已使用")
		return
	}

	tokenResponse, err := h.client.OauthAccessToken(c.Request.Context(), sdk.OauthAccessTokenReq{
		AuthCode: code,
	})
	if err != nil || tokenResponse == nil || tokenResponse.Code != 0 || tokenResponse.Data == nil {
		log.Printf("MCP OAuth exchange failed: %v", err)
		fail(http.StatusBadGateway, "授权失败", "千川授权码交换失败")
		return
	}
	if err := h.tokens.SaveOAuthSession(c.Request.Context(), *tenant, tokenResponse.Data, accountID); err != nil {
		if errors.Is(err, mcp.ErrTenantAccountMismatch) {
			fail(http.StatusForbidden, "授权拒绝", "本次授权的广告主与租户绑定账户不一致")
			return
		}
		log.Printf("MCP OAuth token persistence failed: %v", err)
		fail(http.StatusInternalServerError, "授权失败", "租户授权凭证保存失败")
		return
	}
	h.renderResult(c, http.StatusOK, "千川授权完成", fmt.Sprintf(
		"租户 %s 已写入本次授权中属于该租户的广告账户凭证",
		tenant.Name,
	))
}

func (h *OAuthHandler) renderResult(c *gin.Context, status int, title, detail string) {
	c.Header("Cache-Control", "no-store")
	c.Data(status, "text/html; charset=utf-8", []byte(oauthResultHTML(title, detail)))
}

func oauthResultHTML(title, detail string) string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` +
		html.EscapeString(title) + `</title><style>body{margin:0;background:#f5f3ee;color:#1b1f21;font-family:"Microsoft YaHei",sans-serif;display:grid;place-items:center;min-height:100vh}.panel{width:min(560px,calc(100% - 48px));background:#fff;border:1px solid #ddd8cc;border-radius:20px;padding:36px;box-shadow:0 18px 60px -40px #111}h1{margin:0 0 12px;font-size:24px}.status{display:inline-block;padding:6px 10px;border-radius:8px;background:#fff2df;color:#9a4d00;font-size:12px;font-weight:700;margin-bottom:20px}p{margin:0;color:#5f6368;line-height:1.7}code{display:block;margin-top:18px;padding:14px;background:#f7f6f2;border-radius:10px;color:#303633;font-size:13px}</style></head><body><main class="panel"><div class="status">千川 OAuth</div><h1>` +
		html.EscapeString(title) + `</h1><p>` + html.EscapeString(detail) + `</p><code>现在可关闭此页，回到 MCP 客户端调用 qianchuan_get_status。</code></main></body></html>`
}
