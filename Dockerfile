FROM golang:1.22-alpine AS builder

WORKDIR /app
RUN go env -w GOPROXY=https://goproxy.cn,direct
COPY main.go .
COPY public/ ./public/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o proxy main.go

FROM node:alpine

# 安装必要的系统工具和 Nginx
RUN apk add --no-cache nginx git curl bash

# 安装 RTK (Rust Token Killer) 到 /usr/local/bin
RUN curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh

# 克隆 NodeJS-Argo (这里使用较为通用的 eooce 版本或直接使用 yutian81 仓库)
WORKDIR /app
RUN git clone https://github.com/yutian81/nodejs-argo.git /app/nodejs-argo || \
    git clone https://github.com/eooce/nodejs-argo.git /app/nodejs-argo

# 安装 NodeJS-Argo 依赖
WORKDIR /app/nodejs-argo
RUN npm install

# 复制 Go 代理的二进制文件
WORKDIR /app
COPY --from=builder /app/proxy /app/proxy

# 复制 Nginx 配置文件、启动脚本和自愈脚本
COPY nginx.conf /etc/nginx/nginx.conf
COPY entrypoint.sh /app/entrypoint.sh
COPY restart_worker.sh /app/restart_worker.sh

# 复制真正的 OpenCodeFree 节点程序
COPY opencodefree /usr/local/bin/opencodefree
RUN chmod +x /app/entrypoint.sh /app/restart_worker.sh /usr/local/bin/opencodefree

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
