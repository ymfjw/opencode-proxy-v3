#!/bin/sh

rm -f /tmp/cloudflared.log /tmp/tunnel.url

echo "启动内置双模网关代理..."
# 启动 Go 进程（端口统一到 8080）
export PORT=8080
./proxy &

sleep 1

echo "启动 Cloudflare 隧道穿透，映射本地 8080 端口..."
touch /tmp/cloudflared.log

# Cloudflare 连接本地唯一的 8080 端口
cloudflared tunnel --url http://localhost:8080 > /tmp/cloudflared.log 2>&1 &

# 抓取链接写入 /tmp/tunnel.url
tail -f /tmp/cloudflared.log | grep -E -o 'https://[-a-zA-Z0-9]*\.trycloudflare\.com' -m 1 > /tmp/tunnel.url &

# 保持前台挂起
tail -f /tmp/cloudflared.log
