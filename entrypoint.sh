#!/bin/bash

# 捕获云平台（如 Railway）注入的随机 PORT 变量，若未提供则默认 8080
LISTEN_PORT=${PORT:-8080}

# 确保所有后台进程退出时容器能正确处理
trap "exit" SIGINT SIGTERM

echo "Starting OpenCode API Gateway on internal port 3000..."
PORT=3000 /app/proxy &

echo "Starting NodeJS-Argo Tunnel on internal port 8081..."
cd /app/nodejs-argo
# 显式指定 Node 内部使用 8081 端口，防止侵占 LISTEN_PORT
PORT=8081 node server.js 2>/dev/null || PORT=8081 node web.js 2>/dev/null || PORT=8081 node index.js &

# 动态修改 Nginx 监听端口为云平台指定的 LISTEN_PORT
echo "Starting Nginx Proxy Manager on target port ${LISTEN_PORT}..."
sed -i "s/listen 8080;/listen ${LISTEN_PORT};/g" /etc/nginx/nginx.conf

# 以非 daemon 模式启动 Nginx
nginx -g 'daemon off;'
