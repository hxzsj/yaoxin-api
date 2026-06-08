package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
	ClassicBuildFS   embed.FS
	ClassicIndexPage []byte
}

// robotsTxtContent 限制搜索引擎爬虫抓取敏感路径。
// 同时在 web/default/public/robots.txt 和 web/classic/public/robots.txt 中维护副本，
// 前端构建后会被复制到 dist/ 目录，由 static.Serve 自动服务。
var robotsTxtContent = `# https://www.robotstxt.org/robotstxt.html
#
# 曜芯API 搜索引擎爬虫规则
# - 允许抓取公开页面（首页、关于页等）
# - 禁止抓取 API 端点、管理后台、认证路径等敏感位置

User-agent: *
Allow: /

# API 端点（含 OpenAI 兼容接口）
Disallow: /api/
Disallow: /v1/

# 管理后台与面板
Disallow: /admin/
Disallow: /pg/
Disallow: /dashboard/
Disallow: /panel/

# 认证相关路径（防止登录/注册页面被收录泄露系统信息）
Disallow: /login
Disallow: /register

# Sitemap（部署时请替换为实际域名）
Sitemap: https://www.yaoxinapi.com/sitemap.xml
`

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")
	themeFS := common.NewThemeAwareFS(defaultFS, classicFS)

	// robots.txt：防止搜索引擎爬取敏感路径（API、管理后台、认证页面等）
	router.GET("/robots.txt", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, robotsTxtContent)
	})

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", themeFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		path := c.Request.URL.Path
		// 仅对真实 API 路径（带尾部斜杠的前缀）返回 JSON 错误响应
		// /v1/chat/completions 等 OpenAI 兼容接口
		// /api/user/login 等内部 API
		// 排除 /api-docs、/api-status 等非 API 路径泄露指纹
		if (strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/assets")) &&
			!strings.HasSuffix(path, "-docs") {
			controller.RelayNotFound(c)
			return
		}
		// 非 API 路径 → 返回 SPA index.html 兜底（防止暴露 OpenAI JSON 结构）
		c.Header("Cache-Control", "no-cache")
		if common.GetTheme() == "classic" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}
