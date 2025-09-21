#!/bin/bash

# Go WebSocket 客户端构建脚本

echo "=== Go WebSocket 客户端构建脚本 ==="

# 设置Go环境
export PATH=$PATH:/usr/local/go/bin
export GOPROXY=https://goproxy.cn,direct

# 清理之前的构建
echo "清理之前的构建文件..."
rm -f ws-client ws-client-linux ws-client-windows.exe ws-client-darwin

# 构建当前平台版本
echo "构建当前平台版本..."
go build -o ws-client main.go
if [ $? -eq 0 ]; then
    echo "✅ 当前平台版本构建成功: ws-client"
    ls -lh ws-client
else
    echo "❌ 当前平台版本构建失败"
    exit 1
fi

# 构建Linux版本
echo "构建Linux版本..."
GOOS=linux GOARCH=amd64 go build -o ws-client-linux main.go
if [ $? -eq 0 ]; then
    echo "✅ Linux版本构建成功: ws-client-linux"
    ls -lh ws-client-linux
else
    echo "❌ Linux版本构建失败"
fi

# 构建Windows版本
echo "构建Windows版本..."
GOOS=windows GOARCH=amd64 go build -o ws-client-windows.exe main.go
if [ $? -eq 0 ]; then
    echo "✅ Windows版本构建成功: ws-client-windows.exe"
    ls -lh ws-client-windows.exe
else
    echo "❌ Windows版本构建失败"
fi

# 构建macOS版本
echo "构建macOS版本..."
GOOS=darwin GOARCH=amd64 go build -o ws-client-darwin main.go
if [ $? -eq 0 ]; then
    echo "✅ macOS版本构建成功: ws-client-darwin"
    ls -lh ws-client-darwin
else
    echo "❌ macOS版本构建失败"
fi

echo ""
echo "=== 构建完成 ==="
echo "可执行文件列表:"
ls -lh ws-client*

echo ""
echo "使用方法:"
echo "  ./ws-client -h                    # 查看帮助"
echo "  ./ws-client -clients 5 -log debug # 运行程序"
echo "  ./ws-client -url wss://example.com/ws -clients 10 # 指定URL和连接数"
