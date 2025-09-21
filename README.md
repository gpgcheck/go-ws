# Go WebSocket 并发客户端

这是一个用Go语言编写的并发WebSocket客户端程序，可以同时连接到指定的WebSocket服务器。

## 功能特性

- 支持多个并发WebSocket连接
- 自动重连和错误处理
- 心跳保持连接活跃
- 实时消息接收和日志记录
- 优雅关闭和资源清理
- 可配置的连接数量
- **忽略消息模式**：只保持连接，不处理消息内容（节省CPU和内存）
- 内置默认配置，支持跨平台直接运行

## 安装依赖

```bash
go mod tidy
```

## 使用方法

### 环境变量配置

程序支持通过环境变量进行配置，优先级：命令行参数 > 环境变量 > 默认值

#### 环境变量说明
- `WS_CLIENTS`: 并发连接数量 (默认: 10)
- `WS_URL`: WebSocket服务器URL (默认: wss://example.com/v1/ws/public)
- `LOG_LEVEL`: 日志级别 (默认: error)
  - `error`: 只显示错误信息
  - `debug`: 显示所有日志信息（包括连接状态、接收消息等）
- `WS_RECONNECT`: 是否启用自动重连 (默认: true)
- `WS_MAX_RETRIES`: 最大重试次数 (默认: 5)
- `WS_RETRY_DELAY`: 重试延迟秒数 (默认: 5)
- `WS_IGNORE_MSG`: 是否忽略接收的消息 (默认: false)
  - `false`: 正常处理消息内容
  - `true`: 忽略消息内容，只保持连接（节省CPU和内存）

#### 使用环境变量
```bash
# 设置环境变量
export WS_CLIENTS=20
export WS_URL=wss://example.com/ws
export LOG_LEVEL=debug
export WS_RECONNECT=true
export WS_MAX_RETRIES=10
export WS_RETRY_DELAY=3

# 运行程序
go run main.go
```

#### 使用配置文件
```bash
# 复制配置文件
cp env.example .env

# 编辑配置文件
vim .env

# 直接运行程序（自动加载.env文件）
./ws-client
```

### 基本使用
```bash
# 使用默认配置
go run main.go

# 使用编译后的程序
./ws-client
```

### 自定义参数
```bash
# 指定连接数量
go run main.go -clients 20

# 指定WebSocket URL
go run main.go -url wss://example.com/ws

# 指定日志级别
go run main.go -log debug

# 指定重连配置
go run main.go -reconnect=true -retries=10 -delay=3

# 启用忽略消息模式（节省CPU和内存）
go run main.go -ignore-msg

# 组合使用
go run main.go -clients 50 -url wss://example.com/v1/ws/public -log debug -reconnect=true -retries=10

# 使用编译后的程序并传递参数
./ws-client -clients 30 -log debug -retries=5

# 忽略消息模式（只保持连接，不处理消息）
./ws-client -ignore-msg -clients 100
```

### 编译运行
```bash
# 编译
go build -o ws-client main.go

# 运行
./ws-client -clients 10

# 使用环境变量运行
WS_CLIENTS=20 WS_URL=wss://example.com/ws LOG_LEVEL=debug WS_RECONNECT=true WS_MAX_RETRIES=10 ./ws-client
```

## 配置方式

### 1. 命令行参数（最高优先级）
- `-clients`: 并发连接数量
- `-url`: WebSocket服务器URL
- `-log`: 日志级别 (error/debug)
- `-reconnect`: 是否启用自动重连 (true/false)
- `-retries`: 最大重试次数
- `-delay`: 重试延迟秒数
- `-ignore-msg`: 是否忽略接收的消息 (true/false)

### 2. 环境变量
- `WS_CLIENTS`: 并发连接数量
- `WS_URL`: WebSocket服务器URL
- `LOG_LEVEL`: 日志级别 (error/debug)
- `WS_RECONNECT`: 是否启用自动重连 (true/false)
- `WS_MAX_RETRIES`: 最大重试次数
- `WS_RETRY_DELAY`: 重试延迟秒数
- `WS_IGNORE_MSG`: 是否忽略接收的消息 (true/false)

### 3. 配置文件
- 创建 `.env` 文件（参考 `env.example` 示例）
- 程序自动加载 `.env` 文件

## 程序特性

1. **并发连接**: 使用goroutine同时建立多个WebSocket连接
2. **错误处理**: 完善的错误处理和日志记录
3. **心跳机制**: 每30秒发送ping消息保持连接
4. **优雅关闭**: 支持Ctrl+C优雅关闭所有连接
5. **状态监控**: 每10秒报告活跃连接数量
6. **消息处理**: 实时接收和显示服务器消息
7. **日志级别**: 支持error和debug两个日志级别
   - `error`: 只显示错误信息，适合生产环境
   - `debug`: 显示所有日志，包括连接状态和接收的消息
8. **自动重连**: 支持断线自动重连功能
   - 可配置是否启用重连
   - 可配置最大重试次数
   - 可配置重试延迟时间
   - 连接丢失时自动尝试重连
9. **忽略消息模式**: 优化资源使用的特殊模式
   - 只保持WebSocket连接活跃
   - 不处理接收的消息内容
   - 显著节省CPU和内存使用
   - 网络带宽使用不变
   - 适合压力测试和连接池场景

## 示例输出

### Error模式（默认）
```
# 只显示错误信息，无其他输出
```

### Debug模式
```
2024/01/01 10:00:00 [DEBUG] Starting 10 concurrent WebSocket connections to wss://example.com/v1/ws/public
2024/01/01 10:00:01 [DEBUG] Client 1 connected to wss://example.com/v1/ws/public
2024/01/01 10:00:01 [DEBUG] Client 2 connected to wss://example.com/v1/ws/public
2024/01/01 10:00:01 [DEBUG] Client 1 received: {"data":{"contractAddress":"0x...","tokenPrice":0.000004975700783639},"type":"bsc_token_ranking_price"}
...
2024/01/01 10:00:10 [DEBUG] Status: 10/10 active connections
2024/01/01 10:00:20 [DEBUG] Status: 10/10 active connections
```

### 重连示例
```
2024/01/01 10:05:00 [ERROR] Client 3 connection lost: websocket: close 1006 (abnormal closure): unexpected EOF
2024/01/01 10:05:00 [DEBUG] Client 3 attempting to reconnect (attempt 1/5)...
2024/01/01 10:05:05 [DEBUG] Client 3 reconnected successfully
2024/01/01 10:05:05 [DEBUG] Client 3 received: {"data":{"contractAddress":"0x...","tokenPrice":0.000004975700783639},"type":"bsc_token_ranking_price"}
```

### 忽略消息模式
```
=== WebSocket 并发客户端 ===
配置信息:
  并发连接数: 100
  WebSocket URL: wss://example.com/v1/ws/public
  日志级别: debug
  自动重连: true
  最大重试次数: 5
  重试延迟: 5秒
  忽略消息: true

2024/01/01 10:00:00 [DEBUG] Starting 100 concurrent WebSocket connections to wss://example.com/v1/ws/public
2024/01/01 10:00:01 [DEBUG] Client 1 connected to wss://example.com/v1/ws/public
2024/01/01 10:00:01 [DEBUG] Client 2 connected to wss://example.com/v1/ws/public
...
2024/01/01 10:00:10 [DEBUG] Status: 100/100 active connections
2024/01/01 10:00:20 [DEBUG] Status: 100/100 active connections
# 注意：忽略消息模式下不会显示接收到的消息内容，节省CPU和内存
```

## 注意事项

- 请确保目标WebSocket服务器支持并发连接
- 根据服务器性能调整并发连接数量
- 程序会自动处理连接断开和重连
- 使用Ctrl+C可以优雅关闭所有连接
