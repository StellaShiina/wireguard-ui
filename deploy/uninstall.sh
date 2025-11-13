#!/usr/bin/env bash

set -euo pipefail

# Uninstall script for WireGuard UI
# Reverses actions done by deploy/install.sh:
# - Stops and disables systemd service
# - Optionally stops/removes Docker Compose resources
# - Optionally reverts sysctl IPv4/IPv6 forwarding changes
# - Cleans up state files

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

SUDO=""
if [ "$(id -u)" -ne 0 ]; then SUDO="sudo"; fi

YES=0
PURGE_DOCKER=0
REVERT_SYSCTL=0

usage() {
  cat <<'USAGE'
WireGuard UI Uninstaller

Usage:
  bash deploy/uninstall.sh [options]

Options:
  --yes                Non-interactive mode, assume "yes" for prompts
  --purge-docker       Stop and remove Docker Compose resources (postgres, volumes)
  --revert-sysctl      Revert IPv4/IPv6 forwarding in /etc/sysctl.conf (comment lines or restore backup)
  -h, --help           Show this help

Notes:
  - This script stops and disables the systemd service, removes installed files,
    and can optionally tear down Docker resources and revert sysctl changes.
USAGE
}

while [ "${1:-}" != "" ]; do
  case "$1" in
    --yes) YES=1 ; shift ;;
    --purge-docker) PURGE_DOCKER=1 ; shift ;;
    --revert-sysctl) REVERT_SYSCTL=1 ; shift ;;
    -h|--help) usage ; exit 0 ;;
    *) ERR "Unknown option: $1" ; usage ; exit 1 ;;
  esac
done

INFO "Starting uninstallation for WireGuard UI"

########################################
# 1) Stop and disable systemd service  #
########################################
SERVICE_NAME="wireguard-ui"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

if systemctl list-unit-files | grep -q "^${SERVICE_NAME}\.service"; then
  if systemctl is-active ${SERVICE_NAME} >/dev/null 2>&1; then
    INFO "Stopping service ${SERVICE_NAME}"
    $SUDO systemctl stop ${SERVICE_NAME} || true
  else
    INFO "Service ${SERVICE_NAME} is not running"
  fi
  if systemctl is-enabled ${SERVICE_NAME} >/dev/null 2>&1; then
    INFO "Disabling service ${SERVICE_NAME}"
    $SUDO systemctl disable ${SERVICE_NAME} || true
  else
    INFO "Service ${SERVICE_NAME} is not enabled"
  fi
else
  INFO "Service ${SERVICE_NAME} not registered"
fi

if [ -f "$SERVICE_PATH" ]; then
  INFO "Removing unit file $SERVICE_PATH"
  $SUDO rm -f "$SERVICE_PATH"
  $SUDO systemctl daemon-reload || true
else
  INFO "Unit file not found at $SERVICE_PATH"
fi

########################################
# 2) Remove installed files            #
########################################
INSTALL_DIR="/opt/wireguard-ui"
ENV_DIR="/etc/wireguard-ui"
STATE_DIR="/var/lib/wireguard-ui"

if [ -d "$INSTALL_DIR" ]; then
  INFO "Removing $INSTALL_DIR"
  $SUDO rm -rf "$INSTALL_DIR"
else
  INFO "$INSTALL_DIR not present"
fi

if [ -d "$ENV_DIR" ]; then
  INFO "Removing $ENV_DIR"
  $SUDO rm -rf "$ENV_DIR"
else
  INFO "$ENV_DIR not present"
fi

if [ -d "$STATE_DIR" ]; then
  INFO "Removing $STATE_DIR"
  $SUDO rm -rf "$STATE_DIR"
else
  INFO "$STATE_DIR not present"
fi

########################################
# 3) Docker Compose teardown (optional)#
########################################
CONTAINER_NAME="wireguard-postgres"

if [ "$PURGE_DOCKER" -eq 1 ]; then
  INFO "Purging Docker resources"
  DOCKER_COMPOSE_CMD=""
  if docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE_CMD="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    DOCKER_COMPOSE_CMD="docker-compose"
  else
    WARN "Docker Compose not available; will attempt to stop/remove container directly"
  fi

  if [ -n "$DOCKER_COMPOSE_CMD" ] && [ -f "$REPO_ROOT/docker-compose.yml" ]; then
    INFO "Running compose down -v"
    $DOCKER_COMPOSE_CMD -f "$REPO_ROOT/docker-compose.yml" down -v || WARN "Compose down returned errors"
  else
    if docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
      INFO "Stopping and removing container $CONTAINER_NAME"
      docker rm -f "$CONTAINER_NAME" || true
    else
      INFO "Container $CONTAINER_NAME not found"
    fi
  fi
else
  if docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    if [ "$YES" -eq 1 ] || ask_yes_no "Stop container $CONTAINER_NAME?"; then
      INFO "Stopping container $CONTAINER_NAME"
      docker stop "$CONTAINER_NAME" || true
    else
      INFO "Leaving container $CONTAINER_NAME running"
    fi
  fi
fi

########################################
# 4) Revert sysctl changes (optional) #
########################################
SYSCTL_FILE="/etc/sysctl.conf"
BACKUP_CANDIDATE=""
if [ -f "$SYSCTL_FILE" ]; then
  BACKUP_CANDIDATE="$($SUDO ls -1t /etc/sysctl.conf.bak.* 2>/dev/null | head -n1 || true)"
fi

revert_sysctl() {
  if [ -n "$BACKUP_CANDIDATE" ] && [ -f "$BACKUP_CANDIDATE" ]; then
    INFO "Restoring backup $BACKUP_CANDIDATE to $SYSCTL_FILE"
    $SUDO cp "$BACKUP_CANDIDATE" "$SYSCTL_FILE" || WARN "Failed to restore backup; proceeding to comment lines"
  fi
  if [ -f "$SYSCTL_FILE" ]; then
    INFO "Commenting forwarding lines in $SYSCTL_FILE"
    $SUDO sed -i 's/^\s*net\.ipv4\.ip_forward\s*=\s*1\s*$/# net.ipv4.ip_forward=1/' "$SYSCTL_FILE" || true
    $SUDO sed -i 's/^\s*net\.ipv6\.conf\.all\.forwarding\s*=\s*1\s*$/# net.ipv6.conf.all.forwarding=1/' "$SYSCTL_FILE" || true
    INFO "Applying sysctl -p"
    $SUDO sysctl -p || WARN "sysctl -p reported warnings; verify $SYSCTL_FILE manually if needed"
  else
    WARN "$SYSCTL_FILE not found; skipping sysctl revert"
  fi
}

if [ "$REVERT_SYSCTL" -eq 1 ]; then
  revert_sysctl
else
  if [ "$YES" -eq 1 ]; then
    INFO "Skipping sysctl revert (non-interactive)"
  else
    if ask_yes_no "Revert IPv4/IPv6 forwarding changes in $SYSCTL_FILE?"; then
      revert_sysctl
    else
      INFO "Leaving sysctl settings as is"
    fi
  fi
fi

INFO "Uninstallation completed."