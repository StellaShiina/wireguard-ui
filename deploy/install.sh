#!/usr/bin/env bash

set -euo pipefail

# One-click install and deployment script for WireGuard UI
# This script will:
# 1) Ensure Docker (and Compose) are installed; prompt to install Docker if missing.
# 2) Launch PostgreSQL using the repository's docker-compose and init.sql.
# 3) Install the pre-built backend binary, templates, and .env to proper locations.
# 4) Register and start the systemd service.
#
# Notes:
# - This script DOES NOT install WireGuard on client devices.
#   Clients must install WireGuard themselves to use generated configs.
# - Provide the path to your pre-built binary via the WIREGUARD_UI_BIN env var
#   or place the binary at "wireguard-ui" in the repository root.

INFO() { echo -e "\033[1;34m[INFO]\033[0m $*"; }
WARN() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
ERR()  { echo -e "\033[1;31m[ERROR]\033[0m $*"; }

ask_yes_no() {
  local prompt=${1:-"Proceed?"}
  read -r -p "$prompt [y/N]: " ans || true
  case "$ans" in
    y|Y|yes|YES|Yes) return 0 ;;
    *) return 1 ;;
  esac
}

# Resolve repo root (this script lives in deploy/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Require sudo for system-level install if not root
SUDO=""
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo"; fi

INFO "Starting one-click installation for WireGuard UI"
INFO "Clients must install WireGuard themselves to use peer configs."

########################################
# 0) Optional WireGuard install        #
########################################
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    echo "${ID:-}"
  else
    echo "unknown"
  fi
}

install_wireguard_if_needed() {
  if command -v wg-quick >/dev/null 2>&1; then
    INFO "WireGuard 'wg-quick' is already installed; skipping step 0"
    return 0
  fi
  WARN "WireGuard ('wg-quick') not found."
  if ! ask_yes_no "Install WireGuard now (Ubuntu/Debian supported)?"; then
    INFO "Skipping WireGuard installation per user choice."
    return 0
  fi
  local os_id
  os_id="$(detect_os)"
  INFO "Detected OS: ${os_id}"
  case "$os_id" in
    ubuntu|debian)
      $SUDO apt-get update -y || true
      # Try installing the meta package; fall back to wireguard-tools if needed
      if ! $SUDO apt-get install -y wireguard >/dev/null 2>&1; then
        INFO "Installing wireguard-tools as fallback"
        $SUDO apt-get install -y wireguard-tools || { ERR "Failed to install WireGuard tools"; return 1; }
      fi
      ;;
    *)
      WARN "Unsupported or unknown OS for automated WireGuard install. Please install WireGuard manually."
      return 0
      ;;
  esac
  if command -v wg-quick >/dev/null 2>&1; then
    INFO "WireGuard installed successfully (wg-quick detected)."
  else
    WARN "Installation completed but 'wg-quick' still not detected. Please verify manually."
  fi
}

install_wireguard_if_needed

########################################
# 1) Docker check and optional install #
########################################
if ! command -v docker >/dev/null 2>&1; then
  WARN "Docker is not installed."
  if ask_yes_no "Install Docker using the official convenience script?"; then
    INFO "Installing Docker..."
    curl -fsSL https://get.docker.com | $SUDO sh || { ERR "Docker installation failed"; exit 1; }
    INFO "Docker installation completed."
  else
    ERR "Docker is required. Please install Docker and re-run this script."
    exit 1
  fi
else
  INFO "Docker found: $(docker --version)"
fi

# Determine Docker Compose command
DOCKER_COMPOSE_CMD=""
if docker compose version >/dev/null 2>&1; then
  DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DOCKER_COMPOSE_CMD="docker-compose"
else
  ERR "Docker Compose is not available. Please install Docker Compose plugin or docker-compose."
  exit 1
fi

########################################
# 2) Start PostgreSQL via Compose      #
########################################
STATE_DIR="/var/lib/wireguard-ui"
$SUDO mkdir -p "$STATE_DIR"
COMPOSE_HASH="$(sha256sum "$REPO_ROOT/docker-compose.yml" | awk '{print $1}')"
STATE_FILE="$STATE_DIR/compose.sha256"
CONTAINER_NAME="wireguard-postgres"

ALREADY_RUNNING=false
if docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
  if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null)" = "true" ]; then
    ALREADY_RUNNING=true
  fi
fi

if $ALREADY_RUNNING && [ -f "$STATE_FILE" ] && grep -q "$COMPOSE_HASH" "$STATE_FILE"; then
  INFO "PostgreSQL container '$CONTAINER_NAME' is running with the same docker-compose; skipping step 2"
else
  INFO "Starting PostgreSQL using docker-compose..."
  $DOCKER_COMPOSE_CMD -f "$REPO_ROOT/docker-compose.yml" up -d postgres || { ERR "Failed to start postgres via compose"; exit 1; }
  INFO "PostgreSQL container started. Waiting briefly for initialization..."
  sleep 5
  echo "$COMPOSE_HASH" | $SUDO tee "$STATE_FILE" >/dev/null || true
fi

########################################
# 3) Install binary, templates, .env   #
########################################
INSTALL_DIR="/opt/wireguard-ui"
ENV_DIR="/etc/wireguard-ui"
SERVICE_SRC="$REPO_ROOT/deploy/wireguard-ui.service"
SERVICE_DST="/etc/systemd/system/wireguard-ui.service"

$SUDO mkdir -p "$INSTALL_DIR" "$INSTALL_DIR/templates" "$ENV_DIR"

INFO "Copying frontend templates to $INSTALL_DIR/templates"
$SUDO cp -r "$REPO_ROOT/templates/"* "$INSTALL_DIR/templates/"

# Handle pre-built binary
BIN_SRC="${WIREGUARD_UI_BIN:-$REPO_ROOT/wireguard-ui}"
if [ ! -f "$BIN_SRC" ]; then
  WARN "Pre-built binary not found at $BIN_SRC"
  read -r -p "Enter path to pre-built wireguard-ui binary (or leave blank to abort): " BIN_SRC
  if [ -z "$BIN_SRC" ] || [ ! -f "$BIN_SRC" ]; then
    ERR "Binary not provided. Place the compiled binary and re-run."
    echo "Hint: Publish your binary to GitHub Releases and download it before running this script."
    exit 1
  fi
fi
BIN_DST="$INSTALL_DIR/wireguard-ui"
SRC_HASH="$(sha256sum "$BIN_SRC" | awk '{print $1}')"
DST_HASH=""
if [ -f "$BIN_DST" ]; then
  DST_HASH="$(sha256sum "$BIN_DST" | awk '{print $1}')"
fi
if [ "$SRC_HASH" = "$DST_HASH" ]; then
  INFO "Binary already installed with the same hash; skipping copy"
else
  INFO "Installing binary to $BIN_DST"
  $SUDO install -m 0755 "$BIN_SRC" "$BIN_DST"
fi

# Install and optionally configure environment variables (first-run and re-run)
ENV_FILE_REPO="$REPO_ROOT/.env"
ENV_FILE_ETC="$ENV_DIR/.env"

# Defaults matching application config
DEF_AUTH_USERNAME="ka9"
DEF_AUTH_PASSWORD="333kaa9"
DEF_JWT_SECRET="fly-me-to-the-moon"
DEF_DB_HOST="127.0.0.1"
DEF_DB_PORT="5432"
DEF_UI_ADDR="localhost"
DEF_UI_PORT="60000"

get_env() {
  local file=$1 var=$2
  [ -f "$file" ] || return 1
  local line
  line=$(grep -E "^${var}=" "$file" | tail -n1 || true)
  [ -n "$line" ] || return 1
  echo "${line#*=}"
}

set_env_repo() {
  local var=$1 val=$2
  if grep -qE "^${var}=" "$ENV_FILE_REPO" 2>/dev/null; then
    sed -i "s|^${var}=.*|${var}=${val}|" "$ENV_FILE_REPO"
  else
    echo "${var}=${val}" >> "$ENV_FILE_REPO"
  fi
}

prompt_var() {
  local label=$1 default=$2 out
  read -r -p "Enter ${label} [${default}]: " out || true
  if [ -z "$out" ]; then echo "$default"; else echo "$out"; fi
}

# Ensure repo .env exists (create baseline if missing)
if [ ! -f "$ENV_FILE_REPO" ]; then
  INFO "Creating baseline .env in repo root"
  cat > "$ENV_FILE_REPO" <<'EOF'
# WireGuard UI environment
# Adjust values as needed. The service prioritizes /etc/wireguard-ui/.env

AUTH_USERNAME=admin
AUTH_PASSWORD=admin123
JWT_SECRET=wireguard-ui-secrect

DB_HOST=127.0.0.1
DB_PORT=6543
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=wireguard
DB_SSL_MODE=disable

WG_CONF_DIR=/etc/wireguard
WG_CLIENTS_DIR=/etc/wireguard/clients
WG_EXTERNAL_IF=
WG_INTERFACE=wg0
WG_MODE=wg

UI_ADDR=localhost
UI_PORT=60000
EOF
fi

FIRST_RUN=true
if [ -f "$ENV_FILE_ETC" ]; then FIRST_RUN=false; fi

# Determine current values from ETC (preferred) or repo (fallback) or defaults
CUR_AUTH_USERNAME=$(get_env "$ENV_FILE_ETC" AUTH_USERNAME || get_env "$ENV_FILE_REPO" AUTH_USERNAME || echo "$DEF_AUTH_USERNAME")
CUR_AUTH_PASSWORD=$(get_env "$ENV_FILE_ETC" AUTH_PASSWORD || get_env "$ENV_FILE_REPO" AUTH_PASSWORD || echo "$DEF_AUTH_PASSWORD")
CUR_JWT_SECRET=$(get_env "$ENV_FILE_ETC" JWT_SECRET || get_env "$ENV_FILE_REPO" JWT_SECRET || echo "$DEF_JWT_SECRET")
CUR_DB_HOST=$(get_env "$ENV_FILE_ETC" DB_HOST || get_env "$ENV_FILE_REPO" DB_HOST || echo "$DEF_DB_HOST")
CUR_DB_PORT=$(get_env "$ENV_FILE_ETC" DB_PORT || get_env "$ENV_FILE_REPO" DB_PORT || echo "$DEF_DB_PORT")
CUR_UI_ADDR=$(get_env "$ENV_FILE_ETC" UI_ADDR || get_env "$ENV_FILE_REPO" UI_ADDR || echo "$DEF_UI_ADDR")
CUR_UI_PORT=$(get_env "$ENV_FILE_ETC" UI_PORT || get_env "$ENV_FILE_REPO" UI_PORT || echo "$DEF_UI_PORT")

DO_RECONFIG=false
if $FIRST_RUN; then
  INFO "First run: configure core settings (auth, DB host/port, UI addr/port)"
  if ask_yes_no "Customize settings now?"; then DO_RECONFIG=true; fi
else
  INFO "Existing installation detected."
  if ask_yes_no "Reconfigure auth/UI addr:port or DB host:port now?"; then DO_RECONFIG=true; fi
fi

TS=$(date +%Y%m%d%H%M%S)
REPO_ENV_BAK="${ENV_FILE_REPO}.bak.${TS}"
ETC_ENV_BAK="${ENV_FILE_ETC}.bak.${TS}"

AUTH_CHANGED=false
UI_CHANGED=false
DB_CHANGED=false
RECONFIG_SERVICE_CHANGE=false

if $DO_RECONFIG; then
  # Back up current env files (if present)
  if [ -f "$ENV_FILE_REPO" ]; then cp "$ENV_FILE_REPO" "$REPO_ENV_BAK" || true; fi
  if [ -f "$ENV_FILE_ETC" ]; then $SUDO cp "$ENV_FILE_ETC" "$ETC_ENV_BAK" || true; fi

  NEW_AUTH_USERNAME=$(prompt_var "AUTH_USERNAME" "$CUR_AUTH_USERNAME")
  NEW_AUTH_PASSWORD=$(prompt_var "AUTH_PASSWORD" "$CUR_AUTH_PASSWORD")
  NEW_JWT_SECRET=$(prompt_var "JWT_SECRET" "$CUR_JWT_SECRET")
  NEW_DB_HOST=$(prompt_var "DB_HOST" "$CUR_DB_HOST")
  NEW_DB_PORT=$(prompt_var "DB_PORT" "$CUR_DB_PORT")
  NEW_UI_ADDR=$(prompt_var "UI_ADDR" "$CUR_UI_ADDR")
  NEW_UI_PORT=$(prompt_var "UI_PORT" "$CUR_UI_PORT")

  # Update repo .env
  set_env_repo AUTH_USERNAME "$NEW_AUTH_USERNAME"
  set_env_repo AUTH_PASSWORD "$NEW_AUTH_PASSWORD"
  set_env_repo JWT_SECRET "$NEW_JWT_SECRET"
  set_env_repo DB_HOST "$NEW_DB_HOST"
  set_env_repo DB_PORT "$NEW_DB_PORT"
  set_env_repo UI_ADDR "$NEW_UI_ADDR"
  set_env_repo UI_PORT "$NEW_UI_PORT"

  # Detect changes compared to ETC values
  OLD_AUTH_USERNAME="$CUR_AUTH_USERNAME"
  OLD_AUTH_PASSWORD="$CUR_AUTH_PASSWORD"
  OLD_JWT_SECRET="$CUR_JWT_SECRET"
  OLD_DB_HOST="$CUR_DB_HOST"
  OLD_DB_PORT="$CUR_DB_PORT"
  OLD_UI_ADDR="$CUR_UI_ADDR"
  OLD_UI_PORT="$CUR_UI_PORT"

  if [ "$NEW_AUTH_USERNAME" != "$OLD_AUTH_USERNAME" ] || [ "$NEW_AUTH_PASSWORD" != "$OLD_AUTH_PASSWORD" ] || [ "$NEW_JWT_SECRET" != "$OLD_JWT_SECRET" ]; then AUTH_CHANGED=true; fi
  if [ "$NEW_UI_ADDR" != "$OLD_UI_ADDR" ] || [ "$NEW_UI_PORT" != "$OLD_UI_PORT" ]; then UI_CHANGED=true; fi
  if [ "$NEW_DB_HOST" != "$OLD_DB_HOST" ] || [ "$NEW_DB_PORT" != "$OLD_DB_PORT" ]; then DB_CHANGED=true; fi
  if $AUTH_CHANGED || $UI_CHANGED; then RECONFIG_SERVICE_CHANGE=true; fi
fi

# Copy repo .env to /etc
INFO "Installing .env to $ENV_FILE_ETC"
$SUDO cp "$ENV_FILE_REPO" "$ENV_FILE_ETC"

# React based on changes
if $DO_RECONFIG; then
  # Restart service if auth or UI changed
  if $AUTH_CHANGED || $UI_CHANGED; then
INFO "Restarting wireguard-ui service due to auth/UI changes"
    if systemctl list-unit-files | grep -q '^wireguard-ui\.service'; then
      if ! $SUDO systemctl restart wireguard-ui; then
        WARN "Service restart failed; reverting to previous .env"
        if [ -f "$ETC_ENV_BAK" ]; then $SUDO cp "$ETC_ENV_BAK" "$ENV_FILE_ETC" || true; fi
        if [ -f "$REPO_ENV_BAK" ]; then cp "$REPO_ENV_BAK" "$ENV_FILE_REPO" || true; fi
        $SUDO systemctl restart wireguard-ui || WARN "Retry restart failed; please check service logs"
      fi
    else
      INFO "Service unit not registered; skipping service restart"
    fi
  fi

  # Restart compose postgres if DB changed (do not clear volumes)
  if $DB_CHANGED; then
    INFO "Updating docker-compose.yml postgres port mapping and restarting (volumes preserved)"
    COMPOSE_FILE="$REPO_ROOT/docker-compose.yml"
    COMPOSE_BAK="$COMPOSE_FILE.bak.$TS"
    if [ -f "$COMPOSE_FILE" ]; then
      cp "$COMPOSE_FILE" "$COMPOSE_BAK" || true
      INFO "Backup saved: $COMPOSE_BAK"
      # Update the ports mapping line to NEW_DB_HOST:NEW_DB_PORT:5432 while preserving indentation
      $SUDO sed -E -i "s|^([[:space:]]*)- \"[^\"]+:([0-9]+):5432\"|\\1- \"$NEW_DB_HOST:$NEW_DB_PORT:5432\"|" "$COMPOSE_FILE" || WARN "Failed to update postgres port mapping in compose file"
    else
      WARN "docker-compose.yml not found; skipping port update"
    fi

    if [ -n "$DOCKER_COMPOSE_CMD" ] && [ -f "$COMPOSE_FILE" ]; then
      if ! $DOCKER_COMPOSE_CMD -f "$COMPOSE_FILE" up -d postgres; then
        WARN "Compose up failed; restoring backup compose file and retrying"
        [ -f "$COMPOSE_BAK" ] && cp "$COMPOSE_BAK" "$COMPOSE_FILE" || true
        $DOCKER_COMPOSE_CMD -f "$COMPOSE_FILE" up -d postgres || WARN "Compose up retry failed; check Docker setup"
      fi
      # Update stored compose hash state
      NEW_COMPOSE_HASH=$(sha256sum "$COMPOSE_FILE" | awk '{print $1}')
      echo "$NEW_COMPOSE_HASH" | $SUDO tee "$STATE_FILE" >/dev/null || true
    else
      WARN "Compose not available or docker-compose.yml missing; skipping restart"
    fi

    # Always restart service after DB change
    if systemctl list-unit-files | grep -q '^wireguard-ui\.service'; then
      if ! $SUDO systemctl restart wireguard-ui; then
        WARN "Service restart after DB change failed; reverting to previous configuration"
        # Restore env and compose backups if present
        if [ -f "$ETC_ENV_BAK" ]; then $SUDO cp "$ETC_ENV_BAK" "$ENV_FILE_ETC" || true; fi
        if [ -f "$REPO_ENV_BAK" ]; then cp "$REPO_ENV_BAK" "$ENV_FILE_REPO" || true; fi
        if [ -f "$COMPOSE_BAK" ]; then cp "$COMPOSE_BAK" "$COMPOSE_FILE" || true; fi
        # Try to bring postgres back up with restored compose
        if [ -n "$DOCKER_COMPOSE_CMD" ] && [ -f "$COMPOSE_FILE" ]; then
          $DOCKER_COMPOSE_CMD -f "$COMPOSE_FILE" up -d postgres || true
        fi
        # Retry service start
        $SUDO systemctl restart wireguard-ui || WARN "Retry restart failed; please check service logs"
      fi
    fi
  fi
fi

########################################
# 4) Register and start systemd unit   #
########################################
if [ ! -f "$SERVICE_SRC" ]; then
  ERR "Service file not found at $SERVICE_SRC"
  exit 1
fi
INFO "Installing systemd unit to $SERVICE_DST (if changed)"
UNIT_HASH_SRC="$(sha256sum "$SERVICE_SRC" | awk '{print $1}')"
UNIT_HASH_DST=""
if [ -f "$SERVICE_DST" ]; then
  UNIT_HASH_DST="$(sha256sum "$SERVICE_DST" | awk '{print $1}')"
fi
if [ "$UNIT_HASH_SRC" != "$UNIT_HASH_DST" ]; then
  $SUDO cp "$SERVICE_SRC" "$SERVICE_DST"
  $SUDO systemctl daemon-reload
else
  INFO "Service unit unchanged."
fi

if systemctl is-enabled wireguard-ui >/dev/null 2>&1; then
  INFO "Service is enabled."
else
  $SUDO systemctl enable wireguard-ui || true
fi
if systemctl is-active wireguard-ui >/dev/null 2>&1; then
  INFO "Service is already running."
else
  if $SUDO systemctl start wireguard-ui; then
    INFO "Service wireguard-ui started."
  else
    WARN "Service start failed."
    if [ "$RECONFIG_SERVICE_CHANGE" = true ]; then
      WARN "Reverting to previous .env due to start failure"
      if [ -f "$ETC_ENV_BAK" ]; then $SUDO cp "$ETC_ENV_BAK" "$ENV_FILE_ETC" || true; fi
      if [ -f "$REPO_ENV_BAK" ]; then cp "$REPO_ENV_BAK" "$ENV_FILE_REPO" || true; fi
      $SUDO systemctl start wireguard-ui || WARN "Retry start failed; check logs and configuration"
    fi
  fi
fi

########################################
# Done                                 #
########################################
INFO "Installation complete."
INFO "Default UI address: http://localhost:60000/ (configurable via UI_ADDR/UI_PORT in /etc/wireguard-ui/.env)"
INFO "Reminder: Clients must install WireGuard to use generated peer configs."

########################################
# 5) Enable IP forwarding (IPv4/IPv6)  #
########################################
SYSCTL_FILE="/etc/sysctl.conf"
BACKUP_FILE="/etc/sysctl.conf.bak.$(date +%Y%m%d%H%M%S)"
if [ -f "$SYSCTL_FILE" ]; then
  INFO "Backing up $SYSCTL_FILE to $BACKUP_FILE"
  $SUDO cp "$SYSCTL_FILE" "$BACKUP_FILE" || true
fi

# Ensure IPv4 forwarding
if [ -f "$SYSCTL_FILE" ] && grep -Eq "^[[:space:]]*net\.ipv4\.ip_forward[[:space:]]*=[[:space:]]*1" "$SYSCTL_FILE"; then
  INFO "IPv4 forwarding already enabled"
else
  if [ -f "$SYSCTL_FILE" ] && grep -Eq "^[[:space:]]*#?[[:space:]]*net\.ipv4\.ip_forward[[:space:]]*=.*" "$SYSCTL_FILE"; then
    INFO "Uncommenting and setting net.ipv4.ip_forward=1"
    $SUDO sed -i 's/^[[:space:]]*#\?[[:space:]]*net\.ipv4\.ip_forward.*/net.ipv4.ip_forward=1/' "$SYSCTL_FILE" || true
  else
    INFO "Appending net.ipv4.ip_forward=1 to $SYSCTL_FILE"
    echo "net.ipv4.ip_forward=1" | $SUDO tee -a "$SYSCTL_FILE" >/dev/null || true
  fi
fi

# Ensure IPv6 forwarding
if [ -f "$SYSCTL_FILE" ] && grep -Eq "^[[:space:]]*net\.ipv6\.conf\.all\.forwarding[[:space:]]*=[[:space:]]*1" "$SYSCTL_FILE"; then
  INFO "IPv6 forwarding already enabled"
else
  if [ -f "$SYSCTL_FILE" ] && grep -Eq "^[[:space:]]*#?[[:space:]]*net\.ipv6\.conf\.all\.forwarding[[:space:]]*=.*" "$SYSCTL_FILE"; then
    INFO "Uncommenting and setting net.ipv6.conf.all.forwarding=1"
    $SUDO sed -i 's/^[[:space:]]*#\?[[:space:]]*net\.ipv6\.conf\.all\.forwarding.*/net.ipv6.conf.all.forwarding=1/' "$SYSCTL_FILE" || true
  else
    INFO "Appending net.ipv6.conf.all.forwarding=1 to $SYSCTL_FILE"
    echo "net.ipv6.conf.all.forwarding=1" | $SUDO tee -a "$SYSCTL_FILE" >/dev/null || true
  fi
fi

INFO "Applying sysctl settings from $SYSCTL_FILE"
$SUDO sysctl -p || WARN "sysctl -p reported warnings; verify $SYSCTL_FILE manually if needed"