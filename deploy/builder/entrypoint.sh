#!/bin/sh
# The app builder's start (S41): the builder runs as user 65532, not as root. Started as root
# (Docker Compose, a Kubernetes pod without runAsUser) this script first gives the builds volume
# to 65532 when something in it belongs to another user — a volume a builder that still ran as
# root filled — and then starts the builder as 65532 (su-exec: no root left in the process).
# Started as another user (Kubernetes runAsUser/fsGroup) it starts the builder as it is.
set -eu
builds="${APP_BUILDS_DIR:-/builds}"
if [ "$(id -u)" = "0" ]; then
	if [ -d "$builds" ] && [ -n "$(find "$builds" ! -user 65532 -print -quit)" ]; then
		echo "app-builder: giving $builds to user 65532"
		chown -R 65532:65532 "$builds"
	fi
	exec su-exec 65532:65532 app-builder "$@"
fi
exec app-builder "$@"
