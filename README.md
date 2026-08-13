# 🚀 OpenCode Free 通用代理网关 (Multi-Platform Anti-429 Proxy)

[![GitHub](https://img.shields.io/badge/GitHub-ymfjw%2Fopencode--proxy--v3-blue.svg)](https://github.com/ymfjw/opencode-proxy-v3)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

本仓库提供支持 **全量防 429 频控、多维物理客户端指纹伪装、动态 UUIDv4 Session 隔离与精确 JSON 报文修剪** 的通用 OpenCode 代理网关。

---

## 🌟 核心防 429 与稳定性增强特性

1. **Chromium 物理客户端全维度标头伪装**：
   - 自动覆盖默认 `Go-http-client` 特征，伪装为最新桌面端 Chrome/Electron 物理标头。
   - 注入标准 `sec-ch-ua` (Client-Hints) 及 `sec-fetch-*` 跨域元数据，对抗上游高级风控引擎。
2. **动态 UUIDv4 请求隔离**：
   - 每一笔 API 请求均独立伪造 36 位随机 UUID 作为 `x-opencode-session-id` 与 `x-request-id`，打散单会话配额墙。
3. **精准响应拦截与尾部污染修剪**：
   - **非流式 (JSON)**：全量读取替换并重新精确计算 `Content-Length`，彻底解决客户端 `invalid character ':' after top-level value` 错误。
   - **流式 (SSE)**：真正无延迟字节流输出与余量缓冲区截断。
4. **支持 `hy3` 最新模型**：
   - 无缝重写映射至 `hy3-free`，并保持模型列表与流式还原。
5. **双活/多活 Worker 与 429 自动重启解封**：
   - 支持后端多 Worker 轮询代理与 429 自动触发刷新与自愈解封。

---

## 📦 各平台部署方式分类指南

### 分类一：全功能 Docker 容器类平台
> **适用平台**：`Railway`、`Northflank`、`Zerops`、`Appwrite (容器部署)`、`VPS`

1. **直接关联 GitHub 仓库**：连接 `ymfjw/opencode-proxy-v3` 仓库。
2. **构建方式**：选择 **Dockerfile**。
3. **端口设置**：`8080`（或随环境变量 `PORT` 自动绑定）。
4. **Zerops 平台专享**：仓库内根目录已内置 `zerops.yml` 配置文件，导入即可一键完成拉起！

---

### 分类二：Node.js 运行时 PaaS 平台
> **适用平台**：`Railway (Node 模式)`、`Zerops (Node 组建)`、`Render`、`Fly.io`、`Zeabur`

1. **构建方式**：选择 **Node.js Environment**。
2. **启动命令 (Start Command)**：`npm start`
3. **运行原理**：仓库内已内置预编译好的 Linux 64位无依赖二进制文件 `proxy` 与 `index.js` 引导器，启动后将由 Node.js `child_process` 无缝挂载并守护 Go 代理主进程。

---

### 分类三：Serverless & 边缘函数平台
> **适用平台**：`Vercel`、`Netlify`、`Tencent EdgeOne`

1. **Vercel**：已部署至专享仓库 [ymfjw/opencode-vercel](https://github.com/ymfjw/opencode-vercel.git)，全量 Serverless Function 入口位于 `api/index.go`。
2. **Netlify / EdgeOne**：连接 Vercel 仓库或导入 Serverless 代码，直接作为 Go 函数挂载运行。

---

## 🔑 客户端调用示例

- **接口地址**：`https://您的代理域名/v1`
- **默认 API Key**：`sk-mimo`
- **支持模型**：`hy3`、`deepseek-v4-flash`、`mimo-v2.5-pro`、`mimo-v2.5` 等
