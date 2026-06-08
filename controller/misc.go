package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

func TestStatus(c *gin.Context) {
	err := model.PingDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "数据库连接失败",
		})
		return
	}
	// 获取HTTP统计信息
	httpStats := middleware.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Server is running",
		"http_stats": httpStats,
	})
	return
}

// GetStatus 返回系统状态信息。采用认证感知双层返回策略：
//   - 未认证用户：仅返回登录/注册页面渲染所需的公开字段（~33 个），收敛攻击面
//   - 已认证用户：返回完整系统配置（含定价、模块开关、版本号等）
//
// OAuth Client ID、Turnstile Site Key、Passkey RP ID 等字段属于协议级公开参数
//（客户端必须持有才能发起 OAuth 流程 / WebAuthn 注册），因此保留在公共字段中。
func GetStatus(c *gin.Context) {

	cs := console_setting.GetConsoleSetting()
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	passkeySetting := system_setting.GetPasskeySettings()
	legalSetting := system_setting.GetLegalSettings()

	// ====== 公共字段（未认证用户可见）======
	// 包含：品牌 UI、认证方式开关 + 协议级公开凭据、注册控制、法律文档、安装向导
	data := gin.H{
		// 品牌 & UI
		"system_name": common.SystemName,
		"logo":        common.Logo,
		"footer_html": common.Footer,
		"theme":       system_setting.GetThemeSettings().Frontend,

		// 认证方式开关 + 公开凭据（Client ID 等为 OAuth/WebAuthn 协议要求的公开参数）
		"email_verification":          common.EmailVerificationEnabled,
		"github_oauth":                common.GitHubOAuthEnabled,
		"github_client_id":            common.GitHubClientId,
		"discord_oauth":               system_setting.GetDiscordSettings().Enabled,
		"discord_client_id":           system_setting.GetDiscordSettings().ClientId,
		"linuxdo_oauth":               common.LinuxDOOAuthEnabled,
		"linuxdo_client_id":           common.LinuxDOClientId,
		"telegram_oauth":              common.TelegramOAuthEnabled,
		"telegram_bot_name":           common.TelegramBotName,
		"wechat_login":                common.WeChatAuthEnabled,
		"wechat_qrcode":               common.WeChatAccountQRCodeImageURL,
		"oidc_enabled":                system_setting.GetOIDCSettings().Enabled,
		"oidc_client_id":              system_setting.GetOIDCSettings().ClientId,
		"oidc_authorization_endpoint": system_setting.GetOIDCSettings().AuthorizationEndpoint,

		// Passkey（WebAuthn 公开配置 — navigator.credentials.create 需要 RP ID）
		"passkey_login":             passkeySetting.Enabled,
		"passkey_display_name":      passkeySetting.RPDisplayName,
		"passkey_rp_id":             passkeySetting.RPID,
		"passkey_origins":           passkeySetting.Origins,
		"passkey_allow_insecure":    passkeySetting.AllowInsecureOrigin,
		"passkey_user_verification": passkeySetting.UserVerification,
		"passkey_attachment":        passkeySetting.AttachmentPreference,

		// Turnstile（Site Key 为公开参数，Secret Key 始终不暴露）
		"turnstile_check":    common.TurnstileCheckEnabled,
		"turnstile_site_key": common.TurnstileSiteKey,

		// 注册控制
		"register_enabled":          common.RegisterEnabled,
		"password_register_enabled": common.PasswordRegisterEnabled,
		"demo_site_enabled":         operation_setting.DemoSiteEnabled,
		"self_use_mode_enabled":     operation_setting.SelfUseModeEnabled,

		// 法律文档
		"user_agreement_enabled": legalSetting.UserAgreement != "",
		"privacy_policy_enabled":  legalSetting.PrivacyPolicy != "",

		// API 地址（前端构建 OAuth 回调 URL 及 API Keys 复制需要）
		"server_address": system_setting.ServerAddress,

		// 安装向导状态
		"setup": constant.Setup,
	}

	// 注入自定义 OAuth 提供商（登录页 OAuth 按钮渲染依赖 client_id 和 authorization_endpoint）
	customProviders := oauth.GetEnabledCustomProviders()
	if len(customProviders) > 0 {
		type CustomOAuthInfo struct {
			Id                    int    `json:"id"`
			Name                  string `json:"name"`
			Slug                  string `json:"slug"`
			Icon                  string `json:"icon"`
			ClientId              string `json:"client_id"`
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			Scopes                string `json:"scopes"`
		}
		providersInfo := make([]CustomOAuthInfo, 0, len(customProviders))
		for _, p := range customProviders {
			config := p.GetConfig()
			providersInfo = append(providersInfo, CustomOAuthInfo{
				Id:                    config.Id,
				Name:                  config.Name,
				Slug:                  config.Slug,
				Icon:                  config.Icon,
				ClientId:              config.ClientId,
				AuthorizationEndpoint: config.AuthorizationEndpoint,
				Scopes:                config.Scopes,
			})
		}
		data["custom_oauth_providers"] = providersInfo
	}

	// ====== 未认证用户仅返回公共子集 ======
	if !middleware.IsAuthenticatedUser(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    data,
		})
		return
	}

	// ====== 已认证用户：追加完整运营数据 ======

	// 版本与运行时信息（攻击者可据此匹配已知漏洞或判断重启周期）
	data["version"] = common.Version
	data["start_time"] = common.StartTime

	// LinuxDO 社区信任等级门槛
	data["linuxdo_minimum_trust_level"] = common.LinuxDOMinimumTrustLevel
	data["default_use_auto_group"] = setting.DefaultUseAutoGroup

	// 文档链接 & 定价策略（运营情报，无需向匿名用户暴露）
	data["docs_link"] = operation_setting.GetGeneralSetting().DocsLink

	// 额度与计费配置
	data["quota_per_unit"] = common.QuotaPerUnit
	// 兼容旧前端：保留 display_in_currency，同时提供新的 quota_display_type
	data["display_in_currency"] = operation_setting.IsCurrencyDisplay()
	data["quota_display_type"] = operation_setting.GetQuotaDisplayType()
	data["custom_currency_symbol"] = operation_setting.GetGeneralSetting().CustomCurrencySymbol
	data["custom_currency_exchange_rate"] = operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	data["usd_exchange_rate"] = operation_setting.USDExchangeRate
	data["price"] = operation_setting.Price
	data["stripe_unit_price"] = setting.StripeUnitPrice

	// 功能开关（内部模块启停状态）
	data["enable_batch_update"]      = common.BatchUpdateEnabled
	data["enable_drawing"]           = common.DrawingEnabled
	data["enable_task"]              = common.TaskEnabled
	data["enable_data_export"]       = common.DataExportEnabled
	data["data_export_default_time"] = common.DataExportDefaultTime
	data["default_collapse_sidebar"] = common.DefaultCollapseSidebar
	data["mj_notify_enabled"]        = setting.MjNotifyEnabled
	data["chats"]                    = setting.Chats
	data["checkin_enabled"]          = operation_setting.GetCheckinSetting().Enabled

	// Dashboard 面板启用开关
	data["api_info_enabled"]      = cs.ApiInfoEnabled
	data["uptime_kuma_enabled"]   = cs.UptimeKumaEnabled
	data["announcements_enabled"] = cs.AnnouncementsEnabled
	data["faq_enabled"]           = cs.FAQEnabled

	// 模块管理配置（导航栏/侧边栏结构属于后台配置详情）
	data["HeaderNavModules"]    = common.OptionMap["HeaderNavModules"]
	data["SidebarModulesAdmin"] = common.OptionMap["SidebarModulesAdmin"]

	// 根据启用状态注入可选内容（API 信息、公告、FAQ）
	if cs.ApiInfoEnabled {
		data["api_info"] = console_setting.GetApiInfo()
	}
	if cs.AnnouncementsEnabled {
		data["announcements"] = console_setting.GetAnnouncements()
	}
	if cs.FAQEnabled {
		data["faq"] = console_setting.GetFAQ()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
	return
}

func GetNotice(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Notice"],
	})
	return
}

func GetAbout(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["About"],
	})
	return
}

func GetUserAgreement(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system_setting.GetLegalSettings().UserAgreement,
	})
	return
}

func GetPrivacyPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system_setting.GetLegalSettings().PrivacyPolicy,
	})
	return
}

func GetMidjourney(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Midjourney"],
	})
	return
}

func GetHomePageContent(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["HomePageContent"],
	})
	return
}

func SendEmailVerification(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的邮箱地址",
		})
		return
	}
	localPart := parts[0]
	domainPart := parts[1]
	if common.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range common.EmailDomainWhitelist {
			if domainPart == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The administrator has enabled the email domain name whitelist, and your email address is not allowed due to special symbols or it's not in the whitelist.",
			})
			return
		}
	}
	if common.EmailAliasRestrictionEnabled {
		containsSpecialSymbols := strings.Contains(localPart, "+") || strings.Contains(localPart, ".")
		if containsSpecialSymbols {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。",
			})
			return
		}
	}

	if model.IsEmailAlreadyTaken(email) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "邮箱地址已被占用",
		})
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, code, common.VerificationValidMinutes)
	err := common.SendEmail(subject, email, content)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func SendPasswordResetEmail(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if model.IsEmailAlreadyTaken(email) {
		code := common.GenerateVerificationCode(0)
		common.RegisterVerificationCodeWithKey(email, code, common.PasswordResetPurpose)
		link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", system_setting.ServerAddress, email, code)
		subject := fmt.Sprintf("%s密码重置", common.SystemName)
		content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
			"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
			"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
			"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.SystemName, link, link, common.VerificationValidMinutes)
		err := common.SendEmail(subject, email, content)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send password reset email to %s: %s", email, err.Error()))
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if req.Email == "" || req.Token == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if !common.VerifyCodeWithKey(req.Email, req.Token, common.PasswordResetPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "重置链接非法或已过期",
		})
		return
	}
	password := common.GenerateVerificationCode(12)
	err = model.ResetUserPasswordByEmail(req.Email, password)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.DeleteKey(req.Email, common.PasswordResetPurpose)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
	return
}
