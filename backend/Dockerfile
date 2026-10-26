# Stage 1: build the API binary. Module root is backend/, entrypoint cmd/api.
# go.mod requires go >= 1.26 (dependency floor), so the builder must match.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Stage 2: minimal runtime. iputils provides the `ping` binary used by ICMP
# probes — those probes open raw sockets, so the service needs
# `cap_add: [NET_RAW]` in docker-compose.yml (already set).
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata iputils
COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
