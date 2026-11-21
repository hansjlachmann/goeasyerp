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
git clone https://github.com/hansjlachmann/goeasyerp.git ~/openerp && cd ~/openerp
cp prod.env.example prod.env && chmod 600 prod.env
# edit prod.env: APP_VERSION, JWT_SECRET (openssl rand -hex 32), POSTGRES_PASSWORD (a strong one),
#                BUILDER_DB_PASSWORD (openssl rand -hex 24)
./generate-cert.sh            # or put your own certificate into certs/
scripts/prod.sh pull && scripts/prod.sh up -d
```

On a fresh database `POSTGRES_PASSWORD` becomes the password of the database role the server
works as, `goeasyerp` (see Database roles).

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

**nginx version:** the compose file pins nginx to a fixed version (`nginx:1.31.9-alpine`); a
release that raises it (an nginx security fix) says so in its notes. After `git pull`,
`scripts/prod.sh pull && scripts/prod.sh up -d` starts the new nginx; check it with
`scripts/prod.sh exec nginx nginx -v`.

**Rollback:** set the previous `APP_VERSION` and `pull` + `up -d`. If a migration changed data in a
way the old version cannot read, restore the backup taken before the upgrade (see below).

## Extension apps

An app's code is compiled into the server program, so installing an app takes a build. **App
Build Setup** (Installed Apps → Build → App Build Setup) chooses who builds. It is one or the
other, never both: a pipeline building an older list would drop an app uploaded in the UI.

**Which route?**

| | Local builder | CI pipeline |
|---|---|---|
| Needs | the builder service, about 2.5 GB of memory while it builds, and 2 GB of disk for its volume | an installation repository on GitHub (and its registry) |
| A change of the apps | Build Apps in the UI; the server switches by itself | Build List, commit `apps.yaml`, then pull and restart |
| Build time on a Raspberry Pi (one app) | about 1½ min after an upgrade or an app change, under 20 s for the same apps again | none on the server: GitHub builds, the server only pulls |
| Choose it for | one-computer installations and small teams that install apps themselves | small servers with many apps or little memory, and production that tests a build in a test environment first (the same image runs in both) |

The first build after an upgrade or an app change compiles most of the server again: an app
adds fields to the base tables, and the server's table code is one package, so everything that
uses it changes. That time grows with the number of apps and the base; builds of the same apps
again come from the build cache.

The builder uses every CPU it gets: **give it no CPU limit** (Docker `cpus:`, a Kubernetes CPU
limit) — a limit slows the build directly. Set `GOFLAGS=-p=1` in its environment only when a
small server runs short of memory (fewer packages at a time, a longer build).

### Local builder (no repository needed)

The installation builds its apps itself. The build list and the build history are in the
database, so `scripts/backup.sh` covers them.

1. Start the builder: put `COMPOSE_PROFILES=apps` in `prod.env` (the one-computer trial: in the
   `.env` next to the compose file), then `scripts/prod.sh pull && scripts/prod.sh up -d`.
   The builder image is large (the Go toolchain), so it is downloaded only with this profile.
   A build needs about **2.5 GB of memory** while it compiles (the largest compile, the
   server's generated table code, peaks at about 1.5 GB). The builder compiles that package
   with one thread, `GOGC=25` and two packages at a time to stay there; a server with less
   memory should use the CI pipeline route below.
   `GOFLAGS` (`-p=`, `-gcflags`) and `GOGC` in the builder's environment replace these settings.
2. In App Build Setup, set **Builds By** to *Local Builder*, **Allow Upload** if apps may
   come as packages, and **Allowed Repositories**: the GitHub owners (`acme`) or repositories
   (`acme/loyalty`) apps may be installed from, separated by commas. Blank allows none. Name only
   sources you trust: an app's code runs inside the server.
3. Choose the apps in Installed Apps:
   - **Install App** takes a repository and a tag, and pins the app to the commit the tag points
     to (Repository Commit). Every build takes exactly that commit, and refuses to build when the
     tag points elsewhere. An app installed before commits were recorded has none: its builds warn, and
     Install App with the same version pins it;
   - **Upload App** takes an app package (`.zip`, `EXTENSIONS.md` §10);
   - **Uninstall App** removes one.
4. Run **Build → Build Apps**. The builder builds the program with the apps into the `app_builds`
   volume. Follow it under **Build → App Builds**, where Show Log gives the builder's output.
   When the build is done the server restarts by itself on the new program (users are logged
   out briefly) and the apps show as Installed. The port stays open while it restarts: the old
   program hands its socket to the new one, and until that one is ready the page shows
   "GoEasyERP is starting" and opens again by itself, instead of an error 502.

**Upgrading** works as above (`APP_VERSION`, `pull`, `up -d`), and the builder follows the same
version. A server of a new version finds no build of its apps for that version. It requests one
and waits in **maintenance** until it is built: the login page says so, and `/health` answers.
- If that build fails, the maintenance message names the error and
  `scripts/prod.sh logs builder` shows it.
- To recover, go back to the previous `APP_VERSION`, or restart the backend to try again.
- The last three builds stay in the volume.
- The volume also holds the builder's Go build cache (`.cache/`, about 1–1.5 GB), so allow
  about 2 GB plus 100 MB per kept build. With the cache, building the same apps again takes
  seconds; the first build after an upgrade compiles the parts of the server the apps change.
  Go removes cache entries no build has used for five days. Deleting the folder is safe: the
  next build is slower.
- Each build's log (App Builds → Show Log) ends with the time of each step: copy, fetch, merge,
  generate, compile, bundle.

**Kubernetes:** the same images.
- A builder Deployment (`ghcr.io/hansjlachmann/goeasyerp/builder:<version>`, the backend's
  `DB_*` environment, `APP_BUILDS_DIR=/builds`).
- A PersistentVolumeClaim mounted at `/builds`: read-write by the builder, read-only by the
  backend, which gets `APP_BUILDS_DIR=/builds` too.
- With more than one backend replica the claim must be ReadWriteMany.
- Keep the builder's image tag equal to the backend's.
- Give the builder a memory limit of at least 2.5 GiB and no CPU limit (see "Which route?"
  above). A build killed for memory shows in the node's kernel log (`dmesg`) as an
  out-of-memory kill of `compile`.

### CI pipeline (installation repository)

The installation runs its own backend image, built by its installation repository (template
and steps: `deploy/installation-template/README.md`):
- In `prod.env`, `BACKEND_IMAGE` names that image. The frontend image stays the base release's
  (`APP_VERSION`).
- The image is private, so log in to the registry once with `docker login ghcr.io`.
- Installing, updating and removing apps: Installed Apps → Install App / Uninstall App, then
  Build → Build List, then commit `apps.yaml` to the installation repository. After that run
  `scripts/backup.sh` and `scripts/prod.sh pull && scripts/prod.sh up -d` as for an upgrade.
- Without `BACKEND_IMAGE` the base release's backend image runs.

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

**Client addresses and logins:** the backend limits login attempts per client address and per
user ID (10 failed logins in 15 minutes lock the user ID for 15 minutes). It takes the client's
address from `X-Forwarded-For` only on connections from a trusted proxy — `TRUSTED_PROXIES` in
prod.env (addresses or CIDR ranges; empty: loopback and the private ranges, where the compose
file's nginx runs). nginx overwrites the header with the address it sees, so a client cannot send
its own. A proxy on another machine with a public address must be listed in `TRUSTED_PROXIES`, and
must overwrite the header too.

## Database roles

The database image makes `openerp` (`POSTGRES_USER`) the database server's superuser: it could
drop every database, read the server's files and run programs there. The backend does not work
as it. On the first start that finds `openerp` a superuser (a new installation, or the first
start of a release with this change) the backend:

- makes the role `goeasyerp` with the password it was given (`POSTGRES_PASSWORD`) — the owner of
  the database and its tables, without superuser rights, and unable to create databases or roles;
- makes the role `goeasyerp_builder` for the app builder when `BUILDER_DB_PASSWORD` is set — it
  may read and change the build tables only (App Build; reads App Package, Installed App and App
  Build Setup);
- hands `goeasyerp` everything `openerp` owns in the database, and takes `openerp`'s password away.

The log says so (`database: … handed over to goeasyerp`). From then on the backend's settings
(`DB_USER: openerp` and the password) open only `goeasyerp`: the backend tries it first. The
superuser `openerp` is reached only inside the database container, which trusts local
connections — `scripts/prod.sh exec db psql -U openerp`, as the backups do.

Set `BUILDER_DB_PASSWORD` in `prod.env` before the first start of that release: afterwards the
backend no longer has the right to make the builder's role (it logs how to make it by hand, as
`openerp` inside the container). Without it the builder works as `goeasyerp`.

`DB_KEEP_SUPERUSER=true` in the backend's environment leaves `DB_USER` as it is (development
databases whose superuser password tests need).

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
scripts/prod.sh exec -T db pg_restore -U goeasyerp -d openerp --clean --if-exists --no-owner < ~/backups/<file>.dump
scripts/prod.sh start backend
```

Restore as `goeasyerp` (`-U goeasyerp`), so the restored tables belong to the backend's role; a
dump made before the database roles existed restores the same way.

To look at a backup without touching production, restore it into a scratch database:

```bash
scripts/prod.sh exec -T db createdb -U openerp -O goeasyerp restore_test
scripts/prod.sh exec -T db pg_restore -U goeasyerp -d restore_test --no-owner < ~/backups/<file>.dump
scripts/prod.sh exec -T db psql -U openerp -d restore_test     # look around, then:
scripts/prod.sh exec -T db dropdb -U openerp restore_test
```

## Changing the database password

`POSTGRES_PASSWORD` is only applied when the volume is created. On an existing installation
change it in the database first, then in `prod.env`, then restart:

```bash
scripts/prod.sh exec db psql -U openerp -d openerp -c "ALTER ROLE goeasyerp PASSWORD 'new-strong-password';"
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=new-strong-password/" prod.env
scripts/prod.sh up -d                          # backend reconnects with the new password
```

The app builder's password the same way: `ALTER ROLE goeasyerp_builder PASSWORD '…'`, then
`BUILDER_DB_PASSWORD` in `prod.env`.

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
