package middleware

import (
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 返回一个 Gin 中间件，为每个 HTTP 响应添加核心安全响应头。
// 包含：HSTS、CSP、X-Frame-Options、X-Content-Type-Options、Referrer-Policy、Permissions-Policy
//
// CSP 策略可通过环境变量 CSP_DIRECTIVES 自定义（覆盖默认值），
// 便于后续根据实际引入的第三方脚本动态调整。
func SecurityHeaders() gin.HandlerFunc {
	// 从环境变量读取自定义 CSP 值；若未设置则使用安全的默认策略
	cspDirectives := os.Getenv("CSP_DIRECTIVES")
	if cspDirectives == "" {
		cspDirectives = defaultCSP()
	}

	return func(c *gin.Context) {
		// HSTS：强制浏览器使用 HTTPS 访问，有效期 1 年，包含子域名，支持 preload 列表提交
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content-Security-Policy：限制页面可加载资源的来源，防止 XSS 攻击
		c.Header("Content-Security-Policy", cspDirectives)

		// X-Frame-Options：禁止页面被 iframe/frame 嵌套，防止点击劫持
		c.Header("X-Frame-Options", "DENY")

		// X-Content-Type-Options：禁止浏览器对响应内容进行 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer-Policy：跨域请求仅发送 origin（不含路径和查询字符串），保护用户隐私
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy：禁用摄像头/麦克风/地理位置等敏感浏览器 API（按需开启）
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		c.Next()
	}
}

// defaultCSP 返回与当前前端架构兼容的 Content-Security-Policy 默认值。
//
// 说明：
//   - unsafe-inline / unsafe-eval：兼容现有 React SPA 的内联样式和动态 eval 调用，
//     后续应通过 nonce/hash 方案逐步移除。
//   - connect-src 'self' https://api.cloudflare.com：放行 Cloudflare Turnstile 验证接口。
//   - img-src data: https:：允许 data URI 图片及外部 CDN 图片加载。
//   - frame-ancestors 'none'：等效于 X-Frame-Options: DENY，作为现代替代。
func defaultCSP() string {
	// 检查是否在开发环境（dev server 通常需要额外放宽限制）
	isDev := false
	if env := os.Getenv("RSBUILD_DEV_SERVER"); env != "" {
		isDev = true
	}
	// 也检查 NODE_ENV 作为备选判断
	if env := os.Getenv("NODE_ENV"); env == "development" {
		isDev = true
	}

	connectSrc := "'self' https://api.cloudflare.com"
	if isDev {
		// 开发环境需要允许 WebSocket 连接（HMR）和本地 dev server
		connectSrc += " ws: wss: http://localhost:* http://127.0.0.1:*"
	}

	// 构建完整 CSP 指令
	directives := []struct {
		key   string
		value string
	}{
		{"default-src", "'self'"},
		{"script-src", "'self' 'unsafe-inline' 'unsafe-eval'"},
		{"style-src", "'self' 'unsafe-inline'"},
		{"img-src", "'self' data: blob: https:"},
		{"font-src", "'self' data:"},
		{"connect-src", connectSrc},
		{"frame-ancestors", "'none'"},
		{"base-uri", "'self'"},
		{"form-action", "'self'"},
	}

	result := ""
	for i, d := range directives {
		if i > 0 {
			result += "; "
		}
		result += d.key + " " + d.value
	}
	return result
}

// GetHSTSMaxAge 返回 HSTS max-age 配置值（单位：秒），用于 Nginx 层同步配置。
// 可通过环境变量 HSTS_MAX_AGE 覆盖默认值（31536000 = 1年）
func GetHSTSMaxAge() int {
	if v := os.Getenv("HSTS_MAX_AGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 31536000
}
