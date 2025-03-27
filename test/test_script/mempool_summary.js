const fs = require('fs');

// 读取交易结果数据
function readTxData() {
  try {
    // 首先尝试读取tx_results.json
    const data = fs.readFileSync('./tx_results.json', 'utf8');
    return JSON.parse(data);
  } catch (err) {
    console.error('读取tx_results.json失败:', err.message);
    
    // 尝试读取扩展版本的文件
    try {
      const data = fs.readFileSync('./tx_results_extended.json', 'utf8');
      return JSON.parse(data);
    } catch (err2) {
      console.error('读取替代结果文件失败:', err2.message);
      return [];
    }
  }
}

// 格式化交易池数据，使用千分位分隔符
function formatPoolCount(count) {
  return count ? count.toLocaleString() : '未知';
}

// 格式化时间（毫秒转换为秒）
function formatTime(ms) {
  return (ms / 1000).toFixed(2) + '秒';
}

// 生成简洁的交易详情表格，重点突出交易池数据
function generateSimpleTable(txData) {
  if (!txData || txData.length === 0) {
    console.log('没有找到交易数据');
    return;
  }

  // 表头 - 包含交易池信息
  console.log('| 序号 | Gas价格倍数 | 实际Gas价格 | 确认时间 | 交易池待处理 | 交易池总数 | 区块交易数 |');
  console.log('|------|------------|-------------|----------|--------------|------------|------------|');
  
  // 表格内容
  txData.forEach(tx => {
    const index = tx.index;
    const multiplier = tx.gasPriceMultiplier;
    
    // 获取Gas价格
    let gasPrice = '未知';
    if (tx.transactionDetails && tx.transactionDetails.gasPrice) {
      gasPrice = (BigInt(tx.transactionDetails.gasPrice) / BigInt(1000000000)).toString() + ' Gwei';
    }
    
    const confirmTime = tx.confirmationTime ? formatTime(tx.confirmationTime) : '未确认';
    
    // 交易池状态 - 使用交易前的状态
    const pendingBefore = formatPoolCount(tx.beforePool ? tx.beforePool.pendingCount : null);
    const totalBefore = formatPoolCount(tx.beforePool ? tx.beforePool.totalCount : null);
    
    // 区块中的交易数
    const txCountInBlock = tx.blockInfo ? tx.blockInfo.transactionCount : '未知';
    
    console.log(`| ${index} | ${multiplier}x | ${gasPrice} | ${confirmTime} | ${pendingBefore} | ${totalBefore} | ${txCountInBlock} |`);
  });
}

// 生成交易池状态变化分析
function generatePoolChangeAnalysis(txData) {
  if (!txData || txData.length === 0) {
    return;
  }

  console.log('\n## 交易池状态变化详情');
  console.log('\n| 序号 | Gas价格倍数 | 交易前待处理 | 交易确认后待处理 | 减少交易数 | 减少比例 | 确认时间 |');
  console.log('|------|------------|--------------|------------------|------------|----------|----------|');
  
  txData.forEach(tx => {
    const index = tx.index;
    const multiplier = tx.gasPriceMultiplier;
    
    if (tx.beforePool && tx.afterConfirmationPool) {
      const pendingBefore = tx.beforePool.pendingCount;
      const pendingAfter = tx.afterConfirmationPool.pendingCount;
      const pendingDiff = pendingBefore - pendingAfter;
      const pendingPct = ((pendingDiff / pendingBefore) * 100).toFixed(2) + '%';
      
      const confirmTime = tx.confirmationTime ? formatTime(tx.confirmationTime) : '未确认';
      
      console.log(`| ${index} | ${multiplier}x | ${formatPoolCount(pendingBefore)} | ${formatPoolCount(pendingAfter)} | ${formatPoolCount(pendingDiff)} | ${pendingPct} | ${confirmTime} |`);
    } else {
      console.log(`| ${index} | ${multiplier}x | 未知 | 未知 | 未知 | 未知 | 未知 |`);
    }
  });
}

// 创建总结图表
function generateSummaryTable(txData) {
  if (!txData || txData.length === 0) {
    return;
  }
  
  const firstTx = txData[0];
  const lastTx = txData[txData.length - 1];
  
  if (!firstTx.beforePool || !lastTx.afterConfirmationPool) {
    return;
  }
  
  // 初始状态和最终状态
  const initialPending = firstTx.beforePool.pendingCount;
  const finalPending = lastTx.afterConfirmationPool.pendingCount;
  const pendingChange = finalPending - initialPending;
  const pendingPct = ((pendingChange / initialPending) * 100).toFixed(2);
  
  const initialQueued = firstTx.beforePool.queuedCount;
  const finalQueued = lastTx.afterConfirmationPool.queuedCount;
  const queuedChange = finalQueued - initialQueued;
  const queuedPct = initialQueued ? ((queuedChange / initialQueued) * 100).toFixed(2) : '0.00';
  
  const initialTotal = firstTx.beforePool.totalCount;
  const finalTotal = lastTx.afterConfirmationPool.totalCount;
  const totalChange = finalTotal - initialTotal;
  const totalPct = ((totalChange / initialTotal) * 100).toFixed(2);
  
  // 获取时间范围
  const startTime = new Date(firstTx.timestamp).toLocaleString();
  const endTime = new Date(lastTx.timestamp).toLocaleString();
  
  const experimentDuration = (new Date(lastTx.timestamp) - new Date(firstTx.timestamp)) / 1000;
  const txCount = txData.length;
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed').length;
  
  console.log('\n## 交易池状态总结');
  console.log(`\n- 实验开始时间: ${startTime}`);
  console.log(`- 实验结束时间: ${endTime}`);
  console.log(`- 实验持续时间: ${experimentDuration.toFixed(0)}秒`);
  console.log(`- 交易总数: ${txCount}, 成功交易: ${successfulTxs}`);
  
  console.log('\n### 交易池状态变化总结');
  console.log('\n| 状态类型 | 初始值 | 最终值 | 变化数量 | 变化百分比 |');
  console.log('|----------|--------|--------|----------|------------|');
  console.log(`| 待处理交易 | ${formatPoolCount(initialPending)} | ${formatPoolCount(finalPending)} | ${formatPoolCount(pendingChange)} | ${pendingPct}% |`);
  console.log(`| 排队交易 | ${formatPoolCount(initialQueued)} | ${formatPoolCount(finalQueued)} | ${formatPoolCount(queuedChange)} | ${queuedPct}% |`);
  console.log(`| 总交易数 | ${formatPoolCount(initialTotal)} | ${formatPoolCount(finalTotal)} | ${formatPoolCount(totalChange)} | ${totalPct}% |`);
  
  // Gas价格与交易池减少关系统计
  console.log('\n### Gas价格与交易池变化效率');
  
  // 按Gas价格倍数分组
  const txByMultiplier = {};
  txData.forEach(tx => {
    if (tx.status !== 'confirmed') return;
    
    if (!txByMultiplier[tx.gasPriceMultiplier]) {
      txByMultiplier[tx.gasPriceMultiplier] = [];
    }
    txByMultiplier[tx.gasPriceMultiplier].push(tx);
  });
  
  console.log('\n| Gas价格倍数 | 平均确认时间 | 区块平均交易数 | 区块总交易数 | 单位气价效率 |');
  console.log('|------------|--------------|----------------|--------------|--------------|');
  
  Object.keys(txByMultiplier)
    .sort((a, b) => parseFloat(a) - parseFloat(b))
    .forEach(multiplier => {
      const txs = txByMultiplier[multiplier];
      
      // 计算平均确认时间
      const avgConfirmTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
      
      // 计算区块平均交易数和总交易数
      const totalBlockTxs = txs.reduce((acc, tx) => acc + (tx.blockInfo ? tx.blockInfo.transactionCount : 0), 0);
      const avgBlockTxs = totalBlockTxs / txs.length;
      
      // 计算单位气价效率（确认时间/气价倍数，越低越好）
      const efficiency = (avgConfirmTime / 1000) / parseFloat(multiplier);
      
      console.log(`| ${multiplier}x | ${formatTime(avgConfirmTime)} | ${Math.round(avgBlockTxs).toLocaleString()} | ${totalBlockTxs.toLocaleString()} | ${efficiency.toFixed(2)} |`);
    });
}

// 主函数
function main() {
  console.log('正在读取交易数据...');
  const txData = readTxData();
  console.log(`找到 ${txData.length} 条交易记录\n`);
  
  // 将输出重定向到字符串
  const originalConsoleLog = console.log;
  let output = '';
  console.log = function(message) {
    output += message + '\n';
    originalConsoleLog(message);
  };
  
  console.log('# XLayer交易与交易池状态分析');
  console.log('生成时间: ' + new Date().toLocaleString());
  
  console.log('\n## 交易详情表格（含交易池数据）');
  generateSimpleTable(txData);
  
  generatePoolChangeAnalysis(txData);
  
  generateSummaryTable(txData);
  
  // 恢复原始的console.log
  console.log = originalConsoleLog;
  
  // 保存到文件
  fs.writeFileSync('./mempool_summary.md', output);
  console.log('\n分析结果已保存到 mempool_summary.md');
}

// 执行主函数
main(); 