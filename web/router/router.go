package router

import (
	"github.com/gin-gonic/gin"
	"github.com/Tumb1er1376/komari-monitor-lite/web/api"
	"github.com/Tumb1er1376/komari-monitor-lite/web/api/admin"
	"github.com/Tumb1er1376/komari-monitor-lite/web/api/client"
	public_api "github.com/Tumb1er1376/komari-monitor-lite/web/api/public"
	"github.com/Tumb1er1376/komari-monitor-lite/web/public"
	jsonRpc "github.com/Tumb1er1376/komari-monitor-lite/web/rpc/jsonrpc"
)

// Register binds all HTTP, WebSocket, JSON-RPC and static frontend routes.
//
// 设计：JSON 类接口统一经声明式路由桥 jsonRpc.Bind 绑定到对应 RPC2 方法，
// 不再有 per-resource gin handler 层。仅二进制/流/重定向/特殊鉴权类接口保留为 REST handler。
func Register(r *gin.Engine) {
	r.Any("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	registerPublicRoutes(r)
	registerAgentRoutes(r)
	registerAdminRoutes(r)

	public.Static(r.Group("/"), func(handlers ...gin.HandlerFunc) {
		r.NoRoute(handlers...)
	})
}

// registerPublicRoutes 公开路由。JSON 读接口经 Bind 绑定到 public: 命名空间方法。
func registerPublicRoutes(r *gin.Engine) {
	// 非 JSON / 特殊流程，保留 REST handler。
	r.POST("/api/login", public_api.Login)
	r.GET("/api/setup/status", api.SetupStatus)
	r.POST("/api/setup/create-admin", api.SetupCreateAdmin)
	installScriptHandler := func(c *gin.Context) {
		c.Data(200, "text/x-shellscript; charset=utf-8", public.AgentInstallScript)
	}
	r.GET("/download/agent-install.sh", installScriptHandler)
	r.GET("/install.sh", installScriptHandler)
	r.GET("/download/agent/:version/:asset", public.ServeAgentAsset)
	r.POST("/api/logout", public_api.Logout)
	registerLiteDisabledRoutes(r)
	// /api/clients 是 WebSocket 端点（客户端发 "get"/"get <uuid>" 拉取在线列表与最新上报），
	// 非 JSON-RPC，保留为 WS handler。
	r.GET("/api/clients", api.GetClients)

	// JSON 接口 -> RPC2。
	r.GET("/api/me", jsonRpc.Bind("public:getMe", jsonRpc.WithRaw()))
	r.GET("/api/public", jsonRpc.Bind("public:getPublicSettings"))
	r.GET("/api/version", jsonRpc.Bind("public:getVersion"))
	r.GET("/api/task/ping", jsonRpc.Bind("public:getPublicPingTasks"))

	// JSON-RPC 直连入口。
	r.GET("/api/rpc2", jsonRpc.OnRpcRequest)
	r.POST("/api/rpc2", jsonRpc.OnRpcRequest)
}

// registerLiteDisabledRoutes 显式 404 掉 Lite 版未实现的 API 路径。
//
// 该黑名单的真正作用是防止 SPA fallback（NoRoute -> 静态文件服务）在这些
// API 路径上返回 HTML（index.html），而非安全防线：未注册的路由本身就不
// 存在对应 handler，任何请求都到不了业务逻辑，这里只是把"返回 HTML 的
// 404"换成"返回 JSON 的 404"，避免客户端把 SPA 页面误当 API 响应解析。
//
// 因此，上游新增敏感路由时 Lite 不会自动继承（handler 从未注册，无暴露），
// 新增的暴露风险仅在于 SPA fallback 语义——即这些路径会返回 HTML 而非
// JSON 404。若上游新增的路径需要保持 JSON 语义，需手动加入此列表。
func registerLiteDisabledRoutes(r *gin.Engine) {
	disabled := func(c *gin.Context) {
		c.AbortWithStatusJSON(404, gin.H{"status": "error", "message": "Not found"})
	}
	for _, path := range []string{
		"/api/clients/v1/rpc",
		"/api/oauth",
		"/api/oauth_callback",
		"/api/terminal",
		"/api/restore",
		"/api/upload",
		"/api/file",
		"/api/clipboard",
		"/api/tasks",
		"/api/pprof",
		"/api/admin/terminal",
		"/api/admin/backup/create",
		"/api/admin/backup/status",
		"/api/admin/dbquery",
		"/api/admin/database",
		"/api/admin/backup/restore",
		"/api/admin/pprof",
		"/api/admin/settings/xtermjs",
		"/api/admin/settings/oidc",
		"/download/agent-migration.sh",
		"/api/admin/clipboard",
	} {
		r.Any(path, disabled)
	}
	for _, path := range []string{
		"/api/terminal/*path",
		"/api/restore/*path",
		"/api/upload/*path",
		"/api/file/*path",
		"/api/clipboard/*path",
		"/api/tasks/*path",
		"/api/pprof/*path",
		"/api/clients/transfer/*path",
		"/api/admin/pprof/*path",
		"/api/admin/oauth2/*path",
		"/api/admin/notification/load",
		"/api/admin/notification/load/*path",
		"/api/admin/backup/status/*path",

		"/api/admin/clipboard/*path",

		"/api/admin/task/*path",
		"/api/admin/client/:uuid/terminal",
		"/api/admin/client/:uuid/file/*path",
	} {
		r.Any(path, disabled)
	}
	r.Any("/api/admin/theme", disabled)
	r.Any("/api/admin/theme/*path", disabled)
}

// registerAgentRoutes agent（客户端）上报与拉取路由。
func registerAgentRoutes(r *gin.Engine) {
	tokenAuthorized := r.Group("/api/clients", api.RequireRole(api.RoleAdmin, api.RoleClient))
	{
		// Agent 上报统一使用 v2 JSON-RPC。
		tokenAuthorized.GET("/v2/rpc", client.WebSocketV2RPC)
		tokenAuthorized.POST("/v2/rpc", client.UploadV2RPC)
	}
}

// registerAdminRoutes 管理员路由。除二进制/流类外全部经 Bind 绑定到 admin: 命名空间方法。
func registerAdminRoutes(r *gin.Engine) {
	g := r.Group("/api/admin", api.RequireRole(api.RoleAdmin))

	// --- 二进制/流/重定向类，保留 REST handler ---
	g.GET("/download/backup", admin.DownloadBackup)
	g.GET("/test/geoip", jsonRpc.Bind("admin:testGeoip", jsonRpc.WithQuery("ip")))
	g.POST("/test/sendMessage", jsonRpc.Bind("admin:testSendMessage"))
	g.POST("/update/mmdb", admin.UpdateMmdbGeoIP)
	g.POST("/update/user", admin.UpdateUser)

	// 2FA 含二维码 PNG / 敏感操作，保留 REST handler。
	twoFactor := g.Group("/2fa")
	{
		twoFactor.GET("/generate", admin.Generate2FA)
		twoFactor.POST("/enable", admin.Enable2FA)
		twoFactor.POST("/disable", api.RequireSensitive2FA(), admin.Disable2FA)
	}

	// --- 以下全部 JSON -> RPC2 ---

	// settings
	settings := g.Group("/settings")
	{
		settings.GET("/", jsonRpc.Bind("admin:getSettings"))
		settings.POST("/", jsonRpc.Bind("admin:editSettings"))
		settings.POST("/message-sender", jsonRpc.Bind("admin:setMessageSenderProvider"))
		settings.GET("/message-sender", jsonRpc.Bind("admin:getMessageSenderProvider", jsonRpc.WithQuery("provider")))
	}

	// clients
	clientGroup := g.Group("/client")
	{
		clientGroup.POST("/add", jsonRpc.Bind("admin:addClient", jsonRpc.WithFlat()))
		clientGroup.GET("/list", jsonRpc.Bind("admin:listClients", jsonRpc.WithRaw()))
		clientGroup.GET("/:uuid", jsonRpc.Bind("admin:getClient", jsonRpc.WithPath("uuid"), jsonRpc.WithRaw()))
		clientGroup.POST("/:uuid/edit", jsonRpc.Bind("admin:editClient", jsonRpc.WithPath("uuid")))
		clientGroup.POST("/:uuid/remove", jsonRpc.Bind("admin:removeClient", jsonRpc.WithPath("uuid")))
		clientGroup.GET("/:uuid/token", jsonRpc.Bind("admin:getClientToken", jsonRpc.WithPath("uuid"), jsonRpc.WithFlat()))
		clientGroup.POST("/order", jsonRpc.Bind("admin:orderClients"))
	}

	// records
	record := g.Group("/record")
	{
		record.POST("/clear", jsonRpc.Bind("admin:clearRecords"))
		record.POST("/clear/all", jsonRpc.Bind("admin:clearAllRecords"))
	}

	// sessions
	session := g.Group("/session")
	{
		session.GET("/get", jsonRpc.Bind("admin:getSessions", jsonRpc.WithFlat()))
		session.POST("/remove", jsonRpc.Bind("admin:deleteSession"))
		session.POST("/remove/all", jsonRpc.Bind("admin:deleteAllSessions"))
	}

	g.GET("/logs", jsonRpc.Bind("admin:getLogs", jsonRpc.WithQuery("limit", "page")))

	// notifications
	notificationGroup := g.Group("/notification")
	{
		notificationGroup.GET("/offline", jsonRpc.Bind("admin:listOfflineNotifications"))
		notificationGroup.POST("/offline/edit", jsonRpc.Bind("admin:editOfflineNotification"))
		notificationGroup.POST("/offline/enable", jsonRpc.Bind("admin:enableOfflineNotification"))
		notificationGroup.POST("/offline/disable", jsonRpc.Bind("admin:disableOfflineNotification"))
	}

	// ping tasks
	pingTask := g.Group("/ping")
	{
		pingTask.GET("/", jsonRpc.Bind("admin:getAllPingTasks"))
		pingTask.POST("/add", jsonRpc.Bind("admin:addPingTask"))
		pingTask.POST("/delete", jsonRpc.Bind("admin:deletePingTask"))
		pingTask.POST("/edit", jsonRpc.Bind("admin:editPingTask"))
		pingTask.POST("/order", jsonRpc.Bind("admin:orderPingTask"))
	}

	// legacy monitoring migration（升级向导：检查只读，执行为敏感操作需 2FA）
	migration := g.Group("/migration")
	{
		migration.GET("/legacy", jsonRpc.Bind("admin:getLegacyMigrationStatus"))
		migration.POST("/legacy", jsonRpc.Bind("admin:runLegacyMigration"))
	}

	// agent 自动升级（敏感操作需 2FA）
	g.GET("/agent-asset-version", jsonRpc.Bind("admin:getAgentAssetVersion"))
	g.POST("/upgrade-agents", jsonRpc.Bind("admin:upgradeAgents"))
}
