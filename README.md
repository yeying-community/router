<p align="right">
  <strong>中文</strong> | <a href="./README.en.md">English</a> | <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/yeying-community/router"><img src="https://raw.githubusercontent.com/yeying-community/router/main/web/public/logo.png" width="150" height="150" alt="router logo"></a>
</p>

<div align="center">

# Router

_通过标准的 OpenAI API 格式访问多家大模型服务，带管理后台，前端可内置到二进制中。_

</div>

## 快速开始（开发）

### 环境要求
- Go 1.22+
- Node.js 16+
- npm/yarn

### 启动后端
```bash
git clone https://github.com/yeying-community/router.git
cd router

cp .env.template .env   # 可选，按需配置

go mod download
go run ./cmd/router --log-dir ./logs
```
访问 http://localhost:3011 登录（默认管理员：用户名 `root`，密码 `123456`）。

### 编译后端
```bash
mkdir -p build
go build -o build/router ./cmd/router
```

### 启动前端热更新（可选）
```bash
npm install --prefix web
npm start --prefix web   # 自动代理到 http://localhost:3011
```
如需把前端打包进后端：`npm run build --prefix web` 后重启后端。

## 配置与文档
- 环境变量说明与示例：`.env.template`
- API 文档：`docs/API.md`

## 许可证
MIT License
