# Komari Web UI

参与翻译Komari？
- 直接提PR

We use AI to assist with translations. If you find any issues, please let us know!

How to contribute to Komari translations?
- Directly PR

## 开发环境配置

> 我不是计科专业的，代码质量可能达不到平均水平，React是边学边写的，在此之前我从未接触过前端开发，请多包涵。

### 前置 Nodejs

如果未安装，请访问 [Node.js 官网](https://nodejs.org/) 下载并安装。版本建议为 22 及以上。

### 安装依赖

```bash
npm install
```

> 所有指令均在项目根目录下执行

### 修改API地址

1. 复制 `.env.example` 文件并重命名为 `.env.development`。

2. 修改 `.env.development` 文件中的 `VITE_API_TARGET` 为你的开发环境地址。

### 启动开发服务器

```bash
npm run dev
```

### 构建

```bash
npm run build
```

## 主题相关

Lite 版本不提供主题市场、上传、删除或在线更新。内置前端与后端统一发布；构建和嵌入步骤见仓库根目录 `README.md` 以及 `web/public/readme.md`。

已有本地主题仍可从 `data/theme/<short>/` 发现、选择和配置。每个主题目录至少包含：

```text
data/theme/<short>/
├── komari-theme.json
└── dist/
    └── index.html
```

`configuration.type` 仅使用 `managed`；`configuration.data` 为后台生成主题设置表单所需的配置项数组。
