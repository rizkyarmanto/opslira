# Stage 1: build the runner binary
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go/go.mod ./
COPY go/main.go ./
COPY go/internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/runner .

# Stage 2: minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/runner /runner

ENTRYPOINT ["/runner"]
