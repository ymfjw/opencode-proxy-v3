#!/bin/bash
PORT=$1
if [ -z "$PORT" ]; then
    echo "Usage: $0 <port>"
    exit 1
fi

echo "[AutoHeal] 正在重启占据 $PORT 端口的 opencodefree Worker..."
# 使用 pkill 精准杀死指定端口的进程 (兼容 Alpine 基础镜像)
pkill -f "opencodefree.*$PORT" 2>/dev/null || true
sleep 1

# 重新启动并放入后台
nohup /usr/local/bin/opencodefree --port $PORT >/dev/null 2>&1 &

echo "[AutoHeal] 重启指令已发送！"
