package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/seefs001/wechat-server/config"
	"github.com/seefs001/wechat-server/store"
)

// AuthMiddleware API Token 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// 如果未配置 token，跳过验证
		if cfg.Server.APIToken == "" {
			c.Next()
			return
		}

		// 从 Authorization 头获取 token
		auth := c.GetHeader("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		token = strings.TrimSpace(token)

		if token != cfg.Server.APIToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未授权访问",
				"data":    "",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUser 验证验证码并返回用户信息（Yi-API 调用）
func GetUser(c *gin.Context) {
	code := c.Query("code")
	appID := c.Query("app_id") // 可选，用于多公众号模式

	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码不能为空",
			"data":    "",
		})
		return
	}

	// 验证验证码
	openID := store.GetStore().VerifyCode(code, appID)
	if openID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码错误或已过期",
			"data":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    openID,
	})
}

// GetStats 获取服务状态统计
func GetStats(c *gin.Context) {
	codeCount, userCount := store.GetStore().Stats()
	cfg := config.Get()

	accounts := make([]gin.H, 0, len(cfg.Accounts))
	for _, acc := range cfg.Accounts {
		accounts = append(accounts, gin.H{
			"app_id": acc.AppID,
			"name":   acc.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"active_codes": codeCount,
			"active_users": userCount,
			"accounts":     accounts,
		},
	})
}
