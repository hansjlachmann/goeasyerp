# Build stage
FROM golang:1.24-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=linux go build -o extmerge ./tools/extmerge

# If EXTENSIONS_PATH is provided, merge the apps (their dependencies checked, in dependency
# order) into backend/definitions and their texts into translations, then generate the table
# code again from the merged definitions: extension fields become fields of the generated
# records, the server lists the apps under Installed Apps (backend/definitions/apps), and the
# apps' Go modules are linked in (go.work, cmd/api-server/apps_gen.go)
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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X github.com/hansjlachmann/goeasyerp/backend/api.Version=${VERSION}" -o api-server ./cmd/api-server

# Runtime stage
FROM alpine:3.21

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

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
