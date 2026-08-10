const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// 获取平台注入的 PORT 环境变量，默认 8080
const port = process.env.PORT || '8080';

// 动态匹配操作系统下的可执行文件名
const binaryName = process.platform === 'win32' ? 'proxy.exe' : 'proxy';
const binaryPath = path.join(__dirname, binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error(`[Error] 找不到 Go 代理二进制文件: ${binaryPath}`);
  console.error(`[提示] 部署前请确保已有编译好的 Linux 二进制文件 'proxy'。`);
  console.error(`[编译命令] CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o proxy main.go`);
  process.exit(1);
}

// 赋予 Linux 下可执行权限
if (process.platform !== 'win32') {
  try {
    fs.chmodSync(binaryPath, '755');
  } catch (err) {
    console.warn(`[Warning] 设置可执行权限失败: ${err.message}`);
  }
}

console.log(`[Node.js 引导器] 正在拉起 Go 代理网关主进程... 监听端口: ${port}`);

// 唤起后台 Go 二进制网关程序
const child = spawn(binaryPath, [], {
  env: {
    ...process.env,
    PORT: port,
    WORKERS: process.env.WORKERS || 'https://opencode.ai'
  },
  stdio: 'inherit'
});

child.on('error', (err) => {
  console.error(`[Node.js 引导器] 启动子进程失败:`, err);
});

child.on('close', (code) => {
  console.log(`[Node.js 引导器] 子进程已退出，退出代码: ${code}`);
  process.exit(code || 0);
});

// 监听进程终止信号
process.on('SIGINT', () => child.kill('SIGINT'));
process.on('SIGTERM', () => child.kill('SIGTERM'));
