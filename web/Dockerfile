# 构建阶段：与 GrayDraft_Go/deploy/docker-compose 联用时，仓库需为同级目录（见 deploy 注释）
FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

ARG VITE_API_BASE_URL=
ARG VITE_WS_URL=
ARG VITE_OSS_BASE_URL=
ARG VITE_JWT_KEY=
ARG VITE_DEFAULT_LOCALE=zh-CN
ARG VITE_LANGUAGE_PACK_HELP_URL=/language-pack-help.html

ENV VITE_API_BASE_URL=$VITE_API_BASE_URL \
    VITE_WS_URL=$VITE_WS_URL \
    VITE_OSS_BASE_URL=$VITE_OSS_BASE_URL \
    VITE_JWT_KEY=$VITE_JWT_KEY \
    VITE_DEFAULT_LOCALE=$VITE_DEFAULT_LOCALE \
    VITE_LANGUAGE_PACK_HELP_URL=$VITE_LANGUAGE_PACK_HELP_URL

RUN npm run build

FROM nginx:1.27-alpine

COPY nginx.docker.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
