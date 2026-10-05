import dayjs from 'dayjs'
/**
 * ECharts 共享配置
 *
 * 统一注册所有图表组件，避免在各个组件中重复注册
 */
import { LineChart, MapChart, ScatterChart } from 'echarts/charts'
import {
  GeoComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  TooltipComponent,
} from 'echarts/components'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'

// 一次性注册所有需要的 ECharts 组件
use([
  LineChart,
  MapChart,
  ScatterChart,
  GridComponent,
  GeoComponent,
  TooltipComponent,
  LegendComponent,
  MarkLineComponent,
  CanvasRenderer,
])

interface ChartThemeColors {
  text: string
  textSecondary: string
  textTertiary: string
  borderColor: string
  splitLineColor: string
  tooltipBg: string
  tooltipShadow: string
  crosshairColor: string
  fontFamily: string
}

export function getChartThemeColors(isDark: boolean): ChartThemeColors {
  return {
    text: isDark ? 'rgba(230, 227, 223, 0.90)' : 'rgba(44, 40, 37, 0.90)',
    textSecondary: isDark ? 'rgba(230, 227, 223, 0.65)' : 'rgba(44, 40, 37, 0.68)',
    textTertiary: isDark ? 'rgba(230, 227, 223, 0.45)' : 'rgba(44, 40, 37, 0.45)',
    borderColor: isDark ? 'rgba(230, 227, 223, 0.15)' : 'rgba(44, 40, 37, 0.12)',
    splitLineColor: isDark ? 'rgba(230, 227, 223, 0.08)' : 'rgba(44, 40, 37, 0.08)',
    tooltipBg: isDark ? 'rgba(26, 25, 24, 0.96)' : 'rgba(253, 252, 250, 0.96)',
    tooltipShadow: isDark ? 'rgba(0, 0, 0, 0.5)' : 'rgba(0, 0, 0, 0.08)',
    crosshairColor: isDark ? 'rgba(230, 227, 223, 0.2)' : 'rgba(44, 40, 37, 0.15)',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  }
}

export function formatChartTimeForTooltip(time: string, hours: number): string {
  const date = dayjs(time)
  if (hours < 24) {
    return date.format('HH:mm:ss')
  }
  return date.format('MM/DD HH:mm')
}
