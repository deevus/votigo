# === Build Stage ===
FROM alpine:latest AS builder

# Install dependencies for mise and build
RUN apk add --no-cache bash curl git gcc musl-dev

# Install mise
RUN curl https://mise.run | sh
ENV PATH="/root/.local/bin:$PATH"

WORKDIR /app

# Copy dependency files first for better layer caching
COPY mise.toml go.mod go.sum ./

# Install tools (Go, Tailwind)
RUN mise trust && mise install

# Copy source code
COPY . .

# Build: compile CSS, vendor JS, build Go binary
RUN mise run css && mise run vendor && mise run build

# === Runtime Stage ===
FROM alpine:latest

# User requirement: bash for CLI access
RUN apk add --no-cache bash

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/votigo .

# Copy entrypoint script
COPY docker-entrypoint.sh .
RUN chmod +x docker-entrypoint.sh

# Create data directory for SQLite volume
RUN mkdir -p /data

# SQLite database volume
VOLUME /data

# Legacy server (HTML 4.01), Modern server (Tailwind + HTMX)
EXPOSE 8000 8001

ENTRYPOINT ["./docker-entrypoint.sh"]
