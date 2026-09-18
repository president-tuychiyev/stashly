# Server is amd64 — the image must be amd64 even when built on an arm64 machine.
# Builder runs on the host arch (amd64 Go toolchain under QEMU segfaults on arm64)
# and cross-compiles a static amd64 binary.
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/main .

FROM --platform=linux/amd64 alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/main /app/main
COPY --from=builder /src/public /app/public

RUN mkdir -p /app/storage/app /app/storage/logs

ENV APP_ENV=production \
    APP_HOST=0.0.0.0 \
    APP_PORT=8807

EXPOSE 8807

CMD ["/app/main"]
