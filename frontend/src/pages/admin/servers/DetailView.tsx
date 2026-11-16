import { copyToClipboard } from "@/utils/clipboard";
import React from "react";
import { useState } from "react";
import { useRPC2Call } from "@/contexts/useRPC2";
import type { GoogleRegionChecks, NodeDetail, RegionProbe } from "@/contexts/node-details-context";
import { Button, IconButton } from "@/components/ui/radix-shim";
import { Copy } from "lucide-react";
import { useTranslation } from "react-i18next";
import Flag from "@/components/Flag";
import { useIsMobile } from "@/hooks/use-mobile";
import { Drawer, DrawerClose, DrawerContent, DrawerDescription, DrawerFooter, DrawerHeader, DrawerTitle, DrawerTrigger } from "@/components/ui/drawer";
import { formatBytes } from "@/utils/unitHelper";

function DetailView({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const { call } = useRPC2Call();
  const [googleRegions, setGoogleRegions] = useState<GoogleRegionChecks | undefined>(node.google_regions);
  const refreshRegionChecks = React.useCallback(async () => {
    try {
      const detail = await call<{ uuid: string }, { google_regions?: GoogleRegionChecks }>("admin:getClient", { uuid: node.uuid });
      setGoogleRegions(detail.google_regions);
    } catch {
      setGoogleRegions(undefined);
    }
  }, [call, node.uuid]);
  const regionReasonLabel = (reason?: string) => {
    const labels: Record<string, string> = {
      not_checked: t("admin.nodeDetail.googleRegion.notChecked", "尚未检测"),
      captcha: t("admin.nodeDetail.googleRegion.challenge", "遇到验证码或访问挑战"),
      marker_missing: t("admin.nodeDetail.googleRegion.markerMissing", "未识别到地区字段"),
      marker_invalid: t("admin.nodeDetail.googleRegion.markerInvalid", "地区字段格式异常"),
      rate_limited: t("admin.nodeDetail.googleRegion.rateLimited", "请求被限流"),
      blocked: t("admin.nodeDetail.googleRegion.blocked", "请求被拒绝"),
      timeout: t("admin.nodeDetail.googleRegion.timeout", "请求超时"),
      network_error: t("admin.nodeDetail.googleRegion.networkError", "网络错误"),
      response_too_large: t("admin.nodeDetail.googleRegion.responseTooLarge", "响应超限"),
      proxy_egress_unknown: t("admin.nodeDetail.googleRegion.proxyUnknown", "代理环境无法确认出口地址族"),
    };
    if (!reason) return t("admin.nodeDetail.googleRegion.unknown", "未知");
    if (reason.startsWith("http_")) return `HTTP ${reason.slice(5)}`;
    return labels[reason] ?? t("admin.nodeDetail.googleRegion.unknown", "未知");
  };
  const regionProbeLabel = (probe: RegionProbe | undefined) => {
    if (!probe || probe.status !== "ok" || !probe.country) {
      const reason = regionReasonLabel(probe?.reason);
      return probe?.country
        ? `${t("admin.nodeDetail.googleRegion.lastKnown", "上次")} ${probe.country} · ${t("admin.nodeDetail.googleRegion.currentUnknown", "当前未知")} · ${reason}`
        : `${t("admin.nodeDetail.googleRegion.unknown", "未知")} · ${reason}`;
    }
    return probe.country === "CN" ? t("admin.nodeDetail.googleRegion.possibleChina", "CN · 可能送中") : probe.country;
  };
  const regionProbeClass = (probe: RegionProbe | undefined) =>
    probe?.status === "ok" && probe.country === "CN"
      ? "text-rose-600 dark:text-rose-400 font-semibold"
      : "text-foreground/90";

  return (
    <Drawer direction={isMobile ? "bottom" : "right"} onOpenChange={(open) => { if (open) void refreshRegionChecks(); }}>
      <DrawerTrigger asChild>
        <div className="flex items-center gap-2.5 py-1 hover:underline cursor-pointer group">
          <div className="relative shrink-0 w-6 h-5 flex items-center justify-center">
            <Flag flag={node.region} size="5" className="m-0 shrink-0 rounded-xs" />
            <span
              className={`absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full ring-1.5 ring-card ${
                node.online ? "bg-emerald-500" : "bg-rose-500"
              }`}
              title={node.online ? t("nodeCard.online", "在线") : t("nodeCard.offline", "离线")}
            />
          </div>
          <div className="flex flex-col min-w-0">
            <div className="flex items-center gap-1.5 min-w-0">
              <span className="font-semibold text-sm text-foreground group-hover:text-primary transition-colors truncate max-w-[380px] 2xl:max-w-[500px]" title={node.name}>
                {node.name}
              </span>
              {node.public_remark && (
                <span
                  className="text-[10px] px-1.5 py-0.2 rounded bg-muted/70 text-muted-foreground border border-border/60 font-normal truncate max-w-[120px] shrink-0"
                  title={`备注: ${node.public_remark}`}
                >
                  {node.public_remark}
                </span>
              )}
              {node.tags && (
                <span
                  className="text-[10px] px-1.5 py-0.2 rounded bg-muted/70 text-muted-foreground border border-border/60 font-normal truncate max-w-[120px] shrink-0"
                  title={`标签: ${node.tags}`}
                >
                  {node.tags}
                </span>
              )}
            </div>
            <span className="text-[11px] text-muted-foreground/75 font-mono truncate max-w-[380px]">
              {node.os || "Linux"} {node.arch ? `· ${node.arch}` : ""} {node.version ? `· v${node.version}` : ""}
            </span>
          </div>
        </div>
      </DrawerTrigger>
      <DrawerContent>
        <DrawerHeader className="gap-1">
          <div className="flex items-center gap-2">
            <DrawerTitle>{node.name}</DrawerTitle>
            <span
              className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                node.online
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                  : "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20"
              }`}
            >
              {node.online ? t("nodeCard.online", "在线") : t("nodeCard.offline", "离线")}
            </span>
          </div>
          <DrawerDescription>
            {t("admin.nodeDetail.machineDetail", "机器详细信息")}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex flex-col gap-4 overflow-y-auto px-4 text-sm">
          <form className="flex flex-col gap-4">
            <section className="flex flex-col gap-2 rounded-md border border-border/70 p-3" aria-label={t("admin.nodeDetail.googleRegion.title", "Google 区域检测")}>
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{t("admin.nodeDetail.googleRegion.title", "Google / YouTube 地区信号")}</span>
                <span className="text-xs text-muted-foreground">{t("admin.nodeDetail.googleRegion.caption", "服务地区信号，不代表账号或搜索最终判定")}</span>
              </div>
              {(["ipv4", "ipv6"] as const).map((family) => {
                const checks = googleRegions?.[family];
                const familyLabel = family === "ipv4" ? t("admin.nodeDetail.googleRegion.ipv4", "IPv4") : t("admin.nodeDetail.googleRegion.ipv6", "IPv6");
                return (
                  <div key={family} className="grid grid-cols-[3.5rem_repeat(3,minmax(0,1fr))] items-baseline gap-x-2 gap-y-1.5 text-xs">
                    <span className="row-span-2 font-mono text-muted-foreground">{familyLabel}</span>
                    {(["google_accounts", "youtube", "youtube_premium"] as const).map((source) => {
                      const probe = checks?.[source];
                      const sourceLabel = source === "google_accounts" ? t("admin.nodeDetail.googleRegion.account", "Google 账号页") : source === "youtube" ? t("admin.nodeDetail.googleRegion.youtube", "YouTube") : t("admin.nodeDetail.googleRegion.premium", "Premium");
                      return (
                        <div key={source} className="flex min-w-0 flex-col gap-1">
                          <span className="text-[10px] text-muted-foreground">{sourceLabel}</span>
                          <span className={`break-words ${regionProbeClass(probe)}`}>
                            {regionProbeLabel(probe)}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                );
              })}
              <span className="text-[11px] text-muted-foreground">
                {t("admin.nodeDetail.googleRegion.checkedAt", "最近检测")}：{googleRegions?.checked_at ? new Date(googleRegions.checked_at).toLocaleString() : t("admin.nodeDetail.googleRegion.notChecked", "尚未检测")}
              </span>
            </section>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-ip">
                  {t("admin.nodeDetail.ipAddress", "IP 地址")}
                </label>
                <div className="flex flex-col gap-1">
                  {node.ipv4 && (
                    <div className="flex items-center gap-1">
                      <span
                        id="detail-ipv4"
                        className="bg-muted px-3 py-2 rounded border flex-1 min-w-0 select-text"
                      >
                        {node.ipv4}
                      </span>
                      <IconButton
                        variant="ghost"
                        className="size-5"
                        type="button"
                        onClick={() => {
                          copyToClipboard(node.ipv4!, {
                            successMessage: `已复制 IPv4: ${node.ipv4}`,
                          });
                        }}
                      >
                        <Copy size={16} />
                      </IconButton>
                    </div>
                  )}
                  {node.ipv6 && (
                    <div className="flex items-center gap-1">
                      <span
                        id="detail-ipv6"
                        className="bg-muted px-3 py-2 rounded border flex-1 min-w-0 select-text"
                      >
                        {node.ipv6}
                      </span>
                      <IconButton
                        variant="ghost"
                        className="size-5"
                        type="button"
                        onClick={() => {
                          copyToClipboard(node.ipv6!, {
                            successMessage: `已复制 IPv6: ${node.ipv6}`,
                          });
                        }}
                      >
                        <Copy size={16} />
                      </IconButton>
                    </div>
                  )}
                </div>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-version">
                  {t("admin.nodeDetail.clientVersion", "客户端版本")}
                </label>
                <span
                  id="detail-version"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.version || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-os">
                  {t("admin.nodeDetail.os", "操作系统")}
                </label>
                <span
                  id="detail-os"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.os || <span className="text-muted-foreground">-</span>}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-arch">
                  {t("admin.nodeDetail.arch", "架构")}
                </label>
                <span
                  id="detail-arch"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.arch || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-cpu_name">
                  {t("admin.nodeDetail.cpu", "CPU")}
                </label>
                <span
                  id="detail-cpu_name"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.cpu_name || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-cpu_cores">
                  {t("admin.nodeDetail.cpuCores", "CPU 核心数")}
                </label>
                <span
                  id="detail-cpu_cores"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.cpu_cores?.toString() || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-mem_total">
                  {t("admin.nodeDetail.memTotal", "总内存 (Bytes)")}
                </label>
                <span
                  id="detail-mem_total"
                  className="bg-muted px-3 py-2 rounded border select-text"
                  title={
                    node.mem_total ? String(node.mem_total) + " Bytes" : "-"
                  }
                >
                  {formatBytes(node.mem_total)}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-disk_total">
                  {t("admin.nodeDetail.diskTotal", "总磁盘空间 (Bytes)")}
                </label>
                <span
                  id="detail-disk_total"
                  className="bg-muted px-3 py-2 rounded border select-text"
                  title={
                    node.disk_total ? String(node.disk_total) + " Bytes" : "-"
                  }
                >
                  {formatBytes(node.disk_total)}
                </span>
              </div>
            </div>
            <div className="flex flex-col gap-3">
              <label htmlFor="detail-gpu_name">
                {t("admin.nodeDetail.gpu", "GPU")}
              </label>
              <span
                id="detail-gpu_name"
                className="bg-muted px-3 py-2 rounded border select-text"
              >
                {node.gpu_name || (
                  <span className="text-muted-foreground">-</span>
                )}
              </span>
            </div>
            <div className="flex flex-col gap-3">
              <label htmlFor="detail-uuid">
                {t("admin.nodeDetail.uuid", "UUID")}
              </label>
              <span
                id="detail-uuid"
                className="bg-muted px-3 py-2 rounded border select-text"
              >
                {node.uuid || <span className="text-muted-foreground">-</span>}
              </span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-createdAt">
                  {t("admin.nodeDetail.createdAt", "创建时间")}
                </label>
                <span
                  id="detail-createdAt"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.created_at ? (
                    new Date(node.created_at).toLocaleString()
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-updatedAt">
                  {t("admin.nodeDetail.updatedAt", "更新时间")}
                </label>
                <span
                  id="detail-updatedAt"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.updated_at ? (
                    new Date(node.updated_at).toLocaleString()
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
          </form>
        </div>
        <DrawerFooter>
          <DrawerClose asChild>
            <Button>{t("admin.nodeDetail.done", "完成")}</Button>
          </DrawerClose>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  );
}


export { DetailView };
