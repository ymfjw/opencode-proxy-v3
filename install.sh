#!/bin/bash
set -e

# ==========================================
# OpenCode AI 网关全自动自适应一键安装脚本
# ==========================================

# 默认监听端口
PORT=${1:-8080}
# 您的 Github 仓库地址（如果是私有仓库，需要处理鉴权，这里假设是公开仓库）
# 部署前请替换此处为真实的用户名和仓库名
REPO="ymfjw/opencode-proxy-v3"

echo "=========================================="
echo "开始安装 OpenCode AI 网关 (自适应版)..."
echo "=========================================="

# 1. 检查并安装依赖
echo "[1/4] 检查系统依赖..."
if command -v apt-get >/dev/null; then
    apt-get update -y && apt-get install -y wget curl
elif command -v apk >/dev/null; then
    apk update && apk add wget curl
elif command -v yum >/dev/null; then
    yum install -y wget curl
fi

# 2. 下载单文件二进制程序
echo "[2/4] 从 Github 抓取最新版二进制..."
mkdir -p /app
# 借助 Github API 自动获取最新的 release 下载地址
LATEST_URL=$(curl -s https://api.github.com/repos/${REPO}/releases/latest | grep "browser_download_url" | grep "proxy_linux_amd64" | cut -d '"' -f 4)

if [ -z "$LATEST_URL" ]; then
    echo "获取最新版本失败！请确认 Github 仓库已公开发布 Release。"
    echo "您可以修改脚本填写硬编码的固定直链。"
    exit 1
fi

echo "正在下载: $LATEST_URL"
wget -O /app/proxy_linux_amd64 "$LATEST_URL"
chmod +x /app/proxy_linux_amd64

# 3. 探测服务管理器并注册守护进程
echo "[3/4] 探测系统环境并配置常驻后台..."

if command -v systemctl >/dev/null && [ -d /etc/systemd/system ]; then
    echo "探测到 Systemd (Ubuntu/Debian/CentOS)..."
    cat << EOF > /etc/systemd/system/opencode-proxy.service
[Unit]
Description=OpenCode API Proxy Gateway
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/app
Environment="PORT=${PORT}"
ExecStart=/app/proxy_linux_amd64
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable opencode-proxy
    systemctl restart opencode-proxy
    echo "服务已通过 Systemd 成功拉起！"

elif command -v rc-service >/dev/null && [ -d /etc/init.d ]; then
    echo "探测到 OpenRC (Alpine)..."
    cat << 'EOF' > /etc/init.d/opencode-proxy
#!/sbin/openrc-run
name="opencode-proxy"
description="OpenCode API Proxy Gateway"
command="/app/proxy_linux_amd64"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
output_log="/var/log/opencode-proxy.log"
error_log="/var/log/opencode-proxy.err"

depend() {
    need net
}

start_pre() {
EOF
    echo "    export PORT=${PORT}" >> /etc/init.d/opencode-proxy
    echo "}" >> /etc/init.d/opencode-proxy

    chmod +x /etc/init.d/opencode-proxy
    rc-update add opencode-proxy default
    rc-service opencode-proxy restart
    echo "服务已通过 OpenRC 成功拉起！"

else
    echo "未探测到常规的守护进程管理器 (可能是严苛受限的 LXC 容器)..."
    echo "降级采用 nohup 后台强行运行模式..."
    # 杀掉可能存在的老进程
    pkill -f proxy_linux_amd64 || true
    # 后台启动
    export PORT=${PORT}
    cd /app
    nohup /app/proxy_linux_amd64 > /var/log/opencode-proxy.log 2>&1 &
    
    # 尝试写入 crontab 做保活
    if command -v crontab >/dev/null; then
        (crontab -l 2>/dev/null | grep -v "proxy_linux_amd64"; echo "* * * * * pgrep -f proxy_linux_amd64 > /dev/null || (export PORT=${PORT} && cd /app && nohup /app/proxy_linux_amd64 > /var/log/opencode-proxy.log 2>&1 &)") | crontab -
        echo "已添加 crontab 守护规则 (每分钟检查存活)！"
    fi
    echo "服务已通过 nohup 成功拉起！"
fi

# 4. 竣工测试
echo "[4/4] 检查端口监听状态..."
sleep 2
if command -v netstat >/dev/null; then
    netstat -lntp | grep ${PORT} || echo "警告：端口可能未正常监听，请查看日志。"
else
    echo "netstat 不可用，请直接外部调用测试。"
fi

echo "=========================================="
echo "部署全部完成！🚀"
echo "您的内网监听端口为: ${PORT}"
echo "调用地址: http://<小鸡IP>:<外网穿透端口>/v1/chat/completions"
echo "=========================================="
