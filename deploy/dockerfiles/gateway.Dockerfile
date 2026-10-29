FROM golang:1.22-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY pkg/gocommon ./pkg/gocommon
COPY internal/gateway/go.mod internal/gateway/go.sum* ./internal/gateway/
WORKDIR /build/internal/gateway
RUN go mod download
COPY internal/gateway/ .
RUN CGO_ENABLED=0 go build -o /graydraft-gateway ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /graydraft-gateway .
COPY internal/gateway/config/config.dev.yaml ./config/config.dev.yaml
COPY internal/gateway/config/config.prod.yaml ./config/config.prod.yaml
ENV CONFIG_PATH=/app/config/config.dev.yaml
EXPOSE 18002
CMD ["./graydraft-gateway"]
