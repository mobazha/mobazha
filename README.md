# Mobazha 3.0

Mobazha 3.0 是一个去中心化的电子商务平台。

## 目录结构

```
/
├── cmd/                    # 命令行入口
│   ├── devnet.go          # 开发网络命令
│   ├── init.go            # 初始化命令
│   ├── start.go           # 启动命令
│   └── status.go          # 状态命令
├── internal/              # 内部包
│   ├── api/               # API接口
│   ├── channels/          # 通道相关
│   ├── common/            # 通用工具
│   ├── config/            # 配置
│   ├── contracts/         # 智能合约
│   ├── core/              # 核心业务逻辑
│   ├── database/          # 数据库操作
│   ├── events/            # 事件相关
│   ├── logger/            # 日志
│   ├── models/            # 数据模型
│   ├── multiwallet/       # 多币种钱包
│   ├── net/               # 网络相关
│   ├── notifications/     # 通知相关
│   ├── orders/            # 订单相关
│   ├── posts/             # 帖子相关
│   ├── repo/              # 仓库相关
│   ├── version/           # 版本相关
│   └── wallet/            # 钱包相关
├── pkg/                   # 可对外暴露的包
│   ├── onion-transport/   # 洋葱路由传输
│   ├── proxyclient/       # 代理客户端
│   └── store-and-forward/ # 存储转发
├── mobile/               # 移动端相关
├── dist/                 # 构建输出
└── scripts/              # 脚本文件

## 依赖项目

- [obcrawler](https://github.com/mobazha/obcrawler) - Mobazha 的爬虫服务

## 构建和运行

### 前置条件

- Go 1.22 或更高版本
- Git

### 构建

```bash
make build
```

### 运行

```bash
./mobazha start    # First run auto-initializes; opens Web UI at http://localhost:4002
```

后台运行：
```bash
mobazha service install   # Install as system service (systemd/launchd)
mobazha service status    # Check service status
```

## 开发

### 目录说明

- `cmd/`: 包含所有命令行入口
- `internal/`: 包含所有内部包，这些包不会被外部项目导入
- `pkg/`: 包含可以被外部项目导入的包
- `mobile/`: 包含移动端相关代码

### 测试

运行所有测试：
```bash
make test
```

## 许可证

[MIT License](LICENSE) 