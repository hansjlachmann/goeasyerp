# Operations (production with Docker)

How to run GoEasyERP in production with `docker-compose.prod.yml` (pre-built images from
`ghcr.io`), upgrade it, back it up and restore it.

## Files on the server

The server has a git checkout of this repository (e.g. `~/openerp`). Besides the tracked files it
holds:

| File | Tracked | What |
|---|---|---|
| `prod.env` | no (`.gitignore`) | Server settings and secrets: `APP_VERSION`, `JWT_SECRET`, `OPENERP_ENCRYPTION_KEY` (optional), `POSTGRES_PASSWORD`, `PUBLISH_HOST`, `NAV_PROXY_URL`. Template: `prod.env.example` |
| `certs/` | no | TLS certificate and key for nginx (`selfsigned.crt`, `selfsigned.key`) |

Always use the wrapper `scripts/prod.sh` instead of plain `docker compose`. It passes
`prod.env` and `docker-compose.prod.yml` and runs from the checkout folder:

```bash
scripts/prod.sh ps
scripts/prod.sh logs backend --since 10m
scripts/prod.sh pull && scripts/prod.sh up -d
```

> **Never run `docker compose down -v` or `docker volume prune`.** They delete the database
> volume (`<folder>_postgres_data`, e.g. `openerp_postgres_data`). The volume name comes from the
> checkout folder's name, so do not rename or move the folder either: compose would create a new,
> empty volume.

## First installation

```bash
git clone https://github.com/hansjlachmann/openerp.git ~/openerp && cd ~/openerp
cp prod.env.example prod.env && chmod 600 prod.env
# edit prod.env: APP_VERSION, JWT_SECRET (openssl rand -hex 32), POSTGRES_PASSWORD (a strong one)
./generate-cert.sh            # or put your own certificate into certs/
scripts/prod.sh pull && scripts/prod.sh up -d
```

On a fresh database `POSTGRES_PASSWORD` becomes the password of the database user `openerp`.

## Switching an existing installation to prod.env

Installations set up before `prod.env` existed run `docker compose -f docker-compose.prod.yml`
with `APP_VERSION` edited in the tracked `.env`. Move to `prod.env` once:

```bash
cd ~/openerp
mkdir -p ~/backups
docker compose -f docker-compose.prod.yml exec -T db pg_dump -U openerp -d openerp -Fc > ~/backups/openerp-$(date +%Y%m%d-%H%M)-pre-prod-env.dump
ls -lh ~/backups | tail -3                     # the dump must not be empty

git checkout -- .env                           # drop the local APP_VERSION edit
git pull                                       # new compose file, scripts, docs

cp prod.env.example prod.env && chmod 600 prod.env
# edit prod.env:
#   APP_VERSION=<release to run>
#   JWT_SECRET=<output of: openssl rand -hex 32>
#   POSTGRES_PASSWORD=openerp                  # keep the current password for now
#   PUBLISH_HOST=127.0.0.1                     # see "Ports" below before closing the ports

scripts/prod.sh pull && scripts/prod.sh up -d
scripts/prod.sh ps                             # all four services "Up"
scripts/prod.sh logs backend --since 5m | grep -iE "migration|SIFT|error|WARNING"
```

With `JWT_SECRET` set, the log no longer shows "WARNING: No JWT_SECRET set". Everyone has to log
in once more after this restart; later restarts keep the sessions.

## Upgrading to a new release

```bash
cd ~/openerp
scripts/backup.sh                              # or a manual pg_dump as above
sed -i 's/^APP_VERSION=.*/APP_VERSION=0.1.82/' prod.env
scripts/prod.sh pull && scripts/prod.sh up -d
scripts/prod.sh logs backend --since 5m | grep -iE "migration|SIFT|error"
```

Migrations run automatically at startup. `git pull` is only needed when the compose file,
nginx config or scripts changed (the release notes say so); it no longer conflicts, because
nothing tracked is edited on the server.

**Rollback:** set the previous `APP_VERSION` and `pull` + `up -d`. If a migration changed data in a
way the old version cannot read, restore the backup taken before the upgrade (see below).

## After a host reboot

All services have `restart: unless-stopped`, so they start again with Docker. Check with
`scripts/prod.sh ps`. A service you stopped on purpose (`scripts/prod.sh stop <service>`) stays
stopped.

## Ports

| Service | Published | Notes |
|---|---|---|
| nginx | `80`, `443` on all interfaces | The way in for users (HTTPS; 80 redirects to 443) |
| backend | `PUBLISH_HOST:8080` | Default `127.0.0.1`: only this host |
| frontend | `PUBLISH_HOST:3000` | Default `127.0.0.1`: only this host |
| db | `127.0.0.1:5432` | Admin tools through an SSH tunnel: `ssh -L 5432:127.0.0.1:5432 <server>` |

Users should only use nginx. Keep `PUBLISH_HOST=127.0.0.1` when nothing outside the host connects
to 3000/8080 — or when only a tunnel/proxy **on the same host** does (e.g. `cloudflared` or a host
nginx pointing at `localhost:3000`). Set `PUBLISH_HOST=0.0.0.0` only if a proxy **on another
machine** connects to them. To see what connects, check the tunnel/proxy configuration and
`sudo ss -tnp | grep -E ':(3000|8080)'` while the system is in use.

## Backups

`scripts/backup.sh` dumps the database through the `db` container (`pg_dump -Fc`) into
`~/backups/openerp-auto-YYYYMMDD-HHMM.dump` and deletes automatic backups older than 14 days
(`BACKUP_DIR`, `KEEP_DAYS` override this). Manual dumps with other names are never deleted.

Daily at 02:15 (`crontab -e`):

```
15 2 * * * $HOME/openerp/scripts/backup.sh >> $HOME/backups/backup.log 2>&1
```

Backups on the same disk do not survive a disk failure: copy them to another machine (e.g.
`rsync` or `scp` from that machine, on its own schedule).

## Restore

Restoring replaces the data of the running database. Take a fresh backup first if the current
state might still be needed.

```bash
cd ~/openerp
scripts/prod.sh stop backend                   # no writes during the restore
scripts/prod.sh exec -T db pg_restore -U openerp -d openerp --clean --if-exists --no-owner < ~/backups/<file>.dump
scripts/prod.sh start backend
```

To look at a backup without touching production, restore it into a scratch database:

```bash
scripts/prod.sh exec -T db createdb -U openerp restore_test
scripts/prod.sh exec -T db pg_restore -U openerp -d restore_test --no-owner < ~/backups/<file>.dump
scripts/prod.sh exec -T db psql -U openerp -d restore_test     # look around, then:
scripts/prod.sh exec -T db dropdb -U openerp restore_test
```

## Changing the database password

`POSTGRES_PASSWORD` is only applied when the volume is created. On an existing installation
change it in the database first, then in `prod.env`, then restart:

```bash
scripts/prod.sh exec db psql -U openerp -d openerp -c "ALTER USER openerp PASSWORD 'new-strong-password';"
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=new-strong-password/" prod.env
scripts/prod.sh up -d                          # backend reconnects with the new password
```

If the backend logs a password error afterwards, the two do not match: fix `prod.env` and run
`up -d` again.

## JWT secret

`JWT_SECRET` signs the session cookies. Without it the backend generates a random key on every
start (and logs a warning): sessions do not survive a restart. Set it once
(`openssl rand -hex 32`) and keep it; changing it logs everyone out once — and, unless
`OPENERP_ENCRYPTION_KEY` is set, the SMTP password has to be entered again (next section).

## Encrypted secrets (SMTP password)

The SMTP password in SMTP Setup is stored encrypted (AES-256-GCM), so a database dump or
backup does not reveal it. The key is `OPENERP_ENCRYPTION_KEY` in `prod.env` (optional) — when
it is empty, a key derived from `JWT_SECRET` is used, so nothing has to be set up. At startup the
backend logs `✓ Stored secrets encrypted (key from …)` and encrypts a password that was saved
before (`✓ SMTP password encrypted`, once). Without either key (development) the password is
stored as typed and the backend warns.

If the key changes (a new `OPENERP_ENCRYPTION_KEY`, or a new `JWT_SECRET` while
`OPENERP_ENCRYPTION_KEY` is empty), the stored password can no longer be read: job e-mails stop
(the log says "enter the SMTP password again") and "Send Test E-mail" says so. Enter the password
again in SMTP Setup. To be able to change `JWT_SECRET` without this, set
`OPENERP_ENCRYPTION_KEY` once (`openssl rand -hex 32`) and enter the SMTP password again.

## NAV report proxy

The NAV Report Runner codeunit (Job Queue) calls a NAV report proxy at `NAV_PROXY_URL` in
`prod.env` (e.g. `http://<proxy-host>:5009`). Empty means `http://localhost:5009` — inside the
backend container that is the container itself, so set the proxy's real address. Releases up to
0.1.84 had the address built in; when upgrading from one of them, add `NAV_PROXY_URL` to
`prod.env` before `scripts/prod.sh up -d`.
