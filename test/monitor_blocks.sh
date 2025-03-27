#!/bin/bash

# 默认RPC端点
RPC_URL=${1:-"http://localhost:8545"}
echo "使用RPC端点: $RPC_URL"

# 初始化变量
last_block_number=""
last_timestamp=""

while true; do
  # 获取当前区块号
  current_block=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -H "Content-Type: application/json" $RPC_URL)
  current_block_number=$(echo $current_block | grep -o '"result":"[^"]*' | cut -d'"' -f4)
  
  if [ -z "$current_block_number" ]; then
    echo "无法获取区块号，请检查RPC连接"
    sleep 2
    continue
  fi
  
  # 如果区块号与上次相同，则继续
  if [ "$current_block_number" == "$last_block_number" ]; then
    echo "等待新区块... 当前区块: $current_block_number"
    sleep 1
    continue
  fi
  
  # 获取区块详情
  block_info=$(curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[\"$current_block_number\", false],\"id\":1}" -H "Content-Type: application/json" $RPC_URL)
  timestamp_hex=$(echo $block_info | grep -o '"timestamp":"[^"]*' | cut -d'"' -f4)
  
  if [ -z "$timestamp_hex" ]; then
    echo "无法获取区块时间戳，请检查RPC连接"
    sleep 2
    continue
  fi
  
  # 转换16进制时间戳为10进制
  timestamp=$((16#${timestamp_hex:2}))
  human_time=$(date -r $timestamp "+%Y-%m-%d %H:%M:%S")
  
  # 输出区块信息
  echo "区块号: $current_block_number, 时间戳: $timestamp, 时间: $human_time"
  
  # 如果有上一个区块的信息，计算间隔
  if [ ! -z "$last_timestamp" ]; then
    time_diff=$((timestamp - last_timestamp))
    echo "与上一区块时间间隔: $time_diff 秒"
    echo "-------------------------------------"
  fi
  
  # 更新上一个区块信息
  last_block_number=$current_block_number
  last_timestamp=$timestamp
  
  sleep 1
done 