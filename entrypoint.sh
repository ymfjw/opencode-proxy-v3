#!/bin/bash

# 确保所有后台进程退出时容器能正确处理
trap "exit" SIGINT SIGTERM

echo "Starting OpenCode API Gateway on port 3000..."
PORT=3000 /app/proxy &

echo "Starting NodeJS-Argo Tunnel on port 8081..."
cd /app/nodejs-argo
export PORT=8081
# eooce 或 yutian81 仓库的主脚本通常是 server.js 或 web.js
if [ -f "server.js" ]; then
    node server.js &
elif [ -f "web.js" ]; then
    node web.js &
else
    echo "NodeJS-Argo start script not found, trying index.js..."
    node index.js &
fi

echo "Starting Nginx Proxy Manager on port 8080..."
# 以非 daemon 模式启动 Nginx，挂住主进程，防止容器退出
nginx -g 'daemon off;'
