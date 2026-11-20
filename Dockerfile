# Build stage — on the build machine's own platform, cross-compiling the program for the image's
# (TARGETOS/TARGETARCH): Go builds an arm64 program on amd64 natively, much faster than building
# under emulation; only the runtime stage below is per platform
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

# Version passed at build time
ARG VERSION=dev
# Optional: an extension app's directory (with its application.yaml), or a directory whose
# subdirectories are extension apps — merged into this build
ARG EXTENSIONS_PATH=""

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the extmerge tool
# (a tool of the build: for the build machine)
RUN CGO_ENABLED=0 go build -o extmerge ./tools/extmerge

# If EXTENSIONS_PATH is provided, merge the apps (their dependencies checked, in dependency
# order) into backend/definitions and their texts into translations, then generate the table
# code again from the merged definitions: extension fields become fields of the generated
# records, the server lists the apps under Installed Apps (backend/definitions/apps), and the
# apps' Go modules are linked in (go.work, backend/installedapps/apps_gen.go)
RUN if [ -n "$EXTENSIONS_PATH" ] && [ -d "$EXTENSIONS_PATH" ]; then \
      echo "Merging extension apps from $EXTENSIONS_PATH..."; \
      if [ -f "$EXTENSIONS_PATH/application.yaml" ]; then apps="--extensions $EXTENSIONS_PATH"; else apps="--apps $EXTENSIONS_PATH"; fi; \
      ./extmerge --core ./backend/definitions $apps --output /tmp/merged \
        --translations ./translations --translations-output /tmp/merged-translations --go-root . --verbose; \
      rm -rf ./backend/definitions ./translations; \
      mv /tmp/merged ./backend/definitions; \
      mv /tmp/merged-translations ./translations; \
      go generate ./backend/generated/tables; \
    else \
      echo "No extensions path provided, using the core definitions"; \
    fi

# Build the application with version embedded
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-X github.com/hansjlachmann/goeasyerp/backend/api.Version=${VERSION}" -o api-server ./cmd/api-server

# The installation's own builder of the extension apps (target app-builder, image
# goeasyerp/builder:<version>; App Build Setup: Builds By = Local Builder): the Go toolchain and
# this release's source, which cmd/app-builder copies for each build, merges the requested apps
# into and builds into a bundle in the builds volume. The build cache is filled here, so a build
# in the installation compiles little more than the apps: with the flags the builder compiles
# with (cmd/app-builder build.go: -trimpath, one compile thread for the table code) — other
# flags give other cache keys, and nothing filled here would be used. The builder adds this
# cache to its own in the builds volume (cache.go).
FROM golang:1.24-alpine AS app-builder

RUN apk add --no-cache git ca-certificates

ENV CGO_ENABLED=0
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /usr/local/bin/app-builder ./cmd/app-builder && \
    go build -trimpath -gcflags=github.com/hansjlachmann/goeasyerp/backend/generated/tables=-c=1 -o /dev/null ./cmd/api-server && \
    go build -o /dev/null ./tools/extmerge && \
    go build -o /dev/null ./tools/tablegen/main.go

ENV BASE_SOURCE=/src
ENV APP_BUILDS_DIR=/builds

ENTRYPOINT ["app-builder"]

# Runtime stage (the default target: the last stage)
FROM alpine:3.21

WORKDIR /app

# The CA certificates for HTTPS, from the build stage: no command runs in this stage, so the
# arm64 image is built without emulation
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy binary from builder
COPY --from=builder /app/api-server .

# Licence and third-party notices
COPY --from=builder /app/LICENSE /app/THIRD_PARTY_NOTICES ./

# Copy go.mod as marker for project root detection
COPY --from=builder /app/go.mod ./go.mod

# Copy translation files
COPY --from=builder /app/translations ./translations

# Copy the definitions (table, page and menu YAML, read at runtime)
COPY --from=builder /app/backend/definitions ./backend/definitions

# Expose port
EXPOSE 8080

# Set environment variables with defaults
ENV PORT=8080
ENV DB_HOST=""
ENV DB_PORT=5432
ENV DB_USER=openerp
ENV DB_PASSWORD=openerp
ENV DB_NAME=openerp

# Run the application
CMD ["./api-server"]
