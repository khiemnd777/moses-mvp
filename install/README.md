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

Local development runs through the dev Compose file, not the production Nginx web image:

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

The local `web` service runs Vite with `frontend/` bind-mounted into `/app`, so source edits are watched and pushed through Vite HMR. The container keeps `/app/node_modules` on a named Docker volume so host bind mounts do not hide installed Linux dependencies. On every `web` startup it runs `bun install` before `bun run dev`, which picks up package changes.

To install or add frontend libraries inside the same container/volume:

```bash
make web-install
make web-add PKG="@tanstack/react-query"
./install/local-compose.sh web-add lucide-react
```

`web-add` updates `frontend/package.json`, the Bun lockfile, and the Docker volume-backed `node_modules`.

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

`sync-secrets.sh` is the single sync entrypoint. It:

- uploads `install/config.sh` to `APP_ROOT/install/config.sh` on the VPS
- generates a GitHub Actions deploy key when one does not already exist
- installs the deploy public key into the VPS user's `authorized_keys`
- sets GitHub environment secrets `VPS_HOST`, `VPS_USER`, `VPS_PORT`, `VPS_KNOWN_HOSTS`, and `VPS_SSH_KEY`
- sets GitHub environment variable `VPS_APP_ROOT`

The app runtime secrets stay on the VPS in `install/config.sh`; GitHub only receives the SSH inputs needed to run the deploy workflow.

## GitHub Actions Deploy

`sync-secrets.sh` configures these GitHub environment secrets automatically:

- `VPS_HOST`
- `VPS_USER`
- `VPS_SSH_KEY`
- optional `VPS_PORT`
- optional `VPS_KNOWN_HOSTS`

`sync-secrets.sh` configures this GitHub environment variable automatically:

- optional `VPS_APP_ROOT`, default `/opt/legal_api/app`

CI runs from `.github/workflows/deploy.yml` on pull requests, pushes to `main`, and pushes to release tags that match `v*`.

Production deploy runs only on:

- pushing a tag that matches `v*`, for example `v2026.05.24` or `v2026.05.24-1`
- `workflow_dispatch`

The workflow SSHes into the VPS, checks out the exact GitHub SHA, and runs:

```bash
INSTALL_CONFIG_FILE="$APP_ROOT/install/config.sh" GIT_COMMIT_SHA="$DEPLOY_SHA" "$APP_ROOT/install/install.sh"
```

If the VPS already has a host-level edge proxy such as Caddy owning ports `80` and `443`, keep that proxy as the public edge. Set the Legal API web container to a localhost-only port in `install/config.sh`, for example:

```bash
export HTTP_BIND="127.0.0.1"
export HTTP_PORT="18081"
export ENABLE_SSL="0"
export WEB_PUBLIC_URL="https://ai.dailyturning.com"
```

Then point the edge proxy for `ai.dailyturning.com` at `http://127.0.0.1:18081`.

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
- `OPENAI_EMBEDDINGS_MODEL` must match the existing Qdrant collection dimension. The default `text-embedding-3-small` uses 1536 dimensions.
- The production compose file is `install/docker-compose.prod.yml`; the local dev compose file is `install/docker-compose.dev.yml`.
- Nginx config is rendered to `install/nginx/rendered/default.conf` and mounted into the `web` container.
- Certbot uses the compose `certbot` service and stores certificates under `LETSENCRYPT_DIR`.
- `VITE_*` values are browser-public build-time values; do not use them for private secrets.
