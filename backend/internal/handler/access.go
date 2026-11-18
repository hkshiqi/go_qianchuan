package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/middleware"
	"github.com/CriarBrand/qianchuan-backend/internal/util"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type accessAttempt struct {
	Count   int
	ResetAt time.Time
}

type AccessHandler struct {
	username     string
	passwordHash string
	mu           sync.Mutex
	attempts     map[string]accessAttempt
}

func NewAccessHandler() *AccessHandler {
	return &AccessHandler{
		username:     strings.TrimSpace(os.Getenv("APP_ACCESS_USERNAME")),
		passwordHash: strings.ToLower(strings.TrimSpace(os.Getenv("APP_ACCESS_PASSWORD_SHA256"))),
		attempts:     make(map[string]accessAttempt),
	}
}

func (h *AccessHandler) Status(c *gin.Context) {
	authenticated, _ := sessions.Default(c).Get(middleware.AccessSessionKey).(bool)
	util.Success(c, gin.H{"authenticated": authenticated})
}

func (h *AccessHandler) Login(c *gin.Context) {
	if !h.allowAttempt(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "尝试次数过多，请 5 分钟后再试",
		})
		return
	}

	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请输入账号和密码")
		return
	}

	usernameHash := sha256.Sum256([]byte(strings.TrimSpace(req.Username)))
	expectedUsernameHash := sha256.Sum256([]byte(h.username))
	passwordHash := sha256.Sum256([]byte(req.Password))
	expectedPasswordHash, err := hex.DecodeString(h.passwordHash)

	usernameOK := subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1
	passwordOK := err == nil && len(expectedPasswordHash) == sha256.Size &&
		subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash) == 1

	if !usernameOK || !passwordOK {
		util.Unauthorized(c, "账号或密码错误")
		return
	}

	h.clearAttempts(c.ClientIP())
	sess := sessions.Default(c)
	sess.Set(middleware.AccessSessionKey, true)
	if err := sess.Save(); err != nil {
		util.ServerError(c, "保存访问会话失败")
		return
	}

	util.Success(c, gin.H{"success": true})
}

func (h *AccessHandler) Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	_ = sess.Save()
	util.Success(c, gin.H{"success": true})
}

func (h *AccessHandler) allowAttempt(ip string) bool {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()

	record := h.attempts[ip]
	if record.ResetAt.Before(now) {
		record = accessAttempt{ResetAt: now.Add(5 * time.Minute)}
	}
	if record.Count >= 10 {
		h.attempts[ip] = record
		return false
	}
	record.Count++
	h.attempts[ip] = record
	return true
}

func (h *AccessHandler) clearAttempts(ip string) {
	h.mu.Lock()
	delete(h.attempts, ip)
	h.mu.Unlock()
}
