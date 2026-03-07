# --- Build frontend (static export) ---
FROM node:20-alpine AS node-builder
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# --- Build backend with embedded frontend ---
FROM golang:1.22-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
# Place the static export where the embed directive expects it
COPY --from=node-builder /build/out internal/frontend/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -tags embed_frontend -o /fabaid-server ./cmd/server

# --- Production image ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates postgresql16-client
WORKDIR /app
COPY --from=go-builder /fabaid-server .

ENV APP_ENV=production

EXPOSE 8080

CMD ["./fabaid-server"]
