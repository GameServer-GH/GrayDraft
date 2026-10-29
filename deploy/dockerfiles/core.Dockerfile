FROM golang:1.22-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY pkg/gocommon ./pkg/gocommon
COPY internal/core/go.mod internal/core/go.sum* ./internal/core/
WORKDIR /build/internal/core
RUN go mod download
COPY internal/core/ .
# Regenerate go.sum with new deps (e.g., otel) then build.
RUN go mod tidy && CGO_ENABLED=0 go build -o /graydraft-core ./cmd/gateway

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /graydraft-core .
COPY internal/core/config/config.dev.yaml ./config/config.dev.yaml
COPY internal/core/config/config.prod.yaml ./config/config.prod.yaml
COPY deploy/migrations ./migrations/
ENV CONFIG_PATH=/app/config/config.dev.yaml
EXPOSE 18080 19090
CMD ["./graydraft-core"]

