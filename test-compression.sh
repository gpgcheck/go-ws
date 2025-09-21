#!/bin/bash

# WebSocket压缩支持测试脚本
# 用于测试服务器端是否支持WebSocket压缩

echo "=== WebSocket压缩支持测试 ==="
echo

# 测试1: 禁用压缩连接
echo "测试1: 禁用压缩连接"
echo "命令: WS_ENABLE_COMPRESSION=false ./ws-client -clients 1 -log debug"
echo "预期结果: 应该能正常连接"
echo "----------------------------------------"
WS_ENABLE_COMPRESSION=false timeout 10s ./ws-client -clients 1 -log debug 2>&1 | head -20
echo
echo "----------------------------------------"
echo

# 测试2: 启用压缩连接
echo "测试2: 启用压缩连接"
echo "命令: WS_ENABLE_COMPRESSION=true ./ws-client -clients 1 -log debug"
echo "预期结果: 如果服务器支持压缩，会显示 (compression: true)"
echo "----------------------------------------"
WS_ENABLE_COMPRESSION=true timeout 10s ./ws-client -clients 1 -log debug 2>&1 | head -20
echo
echo "----------------------------------------"
echo

# 测试3: 检查连接日志
echo "测试3: 分析连接结果"
echo "查找压缩相关日志..."
echo

# 检查是否有压缩相关的日志
if WS_ENABLE_COMPRESSION=true timeout 5s ./ws-client -clients 1 -log debug 2>&1 | grep -q "compression: true"; then
    echo "✅ 服务器支持WebSocket压缩"
    echo "建议: 可以设置 WS_ENABLE_COMPRESSION=true 来节省带宽"
else
    echo "❌ 服务器不支持WebSocket压缩"
    echo "建议: 保持 WS_ENABLE_COMPRESSION=false 以获得更高带宽"
fi

echo
echo "=== 测试完成 ==="
