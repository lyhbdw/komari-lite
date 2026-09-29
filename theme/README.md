<h3 align="center"> Komari Lite Theme </h3>
<p align="center">
基于 Vue 3 + Vite + Reka UI + Tailwind CSS v4 构建的 Komari Lite 专属前台主题
</p>

![preview](/docs/preview.png)

## 说明

本项目为 [Komari Lite](https://github.com/lyhbdw/komari-monitor-lite) 的独立专属前台监控主题，针对轻量服务器监控、多币种资产价值展示、暗色/亮色适配以及模态弹窗体验进行了深度定制与优化。

## 环境要求

- Node.js: `^20.19.0` 或 `>=22.12.0`
- Bun / npm / pnpm

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 代码检查
npm run lint
```

## 构建

```bash
# 类型检查 + 生产构建
npm run build

# 预览生产构建
npm run preview
```

## 技术栈

| 类别     | 技术                             |
| -------- | -------------------------------- |
| 框架     | Vue 3                            |
| 构建工具 | Vite 7                           |
| UI 组件  | reka-ui（shadcn-vue 风格组件）   |
| 样式方案 | Tailwind CSS v4 + tw-animate-css |
| 状态管理 | Pinia 3                          |
| 路由     | Vue Router 5                     |
| 提示系统 | vue-sonner（Toaster）            |
| 图标     | @iconify/vue                     |
| 图表     | vue-echarts                      |
| 3D 地球  | cobe                             |
| 实用工具 | @vueuse/core, dayjs              |

## 鸣谢与渊源

- 基于 [Tokinx/komari-theme-emerald](https://github.com/Tokinx/komari-theme-emerald) 与 [Komari Naive](https://github.com/lyimoexiao/komari-theme-naive) 衍生定制
- [Komari Lite](https://github.com/lyhbdw/komari-monitor-lite)

## License

[MIT](./LICENSE)
