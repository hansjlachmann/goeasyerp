# GoEasyERP on one computer (Windows, Docker Desktop): install, or upgrade an installation made before.
#
#   irm https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.ps1 | iex
#
# Or download it, read it and run it:
#   powershell -ExecutionPolicy Bypass -File goeasyerp.ps1
#
# What it does, in the folder (default C:\GoEasyERP, or $env:GOEASYERP_DIR):
#   1. checks that Docker Desktop runs Linux containers and has "docker compose";
#   2. keeps the settings in .env - a new installation gets a random JWT_SECRET and database
#      password; an older installation's docker-compose.yml gives its JWT_SECRET, port and ORIGIN
#      (the secret also encrypts stored passwords, so it must never change), and its database
#      gets a password of its own instead of the public "openerp" (after the backup);
#   3. backs the database up into backups\ when GoEasyERP already runs;
#   4. downloads the latest docker-compose.yml (the old one stays as docker-compose.yml.bak);
#   5. docker compose pull, docker compose up -d, and waits until GoEasyERP answers.
#
# $env:GOEASYERP_COMPOSE_URL takes another compose file, e.g. a release's:
#   https://raw.githubusercontent.com/hansjlachmann/goeasyerp/v0.1.153/docker-compose.local.yml
#
# Works in Windows PowerShell 5.1 and PowerShell 7. It never calls exit (that would close the
# window it is pasted into): errors are thrown and shown at the end.

function Install-GoEasyERP {
    $ErrorActionPreference = 'Stop'
    $composeUrl = 'https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml'
    if ($env:GOEASYERP_COMPOSE_URL) { $composeUrl = $env:GOEASYERP_COMPOSE_URL }
    $dir = 'C:\GoEasyERP'
    if ($env:GOEASYERP_DIR) { $dir = $env:GOEASYERP_DIR }
    # .env is read by docker compose: UTF-8 without BOM (PowerShell 5's UTF-8 writes a BOM, which
    # breaks the first line), LF line ends
    $utf8 = New-Object System.Text.UTF8Encoding $false

    # docker: runs docker with the arguments, its output on the screen; returns the exit code.
    # Docker writes progress to stderr, which must not stop the script.
    function Invoke-Docker {
        $old = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try { & docker @args 2>&1 | ForEach-Object { Write-Host $_ } } finally { $ErrorActionPreference = $old }
        return $LASTEXITCODE
    }
    # Get-Docker: runs docker and returns its output (stderr dropped), $null when it fails
    function Get-Docker {
        $old = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try { $out = & docker @args 2>$null } finally { $ErrorActionPreference = $old }
        if ($LASTEXITCODE -ne 0) { return $null }
        return ($out -join "`n").Trim()
    }

    # --- Docker -------------------------------------------------------------------------------
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw 'Docker is not installed. Install Docker Desktop (https://www.docker.com/products/docker-desktop/) with "Use WSL 2", start it and run this again.'
    }
    $osType = Get-Docker info --format '{{.OSType}}'
    if (-not $osType) {
        throw 'Docker does not answer. Start Docker Desktop, wait until it shows "Engine running", and run this again.'
    }
    if ($osType -ne 'linux') {
        throw "Docker runs $osType containers; GoEasyERP needs Linux containers: right-click the Docker icon in the taskbar, choose 'Switch to Linux containers', and run this again."
    }
    if (-not (Get-Docker compose version)) {
        throw '"docker compose" is missing: update Docker Desktop.'
    }

    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Set-Location $dir
    Write-Host "GoEasyERP folder: $dir"

    # --- Settings (.env) ----------------------------------------------------------------------
    $envFile = Join-Path $dir '.env'
    $composeFile = Join-Path $dir 'docker-compose.yml'
    $envValues = @{}
    if (Test-Path $envFile) {
        foreach ($line in Get-Content $envFile) {
            if ($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$') {
                # the key first: the next -match replaces $Matches
                $k = $Matches[1]
                $v = $Matches[2].Trim()
                if ($v -match "^'(.*)'$" -or $v -match '^"(.*)"$') { $v = $Matches[1] }
                $envValues[$k] = $v
            }
        }
    }
    # composeValue: KEY's value in an older docker-compose.yml (JWT_SECRET: x), $null when it is
    # a variable; $$ is a $ there
    $composeLines = @()
    if (Test-Path $composeFile) { $composeLines = Get-Content $composeFile }
    function Get-ComposeValue([string]$key) {
        foreach ($line in $composeLines) {
            if ($line -match "^\s*${key}:\s*(.*?)\s*$") {
                $v = $Matches[1]
                if ($v -match "^'(.*)'$" -or $v -match '^"(.*)"$') { $v = $Matches[1] }
                if ($v.Contains('${')) { return $null }
                return $v.Replace('$$', '$')
            }
        }
        return $null
    }

    $add = ''
    $secret = $envValues['JWT_SECRET']
    $dropSecret = $false
    if ($secret -eq 'replace-this-with-a-long-random-text') {
        # the example text of an older compose file: public, so anyone could sign in as any
        # user; the server no longer starts with it (a new one is made below)
        Write-Host 'Warning: JWT_SECRET was the example text of the compose file; a new one is made.' -ForegroundColor Yellow
        Write-Host '         Everyone logs in again; a password stored in SMTP Setup must be entered again.' -ForegroundColor Yellow
        $secret = $null
        $dropSecret = $true
    }
    if (-not $secret) {
        $secret = Get-ComposeValue 'JWT_SECRET'
        if ($secret -eq 'replace-this-with-a-long-random-text') {
            Write-Host 'Warning: JWT_SECRET of docker-compose.yml was its example text; a new one is made.' -ForegroundColor Yellow
            Write-Host '         A password stored in SMTP Setup must be entered again.' -ForegroundColor Yellow
            $secret = $null
        }
        if ($secret) {
            Write-Host 'Taking JWT_SECRET over from docker-compose.yml.'
        } else {
            if (-not $dropSecret -and (Get-Docker volume inspect goeasyerp_postgres_data)) {
                Write-Host 'Warning: a GoEasyERP database exists but its JWT_SECRET was not found; a new one is made.' -ForegroundColor Yellow
                Write-Host '         Passwords stored encrypted (SMTP Setup) must be entered again.' -ForegroundColor Yellow
            }
            $bytes = New-Object byte[] 32
            $rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
            $rng.GetBytes($bytes)
            $rng.Dispose()
            $secret = ($bytes | ForEach-Object { $_.ToString('x2') }) -join ''
        }
        if ($secret.Contains("'")) {
            throw "JWT_SECRET contains a ' - put it into $envFile yourself (JWT_SECRET=...) and run this again."
        }
        $add += "JWT_SECRET='$secret'`n"
    }
    if ($dropSecret) {
        # the old line goes: the new secret is added below
        $kept = [System.IO.File]::ReadAllLines($envFile) | Where-Object { $_ -notmatch '^\s*JWT_SECRET\s*=' }
        $text = ''
        if ($kept) { $text = (@($kept) -join "`n") + "`n" }
        [System.IO.File]::WriteAllText($envFile, $text, $utf8)
    }

    # The database password (S6). A new installation gets a random one. An installation made
    # before has the public "openerp": the new one is set in the database first (after the
    # backup below), and written to .env only once the database has it.
    $dbPassword = $null
    if (-not $envValues['POSTGRES_PASSWORD']) {
        $bytes = New-Object byte[] 24
        $rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
        $rng.GetBytes($bytes)
        $rng.Dispose()
        $dbPassword = ($bytes | ForEach-Object { $_.ToString('x2') }) -join ''
        if (-not (Get-Docker volume inspect goeasyerp_postgres_data)) {
            $add += "POSTGRES_PASSWORD='$dbPassword'`n"
            $dbPassword = $null
        }
    }

    # The setup token: the first user (the administrator) is made on the login page with it (S37)
    $setupToken = $envValues['SETUP_TOKEN']
    if (-not $setupToken) {
        $bytes = New-Object byte[] 16
        $rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
        $rng.GetBytes($bytes)
        $rng.Dispose()
        $setupToken = ($bytes | ForEach-Object { $_.ToString('x2') }) -join ''
        $add += "SETUP_TOKEN='$setupToken'`n"
    }

    # The app builder's own database password (it may read and change its build tables only)
    if (-not $envValues['BUILDER_DB_PASSWORD']) {
        $bytes = New-Object byte[] 24
        $rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
        $rng.GetBytes($bytes)
        $rng.Dispose()
        $add += "BUILDER_DB_PASSWORD='" + (($bytes | ForEach-Object { $_.ToString('x2') }) -join '') + "'`n"
    }

    $oldBind = $null
    $oldPort = $null
    foreach ($line in $composeLines) {
        if ($line -match '^\s*-\s*"?(([0-9.]*):)?([0-9]+):80"?') {
            $oldBind = $Matches[2]
            $oldPort = $Matches[3]
            break
        }
    }
    if (-not $envValues['GOEASYERP_PORT'] -and $oldPort -and $oldPort -ne '8000') {
        $add += "GOEASYERP_PORT=$oldPort`n"
    }
    if (-not $envValues['GOEASYERP_BIND'] -and $oldPort -and $oldBind -ne '127.0.0.1') {
        if (-not $oldBind) { $oldBind = '0.0.0.0' }
        $add += "GOEASYERP_BIND=$oldBind`n"
    }
    $oldOrigin = Get-ComposeValue 'ORIGIN'
    $defaultPort = '8000'
    if ($oldPort) { $defaultPort = $oldPort }
    if (-not $envValues['GOEASYERP_ORIGIN'] -and $oldOrigin -and $oldOrigin -ne "http://localhost:$defaultPort") {
        $add += "GOEASYERP_ORIGIN=$oldOrigin`n"
    }

    if (-not (Test-Path $envFile)) {
        $text = "# GoEasyERP settings, read by docker compose (docker-compose.yml in this folder).`n" +
            "# Keep JWT_SECRET: it also encrypts stored passwords (SMTP Setup); a new one makes them unreadable.`n" +
            "# POSTGRES_PASSWORD is the database's: to change it, change it in the database first.`n" +
            "# SETUP_TOKEN: asked on the login page when the first user (the administrator) is made.`n" +
            $add +
            "# The port GoEasyERP is opened on, and from where: 127.0.0.1 this computer only, 0.0.0.0 the network too`n" +
            "#GOEASYERP_PORT=8000`n" +
            "#GOEASYERP_BIND=127.0.0.1`n" +
            "# The address the users open, when it is not http://localhost:<port> (e.g. http://<computer name>:8000)`n" +
            "#GOEASYERP_ORIGIN=http://localhost:8000`n"
        [System.IO.File]::WriteAllText($envFile, $text, $utf8)
        Write-Host 'Settings written to .env.'
    } elseif ($add) {
        $existing = [System.IO.File]::ReadAllText($envFile)
        if ($existing.Length -gt 0 -and -not $existing.EndsWith("`n")) { $add = "`n" + $add }
        [System.IO.File]::AppendAllText($envFile, $add, $utf8)
        Write-Host 'Settings added to .env.'
    }

    # --- Backup before an upgrade -------------------------------------------------------------
    $upgrade = Test-Path $composeFile
    if ($upgrade -and (Get-Docker compose ps -q --status running db)) {
        New-Item -ItemType Directory -Force -Path (Join-Path $dir 'backups') | Out-Null
        $file = 'goeasyerp-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.dump'
        Write-Host "Backing the database up to backups\$file ..."
        # pg_dump writes the file inside the container and docker copies it out: PowerShell 5
        # would re-encode a dump sent through a pipe or redirection
        if ((Invoke-Docker compose exec -T db pg_dump -U openerp -Fc -f "/tmp/$file" openerp) -ne 0) {
            throw 'the backup failed; nothing was changed.'
        }
        if ((Invoke-Docker compose cp "db:/tmp/$file" "backups/$file") -ne 0) {
            throw 'the backup could not be copied; nothing was changed.'
        }
        Invoke-Docker compose exec -T db rm -f "/tmp/$file" | Out-Null
    }

    # --- The database's own password (an installation made before it had "openerp") -----------
    if ($dbPassword) {
        if (-not $upgrade) {
            throw "a GoEasyERP database exists, but there is no docker-compose.yml in $dir`: run this for the folder it was installed in, or put POSTGRES_PASSWORD=<its password> into .env."
        }
        Write-Host 'Giving the database a password of its own (it had the one everybody knows) ...'
        if ((Invoke-Docker compose up -d db) -ne 0) { throw 'the database does not start; nothing was changed.' }
        $ready = $false
        for ($i = 0; $i -lt 30; $i++) {
            if ((Invoke-Docker compose exec -T db pg_isready -U openerp -q) -eq 0) { $ready = $true; break }
            Start-Sleep -Seconds 2
        }
        if (-not $ready) { throw 'the database does not answer; nothing was changed.' }
        # the role the server works as (goeasyerp); an installation from before it: still openerp
        $role = Get-Docker compose exec -T db psql -U openerp -d openerp -tA -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'goeasyerp') THEN 'goeasyerp' ELSE 'openerp' END"
        if ($role -ne 'goeasyerp' -and $role -ne 'openerp') { throw 'the database could not be read; nothing was changed.' }
        if ((Invoke-Docker compose exec -T db psql -U openerp -d openerp -v ON_ERROR_STOP=1 -q -c "ALTER ROLE $role PASSWORD '$dbPassword'") -ne 0) {
            throw "the database's password could not be set; nothing was changed."
        }
        $line = "POSTGRES_PASSWORD='$dbPassword'`n"
        $existing = [System.IO.File]::ReadAllText($envFile)
        if ($existing.Length -gt 0 -and -not $existing.EndsWith("`n")) { $line = "`n" + $line }
        [System.IO.File]::AppendAllText($envFile, $line, $utf8)
        Write-Host 'The database has a new password (kept in .env).'
    }

    # --- The compose file ---------------------------------------------------------------------
    Write-Host "Downloading $composeUrl ..."
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $newFile = Join-Path $dir 'docker-compose.yml.new'
    Invoke-WebRequest -UseBasicParsing -Uri $composeUrl -OutFile $newFile
    if ((Invoke-Docker compose -f $newFile config -q) -ne 0) {
        Remove-Item $newFile
        throw 'the downloaded compose file does not work with these settings; nothing was changed.'
    }
    if ($upgrade) { Copy-Item $composeFile (Join-Path $dir 'docker-compose.yml.bak') -Force }
    Move-Item $newFile $composeFile -Force

    # --- Start --------------------------------------------------------------------------------
    if ((Invoke-Docker compose pull) -ne 0) { throw 'docker compose pull failed (no internet connection?).' }
    if ((Invoke-Docker compose up -d --remove-orphans) -ne 0) { throw 'docker compose up failed.' }

    $port = '8000'
    $origin = $null
    foreach ($line in Get-Content $envFile) {
        if ($line -match '^\s*GOEASYERP_PORT\s*=\s*(\S+)') { $port = $Matches[1].Trim("'`"") }
        if ($line -match '^\s*GOEASYERP_ORIGIN\s*=\s*(\S+)') { $origin = $Matches[1].Trim("'`"") }
    }
    if (-not $origin) { $origin = "http://localhost:$port" }
    Write-Host 'Waiting for GoEasyERP to start (the first start takes 2-3 minutes) ...'
    for ($i = 0; $i -lt 60; $i++) {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:$port/health" -TimeoutSec 5 | Out-Null
            if ($upgrade) {
                Write-Host "GoEasyERP is upgraded and running: open $origin" -ForegroundColor Green
            } else {
                Write-Host "GoEasyERP is running: open $origin and create the first user" -ForegroundColor Green
                Write-Host "with the setup token $setupToken (it is in .env too)." -ForegroundColor Green
            }
            return
        } catch {
            Start-Sleep -Seconds 5
        }
    }
    throw "GoEasyERP does not answer yet. See what it does with: cd $dir; docker compose logs backend"
}

$goeasyerpLocation = Get-Location
try {
    Install-GoEasyERP
} catch {
    Write-Host "GoEasyERP: $($_.Exception.Message)" -ForegroundColor Red
} finally {
    Set-Location $goeasyerpLocation
}
