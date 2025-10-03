WireGuard UI
================

Overview
--------
- Lightweight web UI for managing WireGuard peers and server settings.
- Backend in Go, HTML templates for the frontend, PostgreSQL for state.
- Ships with a one-click installer that sets up Docker, PostgreSQL, service, and IP forwarding.

Quick Start (One‑Command Install)
---------------------------------
- Requirements: `git`, `curl`, `bash`, and either `root` or `sudo` privileges.
- One-command install:

```bash
git clone https://github.com/StellaShiina/wireguard-ui.git && cd wireguard-ui && bash deploy/install.sh
```


What the Installer Does
-----------------------
- Optional step 0: Offers to install WireGuard (`wg-quick`) on Ubuntu/Debian (skipped if already installed).
- Checks for Docker; offers to install using the official convenience script if missing.
- Starts PostgreSQL via `docker-compose.yml` and initializes schema from `init-scripts/init.sql`.
- Installs the backend binary to `/opt/wireguard-ui/wireguard-ui`.
  - Provide your binary via `WIREGUARD_UI_BIN` env var or place it at `./wireguard-ui` in the repo root.
- Copies frontend templates to `/opt/wireguard-ui/templates/`.
- Installs environment file to `/etc/wireguard-ui/.env` (created if absent).
- Registers and starts the systemd unit `wireguard-ui.service`.
- Enables IPv4 and IPv6 packet forwarding by updating `/etc/sysctl.conf` and running `sysctl -p`.
- Idempotent: skips steps if they’re already done with the same configuration.

Default Access & Credentials
----------------------------
- UI address: `http://localhost:60000/` (configurable via `UI_ADDR` and `UI_PORT`).
- Default credentials (override in `/etc/wireguard-ui/.env`):
  - `AUTH_USERNAME=admin`
  - `AUTH_PASSWORD=admin123`
- JWT secret: `JWT_SECRET=wireguard-ui-secrect` (change this for production).

Configuration
-------------
- Primary config file: `/etc/wireguard-ui/.env`.
- Key variables:
  - `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`
  - `WG_CONF_DIR`, `WG_CLIENTS_DIR`, `WG_EXTERNAL_IF`, `WG_INTERFACE`, `WG_MODE`
  - `UI_ADDR`, `UI_PORT`
- The app reads `/etc/wireguard-ui/.env` with highest priority.

How to Use (For Users)
----------------------
- Open the UI (`http://<server>:<port>/`) and log in.
- Configure server settings and generate peer configs.
- Download or copy peer configs for client devices.
- Note: Clients must install WireGuard themselves.

Getting the Binary
------------------
- From GitHub Releases: publish or download a pre-built `wireguard-ui` binary and pass its path via `WIREGUARD_UI_BIN` to the installer.
- Build from source:

```
go build -o wireguard-ui ./
```

Development (For Developers)
----------------------------
- Dependencies: Go 1.21+, Docker.
- Start PostgreSQL locally:

```
docker compose up -d postgres
```

- Set a dev `.env` in repo root or `/etc/wireguard-ui/.env`:

```
AUTH_USERNAME=dev
AUTH_PASSWORD=devpass
JWT_SECRET=dev-secret
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=wireguard
DB_SSL_MODE=disable
UI_ADDR=localhost
UI_PORT=60000
```

- Run the app:

```
go run ./
```

- Frontend templates live in `templates/`.
- Systemd unit lives in `deploy/wireguard-ui.service` (installed by the installer).
- Installer script is `deploy/install.sh`.

Uninstall
---------
- Run the provided uninstaller:

```
bash deploy/uninstall.sh
```

- Options:
  - `--yes` non-interactive mode
  - `--purge-docker` stop and remove Compose resources and volumes
  - `--revert-sysctl` revert IPv4/IPv6 forwarding in `/etc/sysctl.conf`

- Manual steps (if preferred):
- Stop and disable the service: `sudo systemctl disable --now wireguard-ui`.
- Remove unit file: `sudo rm -f /etc/systemd/system/wireguard-ui.service && sudo systemctl daemon-reload`.
- Remove installed files: `sudo rm -rf /opt/wireguard-ui /etc/wireguard-ui /var/lib/wireguard-ui`.
  - Remove Docker resources as needed: `docker compose -f docker-compose.yml down -v`.
  - Optionally revert sysctl forwarding in `/etc/sysctl.conf` and run `sudo sysctl -p`.

Security Notes
--------------
- Change default credentials and `JWT_SECRET` immediately in production.
- Restrict access to the UI with firewall rules or reverse proxy if exposed publicly.

License
-------
- Licensed under the MIT License. See `LICENSE` for details.