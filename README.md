# Komari Monitor Lite

Komari Monitor Lite is a self-hosted monitoring panel for personal servers and small infrastructure deployments. It focuses on node status, historical metrics, Ping quality, and notifications, while excluding remote control features.

Current first independent release: `1.0.0`

## Scope

This edition is intended for deployments that need to inspect data, receive alerts, and review history:

- Lightweight agent-based node monitoring
- Web dashboard for node status and live data
- Historical CPU, memory, disk, and network metrics
- Ping task configuration and latency history
- Offline and traffic notifications
- Message channel management
- Data backup and recovery
- Local account login and two-factor authentication (2FA)

## Disabled Features

The following features are disabled to reduce the runtime surface and maintenance cost:

- Remote tasks and script execution
- Web terminal
- Remote file management and file transfer
- Clipboard
- Load alerts
- Plugin installation and plugin marketplace
- Theme marketplace and remote theme management
- OAuth/OIDC login
- pprof profiling endpoints

These features are removed from the admin menu and disabled at the HTTP route, JSON-RPC, and runtime module layers when `KOMARI_LITE=1` is enabled. Disabled endpoints return `404` or permission denied.

## Data Compatibility

Lite mode does not delete existing database tables or perform destructive schema cleanup. Existing nodes, historical metrics, Ping tasks, notification settings, and local accounts remain available.

Enable Lite mode with:

```bash
KOMARI_LITE=1
```

Removing the variable restores full route and feature registration, subject to the build you are running. Back up the data directory before switching modes.

## Quick Start

### Build from Source

Requirements:

- Go 1.25 or a compatible version
- Node.js 23 or a compatible version
- npm
- A CGO build environment
- zstd for repacking frontend assets

The frontend source is included in this repository. To rebuild the embedded frontend:

```bash
cd frontend
npm ci
npm run build
mkdir -p ../web/public/defaultTheme
tar -cf /tmp/komari-dist.tar -C dist .
zstd -19 -T0 -q -f /tmp/komari-dist.tar -o ../web/public/defaultTheme/dist.tar.zst
cp komari-theme.json ../web/public/defaultTheme/komari-theme.json
```

Build the backend:

```bash
cd ..
CGO_ENABLED=1 go build \
  -tags sqlite_omit_load_extension \
  -ldflags "-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=1.0.0" \
  -o komari .
```

Run the service:

```bash
KOMARI_LITE=1 ./komari server
```

The default listen address is `0.0.0.0:25774`, and the default data directory is `data/` below the working directory. In production, use an HTTPS reverse proxy and restrict access to the admin entry point.

### Docker Build

```bash
# First build the Linux amd64 binary as shown above
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
  -tags sqlite_omit_load_extension \
  -ldflags "-s -w -X github.com/komari-monitor/komari/utils.CurrentVersion=1.0.0" \
  -o komari-linux-amd64 .

docker build \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  -t komari-monitor-lite:1.0.0 .

docker run -d \
  --name komari \
  -e KOMARI_LITE=1 \
  -p 25774:25774 \
  -v "$(pwd)/data:/app/data" \
  --restart unless-stopped \
  komari-monitor-lite:1.0.0
```

## Deployment Notes

- Back up the entire `data/` directory before production upgrades.
- Do not commit databases, backups, environment files, credentials, or build outputs to Git.
- This project does not provide remote command execution. Use a separate, controlled administration tool when server operations are required.
- Deploy and use this project only on systems you own or are authorized to manage.

## Relationship to the Original Project

This project is derived from Komari:

<https://github.com/komari-monitor/komari>

The Lite edition uses its own version series and repository. Its first release is `1.0.0`. Upstream changes are not merged automatically; synchronize only after testing, backing up the database, and running a functional regression check.

## License

See `LICENSE` and `NOTICE` for the license and original copyright notices.
