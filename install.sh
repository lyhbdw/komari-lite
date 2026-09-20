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

# $EUID 是 bash 专有变量, ash/dash 下未定义, 补 POSIX 回退
EUID=${EUID:-$(id -u)}

# Default values
service_name="komari-agent"
target_dir="/opt/komari"
install_version="" # New parameter for specifying version
migration_mode=false
install_dir_specified=false
service_user="${SUDO_USER:-$(id -un)}"
user_service=false

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
        --migrate-legacy)
            migration_mode=true
            shift
            ;;
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
if [ "$EUID" -ne 0 ] && [ "$install_dir_specified" = false ]; then
    case "$os_name" in
        linux)
            target_dir="${XDG_DATA_HOME:-$HOME/.local/share}/komari"
            ;;
    esac
fi

komari_agent_path="${target_dir}/agent"

# A migration reuses the existing service command so the node UUID and token stay unchanged.
# The token is read locally from the service definition and is never printed by this script.
if [ "$migration_mode" = true ]; then
    detect_legacy_service() {
        if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files >/dev/null 2>&1; then
            legacy_unit=$(systemctl list-unit-files --type=service --no-legend 2>/dev/null |
                awk '$1 ~ /^komari.*agent.*\\.service$/ {print $1; exit}')
            if [ -n "$legacy_unit" ]; then
                service_name="${legacy_unit%.service}"
                legacy_init="systemd"
                return 0
            fi
        fi
        for candidate in /etc/init.d/komari-agent /etc/init.d/komari; do
            if [ -f "$candidate" ]; then
                service_name=$(basename "$candidate")
                legacy_init="openrc"
                return 0
            fi
        done
        return 1
    }

    if ! detect_legacy_service; then
        log_error "No existing Komari Agent service was found"
        exit 1
    fi

    if [ "$legacy_init" = "systemd" ]; then
        legacy_exec=$(systemctl cat "${service_name}.service" 2>/dev/null |
            sed -n 's/^ExecStart=//p' | tail -n 1)
        if [ -z "$legacy_exec" ]; then
            log_error "Could not read ExecStart from ${service_name}.service"
            exit 1
        fi
        legacy_binary=$(printf '%s\\n' "$legacy_exec" | awk '{print $1}')
        if [ -z "$legacy_binary" ] || [ ! -x "$legacy_binary" ]; then
            log_error "Could not locate the existing Agent binary"
            exit 1
        fi
        target_dir=$(dirname "$legacy_binary")
        komari_agent_path="${target_dir}/agent"
        komari_args=${legacy_exec#"$legacy_binary"}
        komari_args="${komari_args# }"
        detected_user=$(systemctl show "${service_name}.service" -p User --value 2>/dev/null || true)
        if [ -n "$detected_user" ]; then
            service_user="$detected_user"
        else
            service_user="root"
        fi
    else
        legacy_file="/etc/init.d/${service_name}"
        legacy_binary=$(sed -n 's/^command=//p' "$legacy_file" | tail -n 1)
        legacy_args=$(sed -n 's/^command_args=//p' "$legacy_file" | tail -n 1)
        if [ -z "$legacy_binary" ] || [ ! -x "$legacy_binary" ]; then
            log_error "Could not locate the existing Agent binary"
            exit 1
        fi
        target_dir=$(dirname "$legacy_binary")
        komari_agent_path="${target_dir}/agent"
        komari_args="$legacy_args"
    fi

    case " $komari_args " in
        *" -e "*|*" --endpoint "*) ;;
        *) log_error "The existing Agent service has no panel endpoint"; exit 1 ;;
    esac
    case " $komari_args " in
        *" -t "*|*" --token "*) ;;
        *) log_error "The existing Agent service has no token"; exit 1 ;;
    esac
    log_config "Migration source: ${GREEN}${service_name}${NC}"
    log_config "Install directory: ${GREEN}${target_dir}${NC}"
fi

# User services are the only service type a non-root Linux installation can manage.
if [ "$EUID" -ne 0 ] && [ "$os_name" = "linux" ]; then
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

# Function to uninstall the previous installation
uninstall_previous() {
    log_step "Checking for previous installation..."
    
    # Stop and disable service if it exists
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
        systemctl stop ${service_name}.service
        systemctl disable ${service_name}.service
        rm -f "/etc/systemd/system/${service_name}.service"
        systemctl daemon-reload
    elif command -v rc-service >/dev/null 2>&1 && [ -f "/etc/init.d/${service_name}" ]; then
        log_info "Stopping and disabling existing OpenRC service..."
        rc-service ${service_name} stop
        rc-update del ${service_name} default
        rm -f "/etc/init.d/${service_name}"
    elif command -v uci >/dev/null 2>&1 && [ -f "/etc/init.d/${service_name}" ]; then
        log_info "Stopping and disabling existing procd service..."
        /etc/init.d/${service_name} stop
        /etc/init.d/${service_name} disable
        rm -f "/etc/init.d/${service_name}"
    elif command -v initctl >/dev/null 2>&1 && [ -f "/etc/init/${service_name}.conf" ]; then
        log_info "Stopping and removing existing upstart service..."
        initctl stop ${service_name}
        rm -f "/etc/init/${service_name}.conf"
    fi
    
    # Remove old binary if it exists
    if [ -f "$komari_agent_path" ]; then
        log_info "Removing old binary..."
        rm -f "$komari_agent_path"
    fi
}

# Uninstall previous installation
uninstall_previous

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
        if [ "$EUID" -ne 0 ]; then
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
    log_info "No version specified, installing the latest version."
fi

# Construct download URL
if [ "$version_to_install" = "latest" ]; then
    download_path="latest/download"
else
    download_path="download/${version_to_install}"
fi

download_url="https://github.com/Tumb1er1376/komari-agent-lite/releases/${download_path}/${file_name}"

log_step "Creating installation directory: ${GREEN}$target_dir${NC}"
mkdir -p "$target_dir"
if [ "$EUID" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$target_dir"
fi

log_step "Downloading $file_name ..."
log_info "URL: ${CYAN}$download_url${NC}"
if ! curl --fail --location --proto '=https' --tlsv1.2 --connect-timeout 15 \
    -o "$komari_agent_path" "$download_url" || [ ! -s "$komari_agent_path" ]; then
    rm -f "$komari_agent_path"
    log_error "Download failed from GitHub Releases"
    exit 1
fi

# Set executable permissions
chmod +x "$komari_agent_path"
if [ "$EUID" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$komari_agent_path"
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
    echo -e "${CYAN}    ExecStart = \"${komari_agent_path} ${komari_args}\";${NC}"
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
command="${komari_agent_path}"
command_args="${komari_args}"
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
ExecStart=${komari_agent_path} ${komari_args}
WorkingDirectory=${target_dir}
Restart=always

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
ExecStart=${komari_agent_path} ${komari_args}
WorkingDirectory=${target_dir}
Restart=always
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

PROG="${komari_agent_path}"
ARGS="${komari_args}"

start_service() {
    procd_open_instance
    # 参数逐个追加, 避免整串拼接可能导致的引号/转义问题
    procd_set_param command "\$PROG"
    # shellcheck disable=SC2086
    procd_append_param command \$ARGS
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
    exec ${komari_agent_path} ${komari_args}
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
echo -e "${WHITE}===========================================${NC}"
if [ -f /etc/NIXOS ]; then
    log_success "Komari-agent binary installed!"
    log_warning "NixOS requires declarative service configuration."
    log_info "Please add the service configuration to your NixOS config and rebuild."
else
    log_success "Komari-agent installation completed!"
fi
log_config "Service: ${GREEN}$service_name${NC}"
log_config "Arguments: ${GREEN}$komari_args${NC}"
echo -e "${WHITE}===========================================${NC}"


