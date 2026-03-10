# Claudestra build container
# Usage: docker build -t claudestra-builder . && docker run --rm -v ./out:/out claudestra-builder

FROM golang:1.23-bookworm AS builder

# System dependencies for Wails + webkit2gtk-4.1
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config \
    libgtk-3-dev libwebkit2gtk-4.1-dev \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Node.js 20 for frontend build
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Wails CLI
RUN go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

WORKDIR /src

# Cache Go dependencies
COPY gui/go.mod gui/go.sum ./gui/
RUN cd gui && go mod download

# Cache Node dependencies
COPY gui/frontend/package.json gui/frontend/package-lock.json ./gui/frontend/
RUN cd gui/frontend && npm ci

# Copy full source
COPY gui/ ./gui/

# Build GUI binary
RUN cd gui && wails build -tags webkit2_41 -o claudestra-gui

# Build CLI tool
RUN cd gui && go build -o /build/claudestra ./cmd/claudestra

RUN cp gui/build/bin/claudestra-gui /build/claudestra-gui

# Output stage — copy binaries to mounted /out
FROM debian:bookworm-slim
COPY --from=builder /build/claudestra-gui /build/claudestra /build/
CMD ["cp", "/build/claudestra-gui", "/build/claudestra", "/out/"]
