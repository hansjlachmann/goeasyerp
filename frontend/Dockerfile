# Build stage — runs on the build machine's own architecture (BUILDPLATFORM), also for the
# arm64 image: the output is plain JavaScript. npm under QEMU emulation hung the arm64 release
# builds again and again (0.1.68, 0.1.82, 0.1.85, 0.1.89, 0.1.90), so no npm command may run in
# the target-architecture stage below.
FROM --platform=$BUILDPLATFORM node:22-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production dependencies only. They must stay plain JavaScript (today clsx, tailwind-merge):
# they are installed here for the build machine's architecture and copied into every image.
RUN npm ci --omit=dev

# Runtime stage (target architecture): only copies, no RUN
FROM node:22-alpine

WORKDIR /app

COPY --from=builder /app/build ./build
COPY --from=builder /app/package*.json ./
COPY --from=builder /app/node_modules ./node_modules

# Expose port
EXPOSE 3000

# Set environment variables
ENV PORT=3000
ENV ORIGIN=http://localhost:3000
ENV BODY_SIZE_LIMIT=5242880

# Not root (S41): the node image's own user; the server only reads its files
USER node

# Run the application
CMD ["node", "build"]
