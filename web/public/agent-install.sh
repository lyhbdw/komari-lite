#!/bin/sh

# Color definitions for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${NC} $1"
}

log_success() {
    echo -e "${GREEN}${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${NC} $1"
}

log_config() {
    echo -e "${CYAN}[CONFIG]${NC} $1"
}

# Keep the script compatible with both POSIX sh and bash.
effective_uid=$(id -u)

# Default values
service_name="komari-agent"
target_dir="/opt/komari"
install_version=""
local_binary=""
download_base="${AGENT_DOWNLOAD_BASE:-}"
sha256_expected="${AGENT_SHA256:-}"
install_dir_specified=false
service_user="${KOMARI_SERVICE_USER:-komari}"
user_service=false
agent_token="${AGENT_TOKEN:-}"
agent_endpoint="${AGENT_ENDPOINT:-}"
credential_file=""
runner_file=""

# Detect OS
os_type=$(uname -s)
case $os_type in
    Linux)
        os_name="linux"
        ;;
    *)
        log_error "Komari Agent Lite supports Linux only"
        exit 1
        ;;
esac

# Parse install-specific arguments
komari_args=""
# [[ ]] -> [ ] (POSIX)
while [ $# -gt 0 ]; do
    case $1 in
        --install-dir)
            target_dir="$2"
            install_dir_specified=true
            shift 2
            ;;
        --install-service-name)
            service_name="$2"
            shift 2
            ;;
        --install-version)
            install_version="$2"
            shift 2
            ;;
        --local-binary)
            local_binary="$2"
            shift 2
            ;;
        --download-base)
            download_base="$2"
            shift 2
            ;;
        --sha256)
            sha256_expected="$2"
            shift 2
            ;;
        --install*)
            log_warning "Unknown install parameter: $1"
            shift
            ;;
        *)
            # Non-install arguments go to komari_args
            komari_args="$komari_args $1"
            shift
            ;;
    esac
done

# Remove leading space from komari_args if present
komari_args="${komari_args# }"

# A direct, unprivileged installation belongs entirely to the invoking user.
if [ "$effective_uid" -ne 0 ] && [ "$install_dir_specified" = false ]; then
    case "$os_name" in
        linux)
            target_dir="${XDG_DATA_HOME:-$HOME/.local/share}/komari"
            ;;
    esac
fi

komari_agent_path="${target_dir}/agent"

# Remove the token from the command line and keep it in a protected file.
# The generated service invokes a wrapper that sources this file.
shell_quote() {
    value=$1
    escaped=$(printf '%s' "$value" | sed "s/'/'\\\\''/g")
    printf "'%s'" "$escaped"
}

extract_token_argument() {
    old_ifs=$IFS
    IFS=' '
    set -f
    set -- $komari_args
    set +f
    IFS=$old_ifs
    rebuilt=""
    skip_next=false
    next_is_endpoint=false
    for arg in "$@"; do
        if [ "$skip_next" = true ]; then
            agent_token="$arg"
            skip_next=false
            continue
        fi
        if [ "$next_is_endpoint" = true ]; then
            agent_endpoint="$arg"
            next_is_endpoint=false
        fi
        case "$arg" in
            -e|--endpoint)
                next_is_endpoint=true
                quoted=$(shell_quote "$arg")
                if [ -n "$rebuilt" ]; then rebuilt="$rebuilt "; fi
                rebuilt="$rebuilt$quoted"
                ;;
            --endpoint=*)
                agent_endpoint=${arg#--endpoint=}
                quoted=$(shell_quote "$arg")
                if [ -n "$rebuilt" ]; then rebuilt="$rebuilt "; fi
                rebuilt="$rebuilt$quoted"
                ;;
            -t|--token)
                skip_next=true
                ;;
            --token=*)
                agent_token=${arg#--token=}
                ;;
            --disable-web-ssh|--web-ssh|--enable-web-ssh|--websocket|--enable-auto-update|--auto-update)
                log_info "Dropping legacy control flag: $arg"
                ;;
            --disable-web-ssh=*|--web-ssh=*|--enable-web-ssh=*|--websocket=*|--enable-auto-update=*|--auto-update=*)
                log_info "Dropping legacy control flag: ${arg%%=*}"
                ;;
            *)
                quoted=$(shell_quote "$arg")
                if [ -n "$rebuilt" ]; then rebuilt="$rebuilt "; fi
                rebuilt="$rebuilt$quoted"
                ;;
        esac
    done
    if [ "$skip_next" = true ]; then
        log_error "Token argument is missing a value"
        exit 1
    fi
    komari_args="$rebuilt"
}

extract_token_argument
case "$agent_token" in
    ""|*[![:print:]]*|*" "*)
        log_error "A non-empty token without whitespace is required"
        exit 1
        ;;
esac

if [ "$effective_uid" -eq 0 ] && [ "$service_user" = "root" ]; then
    log_error "Refusing to install a system service as root; set KOMARI_SERVICE_USER to a dedicated non-root user"
    exit 1
fi
if [ "$effective_uid" -eq 0 ] && ! id "$service_user" >/dev/null 2>&1; then
    login_shell="/sbin/nologin"
    if [ ! -x "$login_shell" ]; then
        login_shell="/usr/sbin/nologin"
    fi
    if ! command -v useradd >/dev/null 2>&1 || ! useradd --system --home-dir "$target_dir" --shell "$login_shell" "$service_user"; then
        log_error "Could not create dedicated service user '$service_user'"
        exit 1
    fi
fi
credential_file="${target_dir}/.agent.env"
runner_file="${target_dir}/run-agent.sh"
# User services are the only service type a non-root Linux installation can manage.
if [ "$effective_uid" -ne 0 ] && [ "$os_name" = "linux" ]; then
    if command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1; then
        user_service=true
    else
        log_error "A non-root Linux installation requires a running systemd user session"
        log_info "Log in through systemd or install with elevated privileges."
        exit 1
    fi
fi

echo -e "${WHITE}===========================================${NC}"
echo -e "${WHITE}    Komari Agent Installation Script     ${NC}"
echo -e "${WHITE}===========================================${NC}"
echo ""
log_config "Installation configuration:"
log_config "  Service name: ${GREEN}$service_name${NC}"
log_config "  Service user: ${GREEN}$service_user${NC}"
log_config "  Install directory: ${GREEN}$target_dir${NC}"
log_config "  Binary arguments: ${GREEN}configured${NC}"
if [ -n "$install_version" ]; then
    log_config "  Specified agent version: ${GREEN}$install_version${NC}"
else
    log_config "  Agent version: ${GREEN}Latest${NC}"
fi
echo ""

new_service_is_active() {
    case "$init_system" in
        systemd)
            systemctl is-active --quiet "${service_name}.service"
            ;;
        systemd-user)
            systemctl --user is-active --quiet "${service_name}.service"
            ;;
        openrc|procd)
            rc-service "$service_name" status >/dev/null 2>&1
            ;;
        upstart)
            initctl status "$service_name" >/dev/null 2>&1
            ;;
        *)
            return 1
            ;;
    esac
}

uninstall_previous() {
    log_step "Checking for previous installation..."

    if [ "$user_service" = true ]; then
        if systemctl --user list-unit-files | grep -q "${service_name}.service"; then
            log_info "Stopping and disabling existing systemd user service..."
            systemctl --user stop "${service_name}.service" || true
            systemctl --user disable "${service_name}.service" || true
            rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/${service_name}.service"
            systemctl --user daemon-reload
        fi
    elif command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files | grep -q "${service_name}.service"; then
        log_info "Stopping and disabling existing systemd service..."
        systemctl stop "${service_name}.service"
        systemctl disable "${service_name}.service"
        rm -f "/etc/systemd/system/${service_name}.service"
        systemctl daemon-reload
    elif command -v rc-service >/dev/null 2>&1 && [ -f "/etc/init.d/${service_name}" ]; then
        log_info "Stopping and disabling existing OpenRC service..."
        rc-service "$service_name" stop
        rc-update del "$service_name" default
        rm -f "/etc/init.d/${service_name}"
    elif command -v uci >/dev/null 2>&1 && [ -f "/etc/init.d/${service_name}" ]; then
        log_info "Stopping and disabling existing procd service..."
        "/etc/init.d/${service_name}" stop
        "/etc/init.d/${service_name}" disable
        rm -f "/etc/init.d/${service_name}"
    elif command -v initctl >/dev/null 2>&1 && [ -f "/etc/init/${service_name}.conf" ]; then
        log_info "Stopping and removing existing upstart service..."
        initctl stop "$service_name"
        rm -f "/etc/init/${service_name}.conf"
    fi
}

install_dependencies() {
    log_step "Checking and installing dependencies..."

    local deps="curl"
    local missing_deps=""
    for cmd in $deps; do
        if ! command -v $cmd >/dev/null 2>&1; then
            missing_deps="$missing_deps $cmd"
        fi
    done

    if [ -n "$missing_deps" ]; then
        if [ "$effective_uid" -ne 0 ]; then
            log_error "Missing required dependencies:$missing_deps"
            log_info "Install them with your system package manager, then run this script again."
            exit 1
        fi
        # Check package manager and install dependencies
        if command -v apt >/dev/null 2>&1; then
            log_info "Using apt to install dependencies..."
            apt update
            apt install -y $missing_deps
        elif command -v yum >/dev/null 2>&1; then
            log_info "Using yum to install dependencies..."
            yum install -y $missing_deps
        elif command -v apk >/dev/null 2>&1; then
            log_info "Using apk to install dependencies..."
            apk add $missing_deps
        elif command -v opkg >/dev/null 2>&1; then # OpenWrt / iStoreOS
            log_info "Using opkg to install dependencies (OpenWrt/iStoreOS)..."
            opkg update
            opkg install $missing_deps
        else
            log_error "No supported package manager found (apt/yum/apk/opkg)"
            exit 1
        fi

        # Verify installation
        for cmd in $missing_deps; do
            if ! command -v $cmd >/dev/null 2>&1; then
                log_error "Failed to install $cmd"
                exit 1
            fi
        done
        log_success "Dependencies installed successfully"
    else
        log_success "Dependencies already satisfied"
    fi
}


# Install dependencies
install_dependencies

if ! command -v sha256sum >/dev/null 2>&1; then
    log_error "sha256sum is required for verified installation"
    exit 1
fi

# Architecture detection
arch=$(uname -m)
case $arch in
    x86_64)
        arch="amd64"
        ;;
    aarch64|arm64)
        arch="arm64"
        ;;
    *)
        log_error "Komari Agent Lite supports Linux amd64 and arm64 only"
        exit 1
        ;;
esac
log_info "Detected OS: ${GREEN}$os_name${NC}, Architecture: ${GREEN}$arch${NC}"

file_name="komari-agent-${os_name}-${arch}"

resolve_snapshot_version() {
    snapshot_api_url="https://api.github.com/repos/Tumb1er1376/komari-agent-lite/releases?per_page=100"
    if ! releases_json=$(curl -fsSL --connect-timeout 15 \
        -H "Accept: application/vnd.github+json" \
        -H "User-Agent: komari-agent-installer" \
        "$snapshot_api_url"); then
        return 1
    fi

    RESOLVED_SNAPSHOT_VERSION=$(printf '%s\n' "$releases_json" |
        grep -o '"tag_name":[[:space:]]*"Snapshot-[^"]*"' |
        sed 's/.*"\(Snapshot-[^"]*\)".*/\1/' |
        LC_ALL=C sort -r |
        head -n 1)
    if [ -n "$RESOLVED_SNAPSHOT_VERSION" ]; then
        return 0
    fi

    return 1
}

version_to_install="latest"
if [ -n "$install_version" ]; then
    if [ "$install_version" = "snapshot" ]; then
        log_info "Resolving the latest snapshot version..."
        if ! resolve_snapshot_version; then
            log_error "Failed to resolve the latest snapshot version."
            exit 1
        fi
        version_to_install="$RESOLVED_SNAPSHOT_VERSION"
        log_success "Latest snapshot version: ${GREEN}$version_to_install${NC}"
    else
        log_info "Attempting to install specified version: ${GREEN}$install_version${NC}"
        version_to_install="$install_version"
    fi
else
    version_to_install="1.0.6"
    log_info "No version specified, defaulting to version: ${GREEN}$version_to_install${NC}"
fi

# Auto-derive download_base from agent_endpoint when not specified
if [ -z "$download_base" ] && [ -n "$agent_endpoint" ]; then
    case "$agent_endpoint" in
        https://*|http://*)
            download_base="${agent_endpoint%/}/download/agent/${version_to_install}"
            log_info "Using controller download base: ${CYAN}$download_base${NC}"
            ;;
    esac
fi

# Construct download URL. A controller-provided base avoids depending on
# GitHub availability while retaining the same mandatory SHA256 verification.
if [ -n "$download_base" ]; then
    case "$download_base" in
        https://*|http://*) ;;
        *)
            log_error "--download-base must use HTTP or HTTPS"
            exit 1
            ;;
    esac
    download_url="${download_base%/}/${file_name}"
else
    if [ "$version_to_install" = "latest" ]; then
        download_path="latest/download"
    else
        download_path="download/${version_to_install}"
    fi
    download_url="https://github.com/Tumb1er1376/komari-agent-lite/releases/${download_path}/${file_name}"
fi

log_step "Creating installation directory: ${GREEN}$target_dir${NC}"
mkdir -p "$target_dir"
if [ "$effective_uid" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$target_dir"
fi

log_step "Preparing $file_name ..."
download_tmp=$(mktemp "${target_dir}/.agent-download.XXXXXX")
cleanup_download() {
    rm -f "$download_tmp"
    cleanup_target_on_exit
}
trap cleanup_download EXIT INT TERM
if [ -n "$local_binary" ]; then
    if [ ! -f "$local_binary" ] || [ ! -r "$local_binary" ]; then
        log_error "Local binary does not exist or is not readable"
        exit 1
    fi
    if [ -z "$sha256_expected" ]; then
        log_error "--local-binary requires --sha256"
        exit 1
    fi
    log_info "Using caller-provided local binary"
    if ! cp -- "$local_binary" "$download_tmp" || [ ! -s "$download_tmp" ]; then
        log_error "Could not stage the local binary"
        exit 1
    fi
else
    log_step "Downloading $file_name ..."
    log_info "URL: ${CYAN}$download_url${NC}"
    if ! curl --fail --location --proto '=https' --tlsv1.2 --connect-timeout 15 \
        -o "$download_tmp" "$download_url" || [ ! -s "$download_tmp" ]; then
        log_error "Download failed from the configured release source"
        exit 1
    fi
fi

if [ -z "$sha256_expected" ]; then
    checksum_url="${download_url}.sha256"
    checksum_tmp=$(mktemp "${target_dir}/.agent-checksum.XXXXXX")
    if ! curl --fail --location --proto '=https' --tlsv1.2 --connect-timeout 15 \
        -o "$checksum_tmp" "$checksum_url"; then
        rm -f "$checksum_tmp"
        log_error "No usable SHA256 checksum was provided; refusing to install an unverified binary"
        exit 1
    fi
    sha256_expected=$(awk 'NF {print $1; exit}' "$checksum_tmp")
    rm -f "$checksum_tmp"
fi
case "$sha256_expected" in
    *[!0-9a-fA-F]*)
        log_error "SHA256 must be exactly 64 hexadecimal characters"
        exit 1
        ;;
esac
if [ "${#sha256_expected}" -ne 64 ]; then
    log_error "SHA256 must be exactly 64 hexadecimal characters"
    exit 1
fi
sha256_actual=$(sha256sum "$download_tmp" | awk '{print $1}')
if [ "$(printf '%s' "$sha256_actual" | tr '[:upper:]' '[:lower:]')" != "$(printf '%s' "$sha256_expected" | tr '[:upper:]' '[:lower:]')" ]; then
    log_error "SHA256 verification failed"
    exit 1
fi

# Only after a verified download may the existing binary be replaced.
if ! mv -f "$download_tmp" "$komari_agent_path"; then
    log_error "Could not install verified binary"
    exit 1
fi
# Service cleanup happens only after the verified binary is in place.
uninstall_previous

# Set executable permissions
chmod +x "$komari_agent_path"
if [ "$effective_uid" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$komari_agent_path"
fi
umask 077
printf 'export AGENT_TOKEN=%s\n' "$(shell_quote "$agent_token")" > "$credential_file"
chmod 0600 "$credential_file"
cat > "$runner_file" << EOF
#!/bin/sh
set -eu
. $(shell_quote "$credential_file")
exec $(shell_quote "$komari_agent_path") ${komari_args}
EOF
chmod 0700 "$runner_file"
if [ "$effective_uid" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$credential_file" "$runner_file"
fi
log_success "Komari-agent installed to ${GREEN}$komari_agent_path${NC}"

# Detect init system and configure service
log_step "Configuring system service..."

# Function to detect actual init system
detect_init_system() {
    # Check if running on NixOS (special case)
    if [ -f /etc/NIXOS ]; then
        echo "nixos"
        return
    fi

    # Alpine Linux MUST be checked first
    # Alpine always uses OpenRC, even in containers where PID 1 might be different
    if [ -f /etc/alpine-release ]; then
        if command -v rc-service >/dev/null 2>&1 || [ -f /sbin/openrc-run ]; then
            echo "openrc"
            return
        fi
    fi

    # Get PID 1 process for other detection
    local pid1_process=$(ps -p 1 -o comm= 2>/dev/null | tr -d ' ')

    # If PID 1 is systemd, use systemd
    if [ "$pid1_process" = "systemd" ] || [ -d /run/systemd/system ]; then
        if command -v systemctl >/dev/null 2>&1; then
            # Additional verification that systemd is actually functioning
            if systemctl list-units >/dev/null 2>&1; then
                echo "systemd"
                return
            fi
        fi
    fi

    # Check for Gentoo OpenRC (PID 1 is openrc-init)
    if [ "$pid1_process" = "openrc-init" ]; then
        if command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
    fi

    # Check for other OpenRC systems (not Alpine, already handled)
    # Some systems use traditional init with OpenRC
    if [ "$pid1_process" = "init" ] && [ ! -f /etc/alpine-release ]; then
        # Check if OpenRC is actually managing services
        if [ -d /run/openrc ] && command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
        # Check for OpenRC files
        if [ -f /sbin/openrc ] && command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
    fi

    # Check for OpenWrt's procd
    if command -v uci >/dev/null 2>&1 && [ -f /etc/rc.common ]; then
        echo "procd"
        return
    fi


    # Fallback: if systemctl exists and appears functional, assume systemd
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl list-units >/dev/null 2>&1; then
            echo "systemd"
            return
        fi
    fi

    # Last resort: check for OpenRC without other indicators
    if command -v rc-service >/dev/null 2>&1 && [ -d /etc/init.d ]; then
        echo "openrc"
        return
    fi

    # check for Upstart (CentOS 6)
    if command -v initctl >/dev/null 2>&1 && [ -d /etc/init ]; then
        echo "upstart"
        return
    fi

    echo "unknown"
}

init_system=$(detect_init_system)
if [ "$user_service" = true ]; then
    init_system="systemd-user"
fi
log_info "Detected init system: ${GREEN}$init_system${NC}"

# Handle each init system
if [ "$init_system" = "nixos" ]; then
    log_warning "NixOS detected. System services must be configured declaratively."
    log_info "Please add the following to your NixOS configuration:"
    echo ""
    echo -e "${CYAN}systemd.services.${service_name} = {${NC}"
    echo -e "${CYAN}  description = \"Komari Agent Service\";${NC}"
    echo -e "${CYAN}  after = [ \"network.target\" ];${NC}"
    echo -e "${CYAN}  wantedBy = [ \"multi-user.target\" ];${NC}"
    echo -e "${CYAN}  serviceConfig = {${NC}"
    echo -e "${CYAN}    Type = \"simple\";${NC}"
    echo -e "${CYAN}    ExecStart = \"${runner_file}\";${NC}"
    echo -e "${CYAN}    WorkingDirectory = \"${target_dir}\";${NC}"
    echo -e "${CYAN}    Restart = \"always\";${NC}"
    echo -e "${CYAN}    User = \"${service_user}\";${NC}"
    echo -e "${CYAN}  };${NC}"
    echo -e "${CYAN}};${NC}"
    echo ""
    log_info "Then run: sudo nixos-rebuild switch"
    log_warning "Service not started automatically on NixOS. Please rebuild your configuration."
elif [ "$init_system" = "openrc" ]; then
    # OpenRC service configuration
    log_info "Using OpenRC for service management"
    service_file="/etc/init.d/${service_name}"
    cat > "$service_file" << EOF
#!/sbin/openrc-run

name="Komari Agent Service"
description="Komari monitoring agent"
command="${runner_file}"
command_user="${service_user}"
directory="${target_dir}"
pidfile="/run/${service_name}.pid"
retry="SIGTERM/30"
supervisor=supervise-daemon

depend() {
    need net
    after network
}
EOF

    # Set permissions and enable service
    chmod +x "$service_file"
    rc-update add ${service_name} default
    rc-service ${service_name} start
    log_success "OpenRC service configured and started"
elif [ "$init_system" = "systemd-user" ]; then
    log_info "Using systemd user service management"
    service_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
    service_file="${service_dir}/${service_name}.service"
    mkdir -p "$service_dir"
    cat > "$service_file" << EOF
[Unit]
Description=Komari Agent Service
After=network.target

[Service]
Type=simple
ExecStart=${runner_file}
WorkingDirectory=${target_dir}
Restart=always
NoNewPrivileges=true
CapabilityBoundingSet=
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
    systemctl --user enable --now "${service_name}.service"
    log_success "Systemd user service configured and started"
elif [ "$init_system" = "systemd" ]; then
    # Systemd service configuration
    log_info "Using systemd for service management"
    service_file="/etc/systemd/system/${service_name}.service"
    cat > "$service_file" << EOF
[Unit]
Description=Komari Agent Service
After=network.target

[Service]
Type=simple
ExecStart=${runner_file}
WorkingDirectory=${target_dir}
Restart=always
NoNewPrivileges=true
CapabilityBoundingSet=
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=${target_dir}
User=${service_user}

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd and start service
    systemctl daemon-reload
    systemctl enable ${service_name}.service
    systemctl start ${service_name}.service
    log_success "Systemd service configured and started"
elif [ "$init_system" = "procd" ]; then
    # procd service configuration (OpenWrt)
    log_info "Using procd for service management"
    service_file="/etc/init.d/${service_name}"
    cat > "$service_file" << EOF
#!/bin/sh /etc/rc.common

START=99
STOP=10

USE_PROCD=1

PROG="${runner_file}"

start_service() {
    procd_open_instance
    procd_set_param command "$PROG"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param user ${service_user}
    procd_close_instance
}

# 移除 killall 版 stop_service:
# USE_PROCD=1 时 rc.common 默认 stop 会通过 procd 正确终止实例,
# 按进程名 killall 反而可能误杀同名进程, 且无法阻止 respawn.

reload_service() {
    stop
    start
}
EOF

    # Set permissions and enable service
    chmod +x "$service_file"
    /etc/init.d/${service_name} enable
    /etc/init.d/${service_name} start
    log_success "procd service configured and started"
elif [ "$init_system" = "upstart" ]; then
    # Upstart service configuration
    log_info "Using upstart for service management"
    service_file="/etc/init/${service_name}.conf"
    cat > "$service_file" << EOF
# KOMARI Agent
description "Komari Agent Service"

chdir ${target_dir}
start on filesystem or runlevel [2345]
stop on runlevel [!2345]

respawn
respawn limit 10 5
umask 022

console none

setuid ${service_user}

pre-start script
    test -x ${komari_agent_path} || { stop; exit 0; }
end script

# Start
script
    exec ${runner_file}
end script
EOF
    # enable Upstart unit
    initctl reload-configuration
    initctl start ${service_name}
    log_success "Upstart service configured and started"
else
    log_error "Unsupported or unknown init system detected: $init_system"
    log_error "Supported init systems: systemd, openrc, procd, upstart"
    exit 1
fi

echo ""
if [ -f /etc/NIXOS ]; then
    log_success "Komari-agent binary installed!"
    log_warning "NixOS requires declarative service configuration."
    log_info "Please add the service configuration to your NixOS config and rebuild."
else
    log_success "Komari-agent installation completed!"
fi
log_config "Service: ${GREEN}$service_name${NC}"
log_config "Arguments: ${GREEN}configured${NC}"
echo -e "${WHITE}===========================================${NC}"
