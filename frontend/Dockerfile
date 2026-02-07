# ── Stage 0: Build WASM Module ───────────────────────────────────────
FROM rust:1.83 AS wasm-builder

# Install wasm-pack
RUN curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh

# Set working directory for WASM build
WORKDIR /wasm

# Copy WASM source from project root context
COPY wasm/Cargo.toml wasm/Cargo.lock* ./
COPY wasm/src ./src

# Build WASM module for web target
# Note: wasm-pack outputs to ./pkg by default
RUN wasm-pack build --target web

# ── Stage 1: Build Frontend ───────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Install dependencies first (cache layer)
COPY frontend/package.json ./
RUN npm install --frozen-lockfile 2>/dev/null || npm install

# Copy source
COPY frontend/ .

# Copy WASM outputs from wasm-builder stage to public directory
COPY --from=wasm-builder /wasm/pkg ./public/wasm

# Build-time env vars (overridable)
ARG PUBLIC_API_URL=/api
ENV PUBLIC_API_URL=${PUBLIC_API_URL}

# Build static site (can now resolve /wasm/stocks_wasm.js)
RUN npm run build

# ── Stage 2: Serve ───────────────────────────────────────────────────
FROM nginx:1.27-alpine

# Remove default config
RUN rm /etc/nginx/conf.d/default.conf

# Custom nginx config with SPA routing
COPY --from=builder /app/dist /usr/share/nginx/html

# Fix: Copy root index.html to /app/index.html for SPA routing
RUN cp /usr/share/nginx/html/index.html /usr/share/nginx/html/app/index.html

# Copy nginx configuration
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
