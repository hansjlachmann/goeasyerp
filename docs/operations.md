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
# edit prod.env: APP_VERSION, JWT_SECRET (openssl rand -hex 32), POSTGRES_PASSWORD (openssl rand -hex 24),
#                BUILDER_DB_PASSWORD (openssl rand -hex 24)
./generate-cert.sh            # or put your own certificate into certs/
scripts/prod.sh pull && scripts/prod.sh up -d
```

On a fresh database `POSTGRES_PASSWORD` becomes the password of the database role the server
works as, `goeasyerp` (see Database roles).

**The first user** (the administrator, SUPER in every company) is made on the login page with the
installation's setup token, so nobody else who reaches a new installation can claim it.
`SETUP_TOKEN` in prod.env sets it. If it is empty, the backend makes a random token and writes it
to its log at every start while there is no user yet:

```bash
scripts/prod.sh logs backend | grep "setup token"
```

Ten wrong tokens from one address lock that address for 15 minutes, like failed logins.

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
#   POSTGRES_PASSWORD=openerp                  # the current one for now; change it afterwards (below)
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

**Certificate files (from 0.1.188):** nginx's master process runs without the capability that lets
root read other users' files, so `certs/` must be readable by root as owner. A key owned by your
user with mode 600 stops nginx with "cannot load certificate key … Permission denied". Before the
first upgrade to 0.1.188 or later, and after every certificate renewal:

```bash
sudo chown root:root certs/*
scripts/prod.sh up -d
```

**Rollback:** set the previous `APP_VERSION` and `pull` + `up -d`. If a migration changed data in a
way the old version cannot read, restore the backup taken before the upgrade (see below).

## Extension apps

An app's code is compiled into the server program, so installing an app takes a build. **App
Build Setup** (Installed Apps → Build → App Build Setup) chooses who builds. It is one or the
other, never both: a pipeline building an older list would drop an app uploaded in the UI.

**The app audit:** every app's code is checked before it is built into the server — by the local
builder before it merges anything, by Upload App before the package is recorded, and in the CI
pipeline before the image build (`cmd/app-audit`). An app that fails is not built; the report is
in the build's log (App Builds → Show Log) or the upload's error. It refuses what would give an
app the server's powers: the environment (`JWT_SECRET` and the other secrets), files, processes,
the network, SQL behind the base's checks, other companies, other users' rights (EXTENSIONS.md,
"What an app's Go code may use"). A SUPER user can approve a finding for one app's exact source
(Installed Apps → App Audit Exceptions); App Build Setup's *Allowed Go Modules* lists third-party
modules apps may require (built in unaudited: only modules you trust). It does not make an app
harmless: an app can still change the data of the companies where it runs.

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
- The builder runs as user 65532. Started as root (no `runAsUser`), it gives the claim to 65532
  itself and then drops root — a pod that drops capabilities must keep `CHOWN`, `SETUID` and
  `SETGID`. With `runAsUser: 65532` it starts as 65532 directly; the claim must then be writable
  by that user (`fsGroup: 65532`, or a claim given to 65532 once).
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

**Sessions and passwords:** a session ends after 2 hours without requests, and always 24 hours
after the login. Set the idle limit with `SESSION_IDLE_TIMEOUT` in prod.env, e.g. `30m` or `8h`;
`0` leaves only the 24 hours. While the user works, the browser's token is renewed every few
minutes. A password has at least 10 characters, is not one of the commonly used passwords and does
not contain the user ID. Users change their own password from the user menu (Change Password).
That needs the current password and logs the user out everywhere else. An administrator setting a
user's password on the User card also logs that user out everywhere. Existing passwords keep
working; the rules apply when a password is set.

**Client addresses and logins:** the backend limits login attempts per client address and per user
ID: 10 failed logins of a user ID from one address within 15 minutes lock that address for that
user ID for 15 minutes; after 10 failed logins of a user ID from anywhere, each further attempt
waits a little longer (1, 2, 4 … up to 60 seconds) — except from an address the user has logged in
from before, so others' guessing never locks a user out of their usual place. It takes the client's
address from `X-Forwarded-For` only on connections from a trusted proxy — `TRUSTED_PROXIES` in
prod.env (addresses or CIDR ranges; empty: loopback and the private ranges, where the compose
file's nginx runs). nginx overwrites the header with the address it sees, so a client cannot send
its own. A proxy on another machine with a public address must be listed in `TRUSTED_PROXIES`, and
must overwrite the header too.

The frontend's own `/api` pass-through (used when a proxy sends everything to the frontend, e.g. a
Kubernetes ingress) drops the client's `X-Forwarded-For` and sends the address it sees itself. When
a proxy stands in front of the frontend, tell the frontend to read the address from that proxy's
header: `ADDRESS_HEADER=X-Forwarded-For` with `XFF_DEPTH=1` (the number of proxies in front of it),
else every user shares the proxy's address.

**Security events:** the backend records logins and failed logins, lockouts, logins refused while
an address is locked, log outs, refused companies, every request refused for want of permission
(403), changes refused because another web site sent them, password changes, the first user and
wrong setup tokens, app installs and removals, enabling and disabling apps, build requests, and
renamed or deleted companies — with the time, the user, the client's address and the company.
Administration → Security Events lists them (SUPER users, or a role with read permission on the
table Security Event). A flood of one kind is cut short: at most five events of a kind per user and
address a minute, then one event saying how many more there were. Each event is also written to the
backend's log (`security: …`).

To be told about them, schedule routine 909 Security Event Alerts in the Job Queue (e.g. every
hour) with a notification e-mail on errors. Each run looks at the events since the last run and
fails — so the job's e-mail goes out — when it finds a lockout, a wrong setup token, a change sent
from another site, a user with 20 or more refused requests, or 50 or more failed logins in all. It
also deletes events older than a year. The Parameter changes the limits: `REFUSALS=20`,
`FAILED_LOGINS=50`, `KEEP_DAYS=365` (0: keep every event). Check Events on the page shows the same
findings without marking anything. The events hold personal data (user IDs, addresses): keep
`KEEP_DAYS` as short as your rules allow.

**Change log:** every change of a record in the tables that matter for security and money is
recorded field by field — who, when, in which company, the record's key, the value before and
after: users and their roles, roles and permissions, companies, installed apps, App Build Setup,
SMTP Setup, bank accounts and the customers' and vendors' bank accounts, the general and tax
posting setups, the posting groups, and the change log's own setup. Administration → Change Log
Entries lists them. A secret (a password, the SMTP password) and a picture are recorded only as
"(changed)", never their value. The entries are written in the change's own transaction (a change
that is undone leaves none), and nobody can delete them through the app. Administration → Change Log
Setup switches it off (Change Log Activated) or, under Tables, sets up other tables: per table,
insertions, modifications and deletions with all fields or some fields (Fields). A table with a
line there follows the line instead of the built-in list. Only SUPER users may change the setup.
Changes made by code that writes many records at once (bulk inserts of demo data and imports) are
not recorded.

**Browser security headers:** the frontend sends a Content Security Policy (scripts only from the
app), `X-Frame-Options: SAMEORIGIN` (other sites cannot frame it), `nosniff`, `Referrer-Policy` and
`Permissions-Policy`; the backend sends `no-store` (answers hold company data and are not kept in
the browser's cache), `nosniff` and `DENY` for framing, and refuses changes sent from another web
site (cross-site request forgery: a request whose `Sec-Fetch-Site` is `cross-site`/`same-site`, or
whose `Origin` names another host). nginx adds HSTS (`Strict-Transport-Security`, a year): once a
browser has seen it, it opens the site with https only — make sure the certificate is renewed in
time. `CORS_ORIGINS` (commas) lists other origins whose pages may call the API with the user's
session; empty (the default) allows none, which is right when the app is served from the same
address as the API.

## Containers and limits

The compose files run every container with only the rights it needs:

- **Users:** the backend runs as user 65532 and the frontend as `node`, not as root. postgres and
  nginx start as root and change to their own users (`postgres`, `nginx`). The app builder starts
  as root only to give its builds volume to user 65532 when something in it belongs to another
  user (a volume an older builder filled), then runs as 65532.
- **Capabilities:** every container drops all Linux capabilities. postgres gets back the five it
  needs to change to its user (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETUID`, `SETGID`). nginx gets
  back `CHOWN`, `SETUID`, `SETGID` and `NET_BIND_SERVICE` (ports 80/443), the app builder
  `CHOWN`, `SETUID` and `SETGID`.
- **No new privileges:** no container can gain rights through a setuid program
  (`no-new-privileges`).
- **Read-only file systems:** the backend, the frontend and (on the server) nginx have read-only
  file systems, with `/tmp` and nginx's cache and pid directories in memory. The database keeps a
  writable one, because the trial install script copies its backups out of `/tmp`.
  nginx's master has no capability to read files of other users: the files in `certs/` must
  belong to root (see "Certificate files" under Upgrading).

**Logs:** Docker keeps at most 5 files of 10 MB per container (`json-file`, rotated). Older lines
are gone, so copy what you need with `docker compose logs <service> > file` in time.

**Requests:** nginx and the backend refuse a request body over 8 MB (answer 413). The backend
gives a client 2 minutes to send a request, and closes an idle connection after 2 minutes. Answers
have no time limit, so long jobs and their progress streams are not cut off. nginx waits up to an
hour for `/api` answers.

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

**Connections.** The backend opens at most 40 database connections at once (`DB_MAX_CONNS` in
`prod.env`; PostgreSQL allows 100 by default). A burst of requests or long reports then waits
for a free connection instead of using up PostgreSQL's connections, which would refuse
everybody — the scheduler, the app builder, backups and psql included. Raise it together with
PostgreSQL's `max_connections` only for many users at once. The same goes for routines and
reports: a user runs at most 4 at once (a fifth is refused with a message), the server at most 16
(more wait for a free one).

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

`POSTGRES_PASSWORD` is required (the compose file refuses to start without it) and is only
applied when the database volume is created. Installations made before it was required run
with `openerp`, which everyone can read in this repository. Since 0.1.191 the backend refuses to
start with it (or with `postgres`, `password`, `changeme`, or none): "Refusing to start: the
database password (DB_PASSWORD) is "openerp" …". Change it before upgrading — the database first,
then `prod.env`, then restart:

```bash
cd ~/openerp
NEWPW=$(openssl rand -hex 24)
scripts/prod.sh exec -T db psql -U openerp -d openerp -v ON_ERROR_STOP=1 -c "ALTER ROLE goeasyerp PASSWORD '$NEWPW';"
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$NEWPW/" prod.env
unset NEWPW
scripts/prod.sh up -d                          # backend and builder reconnect with the new password
scripts/prod.sh logs backend --since 5m | grep -iE "password|error"
```

`psql -U openerp` works inside the database container only (it trusts local connections); the
server's role is `goeasyerp` (Database roles). Without `ON_ERROR_STOP` a failed `ALTER ROLE`
would go unnoticed and the new `prod.env` would lock the backend out.

The app builder's password the same way: `ALTER ROLE goeasyerp_builder PASSWORD '…'`, then
`BUILDER_DB_PASSWORD` in `prod.env`.

If the backend logs a password error afterwards, the two do not match: put the password the
database has into `prod.env` and run `up -d` again.

The one-computer trial does this by itself: its install script (`scripts/goeasyerp.sh` /
`.ps1`) gives an installation that has no `POSTGRES_PASSWORD` in its `.env` a random one — set in
the database first (after its backup), then written to `.env`.

## JWT secret

`JWT_SECRET` signs the session cookies — whoever knows it can make a session for any user. The
backend refuses to start with a database server (`DB_HOST`) and no `JWT_SECRET`, with one
shorter than 16 characters, or with an example text from the repository (it warns below 32);
only a development start without a database server uses a random key (sessions then end at
every restart). Set it once
(`openssl rand -hex 32`) and keep it; changing it logs everyone out once — and, unless
`OPENERP_ENCRYPTION_KEY` is set, the SMTP password has to be entered again (next section).

## Where the server sends mail (SMTP Setup)

Only SUPER users may change SMTP Setup. Its SMTP server is never an address that reaches the
server itself (loopback, `0.0.0.0`) or the link-local network — the cloud metadata service
`169.254.169.254` included — nor a multicast address: such an address is refused when it is
entered, and a name that resolves to one is refused when the server connects (it connects to the
address it checked). Otherwise a user who may change the setup could use "Send Test E-mail" to
find out what listens on the server's network. Private addresses (`10.x`, `172.16–31.x`,
`192.168.x`, `fd…`) are allowed, for an office's own mail relay; set `SMTP_BLOCK_PRIVATE=true` in
prod.env when the mail server is outside your network, and they are refused too.

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
