#!/bin/bash

# 设置变量
SEQUENCER_URL="http://localhost:8123"
RPC_URL="http://localhost:8124"

# 定义获取区块高度的函数
get_block_height() {
    local url=$1
    local height=$(curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $url | grep -o '"result":"0x[^"]*"' | cut -d'"' -f4 | xargs printf "%d\n" 2>/dev/null)
    
    # 如果获取失败，返回-1
    if [ -z "$height" ]; then
        echo -1
    else
        echo $height
    fi
}

# 打印美观的标题
echo "╔════════════════════════╦═══════════════╦═══════════════╦═══════════════╗"
echo "║        时间戳          ║  Sequencer高度 ║    RPC高度     ║     差值      ║"
echo "╠════════════════════════╬═══════════════╬═══════════════╬═══════════════╣"

# 计数器，用于添加分隔线
COUNTER=0

# 监控循环
while true; do
    # 获取当前时间戳
    TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
    
    # 获取区块高度
    SEQ_HEIGHT=$(get_block_height $SEQUENCER_URL)
    RPC_HEIGHT=$(get_block_height $RPC_URL)
    
    # 计算差值
    if [ $SEQ_HEIGHT -ge 0 ] && [ $RPC_HEIGHT -ge 0 ]; then
        DIFF=$((SEQ_HEIGHT - RPC_HEIGHT))
        
        # 为差值添加视觉标记
        if [ $DIFF -gt 10 ]; then
            DIFF_DISPLAY="⚠️ $DIFF"
        elif [ $DIFF -gt 0 ]; then
            DIFF_DISPLAY="$DIFF"
        else
            DIFF_DISPLAY="$DIFF"
        fi
    else
        DIFF="N/A"
        DIFF_DISPLAY="N/A"
    fi
    
    # 格式化输出到控制台（美观的表格形式）
    printf "║ %-20s ║ %13s ║ %13s ║ %13s ║\n" "$TIMESTAMP" "$SEQ_HEIGHT" "$RPC_HEIGHT" "$DIFF_DISPLAY"
    
    # 增加计数器
    COUNTER=$((COUNTER + 1))
    
    # 每10条记录添加一个分隔线
    if [ $((COUNTER % 10)) -eq 0 ]; then
        echo "╟────────────────────────╫───────────────╫───────────────╫───────────────╢"
    fi
    
    # 等待3秒
    sleep 3
done

# 注意：这个脚本现在会在前台运行，按Ctrl+C可以停止