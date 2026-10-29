# 构建阶段：从 GrayDraft/web/ 构建 Vue 前端
FROM node:22-alpine AS builder

WORKDIR /app

COPY web/package.json web/package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com && npm ci

COPY web/ ./
RUN npm run build

# 运行阶段：nginx 提供静态文件 + 反向代理到 graydraftd
FROM nginx:1.27-alpine

COPY web/nginx.docker.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
RUN chown -R nginx:nginx /usr/share/nginx/html

EXPOSE 80
