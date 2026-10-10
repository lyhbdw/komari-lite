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
set -eu
umask 077
effective_uid=$(id -u)

# Default values
service_name="komari-agent"
target_dir="${KOMARI_INSTALL_DIR:-/opt/komari}"
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
systemd_dir="${KOMARI_SYSTEMD_DIR:-/etc/systemd/system}"
init_dir="${KOMARI_INIT_DIR:-/etc/init.d}"
upstart_dir="${KOMARI_UPSTART_DIR:-/etc/init}"
run_dir="${KOMARI_RUN_DIR:-/run}"
rc_common="${KOMARI_RC_COMMON:-/etc/rc.common}"

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
        systemd) systemctl is-active --quiet "${service_name}.service" ;;
        systemd-user) systemctl --user is-active --quiet "${service_name}.service" ;;
        openrc) rc-service "$service_name" status >/dev/null 2>&1 ;;
        procd) "$service_file" running >/dev/null 2>&1 ;;
        upstart)
            # initctl status returns success even for stop/waiting.
            status_output=$(initctl status "$service_name") || return 1
            case "$status_output" in *" start/running"*) return 0 ;; *) return 1 ;; esac
            ;;
        *) return 1 ;;
    esac
}

stop_managed_service() {
    case "$init_system" in
        systemd) systemctl stop "${service_name}.service" ;;
        systemd-user) systemctl --user stop "${service_name}.service" ;;
        openrc) rc-service "$service_name" stop ;;
        procd) "$service_file" stop ;;
        upstart) initctl stop "$service_name" ;;
        nixos) return 0 ;;
        *) return 1 ;;
    esac
}

start_managed_service() {
    case "$init_system" in
        systemd) systemctl start "${service_name}.service" ;;
        systemd-user) systemctl --user start "${service_name}.service" ;;
        openrc) rc-service "$service_name" start ;;
        procd) "$service_file" start ;;
        upstart) initctl start "$service_name" ;;
        *) return 1 ;;
    esac
}

reload_manager() {
    case "$init_system" in
        systemd) systemctl daemon-reload ;;
        systemd-user) systemctl --user daemon-reload ;;
        upstart) initctl reload-configuration ;;
        *) return 0 ;;
    esac
}

set_service_enabled() {
    action=$1
    case "$init_system" in
        systemd) systemctl "$action" "${service_name}.service" ;;
        systemd-user) systemctl --user "$action" "${service_name}.service" ;;
        openrc)
            if [ "$action" = enable ]; then rc-update add "$service_name" default
            else rc-update del "$service_name" default; fi
            ;;
        procd) "$service_file" "$action" ;;
        *) return 0 ;;
    esac
}

service_is_enabled() {
    case "$init_system" in
        systemd) systemctl is-enabled --quiet "${service_name}.service" ;;
        systemd-user) systemctl --user is-enabled --quiet "${service_name}.service" ;;
        openrc) rc-update show default | grep -q "^[[:space:]]*${service_name}[[:space:]]" ;;
        procd) [ -f "$service_file" ] && "$service_file" enabled ;;
        upstart) [ -f "$service_file" ] ;;
        *) return 1 ;;
    esac
}

wait_service_active() {
    attempt=0
    while [ "$attempt" -lt 5 ]; do
        if new_service_is_active; then
            sleep 1
            new_service_is_active && return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    return 1
}

rollback_installation() {
    log_warning "Installation failed; restoring the previous binary and service"
    rollback_ok=true
    stop_managed_service || true
    # Undo newly-enabled autostart before restoring a missing procd/OpenRC file.
    if [ "$previous_enabled" != true ]; then set_service_enabled disable || rollback_ok=false; fi
    for slot in binary credential runner service; do
        case "$slot" in
            binary) dest=$komari_agent_path ;;
            credential) dest=$credential_file ;;
            runner) dest=$runner_file ;;
            service) dest=$service_file ;;
        esac
        [ -n "$dest" ] || continue
        if [ -e "$transaction_dir/$slot" ] || [ -L "$transaction_dir/$slot" ]; then
            # Copy beside the destination then rename, never truncate a live binary.
            cp -a "$transaction_dir/$slot" "${dest}.rollback.$$" &&
                mv -f "${dest}.rollback.$$" "$dest" || rollback_ok=false
        else
            rm -f "$dest" || rollback_ok=false
        fi
    done
    reload_manager || rollback_ok=false
    if [ "$previous_enabled" = true ]; then set_service_enabled enable || rollback_ok=false; fi
    if [ "$previous_running" = true ]; then
        start_managed_service && wait_service_active || rollback_ok=false
    fi
    if [ "$rollback_ok" != true ]; then
        log_error "Rollback incomplete; protected recovery files retained at $transaction_dir"
        return 1
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
    snapshot_api_url="https://api.github.com/repos/lyhbdw/komari-lite/releases?per_page=100"
    if ! releases_json=$(curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 \
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
    version_to_install="latest"
    log_info "No version specified, installing the latest version."
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
    cache_qs="?t=$(date +%s)"
    download_url="${download_base%/}/${file_name}.bin${cache_qs}"
    download_url_fallback="${download_base%/}/${file_name}${cache_qs}"
    checksum_url="${download_base%/}/${file_name}.bin.sha256${cache_qs}"
    checksum_url_fallback="${download_base%/}/${file_name}.sha256${cache_qs}"
else
    if [ "$version_to_install" = "latest" ]; then
        download_path="latest/download"
    else
        download_path="download/${version_to_install}"
    fi
    download_url="https://github.com/lyhbdw/komari-lite/releases/${download_path}/${file_name}"
    download_url_fallback=""
    checksum_url="${download_url}.sha256"
    checksum_url_fallback=""
fi

log_step "Creating installation directory: ${GREEN}$target_dir${NC}"
mkdir -p "$target_dir"
if [ "$effective_uid" -eq 0 ] && [ "$service_user" != "root" ]; then
    chown "$service_user" "$target_dir"
fi

log_step "Preparing $file_name ..."
download_tmp=$(mktemp "${target_dir}/.agent-download.XXXXXX")
checksum_tmp=""
transaction_dir=""
transaction_active=false
cleanup_download() {
    exit_status=$?
    trap - EXIT INT TERM
    set +e
    keep_recovery=false
    if [ "$transaction_active" = true ]; then
        rollback_installation || keep_recovery=true
        [ "$exit_status" -ne 0 ] || exit_status=1
    fi
    rm -f "$download_tmp"
    [ -z "$checksum_tmp" ] || rm -f "$checksum_tmp"
    if [ -n "$transaction_dir" ] && [ "$keep_recovery" = false ]; then rm -rf "$transaction_dir"; fi
    exit "$exit_status"
}
trap cleanup_download EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
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
    download_ok=false
    if curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 --speed-limit 51200 --speed-time 8 \
        -o "$download_tmp" "$download_url" && [ -s "$download_tmp" ]; then
        download_ok=true
    elif [ -n "${download_url_fallback:-}" ]; then
        log_info "Retrying with fallback URL: ${CYAN}$download_url_fallback${NC}"
        if curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 --speed-limit 51200 --speed-time 8 \
            -o "$download_tmp" "$download_url_fallback" && [ -s "$download_tmp" ]; then
            download_ok=true
            download_url="$download_url_fallback"
        fi
    fi

    if [ "$download_ok" = false ]; then
        log_warning "Direct download failed, trying accelerated GitHub mirror fallback..."
        for mirror in "https://ghfast.top" "https://ghproxy.net"; do
            mirror_path="download/${version_to_install}"
            [ "$version_to_install" = "latest" ] && mirror_path="latest/download"
            mirror_url="${mirror}/https://github.com/lyhbdw/komari-lite/releases/${mirror_path}/${file_name}"
            log_info "Trying mirror: ${CYAN}$mirror_url${NC}"
            if curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 --speed-limit 51200 --speed-time 8 \
                -o "$download_tmp" "$mirror_url" && [ -s "$download_tmp" ]; then
                download_ok=true
                download_url="$mirror_url"
                break
            fi
        done
    fi

    if [ "$download_ok" = false ]; then
        log_error "Download failed from the configured release source"
        exit 1
    fi
fi

if [ -z "$sha256_expected" ]; then
    checksum_tmp=$(mktemp "${target_dir}/.agent-checksum.XXXXXX")
    if ! curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 \
        -o "$checksum_tmp" "$checksum_url" || [ ! -s "$checksum_tmp" ]; then
        if [ -n "${checksum_url_fallback:-}" ] && curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 \
            -o "$checksum_tmp" "$checksum_url_fallback" && [ -s "$checksum_tmp" ]; then
            :
        else
            rm -f "$checksum_tmp"
            log_error "No usable SHA256 checksum was provided; refusing to install an unverified binary"
            exit 1
        fi
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
    log_warning "Primary download SHA256 mismatch ($sha256_actual != $sha256_expected), attempting verified GitHub mirror fallback..."
    mirror_verified=false
    for mirror in "https://ghfast.top" "https://ghproxy.net" ""; do
        mirror_path="download/${version_to_install}"
        [ "$version_to_install" = "latest" ] && mirror_path="latest/download"
        if [ -n "$mirror" ]; then
            m_bin="${mirror}/https://github.com/lyhbdw/komari-lite/releases/${mirror_path}/${file_name}"
        else
            m_bin="https://github.com/lyhbdw/komari-lite/releases/${mirror_path}/${file_name}"
        fi
        m_chk="${m_bin}.sha256"
        m_chk_tmp=$(mktemp "${target_dir}/.agent-checksum.XXXXXX")
        if curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 \
            -o "$m_chk_tmp" "$m_chk" && [ -s "$m_chk_tmp" ]; then
            m_exp=$(awk 'NF {print $1; exit}' "$m_chk_tmp")
            rm -f "$m_chk_tmp"
            if curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 30 \
                -o "$download_tmp" "$m_bin" && [ -s "$download_tmp" ]; then
                m_act=$(sha256sum "$download_tmp" | awk '{print $1}')
                if [ "$(printf '%s' "$m_act" | tr '[:upper:]' '[:lower:]')" = "$(printf '%s' "$m_exp" | tr '[:upper:]' '[:lower:]')" ]; then
                    sha256_expected="$m_exp"
                    sha256_actual="$m_act"
                    mirror_verified=true
                    log_success "Verified mirror download successful"
                    break
                fi
            fi
        else
            rm -f "$m_chk_tmp"
        fi
    done
    if [ "$mirror_verified" != true ]; then
        log_error "SHA256 verification failed"
        exit 1
    fi
fi

# Execute the verified, staged file before touching the old binary or service.
# chmod must precede this bounded preflight (architecture/loader/noexec errors).
if ! command -v timeout >/dev/null 2>&1; then
    log_error "timeout is required for the binary preflight"
    exit 1
fi
if ! chmod 0755 "$download_tmp" || ! timeout -k 2 10 "$download_tmp" --help >/dev/null 2>&1; then
    log_error "Downloaded binary failed the bounded --help preflight; existing installation was not changed"
    exit 1
fi

# Function to detect actual init system
detect_init_system() {
    # An explicit supervisor is useful for alternate roots and isolated tests.
    if [ -n "${KOMARI_INIT_SYSTEM:-}" ]; then
        case "$KOMARI_INIT_SYSTEM" in
            systemd|openrc|procd|upstart|nixos) printf '%s\n' "$KOMARI_INIT_SYSTEM"; return ;;
            *) printf 'unknown\n'; return ;;
        esac
    fi
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
    if command -v uci >/dev/null 2>&1 && [ -f "$rc_common" ]; then
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
    if command -v rc-service >/dev/null 2>&1 && [ -d "$init_dir" ]; then
        echo "openrc"
        return
    fi

    # check for Upstart (CentOS 6)
    if command -v initctl >/dev/null 2>&1 && [ -d "$upstart_dir" ]; then
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
case "$init_system" in
    systemd) service_file="${systemd_dir}/${service_name}.service" ;;
    systemd-user) service_file="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/${service_name}.service" ;;
    openrc|procd) service_file="${init_dir}/${service_name}" ;;
    upstart) service_file="${upstart_dir}/${service_name}.conf" ;;
    nixos) service_file="" ;;
    *) log_error "Unsupported init system: $init_system"; exit 1 ;;
esac
[ -z "$service_file" ] || mkdir -p "$(dirname "$service_file")"

# Snapshot every file that will be replaced without exposing its contents.
transaction_dir=$(mktemp -d "${target_dir}/.agent-rollback.XXXXXX")
previous_running=false
previous_enabled=false
new_service_is_active && previous_running=true
service_is_enabled && previous_enabled=true
for slot in binary credential runner service; do
    case "$slot" in
        binary) source=$komari_agent_path ;;
        credential) source=$credential_file ;;
        runner) source=$runner_file ;;
        service) source=$service_file ;;
    esac
    if [ -n "$source" ] && { [ -e "$source" ] || [ -L "$source" ]; }; then
        cp -a "$source" "$transaction_dir/$slot"
    fi
 done
# Rollback becomes armed only after all original files have been saved.
transaction_active=true
if [ "$previous_running" = true ]; then stop_managed_service; fi
# Only now may the existing binary be replaced.
if ! mv -f "$download_tmp" "$komari_agent_path"; then
    log_error "Could not install verified binary"
    exit 1
fi
# Keep the old service definition until it has been safely snapshotted.

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
    service_file="${init_dir}/${service_name}"
    cat > "$service_file" << EOF
#!/sbin/openrc-run

name="Komari Agent Service"
description="Komari monitoring agent"
command="${runner_file}"
command_user="${service_user}"
directory="${target_dir}"
pidfile="${run_dir}/${service_name}.pid"
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
    start_managed_service
    wait_service_active
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
    systemctl --user enable "${service_name}.service"
    start_managed_service
    wait_service_active
    log_success "Systemd user service configured and started"
elif [ "$init_system" = "systemd" ]; then
    # Systemd service configuration
    log_info "Using systemd for service management"
    service_file="${systemd_dir}/${service_name}.service"
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
    start_managed_service
    wait_service_active
    log_success "Systemd service configured and started"
elif [ "$init_system" = "procd" ]; then
    # procd service configuration (OpenWrt)
    log_info "Using procd for service management"
    service_file="${init_dir}/${service_name}"
    cat > "$service_file" << EOF
#!/bin/sh ${rc_common}

START=99
STOP=10

USE_PROCD=1

PROG="${runner_file}"

start_service() {
    procd_open_instance
    procd_set_param command "\$PROG"
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
    "$service_file" enable
    start_managed_service
    wait_service_active
    log_success "procd service configured and started"
elif [ "$init_system" = "upstart" ]; then
    # Upstart service configuration
    log_info "Using upstart for service management"
    service_file="${upstart_dir}/${service_name}.conf"
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
    start_managed_service
    wait_service_active
    log_success "Upstart service configured and started"
else
    log_error "Unsupported or unknown init system detected: $init_system"
    log_error "Supported init systems: systemd, openrc, procd, upstart"
    exit 1
fi

transaction_active=false

echo ""
if [ "$init_system" = nixos ]; then
    log_success "Komari-agent binary installed!"
    log_warning "NixOS requires declarative service configuration."
    log_info "Please add the service configuration to your NixOS config and rebuild."
else
    log_success "Komari-agent installation completed!"
fi
log_config "Service: ${GREEN}$service_name${NC}"
log_config "Arguments: ${GREEN}configured${NC}"
echo -e "${WHITE}===========================================${NC}"
