# GoEasyERP installation repository (template)

An installation that runs extension apps has a repository of its own, made from these files
(a private repository: it names the installation's apps). It holds the installation's build
list, `apps.yaml`, and a workflow that builds the backend image — the GoEasyERP base release
with the apps compiled in — and pushes it to the repository's package registry.

## Setting it up

1. Create a new private repository and copy these files into it (`apps.yaml`,
   `.github/workflows/build.yml`).
2. If an app's repository is private, add a repository secret **APPS_TOKEN**: a token that may
   read the apps' repositories. (Public apps need none.)
3. On the server, set `BACKEND_IMAGE` in `prod.env` to the image the workflow pushes
   (`ghcr.io/<owner>/<this repository>/backend:latest`, or a version tag), and log in once to the
   registry with a token that may read packages: `docker login ghcr.io`.

## Installing, updating and removing apps

1. In GoEasyERP, **Administration → Installed Apps**: **Install App** (the app's GitHub
   repository, its version tag and the folder of its `application.yaml`), **Uninstall App**, or
   **Cancel Pending**. GoEasyERP checks the app — number series, prefix, dependencies and their
   versions — when you choose it, and shows it as Pending Install / Pending Removal.
2. **Build List** downloads `apps.yaml`: the base version and the apps of the next build, in
   dependency order. Commit it to this repository (replacing the old one).
3. The workflow runs on the push: it checks out the base release and each app at its version
   and builds and pushes the image (`<base version>` and `latest` tags) for linux/amd64 and linux/arm64. The base's build checks
   the apps again (dependencies, number series, prefix) and runs nothing when one is refused.
4. On the server: `scripts/backup.sh`, then `scripts/prod.sh pull && scripts/prod.sh up -d`. At
   startup GoEasyERP shows the apps as Installed (and removed apps are gone; their data stays in
   the database).

Upgrading the base release: install nothing new, set the new base version in `apps.yaml`
(Build List writes the version the server runs) and push.
