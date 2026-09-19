# Frontend Build Instructions / 前端构建说明 / フロントエンド構築手順

## English

### Frontend Source

The bundled frontend is maintained in `frontend/` in this repository.

### Build Requirements

1. Enter `frontend/`, install dependencies, and build the static files.
2. Pack the generated `dist` directory as `tar + zstd -19` to `web/public/defaultTheme/dist.tar.zst`.
3. Copy `frontend/komari-theme.json` to `web/public/defaultTheme/`.
4. Ensure the archive contains `index.html` before building the backend.

The frontend and backend are released together from this repository.

---

## 中文

### 前端源码

前端源码维护在本仓库的 `frontend/` 目录。

### 构建要求

1. 进入 `frontend/`，安装依赖并构建静态文件。
2. 将生成的 `dist` 目录使用 `tar + zstd -19` 打包为 `web/public/defaultTheme/dist.tar.zst`。
3. 将 `frontend/komari-theme.json` 复制到 `web/public/defaultTheme/`。
4. 构建后端前，确保归档包含 `index.html`。

前后端统一从本仓库发布。

---

## 日本語

### フロントエンドソース

フロントエンドはこのリポジトリの `frontend/` で管理します。

### ビルド要件

1. `frontend/` に入り、依存関係をインストールして静的ファイルをビルドします。
2. 生成された `dist` を `tar + zstd -19` で `web/public/defaultTheme/dist.tar.zst` に圧縮します。
3. `frontend/komari-theme.json` を `web/public/defaultTheme/` にコピーします。
4. バックエンドをビルドする前に、アーカイブに `index.html` が含まれていることを確認します。

フロントエンドとバックエンドはこのリポジトリからまとめてリリースします。

---

## Quick Setup / 快速设置 / クイックセットアップ

```bash
cd /path/to/komari/frontend
npm ci
npm run build

cd ..
mkdir -p web/public/defaultTheme
tar -cf /tmp/komari-dist.tar -C frontend/dist .
zstd -19 -T0 -f /tmp/komari-dist.tar -o web/public/defaultTheme/dist.tar.zst
rm -f /tmp/komari-dist.tar
cp frontend/komari-theme.json web/public/defaultTheme/
```
