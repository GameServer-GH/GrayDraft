FROM golang:1.24-alpine AS builder

RUN go env -w GO111MODULE=on \
    && go env -w CGO_ENABLED=0 \
    && go env -w GOPROXY=https://goproxy.cn,direct

WORKDIR /src

COPY pkg/gocommon ./pkg/gocommon
COPY internal/graydraft/go.mod internal/graydraft/go.sum ./internal/graydraft/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd internal/graydraft && go mod download

COPY internal/graydraft/ ./internal/graydraft/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd internal/graydraft && go build -o /graydraftd ./cmd/graydraftd

FROM alpine:3.21

ENV TZ=Asia/Shanghai
RUN apk add --no-cache ca-certificates tzdata curl \
    && ln -sf /usr/share/zoneinfo/$TZ /etc/localtime

WORKDIR /app
COPY --from=builder /graydraftd .
RUN mkdir -p /app/config

ENV GRAYDRAFT_DATA_DIR=/data
RUN mkdir -p /data

EXPOSE 8787

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8787/health || exit 1

ENTRYPOINT ["/app/graydraftd"]
CMD ["-addr", ":8787", "-data", "/data"]
