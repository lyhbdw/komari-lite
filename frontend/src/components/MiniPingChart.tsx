import { useEffect, useMemo, useState } from "react";
import { Card, Switch } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import Loading from "@/components/loading";
import UplotChart from "@/components/ui/chartUplot";
import PingMetricStatContent from "@/components/PingMetricStatContent";
import Tips from "@/components/ui/tips";
import { useRPC2Call } from "@/contexts/useRPC2";
import { cn } from "@/lib/utils";
import type {
  MetricSeries,
  MetricTags,
  PingMetricStat,
  PingMetricStatsResponse,
  PublicPingTask,
  QueryMetricsResponse,
} from "@/types/metrics";
import {
  PING_LATENCY_METRIC,
  applyMetricEwma,
  comparePingTaskOrder,
  formatMetricBoundaryTime,
  formatRemainingTags,
  metricSeriesColor,
  metricSeriesDataKey,
  metricSeriesKey,
  metricTags,
  normalizeMetricSeriesList,
  pingMetricStatKey,
  pingTaskId,
  pingTaskName,
  trimMetricChartBoundaryRows,
  type MetricChartRow,
} from "@/utils/metricSeries";

type RenderSeries = {
  dataKey: string;
  stableKey: string;
  taskId?: string;
  name: string;
  color: string;
  pointCount: number;
  tags?: MetricTags;
};

type MiniPingChartProps = {
  uuid: string;
  width?: string | number;
  height?: string | number;
  hours?: number;
};

const MiniPingChart = ({
  uuid,
  width = "100%",
  height = 300,
  hours = 12,
}: MiniPingChartProps) => {
  const { t } = useTranslation();
  const { call } = useRPC2Call();
  const [metricSeries, setMetricSeries] = useState<MetricSeries[]>([]);
  const [tasks, setTasks] = useState<PublicPingTask[]>([]);
  const [stats, setStats] = useState<PingMetricStat[]>([]);
  const [hiddenLines, setHiddenLines] = useState<Record<string, boolean>>({});
  const [ewmaEnabled, setEwmaEnabled] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!uuid) {
      setMetricSeries([]);
      setTasks([]);
      setStats([]);
      return;
    }

    let active = true;
    setLoading(true);
    setError(null);
    setHiddenLines({});

    const taskRequest = call<unknown, PublicPingTask[]>(
      "public:getPublicPingTasks",
    ).catch(() => []);
    const metricRequest = call<unknown, QueryMetricsResponse>(
      "public:queryMetrics",
      {
        metric_keys: [PING_LATENCY_METRIC],
        entity_id: uuid,
        hours,
        max_points: 240,
        aggregation: "avg",
        fill_empty: true,
      },
      { timeout: 30000 },
    );
    const statsRequest = call<unknown, PingMetricStatsResponse>(
      "public:getPingMetricStats",
      { entity_id: uuid, hours, max_points: 240 },
      { timeout: 30000 },
    ).catch(() => null);

    Promise.all([taskRequest, metricRequest, statsRequest])
      .then(([taskList, result, statsResult]) => {
        if (!active) return;
        setTasks(Array.isArray(taskList) ? taskList : []);
        setMetricSeries(normalizeMetricSeriesList(result?.series));
        setStats(Array.isArray(statsResult?.stats) ? statsResult.stats : []);
        setLoading(false);
      })
      .catch((requestError) => {
        if (!active) return;
        setError(
          requestError instanceof Error ? requestError.message : "Error",
        );
        setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [call, hours, uuid]);

  const taskMap = useMemo(
    () => new Map(tasks.map((task) => [String(task.id), task])),
    [tasks],
  );

  const statsMap = useMemo(() => {
    const map = new Map<string, PingMetricStat>();
    for (const stat of stats) {
      map.set(pingMetricStatKey(stat.entity_id, stat.task_id), stat);
    }
    return map;
  }, [stats]);

  const built = useMemo(() => {
    const rows = new Map<string, MetricChartRow>();
    const renderSeries: RenderSeries[] = [];

    metricSeries.forEach((series, index) => {
      const tags = metricTags(series);
      const taskId = pingTaskId(tags);
      const taskLabel = taskId
        ? pingTaskName(taskId, taskMap, (id) => `${t("ping.task")} ${id}`)
        : t("ping.task");
      const remainingTags = formatRemainingTags(tags, ["task_id"]);
      const stableKey = metricSeriesKey(series.metric_key, tags);
      const dataKey = metricSeriesDataKey(series.metric_key, tags);
      renderSeries.push({
        dataKey,
        stableKey,
        taskId,
        name: remainingTags ? `${taskLabel} ${remainingTags}` : taskLabel,
        color: metricSeriesColor(index),
        pointCount: (series.points ?? []).reduce(
          (count, point) => count + (typeof point.value === "number" ? 1 : 0),
          0,
        ),
        tags,
      });

      for (const point of series.points ?? []) {
        const time = new Date(point.time).toISOString();
        const row = rows.get(time) ?? { time };
        row[dataKey] =
          typeof point.value === "number" && point.value >= 0
            ? point.value
            : null;
        rows.set(time, row);
      }
    });

    const orderedSeries = renderSeries
      .sort((left, right) =>
        comparePingTaskOrder(left.tags, right.tags, taskMap),
      )
      .map((series, index) => ({
        ...series,
        color: metricSeriesColor(index),
      }));

    return {
      rows: Array.from(rows.values()).sort(
        (left, right) =>
          new Date(String(left.time)).getTime() -
          new Date(String(right.time)).getTime(),
      ),
      series: orderedSeries,
    };
  }, [metricSeries, t, taskMap]);

  const chartData = useMemo(
    () =>
      trimMetricChartBoundaryRows(
        applyMetricEwma(built.rows, built.series, ewmaEnabled),
        built.series.map((item) => item.dataKey),
      ),
    [built.rows, built.series, ewmaEnabled],
  );

  const labelFormatter = (value: string | number) =>
    new Date(value).toLocaleString([], {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });

  const toggleLine = (dataKey: string) => {
    setHiddenLines((current) => ({ ...current, [dataKey]: !current[dataKey] }));
  };

  const visibleSeries = built.series.filter(
    (item) => hiddenLines[item.dataKey] !== true,
  );
  const uplotData: [Array<number | null>, ...Array<Array<number | null>>] = [
    chartData.map((row) => new Date(String(row.time)).getTime()),
    ...visibleSeries.map((item) =>
      chartData.map((row) => {
        const value = row[item.dataKey];
        return typeof value === "number" && Number.isFinite(value)
          ? value
          : null;
      }),
    ),
  ];

  return (
    <Card
      style={{ width, height }}
      className="km-mini-ping-chart flex min-h-0 flex-col gap-2 overflow-hidden"
    >
      {loading && (
        <div className="flex min-h-0 flex-1 items-center justify-center">
          <Loading />
        </div>
      )}
      {!loading && error && (
        <div className="flex min-h-0 flex-1 items-center justify-center text-red-500">
          {error}
        </div>
      )}
      {!loading && !error && chartData.length === 0 && (
        <div className="flex min-h-0 flex-1 items-center justify-center text-muted-foreground">
          {t("common.none")}
        </div>
      )}
      {!loading && !error && chartData.length > 0 && (
        <>
          <div className="flex max-h-16 shrink-0 flex-wrap items-center gap-1 overflow-y-auto">
            {built.series.map((item) => {
              const hidden = hiddenLines[item.dataKey] === true;
              const stat = item.taskId
                ? statsMap.get(pingMetricStatKey(uuid, item.taskId))
                : undefined;
              return (
                <div
                  key={item.stableKey}
                  className={cn(
                    "inline-flex max-w-full items-center overflow-hidden rounded-md text-xs",
                    hidden
                      ? "bg-accent-2 text-muted-foreground"
                      : "bg-accent-3 text-accent-12",
                  )}
                >
                  <button
                    type="button"
                    onClick={() => toggleLine(item.dataKey)}
                    className="inline-flex min-w-0 items-center gap-1 px-2 py-1"
                  >
                    <span
                      className="size-2 shrink-0 rounded-[2px]"
                      style={{
                        backgroundColor: hidden ? "var(--gray-8)" : item.color,
                      }}
                    />
                    <span className={cn("truncate", hidden && "line-through")}>
                      {item.name}
                    </span>
                  </button>
                  {stat && (
                    <Tips
                      mode="auto"
                      side="top"
                      className="shrink-0"
                      ariaLabel={`${item.name} ${t("common.details")}`}
                    >
                      <PingMetricStatContent stat={stat} t={t} />
                    </Tips>
                  )}
                </div>
              );
            })}
          </div>

          <UplotChart
            className="km-mini-ping-chart-canvas min-h-0 h-10/12 w-full flex-1"
            data={uplotData}
            isEmpty={chartData.length === 0}
            series={visibleSeries.map((item) => ({
              label: item.name,
              stroke: item.color,
              width: 2,
              showPoints: item.pointCount <= 30,
            }))}
            axisLabels={{
              // Boundary-only x ticks, mirroring MetricBoundaryAxisTick.
              xValues: (values) =>
                values.map((value) => formatMetricBoundaryTime(value)),
              yValues: (splits) =>
                splits.map((value) => `${Math.round(value)}\u00a0ms`),
            }}
            renderTooltip={(_index, xValue, rows) => (
              <div className="km-chart-tooltip">
                <div className="km-chart-tooltip-label">
                  {labelFormatter(xValue)}
                </div>
                <div className="km-chart-tooltip-rows">
                  {rows.map((row) => (
                    <div className="km-chart-tooltip-row" key={row.label}>
                      <span
                        className="km-chart-tooltip-dot"
                        style={{ backgroundColor: row.color }}
                      />
                      <span className="text-muted-foreground">{row.label}</span>
                      <span className="km-chart-tooltip-value">
                        {`${Math.round(Number(row.value))} ms`}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          />

          <div className="flex shrink-0 items-center gap-2">
            <Switch
              size="1"
              checked={ewmaEnabled}
              onCheckedChange={setEwmaEnabled}
              aria-label="EWMA"
            />
            <span className="text-sm font-medium">EWMA</span>
            <Tips mode="auto" side="top">
              <span
                dangerouslySetInnerHTML={{ __html: t("chart.cutPeak_tips") }}
              />
            </Tips>
          </div>
        </>
      )}
    </Card>
  );
};

export default MiniPingChart;
