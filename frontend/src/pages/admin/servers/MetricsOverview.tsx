import React from "react";
import { useEffect, useState } from "react";
import type { NodeDetail } from "@/contexts/node-details-context";
import { CircleDollarSign, CalendarClock, ArrowUpDown } from "lucide-react";
import { useTranslation } from "react-i18next";
import { calculateTrafficAndSpeedFromStatusMap, formatBytes, formatBytesPerSecond } from "@/utils/unitHelper";
import * as financeCore from "../../../../../theme/src/utils/financeCore";

const MetricsOverview = ({
  nodes,
  selectedStatus,
  setSelectedStatus,
  expiringCount,
}: {
  nodes: NodeDetail[];
  selectedStatus: "all" | "online" | "offline" | "expiring";
  setSelectedStatus: (status: "all" | "online" | "offline" | "expiring") => void;
  expiringCount: number;
}) => {
  const { t } = useTranslation();

  const [trafficStats, setTrafficStats] = useState<{
    totalUp: number;
    totalDown: number;
    totalBytes: number;
    speedUp: number;
    speedDown: number;
    isLoading: boolean;
  }>({
    totalUp: 0,
    totalDown: 0,
    totalBytes: 0,
    speedUp: 0,
    speedDown: 0,
    isLoading: true,
  });

  const [exchangeRates, setExchangeRates] = useState<financeCore.ExchangeRates>(
    () => financeCore.getInitialExchangeRates()
  );
  const [baseCurrency, setBaseCurrency] = useState<financeCore.CurrencyCode>(
    () => financeCore.getStoredFinanceCurrency()
  );
  const [excludeFreeNodes, setExcludeFreeNodes] = useState<boolean>(() =>
    financeCore.shouldExcludeFreeNodes()
  );

  useEffect(() => {
    let active = true;
    const syncFinancePrefs = () => {
      setExcludeFreeNodes(financeCore.shouldExcludeFreeNodes());
      setBaseCurrency(financeCore.getStoredFinanceCurrency());
    };
    syncFinancePrefs();
    financeCore.fetchDailyExchangeRates().then(({ rates }) => {
      if (active) {
        setExchangeRates(rates);
      }
    });
    window.addEventListener("storage", syncFinancePrefs);
    return () => {
      active = false;
      window.removeEventListener("storage", syncFinancePrefs);
    };
  }, []);

  const targetExchangeRate = exchangeRates[baseCurrency] || 1;

  const remainingStats = React.useMemo(() => {
    if (!Array.isArray(nodes)) {
      const zeroFmt = financeCore.formatFinanceAmount(0, baseCurrency);
      const zeroStr = `${zeroFmt.symbol}${zeroFmt.value}`;
      return { total: zeroStr, base: zeroStr, premium: zeroStr, hasPremium: false };
    }
    const totalRemaining =
      financeCore.calculateTotalRemainingValueCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const baseRemaining =
      financeCore.calculateTotalBaseRemainingValueCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const totalPremium =
      financeCore.calculateTotalPremiumCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const fmtTotal = financeCore.formatFinanceAmount(totalRemaining, baseCurrency);
    const fmtBase = financeCore.formatFinanceAmount(baseRemaining, baseCurrency);
    const fmtPrem = financeCore.formatFinanceAmount(totalPremium, baseCurrency);
    return {
      total: `${fmtTotal.symbol}${fmtTotal.value}`,
      base: `${fmtBase.symbol}${fmtBase.value}`,
      premium: `${fmtPrem.symbol}${fmtPrem.value}`,
      hasPremium: totalPremium > 0,
    };
  }, [nodes, exchangeRates, excludeFreeNodes, baseCurrency, targetExchangeRate]);

  const monthlyCostStats = React.useMemo(() => {
    if (!Array.isArray(nodes)) {
      const zeroFmt = financeCore.formatFinanceAmount(0, baseCurrency);
      return {
        totalCNY: `${zeroFmt.symbol}${zeroFmt.value}/月`,
        yearCNY: `年均 ${zeroFmt.symbol}${zeroFmt.value}`,
      };
    }
    const totalAmount =
      financeCore.calculateTotalMonthlyAverageCostCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const fmtTotal = financeCore.formatFinanceAmount(totalAmount, baseCurrency);
    const fmtYear = financeCore.formatFinanceAmount(totalAmount * 12, baseCurrency);
    return {
      totalCNY: `${fmtTotal.symbol}${fmtTotal.value}/月`,
      yearCNY: `年均 ${fmtYear.symbol}${fmtYear.value}`,
    };
  }, [nodes, exchangeRates, excludeFreeNodes, baseCurrency, targetExchangeRate]);

  const nodeUuidsRef = React.useRef<string[]>([]);
  useEffect(() => {
    nodeUuidsRef.current = Array.isArray(nodes)
      ? nodes.map((n) => n.uuid).filter(Boolean)
      : [];
  }, [nodes]);

  useEffect(() => {
    let isMounted = true;
    let inFlight = false;
    const fetchTraffic = async () => {
      if (inFlight || (typeof document !== "undefined" && document.hidden))
        return;
      inFlight = true;
      try {
        const res = await fetch("/api/rpc2", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "common:getNodesLatestStatus",
            params: [],
          }),
        });

        if (!res.ok) return;
        const data = await res.json();
        const stats = calculateTrafficAndSpeedFromStatusMap(
          data.result,
          nodeUuidsRef.current
        );

        if (isMounted) {
          setTrafficStats({
            ...stats,
            isLoading: false,
          });
        }
      } catch {
        // ignore
      } finally {
        inFlight = false;
      }
    };

    fetchTraffic();
    const handleVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        fetchTraffic();
      }
    };
    const timer = setInterval(fetchTraffic, 5000);
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      isMounted = false;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  const isExpiringActive = selectedStatus === "expiring";

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
      {/* 1. 资产与预算 */}
      <div className="p-3.5 rounded-xl border border-border/70 bg-card shadow-2xs hover:border-foreground/25 transition-all flex flex-col justify-between h-[84px]">
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight">{t("admin.overview.financial_overview", "资产与预算")}</span>
          <div className="w-6 h-6 rounded-md bg-muted/60 text-muted-foreground flex items-center justify-center shrink-0">
            <CircleDollarSign size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {remainingStats.total}
            </span>
            <span className="text-[11px] text-muted-foreground">剩余</span>
          </div>
          <div className="px-2 py-0.5 rounded-md bg-muted/50 border border-border/40 text-[11px] font-mono text-muted-foreground flex items-center gap-1">
            <span>支出</span>
            <span className="text-foreground/90 font-medium">{monthlyCostStats.totalCNY}</span>
          </div>
        </div>
      </div>

      {/* 2. 流量与速率 */}
      <div className="p-3.5 rounded-xl border border-border/70 bg-card shadow-2xs hover:border-foreground/25 transition-all flex flex-col justify-between h-[84px]">
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight">{t("admin.overview.traffic_total", "累计流量与速率")}</span>
          <div className="w-6 h-6 rounded-md bg-muted/60 text-muted-foreground flex items-center justify-center shrink-0">
            <ArrowUpDown size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {trafficStats.isLoading && trafficStats.totalBytes === 0
                ? "..."
                : formatBytes(trafficStats.totalBytes)}
            </span>
            <span className="text-[11px] text-muted-foreground">累计</span>
          </div>
          <div
            className="px-2 py-0.5 rounded-md bg-muted/50 border border-border/40 text-[11px] font-mono text-muted-foreground flex items-center gap-1.5 cursor-help"
            title={`累计上行: ${formatBytes(trafficStats.totalUp)} · 累计下行: ${formatBytes(trafficStats.totalDown)}`}
          >
            <span className="flex items-center gap-0.5">
              <span className="opacity-60">↑</span>
              <span className="text-foreground/90 font-medium">
                {trafficStats.isLoading ? "..." : formatBytesPerSecond(trafficStats.speedUp)}
              </span>
            </span>
            <span className="opacity-40">·</span>
            <span className="flex items-center gap-0.5">
              <span className="opacity-60">↓</span>
              <span className="text-foreground/90 font-medium">
                {trafficStats.isLoading ? "..." : formatBytesPerSecond(trafficStats.speedDown)}
              </span>
            </span>
          </div>
        </div>
      </div>

      {/* 3. 待续费机器 */}
      <div
        onClick={() => setSelectedStatus(isExpiringActive ? "all" : "expiring")}
        className={`p-3.5 rounded-xl border bg-card shadow-2xs cursor-pointer transition-all duration-150 group select-none flex flex-col justify-between h-[84px] ${
          isExpiringActive
            ? "border-amber-500/80 ring-2 ring-amber-500/20 bg-amber-500/5 dark:bg-amber-500/10"
            : "border-border/70 hover:border-amber-500/50 hover:shadow-xs"
        }`}
        title={isExpiringActive ? "点击恢复展示全部节点" : "点击在下方列表筛选这批临期节点"}
      >
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight group-hover:text-amber-600 dark:group-hover:text-amber-400 transition-colors">
            {t("admin.overview.expiring_soon", "待续费机器")}
          </span>
          <div
            className={`w-6 h-6 rounded-md flex items-center justify-center shrink-0 transition-colors ${
              isExpiringActive
                ? "bg-amber-500/20 text-amber-600 dark:text-amber-400"
                : "bg-muted/60 text-muted-foreground group-hover:bg-amber-500/10 group-hover:text-amber-600 dark:group-hover:text-amber-400"
            }`}
          >
            <CalendarClock size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {expiringCount} 台
            </span>
            <span className="text-[11px] text-muted-foreground">临期</span>
          </div>
          {expiringCount > 0 ? (
            <div className="px-2 py-0.5 rounded-md bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30 text-[11px] font-medium flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-500 inline-block animate-pulse" />
              <span>{isExpiringActive ? "已筛选" : "需要续费"}</span>
            </div>
          ) : (
            <div className="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />
              <span>全部正常</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};


export { MetricsOverview };
