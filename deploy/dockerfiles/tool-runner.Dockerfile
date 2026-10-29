FROM golang:1.24-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY pkg/gocommon ./pkg/gocommon
COPY internal/core/go.mod internal/core/go.sum* ./internal/core/
WORKDIR /build/internal/core
RUN go mod download
COPY internal/core/ .
RUN CGO_ENABLED=0 go build -o /tool-runner ./cmd/tool-runner

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /tool-runner /usr/local/bin/tool-runner
EXPOSE 18090
ENTRYPOINT ["/usr/local/bin/tool-runner"]
