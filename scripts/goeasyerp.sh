#!/bin/sh
# GoEasyERP on one computer: install, or upgrade an installation made before.
#
#   curl -fsSL https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/scripts/goeasyerp.sh | sh
#
# Or download it, read it and run it:  sh goeasyerp.sh [folder]
#
# What it does, in the folder (default ~/goeasyerp, or GOEASYERP_DIR / the first argument):
#   1. checks that Docker runs Linux containers and has "docker compose";
#   2. keeps the settings in .env — a new installation gets a random JWT_SECRET; an older
#      installation's docker-compose.yml gives its JWT_SECRET, port and ORIGIN (the secret also
#      encrypts stored passwords, so it must never change);
#   3. backs the database up into backups/ when GoEasyERP already runs;
#   4. downloads the latest docker-compose.yml (the old one stays as docker-compose.yml.bak);
#   5. docker compose pull, docker compose up -d, and waits until GoEasyERP answers.
#
# GOEASYERP_COMPOSE_URL takes another compose file, e.g. a release's:
#   https://raw.githubusercontent.com/hansjlachmann/goeasyerp/v0.1.153/docker-compose.local.yml
set -eu

COMPOSE_URL="${GOEASYERP_COMPOSE_URL:-https://raw.githubusercontent.com/hansjlachmann/goeasyerp/main/docker-compose.local.yml}"
say() { printf '%s\n' "$*"; }
fail() { printf 'GoEasyERP: %s\n' "$*" >&2; exit 1; }

# The whole script is one function, run at the end: piped into sh, nothing runs before all of it
# has arrived, and no command can read the rest of it from stdin.
main() {
DIR="${1:-${GOEASYERP_DIR:-$HOME/goeasyerp}}"

# --- Docker --------------------------------------------------------------------------------
command -v docker >/dev/null 2>&1 || fail "Docker is not installed. Install Docker Desktop, or on Linux: curl -fsSL https://get.docker.com | sudo sh"
if ! ostype=$(docker info --format '{{.OSType}}' 2>/dev/null); then
	fail "Docker does not answer. Start Docker Desktop (or the docker service); on Linux add yourself to the group docker: sudo usermod -aG docker \$USER, then log out and in again."
fi
[ "$ostype" = linux ] || fail "Docker runs $ostype containers; GoEasyERP needs Linux containers (Docker Desktop: right-click its icon, Switch to Linux containers)."
docker compose version >/dev/null 2>&1 || fail "\"docker compose\" is missing: install the Docker Compose plugin (docker-compose-plugin)."
command -v curl >/dev/null 2>&1 || fail "curl is not installed."

mkdir -p "$DIR"
cd "$DIR"
say "GoEasyERP folder: $(pwd)"

# --- Settings (.env) -----------------------------------------------------------------------
# envValue KEY: the value of KEY in .env (quotes removed), empty when missing
envValue() {
	[ -f .env ] || return 0
	sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" .env | tail -n 1 | sed "s/^'\(.*\)'\$/\1/; s/^\"\(.*\)\"\$/\1/"
}
# composeValue KEY: KEY's value in an older docker-compose.yml (JWT_SECRET: x), empty when it is
# a variable; $$ is a $ there
composeValue() {
	[ -f docker-compose.yml ] || return 0
	v=$(sed -n "s/^[[:space:]]*$1:[[:space:]]*//p" docker-compose.yml | head -n 1 | sed 's/[[:space:]]*$//' | sed "s/^'\(.*\)'\$/\1/; s/^\"\(.*\)\"\$/\1/")
	# shellcheck disable=SC2016 # a literal ${
	case "$v" in *'${'*) return 0 ;; esac
	printf '%s' "$v" | sed 's/\$\$/$/g'
}
# composePort: "127.0.0.1:8000:80" or "8000:80" of an older docker-compose.yml → bind and port
composePorts() {
	[ -f docker-compose.yml ] || return 0
	sed -n 's/^[[:space:]]*-[[:space:]]*"\{0,1\}\([0-9.]*:\)\{0,1\}\([0-9][0-9]*\):80"\{0,1\}.*$/\1 \2/p' docker-compose.yml | head -n 1
}

newEnv=
[ -f .env ] || newEnv=yes
add=

secret=$(envValue JWT_SECRET)
if [ -z "$secret" ]; then
	secret=$(composeValue JWT_SECRET)
	if [ -n "$secret" ]; then
		say "Taking JWT_SECRET over from docker-compose.yml."
	else
		if docker volume inspect goeasyerp_postgres_data >/dev/null 2>&1; then
			say "Warning: a GoEasyERP database exists but its JWT_SECRET was not found; a new one is made."
			say "         Passwords stored encrypted (SMTP Setup) must be entered again."
		fi
		secret=$(od -An -tx1 -N32 /dev/urandom | tr -d ' \n')
	fi
	case "$secret" in *"'"*) fail "JWT_SECRET contains a ' — put it into .env yourself (JWT_SECRET=...) and run this again." ;; esac
	add="${add}JWT_SECRET='$secret'
"
fi
if [ "$secret" = replace-this-with-a-long-random-text ]; then
	say "Warning: JWT_SECRET is still the example text of the compose file. It is kept, because it"
	say "         encrypts stored passwords; anyone who knows it can sign in tokens for this installation."
fi

ports=$(composePorts)
oldBind=$(printf '%s' "$ports" | cut -d' ' -f1 | tr -d ':')
oldPort=$(printf '%s' "$ports" | cut -s -d' ' -f2)
if [ -z "$(envValue GOEASYERP_PORT)" ] && [ -n "$oldPort" ] && [ "$oldPort" != 8000 ]; then
	add="${add}GOEASYERP_PORT=$oldPort
"
fi
if [ -z "$(envValue GOEASYERP_BIND)" ] && [ -n "$ports" ] && [ "$oldBind" != 127.0.0.1 ]; then
	add="${add}GOEASYERP_BIND=${oldBind:-0.0.0.0}
"
fi
oldOrigin=$(composeValue ORIGIN)
if [ -z "$(envValue GOEASYERP_ORIGIN)" ] && [ -n "$oldOrigin" ] && [ "$oldOrigin" != "http://localhost:${oldPort:-8000}" ]; then
	add="${add}GOEASYERP_ORIGIN=$oldOrigin
"
fi

if [ -n "$newEnv" ]; then
	umask 077
	cat >.env <<EOF
# GoEasyERP settings, read by docker compose (docker-compose.yml in this folder).
# Keep JWT_SECRET: it also encrypts stored passwords (SMTP Setup); a new one makes them unreadable.
${add}# The port GoEasyERP is opened on, and from where: 127.0.0.1 this computer only, 0.0.0.0 the network too
#GOEASYERP_PORT=8000
#GOEASYERP_BIND=127.0.0.1
# The address the users open, when it is not http://localhost:<port> (e.g. http://<computer name>:8000)
#GOEASYERP_ORIGIN=http://localhost:8000
EOF
	umask 022
	say "Settings written to .env."
elif [ -n "$add" ]; then
	printf '%s' "$add" >>.env
	say "Settings added to .env."
fi

# --- Backup before an upgrade --------------------------------------------------------------
if [ -f docker-compose.yml ] && [ -n "$(docker compose ps -q --status running db 2>/dev/null)" ]; then
	mkdir -p backups
	file="goeasyerp-$(date +%Y%m%d-%H%M%S).dump"
	say "Backing the database up to backups/$file ..."
	docker compose exec -T db pg_dump -U openerp -Fc -f "/tmp/$file" openerp </dev/null ||
		fail "the backup failed; nothing was changed."
	docker compose cp "db:/tmp/$file" "backups/$file" >/dev/null || fail "the backup could not be copied; nothing was changed."
	docker compose exec -T db rm -f "/tmp/$file" </dev/null || true
fi

upgrade=
[ -f docker-compose.yml ] && upgrade=yes

# --- The compose file ----------------------------------------------------------------------
say "Downloading $COMPOSE_URL ..."
curl -fsSL "$COMPOSE_URL" -o docker-compose.yml.new || fail "the download failed."
if ! docker compose -f docker-compose.yml.new config -q; then
	rm -f docker-compose.yml.new
	fail "the downloaded compose file does not work with these settings; nothing was changed."
fi
[ -f docker-compose.yml ] && cp docker-compose.yml docker-compose.yml.bak
mv docker-compose.yml.new docker-compose.yml

# --- Start ---------------------------------------------------------------------------------
docker compose pull
docker compose up -d --remove-orphans

port=$(envValue GOEASYERP_PORT)
port=${port:-8000}
origin=$(envValue GOEASYERP_ORIGIN)
origin=${origin:-http://localhost:$port}
say "Waiting for GoEasyERP to start (the first start takes 2-3 minutes) ..."
i=0
while [ $i -lt 60 ]; do
	if curl -fs -o /dev/null "http://localhost:$port/health"; then
		if [ -n "$upgrade" ]; then
			say "GoEasyERP is upgraded and running: open $origin"
		else
			say "GoEasyERP is running: open $origin and create the first user."
		fi
		exit 0
	fi
	i=$((i + 1))
	sleep 5
done
say "GoEasyERP does not answer yet. See what it does with: cd \"$(pwd)\" && docker compose logs backend"
exit 1
}

main "$@"
