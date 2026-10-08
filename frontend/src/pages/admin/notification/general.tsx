import { useTranslation } from "react-i18next";
import { Switch } from "@/components/ui/switch";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import Loading from "@/components/loading";
import { toast } from "sonner";
import { useState, useEffect } from "react";
import { CalendarClock, Activity, ShieldAlert, Cpu, Database, HardDrive, Clock } from "lucide-react";

const GeneralNotification = () => {
  const { t } = useTranslation();
  const { settings, loading, error, setSettings } = useSettings();

  // 本地表单暂存状态
  const [expireEnabled, setExpireEnabled] = useState(false);
  const [expireDays, setExpireDays] = useState<number>(14);
  const [loginNotify, setLoginNotify] = useState(false);

  const [trafficPercent, setTrafficPercent] = useState<number>(80);

  const [alertEnabled, setAlertEnabled] = useState(false);
  const [alertCpu, setAlertCpu] = useState<number>(80);
  const [alertMemory, setAlertMemory] = useState<number>(80);
  const [alertDisk, setAlertDisk] = useState<number>(85);
  const [alertCooldown, setAlertCooldown] = useState<number>(30);

  const [savingSection, setSavingSection] = useState<string | null>(null);

  useEffect(() => {
    if (settings) {
      setExpireEnabled(!!settings.expire_notification_enabled);
      setExpireDays(settings.expire_notification_lead_days ?? 14);
      setLoginNotify(!!settings.login_notification);

      setTrafficPercent(settings.traffic_limit_percentage ?? 80);

      setAlertEnabled(!!settings.alert_enabled);
      setAlertCpu(settings.alert_cpu ?? 80);
      setAlertMemory(settings.alert_memory ?? 80);
      setAlertDisk(settings.alert_disk ?? 85);
      setAlertCooldown(settings.alert_cooldown ?? 30);
    }
  }, [settings]);

  if (loading) return <Loading />;
  if (error) return <p className="text-sm text-destructive">{error}</p>;

  // 保存卡片 1：生命周期与登录通知
  const saveExpireAndLogin = async () => {
    setSavingSection("expire");
    try {
      const payload = {
        expire_notification_enabled: expireEnabled,
        expire_notification_lead_days: Number(expireDays),
        login_notification: loginNotify,
      };
      await updateSettingsWithToast(payload, t);
      setSettings((prev: any) => ({ ...prev, ...payload }));
    } finally {
      setSavingSection(null);
    }
  };

  // 保存卡片 2：性能阈值告警
  const saveResourceAlerts = async () => {
    if (alertCpu < 0 || alertCpu > 100 || alertMemory < 0 || alertMemory > 100 || alertDisk < 0 || alertDisk > 100) {
      toast.error(t("admin.notification.alert_range_error", "百分比阈值必须在 0 ~ 100 之间"));
      return;
    }
    setSavingSection("alert");
    try {
      const payload = {
        alert_enabled: alertEnabled,
        alert_cpu: Number(alertCpu),
        alert_memory: Number(alertMemory),
        alert_disk: Number(alertDisk),
        alert_cooldown: Number(alertCooldown),
      };
      await updateSettingsWithToast(payload, t);
      setSettings((prev: any) => ({ ...prev, ...payload }));
    } finally {
      setSavingSection(null);
    }
  };

  // 保存卡片 3：网络流量超额预警
  const saveTrafficLimit = async () => {
    setSavingSection("traffic");
    try {
      const payload = {
        traffic_limit_percentage: Number(trafficPercent),
      };
      await updateSettingsWithToast(payload, t);
      setSettings((prev: any) => ({ ...prev, ...payload }));
    } finally {
      setSavingSection(null);
    }
  };

  const activeFeaturesCount =
    (expireEnabled ? 1 : 0) +
    (loginNotify ? 1 : 0) +
    (alertEnabled ? 1 : 0) +
    (trafficPercent > 0 ? 1 : 0);

  return (
    <div className="space-y-4 max-w-4xl km-page-admin-notification-general">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("admin.notification.general_title", "通用事件通知")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {activeFeaturesCount} 项已激活
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置机器临期、后台登录以及 CPU / 内存 / 磁盘高负载触发的自动推送策略。
          </p>
        </div>
      </div>

      {/* 卡片 1: 节点生命周期与登录安全 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <CalendarClock size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("admin.notification.lifecycle_title", "生命周期与安全审计")}
            </h2>
          </div>
          <button
            type="button"
            onClick={saveExpireAndLogin}
            disabled={savingSection === "expire"}
            className="h-7 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
          >
            {savingSection === "expire" ? t("common.saving", "保存中...") : t("common.save", "保存修改")}
          </button>
        </div>

        {/* 临期通知 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-1">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-foreground">
                {t("admin.notification.expire_enable", "服务器到期提醒")}
              </span>
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  expireEnabled ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("admin.notification.expire_enable_description", "启用后，在服务器即将到期前自动通过已配渠道发送续费提醒。")}
            </p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span className="text-xs text-muted-foreground">
              {expireEnabled ? "已启用" : "未启用"}
            </span>
            <Switch checked={expireEnabled} onCheckedChange={setExpireEnabled} />
          </div>
        </div>

        {expireEnabled && (
          <div className="p-3 rounded-lg bg-muted/40 border border-border/60 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
            <div>
              <span className="font-medium text-foreground">
                {t("admin.notification.expire_time", "提前预警天数")}
              </span>
              <p className="text-[11px] text-muted-foreground mt-0.5">
                在距离到期日少于指定天数时持续发送临期通知。
              </p>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <input
                type="number"
                min="1"
                max="90"
                value={expireDays}
                onChange={(e) => setExpireDays(Number(e.target.value))}
                className="w-20 h-7 px-2 text-center rounded-md border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
              />
              <span className="text-muted-foreground font-medium">天前开始</span>
            </div>
          </div>
        )}

        <div className="h-px bg-border/40" />

        {/* 登录通知 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-foreground">
                {t("admin.notification.login", "后台登录审计提醒")}
              </span>
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  loginNotify ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("admin.notification.login_description", "当有任何管理员会话登录后台时，立即发送登录地点与 IP 通知。")}
            </p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span className="text-xs text-muted-foreground">
              {loginNotify ? "已开启" : "已停用"}
            </span>
            <Switch checked={loginNotify} onCheckedChange={setLoginNotify} />
          </div>
        </div>
      </div>

      {/* 卡片 2: 服务器资源性能阈值告警（4格对齐网格） */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <Activity size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("admin.notification.alert_title", "资源性能阈值告警")}
            </h2>
          </div>
          <button
            type="button"
            onClick={saveResourceAlerts}
            disabled={savingSection === "alert"}
            className="h-7 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
          >
            {savingSection === "alert" ? t("common.saving", "保存中...") : t("common.save", "保存修改")}
          </button>
        </div>

        {/* 总开关 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-1">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-foreground">
                {t("admin.notification.alert_enable", "启用性能高载报警")}
              </span>
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  alertEnabled ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("admin.notification.alert_enable_description", "当任意服务器的 CPU、内存或磁盘负载超过设定阈值时下发告警。")}
            </p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span className="text-xs text-muted-foreground">
              {alertEnabled ? "告警中" : "已静音"}
            </span>
            <Switch checked={alertEnabled} onCheckedChange={setAlertEnabled} />
          </div>
        </div>

        {/* 4 格对齐指标网格 */}
        <div className={`grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2 transition-opacity duration-200 ${alertEnabled ? "opacity-100" : "opacity-45 pointer-events-none"}`}>
          {/* CPU 阈值 */}
          <div className="p-3 rounded-lg border border-border/60 bg-muted/30 flex items-center justify-between gap-2 shadow-2xs">
            <div className="flex items-center gap-2">
              <Cpu size={15} className="text-muted-foreground" />
              <div>
                <span className="text-xs font-medium text-foreground">CPU 使用率阈值</span>
                <p className="text-[10px] text-muted-foreground">设为 0 禁用该项</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min="0"
                max="100"
                value={alertCpu}
                onChange={(e) => setAlertCpu(Number(e.target.value))}
                className="w-16 h-7 px-2 text-center rounded-md border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
              />
              <span className="text-xs text-muted-foreground font-mono">%</span>
            </div>
          </div>

          {/* 内存阈值 */}
          <div className="p-3 rounded-lg border border-border/60 bg-muted/30 flex items-center justify-between gap-2 shadow-2xs">
            <div className="flex items-center gap-2">
              <Database size={15} className="text-muted-foreground" />
              <div>
                <span className="text-xs font-medium text-foreground">内存使用率阈值</span>
                <p className="text-[10px] text-muted-foreground">设为 0 禁用该项</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min="0"
                max="100"
                value={alertMemory}
                onChange={(e) => setAlertMemory(Number(e.target.value))}
                className="w-16 h-7 px-2 text-center rounded-md border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
              />
              <span className="text-xs text-muted-foreground font-mono">%</span>
            </div>
          </div>

          {/* 磁盘阈值 */}
          <div className="p-3 rounded-lg border border-border/60 bg-muted/30 flex items-center justify-between gap-2 shadow-2xs">
            <div className="flex items-center gap-2">
              <HardDrive size={15} className="text-muted-foreground" />
              <div>
                <span className="text-xs font-medium text-foreground">磁盘使用率阈值</span>
                <p className="text-[10px] text-muted-foreground">设为 0 禁用该项</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min="0"
                max="100"
                value={alertDisk}
                onChange={(e) => setAlertDisk(Number(e.target.value))}
                className="w-16 h-7 px-2 text-center rounded-md border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
              />
              <span className="text-xs text-muted-foreground font-mono">%</span>
            </div>
          </div>

          {/* 告警冷却 */}
          <div className="p-3 rounded-lg border border-border/60 bg-muted/30 flex items-center justify-between gap-2 shadow-2xs">
            <div className="flex items-center gap-2">
              <Clock size={15} className="text-muted-foreground" />
              <div>
                <span className="text-xs font-medium text-foreground">告警冷却时间</span>
                <p className="text-[10px] text-muted-foreground">同一指标静默间隔</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min="1"
                max="1440"
                value={alertCooldown}
                onChange={(e) => setAlertCooldown(Number(e.target.value))}
                className="w-16 h-7 px-2 text-center rounded-md border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
              />
              <span className="text-xs text-muted-foreground font-mono">分钟</span>
            </div>
          </div>
        </div>
      </div>

      {/* 卡片 3: 网络流量超额梯度提醒 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-3">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <ShieldAlert size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("admin.notification.traffic", "流量使用超额提醒")}
            </h2>
          </div>
          <button
            type="button"
            onClick={saveTrafficLimit}
            disabled={savingSection === "traffic"}
            className="h-7 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
          >
            {savingSection === "traffic" ? t("common.saving", "保存中...") : t("common.save", "保存修改")}
          </button>
        </div>

        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-1">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-foreground">流量梯度预警阈值</span>
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  trafficPercent > 0 ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5 max-w-lg">
              达到指定百分比时开始发送告警，并以 5% 递增持续追踪。设为 0 则禁用流量告警。
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <input
              type="number"
              min="0"
              max="100"
              value={trafficPercent}
              onChange={(e) => setTrafficPercent(Number(e.target.value))}
              className="w-18 h-8 px-2 text-center rounded-lg border border-border bg-background text-foreground font-mono text-xs outline-none focus:border-foreground/50 shadow-2xs"
            />
            <span className="text-xs text-muted-foreground font-mono">% 触发</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default GeneralNotification;
