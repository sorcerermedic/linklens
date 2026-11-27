# syntax=docker/dockerfile:1

# ---- 1. Build the React dashboard -------------------------------------------
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 2. Build the Go binary with the dashboard embedded ---------------------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/linklens ./cmd/server

# ---- 3. Minimal runtime image ------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/linklens /linklens
ENV ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/linklens"]
