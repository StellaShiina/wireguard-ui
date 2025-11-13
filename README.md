WireGuard UI
================

Overview
--------
- Lightweight web UI for managing WireGuard peers and server settings.
- Backend in Go, HTML templates for the frontend, PostgreSQL for state.
- Ships with a one-click installer that sets up Docker, PostgreSQL, service, and IP forwarding.

Scope & Disclaimer
------------------
- Intended for LAN access to services and devices behind your server.
- No guarantee is given for availability or results outside LAN scenarios (for example, full‑tunnel internet access or arbitrary routing).
- For broader use cases, simple secondary development is possible: adjust configuration generation in `wireguard/generator.go`, extend JSON handlers under `handlers/`, or register additional routes in `main.go`.

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

HTTP API
--------
- Authentication is cookie-based JWT. Obtain a token via `POST /auth/login`; all endpoints under `/api/v1/*` require authentication and will return `401` JSON if the cookie is missing or invalid.

Auth
----
- `POST /auth/login`
  - Request: `{"username":"...", "password":"..."}`
  - Success: `200 {"message":"Login successful","user":"..."}`; sets cookie for subsequent requests.
  - Errors: `400` invalid body, `401` invalid credentials, `500` token generation failure.
- `GET /auth/logout`
  - Success: `200 {"message":"Logout successful"}`
- `GET /auth/check`
  - Success: `200 {"authenticated":true,"username":"..."}`
  - Unauthenticated: `401 {"authenticated":false}`

Configs
-------
- `GET /api/v1/configs`
  - Success: `200 {"server": {...}, "peers": [...] }`
  - Errors: `404` server not initialized; `500` on database errors.
- `POST /api/v1/configs/server/:uuid`
  - Body: any subset of `public_ip`, `port`, `enable_ipv6`, `subnet_v4`, `subnet_v6`.
  - Success: `200 {"message":"server updated; regenerated keys and peer configs"}`
  - Side effects: peers cleared if subnet changes; server and peer configs regenerated.
  - Errors: `404` server not found; `400` invalid body or no fields; `500` DB or generation errors.
- `POST /api/v1/configs/peer`
  - Body: `{"name":"optional"}`
  - Success: `200 {"peer": {...}, "path": "/path/to/clients/<uuid>.conf"}`
- `PUT /api/v1/configs/peer/:uuid`
  - Body: `{"name":"..."}`
  - Success: `200 {"message":"peer updated"}`
- `DELETE /api/v1/configs/peer/:uuid`
  - Success: `200 {"message":"peer deleted"}`
- `GET /api/v1/configs/peer/:uuid`
  - Success: attachment download of the `.conf` file.
  - Errors: `500` server not initialized; `404` peer not found.

WireGuard Control
-----------------
- `POST /api/v1/wg/start`
  - Starts `<WG_MODE>-quick@<WG_INTERFACE>` when the config exists in `WG_CONF_DIR`.
  - Success: `200 {"message":"wireguard started","output":"...","service":"wg-quick@wg0"}`
  - Errors: `400` config missing; `500` enable failed.
- `POST /api/v1/wg/stop`
  - Success: `200 {"message":"wireguard stopped","output":"...","service":"wg-quick@wg0"}`
- `POST /api/v1/wg/restart`
  - Success: `200 {"message":"wireguard restarted","output":"...","service":"wg-quick@wg0"}`
- `GET /api/v1/wg/status`
  - Success: `200 {"status":"ok","output":"...","service":"wg-quick@wg0"}`
  - Inactive: `200 {"status":"error","output":"...","error":"...","service":"wg-quick@wg0"}`
- `GET /api/v1/wg/show`
  - Success: `200 {"output":"..."}`
  - Errors: `500` when `wg show` fails.

Notes
-----
- Client configs route only the server’s subnets (`AllowedIPs` set to `server.SubnetV4` and optionally `server.SubnetV6`). Internet traffic is not tunneled by default; use server-side NAT to reach LAN resources.
- Environment overrides: see Configuration section; common variables include `WG_CONF_DIR`, `WG_CLIENTS_DIR`, `WG_EXTERNAL_IF`, `WG_INTERFACE`, `WG_MODE`, `UI_ADDR`, `UI_PORT`.

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