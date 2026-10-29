FROM node:20-alpine AS frontend
WORKDIR /frontend
COPY internal/console/frontend/package.json ./
RUN npm config set registry https://registry.npmmirror.com && npm install
COPY internal/console/frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /build
COPY internal/console/backend/go.mod internal/console/backend/go.sum* ./
RUN go mod download
COPY internal/console/backend/ ./
RUN go mod tidy && CGO_ENABLED=0 go build -o /graydraft-console ./cmd

FROM alpine:3.20
WORKDIR /app
COPY --from=backend /graydraft-console .
COPY --from=frontend /frontend/dist ./static
COPY internal/console/backend/config/config.dev.yaml ./config/config.dev.yaml
COPY internal/console/backend/config/config.prod.yaml ./config/config.prod.yaml
ENV CONFIG_PATH=/app/config/config.dev.yaml
ENV STATIC_DIR=/app/static
ENV CORE_URL=http://core:18080
EXPOSE 13000
CMD ["./graydraft-console"]
