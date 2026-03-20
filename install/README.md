# VPS Install Flow

This directory contains the deployment flow for a Linux VPS.

## Flow

1. On local machine, upload the deployment bundle:

```bash
cd install
cp secret.sh.sample secret.sh
# edit secret.sh
./copy.sh
```

2. SSH into the VPS manually.

3. On the VPS, finish configuration and run:

```bash
cd /opt/legal_api
cp install/config.sh.sample install/config.sh
# edit install/config.sh
./install/install.sh
```

## What gets uploaded

- `install/` scripts only

## Notes

- Backend runs with Docker Compose.
- Backend installer uses the repo's `backend/docker/docker-compose.yml`.
- Frontend is served by Nginx.
- Frontend installer builds from the repo source on the VPS.
- SSL is issued for `ai.dailyturning.com` by Certbot.
- Installers clone or update the repo from Git on the VPS before running.
- All VPS install variables live in `install/config.sh`.
