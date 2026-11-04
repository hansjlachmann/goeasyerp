<p align="center"><img src="goeasyerp-brand/goeasyerp-wordmark-slate.svg" alt="GoEasyERP" height="64"></p>

# GoEasyERP

Open-source ERP for small businesses: finance, sales, purchasing, inventory, projects and fixed
assets in one system. Go backend, SvelteKit frontend, PostgreSQL or SQLite.

The database, its user and the `OPENERP_*` settings keep the technical name `openerp`.

## Try it on your own computer

Only [Docker Desktop](https://www.docker.com/products/docker-desktop/) is needed (Windows, macOS
or Linux):

1. Download [`docker-compose.local.yml`](https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml)
   and save it as `docker-compose.yml` in a folder of its own (e.g. `C:\GoEasyERP`).
2. Open it in a text editor and replace `JWT_SECRET` with any long random text.
3. In that folder run `docker compose up -d`.
4. After 2-3 minutes open http://localhost:8000 and create the first user.

Stop with `docker compose stop`, start again with `docker compose start`. Never run
`docker compose down -v`: it deletes the database. For a server installation (HTTPS, backups) see
[docs/operations.md](docs/operations.md).

### Windows 10/11 with Docker Desktop

Requirements: Windows 10 22H2 or Windows 11 (64-bit), virtualization enabled in the BIOS/UEFI
(Task Manager → Performance → CPU → "Virtualization: Enabled"), about 4 GB free memory and 10 GB
disk.

1. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) with "Use WSL 2"
   checked, restart when asked and start Docker Desktop until it shows "Engine running".
2. In PowerShell:

   ```powershell
   mkdir C:\GoEasyERP; cd C:\GoEasyERP
   Invoke-WebRequest https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml -OutFile docker-compose.yml
   notepad docker-compose.yml      # replace JWT_SECRET with any long random text, save
   docker compose up -d
   ```

3. After 2-3 minutes open http://localhost:8000 and create the first user.

`docker info --format "{{.OSType}}"` must print `linux`; if it prints `windows`, right-click the
Docker icon in the taskbar and choose "Switch to Linux containers". GoEasyERP starts whenever
Docker Desktop runs (Settings → General → "Start Docker Desktop when you sign in"). To upgrade,
download the file again (keep your `JWT_SECRET`), then `docker compose pull` and
`docker compose up -d` in `C:\GoEasyERP`.

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
mkdir ~/goeasyerp && cd ~/goeasyerp
curl -o docker-compose.yml https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml
nano docker-compose.yml         # replace JWT_SECRET
docker compose up -d
```

The file publishes port 8000 on this computer only. For other computers change the line
`"127.0.0.1:8000:80"` to `"8000:80"`, set `ORIGIN` to `http://<computer name>:8000` and open port
8000 in the firewall (with Docker Engine in WSL2 Windows also needs a port forwarding to WSL —
`netsh interface portproxy` —, which a Linux VM does not).

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

Codeunit 901 "Create Demo Data" fills a company with demo data: countries
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
