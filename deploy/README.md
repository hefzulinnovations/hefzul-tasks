# Deploying Hefzul Tasks

This folder runs Hefzul Tasks on one small server with Docker Compose. It starts four services:

| Service | What it does |
| --- | --- |
| `app` | The Hefzul Tasks server (API and web app) on port 3456, inside the Docker network only |
| `db` | PostgreSQL 17, tuned for a 1 vCPU / 2 GB server |
| `caddy` | HTTPS reverse proxy; gets and renews a Let's Encrypt certificate on its own |
| `backup` | Nightly backup of the database and attachments, kept locally and optionally copied off-site |

The phase 0 gate is a successful restore test (step 6). Do it before relying on the server.

## 1. Server and DNS

- A VPS with 1 vCPU, 2 GB RAM, about 20 GB disk, running a current Debian or Ubuntu.
- Install Docker Engine and the Compose plugin: <https://docs.docker.com/engine/install/>.
- Point an A (and AAAA, if you have IPv6) DNS record for your domain at the server.
- Open ports 80 and 443 in the firewall. Nothing else needs to be public.

## 2. Get the files and configure

```sh
git clone https://github.com/hefzulinnovations/hefzul-tasks.git
cd hefzul-tasks/deploy
cp .env.example .env
openssl rand -hex 32   # paste as HEFZUL_JWT_SECRET
openssl rand -hex 32   # paste as HEFZUL_DB_PASSWORD
nano .env              # set HEFZUL_DOMAIN and HEFZUL_ACME_EMAIL too
```

The image is built by `.github/workflows/hefzul-image.yml` on every push to `main` and pushed to `ghcr.io/hefzulinnovations/hefzul-tasks:main`.
While the repository is private, the package is private too. Log in once with a GitHub personal access token that has the `read:packages` scope:

```sh
echo <token> | docker login ghcr.io -u <github-username> --password-stdin
```

## 3. Start

```sh
docker compose up -d
docker compose logs -f app   # wait for "Ran all migrations successfully", then Ctrl+C
```

Open `https://<your domain>`. Sign-up is closed, so create accounts on the server.

## 4. Create accounts

```sh
docker compose exec app /app/vikunja/vikunja user create -u kamal -e kamal@example.com
docker compose exec app /app/vikunja/vikunja user set-admin kamal --admin
docker compose exec app /app/vikunja/vikunja user create -u <spouse> -e <email>
```

Each command asks for the password. Turn on two-factor sign-in under **Settings → Two-factor authentication**.

## 5. Backups

The `backup` service runs every night at `HEFZUL_BACKUP_TIME` (default 02:30, `Asia/Dhaka`). Each run writes `deploy/backups/<date>T<time>/` with:

- `db.dump`: the whole database (`pg_dump` custom format)
- `files.tar.gz`: attachments and project backgrounds
- `SHA256SUMS`: checksums, verified again on restore

Local backups older than `HEFZUL_BACKUP_KEEP_DAYS` (default 14) are deleted.
Set the `HEFZUL_OFFSITE_*` values in `.env` to copy each backup to S3-compatible storage or WebDAV as well; `.env.example` shows both.
Without an off-site copy, losing the server loses the backups too.

Run a backup now and list the backups:

```sh
docker compose exec backup backup.sh
docker compose run --rm backup restore.sh
```

## 6. Restore (and the quarterly restore test)

```sh
docker compose stop app
docker compose run --rm backup restore.sh 2026-10-14T0230                  # from deploy/backups
docker compose run --rm backup restore.sh --from-offsite 2026-10-14T0230   # from the off-site copy
docker compose start app
```

The restore replaces the current database and files with the backup's.

**Restore test:** make a backup, create a test task, restore the backup, and check that the test task is gone and your earlier tasks and attachments are back.
Do this once before going live (the phase 0 gate) and then every quarter.

**Rebuilding on a new server:** do steps 1 to 3 with the same `.env`, run `docker compose stop app`, copy a backup folder into `deploy/backups/` (or use `--from-offsite`), restore it, start the app and point DNS at the new server.

## 7. Upgrades

```sh
docker compose pull app
docker compose up -d app
```

Database migrations run on start; downtime is usually under a minute. A backup runs every night, but take one by hand before a large upgrade.
To roll back, pin the previous image in `.env` (`HEFZUL_IMAGE=ghcr.io/hefzulinnovations/hefzul-tasks:sha-<short sha>`) and restore the backup taken before the upgrade.

## Licence

Hefzul Tasks is a fork of Vikunja and is licensed under the AGPL-3.0. Anyone who uses your server may ask for its source code, so keep the code you run published in a public repository linked from the app.
