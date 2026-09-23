import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Uplot from "uplot";
import type { Axis, Series as UplotSeries, Cursor as UplotCursor } from "uplot";
import "uplot/dist/uPlot.min.css";
import "./chartUplot.css";

type ChartTooltipRow = {
  label: string;
  value: string;
  color: string;
};

type ChartSeries = {
  /** Shown in the legend/tooltip. */
  label: string;
  /** Line color. */
  stroke: string;
  width?: number;
  /** Draw a circular marker on every data point. */
  showPoints?: boolean;
  /** When omitted the series inherits the chart's primary y scale. */
  scale?: string;
};

type AxisLabels = {
  /** Labels for the boundary x values (first and last sample). */
  xValues: (values: number[]) => string[];
  /** Labels for the y axis grid lines. */
  yValues: (splits: number[]) => string[];
};

type Props = {
  /** Column-aligned data: data[0] is x, data[1..n] are y series. */
  data: [Array<number | null>, ...Array<Array<number | null>>];
  series: ChartSeries[];
  height?: number;
  className?: string;
  /** Called with the hovered row index, or null when the cursor leaves. */
  onActiveIndexChange?: (index: number | null) => void;
  /** Tooltip content for the hovered row index. */
  renderTooltip?: (
    index: number,
    xValue: number,
    rows: ChartTooltipRow[],
  ) => React.ReactNode;
  /** Axis tick labels; omit for theme defaults (numeric). */
  axisLabels?: AxisLabels;
  /** Rendered by the consumer when there is no data. */
  isEmpty?: boolean;
};

const AXIS_FONT = "11px system-ui, -apple-system, sans-serif";
/** Recharts only drew dots with <= 30 points; match that threshold. */
const SPARSE_POINT_LIMIT = 30;
/**
 * Canvas cannot read CSS custom properties, so resolve theme colors from the
 * document root instead of using var() inside uPlot options.
 */
const cssVar = (name: string, fallback: string) => {
  if (typeof window === "undefined") return fallback;
  const resolved = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return resolved || fallback;
};

const themeColors = () => ({
  axis: cssVar("--muted-foreground", "#64748b"),
  grid: cssVar("--border", "#e2e8f0"),
});

/**
 * React wrapper around uPlot that reproduces the visual contract of the
 * Recharts charts it replaces: boundary-only x ticks, horizontal-only grid,
 * y axis on the right, no entry animation, and nulls that break the line
 * rather than being interpolated.
 */
export default function Chart({
  data,
  series,
  height = 220,
  className,
  onActiveIndexChange,
  renderTooltip,
  axisLabels,
  isEmpty = false,
}: Props) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const plotRef = useRef<Uplot | null>(null);
  const activeIndexRef = useRef<number | null>(null);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const [cursorX, setCursorX] = useState<number | null>(null);
  const [width, setWidth] = useState(0);

  // uPlot callbacks outlive a single render, so always read the newest props.
  const latest = useRef({ data, series, axisLabels });
  latest.current = { data, series, axisLabels };

  const hasData = data.some((column) => column.length > 0);
  const renderEmpty = Boolean(isEmpty) && !hasData;

  // Rebuilding a uPlot instance is expensive; only rebuild when the chart's
  // structure (series count/colors/point mode) or size changes.
  const structureKey = useMemo(
    () =>
      series
        .map((item) =>
          [
            item.label,
            item.stroke,
            item.width ?? 2,
            item.scale ?? "",
            item.showPoints ? 1 : 0,
          ].join("\u0000"),
        )
        .join("\u0001"),
    [series],
  );

  const dataKey = useMemo(
    () => data.map((column) => column.join(" ")).join("\u0002"),
    [data],
  );

  const buildData = useCallback(() => {
    const table = latest.current.data;
    const rowCount = Math.max(1, table[0]?.length ?? 0);
    const columns = table.map((column) => {
      const next = new Array<number | null>(rowCount);
      for (let index = 0; index < rowCount; index += 1) {
        const value = column?.[index];
        next[index] =
          typeof value === "number" && Number.isFinite(value) ? value : null;
      }
      return next;
    });
    return columns as unknown as [number[], ...Array<Array<number | null>>];
  }, []);

  // Track host width so the chart stays responsive.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return undefined;

    const applyWidth = (next: number) => {
      const rounded = Math.max(0, Math.round(next));
      setWidth((current) => (current === rounded ? current : rounded));
    };

    applyWidth(host.getBoundingClientRect().width);

    if (typeof ResizeObserver === "undefined") return undefined;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (rect) applyWidth(rect.width);
    });
    observer.observe(host);
    return () => observer.disconnect();
  }, []);

  const setActive = useCallback((index: number | null) => {
    if (activeIndexRef.current === index) return;
    activeIndexRef.current = index;
    setActiveIndex(index);
  }, []);

  useEffect(() => {
    onActiveIndexChange?.(activeIndex);
  }, [activeIndex, onActiveIndexChange]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host || width <= 0) return undefined;

    const { axis: axisColor, grid: gridColor } = themeColors();
    const labels = latest.current.axisLabels;

    const plotSeries: UplotSeries[] = [{ label: "Time" }];
    for (const item of latest.current.series) {
      const sparse = item.showPoints === true;
      plotSeries.push({
        label: item.label,
        stroke: item.stroke,
        width: item.width ?? 2,
        scale: item.scale,
        spanGaps: false,
        points: {
          show: sparse,
          // Recharts only rendered dots when a series had <= 30 points; uPlot
          // hides a point when its neighbours are closer than `space`.
          size: 8,
          space: 8 * SPARSE_POINT_LIMIT,
          width: 2,
          stroke: item.stroke,
          fill: "#ffffff",
        },
      });
    }

    /** First and last x scale value, mirroring the Recharts boundary ticks. */
    const boundaryTimes = (plot: Uplot): number[] => {
      const column = plot.data[0] ?? [];
      if (column.length === 0) return [];
      const first = Number(column[0]);
      const last = Number(column[column.length - 1]);
      if (!Number.isFinite(first) || !Number.isFinite(last)) return [];
      return first === last ? [first] : [first, last];
    };

    const axes: Axis[] = [
      {
        stroke: axisColor,
        grid: { show: false },
        ticks: { show: false },
        border: { show: false },
        font: AXIS_FONT,
        size: 20,
        gap: 8,
        // uPlot expects splits to be scale values, not data indexes; these two
        // values land exactly on the first and last sample.
        splits: (plot: Uplot) => boundaryTimes(plot),
        values: (plot: Uplot) => {
          const times = boundaryTimes(plot);
          return labels ? labels.xValues(times) : times.map(String);
        },
      },
      {
        scale: series[0]?.scale,
        stroke: axisColor,
        grid: { show: true, stroke: gridColor, width: 1 },
        ticks: { show: false },
        border: { show: false },
        font: AXIS_FONT,
        size: 52,
        gap: 5,
        // Recharts rendered the y axis mirrored on the right.
        side: 3,
        values: (_plot: Uplot, splits: number[]) =>
          labels ? labels.yValues(splits) : splits.map(String),
      },
    ];

    // A second y scale is required when series straddle two units.
    if (series.some((item) => item.scale !== series[0]?.scale)) {
      axes.push({
        scale: series[series.length - 1]?.scale ?? "y",
        stroke: axisColor,
        grid: { show: false },
        ticks: { show: false },
        border: { show: false },
        font: AXIS_FONT,
        size: 52,
        gap: 5,
        side: 3,
        values: (_plot: Uplot, splits: number[]) =>
          labels ? labels.yValues(splits) : splits.map(String),
      });
    }

    const cursor: UplotCursor = {
      y: false,
      x: true,
      drag: { setScale: false },
      points: {
        one: false,
        // Canvas hover points would need Series.Points.show (boolean); the cursor
        // points are DOM nodes, so only size/width/stroke/fill are used here.
        size: (_plot: Uplot, seriesIndex: number) =>
          latest.current.series[seriesIndex - 1]?.showPoints === true ? 4 : 0,
        width: 2,
        stroke: (plot: Uplot, seriesIndex: number) =>
          (plot.series[seriesIndex]?.stroke as string | undefined) ?? axisColor,
        fill: "#ffffff",
      },
    };

    const plot = new Uplot(
      {
        width,
        height,
        series: plotSeries,
        scales: {
          x: { time: true },
          y: {
            // Match the Recharts "auto" domain: pad the extremes slightly so
            // lines do not sit on the grid edge.
            range: (_plot: Uplot, min: number, max: number) => [min, max],
          },
        },
        axes,
        cursor,
        legend: { show: false },
        padding: [8, 0, 0, 0],
      },
      // uPlot requires a valid x column at construction; real data is pushed by
      // the ready hook below.
      [],
      host,
    );

    plotRef.current = plot;

    plot.hooks.ready?.push((instance) => instance.setData(buildData(), true));
    plot.hooks.setLegend?.push((instance) => {
      const index = instance.cursor.idx;
      setActive(index === null || index === undefined ? null : Number(index));
    });
    // Track the cursor so the tooltip follows it, the way Recharts anchored the
    // tooltip to the hovered sample.
    plot.hooks.setCursor?.push((instance) => {
      const left = instance.cursor?.left;
      setCursorX(typeof left === "number" && left >= 0 ? left : null);
    });
    plot.hooks.destroy?.push(() => {
      setActive(null);
      setCursorX(null);
    });

    return () => {
      plot.destroy();
      plotRef.current = null;
    };
    // structureKey covers series/size/axis-label changes; dataKey is pushed
    // separately so new data does not rebuild the plot instance.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildData, height, setActive, structureKey, width]);

  // Push data updates in place instead of recreating the plot.
  useEffect(() => {
    const plot = plotRef.current;
    if (!plot) return;
    plot.setData(buildData(), true);
    if (hasData) setActive(null);
  }, [buildData, dataKey, hasData, setActive]);

  // Positions the tooltip above the cursor so it follows the hovered point.
  const tooltip = useMemo(() => {
    if (activeIndex === null || !renderTooltip) return null;
    const rows: ChartTooltipRow[] = [];
    latest.current.series.forEach((item, seriesIndex) => {
      const value = latest.current.data[seriesIndex + 1]?.[activeIndex];
      if (typeof value !== "number" || !Number.isFinite(value)) return;
      rows.push({ label: item.label, value: String(value), color: item.stroke });
    });
    const xValue = latest.current.data[0]?.[activeIndex];
    if (typeof xValue !== "number" || !Number.isFinite(xValue)) return null;
    return renderTooltip(activeIndex, xValue, rows);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeIndex, dataKey, structureKey]);

  return (
    <div
      className={className}
      style={{ width: "100%", height, position: "relative" }}
    >
      {renderEmpty && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-sm text-muted-foreground" />
      )}
      <div
        ref={hostRef}
        className="km-chart-host"
        style={{ width: "100%", height: renderEmpty ? 0 : height }}
        hidden={renderEmpty}
      />
      {tooltip && (
        <div
          className="km-chart-tooltip-anchor"
          style={{ left: cursorX ?? 0, top: 0 }}
        >
          {tooltip}
        </div>
      )}
    </div>
  );
}
