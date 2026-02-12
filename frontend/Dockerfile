# Stage 0: Build WASM
FROM rust:1.85-slim AS wasm-builder
RUN cargo install wasm-pack
WORKDIR /wasm
COPY wasm/ .
RUN wasm-pack build --target web --out-dir /wasm-out

# Stage 1: Build frontend
FROM node:20-alpine AS builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
COPY --from=wasm-builder /wasm-out ./public/wasm/
RUN npm run build

# Stage 2: Serve
FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
