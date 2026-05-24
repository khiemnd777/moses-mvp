# VPS Docker Compose Deploy Flow

This directory contains the production deployment flow for a Linux VPS.

## Production Runtime

Production runs through Docker Compose:

- `postgres`
- `qdrant`
- `api`
- `worker`
- `web` container running Nginx and serving the built frontend
- `certbot` container for certificate issuance

Host Nginx is not installed or reloaded by this flow.

## Local Docker Compose

Local development also runs through Docker Compose:

```bash
cd /Users/khiemnguyen/Works/project_legal_ai/legal_api
make up
```

The first run creates `install/config.local.sh` from `install/config.local.sh.sample`. The local app URL is:

```text
http://localhost:19080
```

Useful local commands:

```bash
make down
make stop
make restart
make log
./install/local-compose.sh ps
./install/local-compose.sh migrate
./install/local-compose.sh verify
```

Local published ports are web `19080`, API `19088`, Postgres `19433`, Qdrant `19334`, and HTTPS `19443`.

Local compose uses `.local/` for Postgres, Qdrant, uploads, Certbot webroot, and local certificate storage.

## One-Time VPS Bootstrap

Before the first GitHub Actions deploy, clone the repository to `APP_ROOT` on the VPS:

```bash
sudo mkdir -p /opt/legal_api
git clone git@github.com:your-org/legal_api.git /opt/legal_api/app
```

The VPS must be able to fetch the repository through `GIT_REPO_URL` from `install/config.sh`.

## One-Time Local Secret Sync

On the local machine:

```bash
cd install
cp secret.sh.sample secret.sh
cp config.sh.sample config.sh
# edit secret.sh for SSH access
# edit config.sh for production app secrets and paths
./sync-secrets.sh
```

`sync-secrets.sh` uploads only `install/config.sh` to `APP_ROOT/install/config.sh` on the VPS. The app secrets stay on the VPS and are not stored in GitHub Actions.

## GitHub Actions Deploy

Configure GitHub repository secrets:

- `VPS_HOST`
- `VPS_USER`
- `VPS_SSH_KEY`
- optional `VPS_PORT`
- optional `VPS_KNOWN_HOSTS`

Configure GitHub repository variable:

- optional `VPS_APP_ROOT`, default `/opt/legal_api/app`

CI runs from `.github/workflows/deploy.yml` on pull requests, pushes to `main`, and pushes to release tags that match `v*`.

Production deploy runs only on:

- pushing a tag that matches `v*`, for example `v2026.05.24` or `v2026.05.24-1`
- `workflow_dispatch`

The workflow SSHes into the VPS, checks out the exact GitHub SHA, and runs:

```bash
INSTALL_CONFIG_FILE="$APP_ROOT/install/config.sh" GIT_COMMIT_SHA="$DEPLOY_SHA" "$APP_ROOT/install/install.sh"
```

Release example:

```bash
git tag v2026.05.24
git push origin v2026.05.24
```

## Manual VPS Run

After secrets are synced, the same deploy can be run manually on the VPS:

```bash
cd /opt/legal_api/app
./install/install.sh
```

## Notes

- `install/config.sh` is the source of truth for production secrets and runtime paths.
- `backend/.env` is rendered on the VPS from `install/config.sh`.
- The production compose file is `install/docker-compose.prod.yml`.
- Nginx config is rendered to `install/nginx/rendered/default.conf` and mounted into the `web` container.
- Certbot uses the compose `certbot` service and stores certificates under `LETSENCRYPT_DIR`.
- `VITE_*` values are browser-public build-time values; do not use them for private secrets.
