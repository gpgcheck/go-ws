#!/bin/bash

# 快速测试WebSocket压缩支持

echo "=== 快速压缩测试 ==="
echo

echo "1. 测试禁用压缩连接..."
WS_ENABLE_COMPRESSION=false timeout 5s ./ws-client -clients 1 -log debug 2>&1 | grep -E "(connected|compression|ERROR)" | head -5

echo
echo "2. 测试启用压缩连接..."
WS_ENABLE_COMPRESSION=true timeout 5s ./ws-client -clients 1 -log debug 2>&1 | grep -E "(connected|compression|ERROR)" | head -5

echo
echo "=== 测试完成 ==="
echo "如果看到 'compression: true' 说明服务器支持压缩"
echo "如果只看到 'compression: false' 说明服务器不支持压缩"
