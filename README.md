<p align="center"><img src="goeasyerp-brand/goeasyerp-wordmark-slate.svg" alt="GoEasyERP" height="64"></p>

# GoEasyERP

Open-source ERP for small businesses: finance, sales, purchasing, inventory, projects and fixed
assets in one system. Go backend, SvelteKit frontend, PostgreSQL or SQLite.

The database, its user and the `OPENERP_*` settings keep the technical name `openerp`.

## Try it on your own computer

Only Docker is needed: [Docker Desktop](https://www.docker.com/products/docker-desktop/) on
Windows or macOS, Docker Engine or Docker Desktop on Linux. Install and upgrade are the same one
line:

**Windows** (PowerShell, after Docker Desktop shows "Engine running"):

```powershell
irm https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.ps1 | iex
```

**Linux, macOS, WSL**:

```bash
curl -fsSL https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.sh | sh
```

After 2-3 minutes open http://localhost:8000 and create the first user.

The script works in a folder of its own — `C:\GoEasyERP` on Windows, `~/goeasyerp` elsewhere
(another one: `$env:GOEASYERP_DIR = 'D:\GoEasyERP'` before the line in PowerShell,
`… | GOEASYERP_DIR=/srv/goeasyerp sh` in a shell). It checks that Docker runs Linux containers, keeps
the settings in `.env` (a new installation gets a random `JWT_SECRET`; an installation made by hand
with an older `docker-compose.yml` keeps its secret, port and address), backs the database up into
`backups/` when GoEasyERP already runs, downloads the latest `docker-compose.yml` (the old one stays
as `docker-compose.yml.bak`), and starts GoEasyERP. Run the line again to upgrade.

To read the script before it runs, download it and run the file:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.ps1 -OutFile goeasyerp.ps1
notepad goeasyerp.ps1
powershell -ExecutionPolicy Bypass -File goeasyerp.ps1
```

```bash
curl -fsSLO https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.sh
less goeasyerp.sh
sh goeasyerp.sh
```

Without the script: save
[`docker-compose.local.yml`](https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml)
as `docker-compose.yml` in a folder of its own, create a file `.env` next to it with the line
`JWT_SECRET=<any long random text>`, and run `docker compose up -d` in that folder. Upgrade: save
the new file over it, then `docker compose pull` and `docker compose up -d`.

Stop with `docker compose stop`, start again with `docker compose start` (in the folder). Never run
`docker compose down -v`: it deletes the database. Keep `JWT_SECRET` in `.env`: it also encrypts
stored passwords (SMTP Setup). A backup is restored with `docker compose exec -T db pg_restore -U
openerp -d openerp --clean --if-exists < backups/<file>.dump` (Linux shell; on Windows copy the file in
with `docker compose cp` first). For a server installation (HTTPS, backups) see
[docs/operations.md](docs/operations.md).

### Windows 10/11 with Docker Desktop

Requirements: Windows 10 22H2 or Windows 11 (64-bit), virtualization enabled in the BIOS/UEFI
(Task Manager → Performance → CPU → "Virtualization: Enabled"), about 4 GB free memory and 10 GB
disk. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) with "Use WSL 2"
checked, restart when asked and start Docker Desktop until it shows "Engine running", then run the
PowerShell line above.

Docker must run Linux containers (the script checks it): if `docker info --format "{{.OSType}}"`
prints `windows`, right-click the Docker icon in the taskbar and choose "Switch to Linux
containers". GoEasyERP starts whenever Docker Desktop runs (Settings → General → "Start Docker
Desktop when you sign in").

Docker Desktop is free for personal use, education and small businesses (fewer than 250 employees
and less than USD 10 million annual revenue); larger companies need a paid Docker subscription — or
use Docker Engine in WSL2 (below), which is free.

**Windows Server**: Docker Desktop is not supported there, and the Docker Engine on Windows Server
runs Windows containers only (`docker info --format "{{.OSType}}"` prints `windows`), which cannot
run GoEasyERP. Use Docker Engine in WSL2 (Windows Server 2022/2025) or a Linux VM in Hyper-V (any
version; better for a server several people use):

```bash
# in Ubuntu (wsl --install -d Ubuntu, then restart) or in the Linux VM
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER   # log out and in again
curl -fsSL https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.sh | sh
```

### Other computers

GoEasyERP answers on this computer only. For other computers set in `.env`:

```
GOEASYERP_BIND=0.0.0.0
GOEASYERP_ORIGIN=http://<computer name>:8000
```

then run `docker compose up -d` in the folder and open port 8000 in the firewall (with Docker Engine
in WSL2 Windows also needs a port forwarding to WSL — `netsh interface portproxy` —, which a Linux
VM does not). `GOEASYERP_PORT` changes the port.

## Project Structure

```
openerp/
├── cmd/api-server/      # Go API server entry point
├── backend/
│   ├── api/             # HTTP handlers, middleware, server
│   ├── definitions/     # Table, page and menu definitions (YAML)
│   ├── generated/       # Code generated from the table definitions (never edit)
│   ├── business-logic/  # Table triggers, codeunits, migrations
│   └── foundation/      # Core framework (types, database, i18n, etc.)
├── frontend/            # SvelteKit frontend
├── tools/tablegen/      # Code generation tool
└── translations/        # i18n files (en-US, nb-NO)
```

## Build Commands

### API Server (Go)
```bash
# Build
cd cmd/api-server && go build -o api-server .

# Run (from project root)
./cmd/api-server/api-server

# Run without building
go run ./cmd/api-server
```

### Frontend (SvelteKit)
```bash
cd frontend

# Install dependencies
npm install

# Development server    
npm run dev

# Production build
npm run build
```

### Code Generation (tablegen)
```bash
# Generate the table code from backend/definitions/tables (after changing a table YAML)
go generate ./backend/generated/tables
```

### General Go Commands
```bash
# Format code
go fmt ./...

# Check code
go vet ./...

# Update dependencies
go mod tidy
```

## Database

Connect to SQLite database:
```bash
sqlite3 test.db
```

Useful SQLite commands:
```sql
-- List all tables
.tables

-- Show table schema
.schema "cronus$Payment Terms"

-- Show all data with formatting
.mode column
.headers on
SELECT * FROM "cronus$Payment Terms";

-- Exit
.exit
```

## Demo Data

Routine 901 "Create Demo Data" fills a company with demo data: countries
(NO, DK, GB), payment terms, and Norwegian, Danish and English customers with a
year of customer ledger entries. The data is in `backend/business-logic/demodata/data/*.yaml`.

1. Create a company, e.g. `demo01` with display name "Demo Company 01", and switch to it (Ctrl+O).
2. Add a Job Queue line: Object ID to Run `901`, Parameter `SMALL`, `LARGE` or `HEAVY`.
3. Run it (F9), or set it to Ready with a Next Start to let the scheduler run it.

| Size  | Customers | Ledger entries | Use |
|-------|-----------|----------------|-----|
| SMALL | 20        | ~540           | demos, screenshots, E2E |
| LARGE | 10,000    | ~140,000       | paging, search and performance tests |
| HEAVY | 200       | ~190,000 (~1,000 per customer, 2 years) | SIFT / FlowField volume tests |

The run is one transaction (all or nothing), produces the same data every time
(fixed random seed, dates relative to the run date). Run again in a company that
already has customers, it first deletes all of the company's data except the job
queue and user setup, and creates the demo again; run from the page it asks first.
Never run it in a company with real data.
Every run is logged in Job Queue Entries.
