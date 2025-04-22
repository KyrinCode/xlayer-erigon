#!/bin/bash

# 定义颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Limbo 测试启动脚本${NC}"
echo "========================================"

# 检查测试环境
echo -e "${YELLOW}[1/5] 检查测试环境...${NC}"
if ! docker ps | grep -q "xlayer-seq"; then
  echo -e "${RED}错误: xlayer-seq 容器未运行！${NC}"
  echo "请先启动测试环境: cd xlayer-erigon/test && make run"
  exit 1
fi

if ! docker ps | grep -q "xlayer-executor"; then
  echo -e "${RED}错误: xlayer-executor 容器未运行！${NC}"
  echo "请先启动测试环境: cd xlayer-erigon/test && make run"
  exit 1
fi

echo -e "${GREEN}✓ 测试环境已启动${NC}"

# 安装依赖
echo -e "${YELLOW}[2/5] 安装依赖...${NC}"
npm install
if [ $? -ne 0 ]; then
  echo -e "${RED}错误: 无法安装依赖${NC}"
  exit 1
fi
echo -e "${GREEN}✓ 依赖安装完成${NC}"

# 部署合约
echo -e "${YELLOW}[3/5] 部署 BlockhashDemo 合约并触发 Limbo...${NC}"
node deploy_with_prefunded.js
if [ $? -ne 0 ]; then
  echo -e "${RED}错误: 部署脚本执行失败${NC}"
  exit 1
fi

# 检查 Sequencer 日志
echo -e "${YELLOW}[4/5] 检查 Sequencer 日志中的 Limbo 状态...${NC}"
docker logs xlayer-seq --tail 100 | grep -E "Limbo Test|limbo|invalid batch" > limbo_logs.txt
if [ -s limbo_logs.txt ]; then
  echo -e "${GREEN}✓ 在 Sequencer 日志中找到 Limbo 相关信息:${NC}"
  cat limbo_logs.txt
else
  echo -e "${RED}警告: 在 Sequencer 日志中未找到 Limbo 相关信息${NC}"
fi

# 检查 Limbo 状态
echo -e "${YELLOW}[5/5] 通过 RPC 查询 Limbo 状态...${NC}"
curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"txpool_limbo","params":[],"id":1}' http://localhost:8123 > limbo_status.json
echo -e "${GREEN}✓ Limbo 状态查询结果:${NC}"
cat limbo_status.json

echo "========================================"
echo -e "${GREEN}测试完成!${NC}" 