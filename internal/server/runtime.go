package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-monitor-lite/database/accounts"
	"github.com/lyhbdw/komari-monitor-lite/database/auditlog"
	"github.com/lyhbdw/komari-monitor-lite/database/clients"

	"github.com/lyhbdw/komari-monitor-lite/database/tasks"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
	"github.com/lyhbdw/komari-monitor-lite/internal/metricstore"
	"github.com/lyhbdw/komari-monitor-lite/internal/scheduler"
	"github.com/lyhbdw/komari-monitor-lite/utils/geoip"
	logger "github.com/lyhbdw/komari-monitor-lite/utils/log"
	"github.com/lyhbdw/komari-monitor-lite/utils/messageSender"
	"github.com/lyhbdw/komari-monitor-lite/utils/notifier"
	"github.com/lyhbdw/komari-monitor-lite/web/api"
	"github.com/lyhbdw/komari-monitor-lite/web/router"
	"github.com/lyhbdw/komari-monitor-lite/web/security"
)

const (
	// Give in-flight HTTP requests time to finish before the listener closes.
	httpShutdownTimeout = 10 * time.Second
	// Keep an independent budget for report flushing and store teardown. Reusing
	// the HTTP deadline here can skip queued metric writes after a slow request.
	resourceCleanupTimeout = 30 * time.Second
	httpReadHeaderTimeout  = 10 * time.Second
	httpReadTimeout        = 30 * time.Second
	httpWriteTimeout       = 60 * time.Second
	httpIdleTimeout        = 120 * time.Second
	httpMaxHeaderBytes     = 1 << 20
)

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: httpReadHeaderTimeout, ReadTimeout: httpReadTimeout, WriteTimeout: httpWriteTimeout, IdleTimeout: httpIdleTimeout, MaxHeaderBytes: httpMaxHeaderBytes}
}

// StartBackground starts scheduled work after all stores are ready.
func (a *App) StartBackground() error {
	registerScheduledWork()
	a.addCleanup("scheduler", func(context.Context) error {
		scheduler.StopAll()
		return nil
	})
	return nil
}

func (a *App) registerReloadHandlers(cors *security.CorsController) {
	a.reload.Register("geoip-provider", func(event config.ConfigEvent) {
		if event.IsChanged(config.GeoIpProviderKey) {
			go geoip.InitGeoIp()
		}
	})
	a.reload.Register("message-sender", func(event config.ConfigEvent) {
		if event.IsChanged(config.NotificationMethodKey) {
			go messageSender.Initialize()
		}
	})
	a.reload.Register("cors", func(event config.ConfigEvent) { cors.Update(event) })
}

// defaultTrustedProxies is the conservative default trust list: only
// loopback. Deployments behind a reverse proxy must explicitly opt in via
// the KOMARI_TRUSTED_PROXIES environment variable (comma-separated CIDRs
// or IPs), otherwise X-Forwarded-* headers from clients are ignored and
// gin derives the client IP from the remote address.
const defaultTrustedProxies = "127.0.0.0/8,::1/128"

// trustedProxyEnv is the environment variable used to override the default
// trusted proxy list. Comma-separated CIDR notation or plain IPs.
const trustedProxyEnv = "KOMARI_TRUSTED_PROXIES"

func resolveTrustedProxies() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv(trustedProxyEnv))
	if raw == "" {
		raw = defaultTrustedProxies
	}
	var proxies []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			if _, _, err := net.ParseCIDR(part); err != nil {
				return nil, fmt.Errorf("invalid CIDR %q in %s: %w", part, trustedProxyEnv, err)
			}
		} else if net.ParseIP(part) == nil {
			return nil, fmt.Errorf("invalid IP %q in %s", part, trustedProxyEnv)
		}
		proxies = append(proxies, part)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("%s must not be empty", trustedProxyEnv)
	}
	return proxies, nil
}

// BuildRouter constructs the normal application router and starts reloads.
func (a *App) BuildRouter() error {
	r := gin.New()
	proxies, err := resolveTrustedProxies()
	if err != nil {
		return err
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		return fmt.Errorf("set trusted proxies: %w", err)
	}
	r.Use(logger.GinLogger(), logger.GinRecovery())
	cors := security.NewCorsController(a.settings.CorsAllowedOrigins)
	r.Use(cors.Middleware(), api.IdentityMiddleware(), noStoreAPIResponses())

	router.Register(r)

	a.registerReloadHandlers(cors)
	a.reload.Start()
	a.engine = r
	return nil
}

// Run starts the normal HTTP server and blocks until shutdown or fatal error.
func (a *App) Run() error {
	// The HTML injector runs outside the hook chain so it sees the final
	// response: plugin hooks can still rewrite the body, then the registered
	// head/body fragments are embedded into every text/html page.
	a.server = newHTTPServer(a.listenAddr, a.engine)
	serverErr := make(chan error, 1)
	logger.Infof("server", "Starting server on %s ...", a.listenAddr)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case err := <-serverErr:
		a.onFatal(err)
		return fmt.Errorf("listen: %w", err)
	case <-quit:
		return a.Shutdown()
	}
}

// Shutdown stops HTTP first, then releases registered resources in LIFO order.
func (a *App) Shutdown() error {
	if a.dbReady {
		auditlog.Log("", "", "server is shutting down", "info")
	}
	httpCtx, cancelHTTP := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancelHTTP()
	if a.server != nil {
		if err := a.server.Shutdown(httpCtx); err != nil {
			logger.Infof("server", "HTTP server forced to shutdown: %v", err)
		}
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), resourceCleanupTimeout)
	defer cancelCleanup()
	return a.runCleanups(cleanupCtx)
}

func (a *App) onFatal(err error) {
	if a.dbReady {
		auditlog.Log("", "", "server encountered a fatal error: "+err.Error(), "error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), resourceCleanupTimeout)
	defer cancel()
	if cleanupErr := a.runCleanups(ctx); cleanupErr != nil {
		logger.Errorf("server", "Cleanup after fatal server error failed: %v", cleanupErr)
	}
}

func (a *App) runCleanups(ctx context.Context) error {
	var cleanupErrors []error
	for i := len(a.cleanups) - 1; i >= 0; i-- {
		cleanup := a.cleanups[i]
		if err := cleanup.fn(ctx); err != nil {
			logger.Errorf("server", "cleanup %q failed: %v", cleanup.name, err)
			cleanupErrors = append(cleanupErrors, fmt.Errorf("cleanup %q: %w", cleanup.name, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

// scheduledTaskGates 防止同一调度任务的重叠执行：
// 上一轮还没跑完时下一轮 tick 直接跳过（与 metricstore 的
// compactOperations.TryAcquire 门同一做法）。
var (
	cleanupGate atomic.Bool
	trafficGate atomic.Bool
	alertGate   atomic.Bool
	expireGate  atomic.Bool
)

func registerScheduledWork() {
	if err := tasks.ReloadPingSchedule(); err != nil {
		logger.ErrorArgs("server", "Failed to reload ping schedule:", err)
	}

	if err := scheduler.AddFunc("records:cleanup", "@every 30m", func() {
		if !cleanupGate.CompareAndSwap(false, true) {
			return
		}
		defer cleanupGate.Store(false)
		cleanupScheduledData()
	}); err != nil {
		logger.ErrorArgs("server", "Failed to add cleanup scheduled task:", err)
	}
	if err := scheduler.AddContextFunc("metrics:compact", "@every 5m", true, compactMetricStore); err != nil {
		logger.ErrorArgs("server", "Failed to add metric compact scheduled task:", err)
	}
	if err := scheduler.AddContextFunc("metrics:retention", "@every 1h", true, cleanupMetricStore); err != nil {
		logger.ErrorArgs("server", "Failed to add metric retention scheduled task:", err)
	}
	if err := scheduler.AddFunc("metrics:orphan-cleanup", "@every 24h", cleanupOrphanMetricEntities); err != nil {
		logger.ErrorArgs("server", "Failed to add metric orphan cleanup scheduled task:", err)
	}
	if err := scheduler.AddFunc("metrics:reclaim", "@every 720h", reclaimMetricStoreSpace); err != nil {
		logger.ErrorArgs("server", "Failed to add metric space reclaim scheduled task:", err)
	}
	if err := scheduler.AddFunc("notifier:traffic", "@every 1m", func() {
		if !trafficGate.CompareAndSwap(false, true) {
			return
		}
		defer trafficGate.Store(false)
		notifier.CheckTraffic()
	}); err != nil {
		logger.ErrorArgs("server", "Failed to add traffic notification task:", err)
	}
	if err := scheduler.AddFunc("notifier:alert", "@every 1m", func() {
		if !alertGate.CompareAndSwap(false, true) {
			return
		}
		defer alertGate.Store(false)
		notifier.CheckAlert()
	}); err != nil {
		logger.ErrorArgs("server", "Failed to add threshold alert task:", err)
	}
	if err := scheduler.AddFunc("notifier:expire", "@every 1h", func() {
		if !expireGate.CompareAndSwap(false, true) {
			return
		}
		defer expireGate.Store(false)
		notifier.CheckExpire()
	}); err != nil {
		logger.ErrorArgs("server", "Failed to add expire notification task:", err)
	}
}

func cleanupScheduledData() {
	auditlog.RemoveOldLogs()
	accounts.RemoveExpiredSessions()
}

func compactMetricStore(ctx context.Context) {
	compactCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	written, err := metricstore.Compact(compactCtx, time.Now().UTC())
	if errors.Is(err, metricstore.ErrCompactInProgress) {
		return
	}
	if err != nil {
		logger.Errorf("server", "Failed to compact metric store after writing %d rollup buckets: %v", written, err)
		return
	}
	if written > 0 {
		logger.Infof("server", "Metric store compacted %d rollup buckets", written)
	}
}

func cleanupMetricStore(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	deleted, err := metricstore.CleanupExpired(cleanupCtx, time.Now().UTC())
	if errors.Is(err, metricstore.ErrCompactInProgress) {
		return
	}
	if err != nil {
		logger.Errorf("server", "Failed to clean expired metric data after deleting %d rows: %v", deleted, err)
		return
	}
	if deleted > 0 {
		logger.Infof("server", "Metric retention cleanup deleted %d rows", deleted)
	}
}

// orphanCleanupGate 防止孤儿清理任务与自身重叠（DeleteEntity 是重操作，
// 且 ReclaimSpace 的独占门会与之互斥，重叠执行只会互相等待）。
var orphanCleanupGate atomic.Bool

// cleanupOrphanMetricEntities 删除 metric store 中已不存在于 clients 表的
// 实体（1.0.0 时代删除节点未清理指标数据留下的残留）。每 24h 跑一次。
func cleanupOrphanMetricEntities() {
	if !orphanCleanupGate.CompareAndSwap(false, true) {
		return
	}
	defer orphanCleanupGate.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	known := make(map[string]struct{})
	if all, err := clients.GetAllClientBasicInfo(); err != nil {
		logger.Errorf("server", "Failed to list clients for metric orphan cleanup: %v", err)
		return
	} else {
		for _, c := range all {
			known[c.UUID] = struct{}{}
		}
	}
	deleted, err := metricstore.CleanupOrphanEntities(ctx, known)
	if err != nil {
		logger.Errorf("server", "Failed to clean orphan metric entities after deleting %d: %v", deleted, err)
		return
	}
	if deleted > 0 {
		logger.Infof("server", "Metric orphan cleanup deleted %d entities", deleted)
	}
}

// reclaimMetricStoreSpace 定期执行 metric store 的物理空间回收（SQLite
// VACUUM）。SQLite 删除只把页挂到 freelist，文件不收缩；90 天保留的
// 滚动删除会让空洞持续累积。每 30 天（720h）在低频窗口自动回收一次，
// 管理员也可随时通过 admin:reclaimSpace 手动触发。
func reclaimMetricStoreSpace() {
	// ReclaimSpace 内部故意忽略 ctx（不可取消的独占维护），这里只限制
	// 等待操作门的时长，避免调度器 cancel 导致 goroutine 泄漏在门上。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := metricstore.ReclaimSpace(ctx)
	if err != nil {
		logger.Errorf("server", "Scheduled metric store space reclaim failed: %v", err)
		return
	}
	logger.Infof("server", "Scheduled metric store space reclaim done (%s): %d -> %d bytes",
		result.Action, result.Before, result.After)
}
