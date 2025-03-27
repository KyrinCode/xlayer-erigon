const fs = require('fs');

// 读取交易结果数据
function readTxData() {
  try {
    const data = fs.readFileSync('./tx_results_extended.json', 'utf8');
    return JSON.parse(data);
  } catch (err) {
    console.error('读取tx_results_extended.json失败:', err.message);
    return [];
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

// 格式化Gas价格（Wei转Gwei）
function formatGasPrice(wei) {
  if (!wei) return '未知';
  return (BigInt(wei) / BigInt(1000000000)).toString() + ' Gwei';
}

// 生成交易详情概览表格
function generateOverviewTable(txData) {
  if (!txData || txData.length === 0) {
    console.log('没有找到交易数据');
    return;
  }

  // 表头
  console.log('| 序号 | Gas价格倍数 | 实际Gas价格 | 确认时间 | 区块号 | 区块内交易数 | 状态 |');
  console.log('|------|------------|-------------|----------|--------|--------------|------|');
  
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
    const blockNumber = tx.transactionDetails ? tx.transactionDetails.blockNumber : '未知';
    
    let txCountInBlock = '未知';
    if (tx.blockInfo && tx.blockInfo.transactionCount) {
      txCountInBlock = tx.blockInfo.transactionCount;
    }
    
    const status = tx.status || '未知';
    
    console.log(`| ${index} | ${multiplier}x | ${gasPrice} | ${confirmTime} | ${blockNumber} | ${txCountInBlock} | ${status} |`);
  });
}

// 生成按Gas价格倍数分组的确认时间分析
function generateGasPriceAnalysis(txData) {
  if (!txData || txData.length === 0) return;
  
  // 过滤成功交易
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length === 0) return;
  
  // 按Gas价格倍数分组
  const byMultiplier = {};
  successfulTxs.forEach(tx => {
    const multiplier = tx.gasPriceMultiplier;
    if (!byMultiplier[multiplier]) {
      byMultiplier[multiplier] = [];
    }
    byMultiplier[multiplier].push(tx);
  });
  
  console.log('\n## Gas价格与确认时间分析');
  console.log('\n| Gas价格倍数 | 实际Gas价格 | 平均确认时间 | 最短确认时间 | 最长确认时间 | 交易数 |');
  console.log('|------------|-------------|--------------|--------------|--------------|--------|');
  
  Object.keys(byMultiplier)
    .sort((a, b) => parseFloat(a) - parseFloat(b))
    .forEach(multiplier => {
      const txs = byMultiplier[multiplier];
      const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
      const minTime = Math.min(...txs.map(tx => tx.confirmationTime));
      const maxTime = Math.max(...txs.map(tx => tx.confirmationTime));
      
      // 获取Gas价格
      let gasPrice = '未知';
      if (txs[0].transactionDetails && txs[0].transactionDetails.gasPrice) {
        gasPrice = (BigInt(txs[0].transactionDetails.gasPrice) / BigInt(1000000000)).toString() + ' Gwei';
      }
      
      console.log(`| ${multiplier}x | ${gasPrice} | ${formatTime(avgTime)} | ${formatTime(minTime)} | ${formatTime(maxTime)} | ${txs.length} |`);
    });
}

// 生成交易池状态变化分析
function generatePoolChangeAnalysis(txData) {
  if (!txData || txData.length === 0) return;
  
  console.log('\n## 交易池状态变化详情');
  console.log('\n| Gas价格倍数 | 交易前待处理 | 交易确认后待处理 | 减少交易数 | 减少比例 | 确认时间 |');
  console.log('|------------|--------------|------------------|------------|----------|----------|');
  
  txData.forEach(tx => {
    if (!tx.beforePool || !tx.afterConfirmationPool || tx.status !== 'confirmed') return;
    
    const multiplier = tx.gasPriceMultiplier;
    const pendingBefore = tx.beforePool.pendingCount;
    const pendingAfter = tx.afterConfirmationPool.pendingCount;
    const pendingDiff = pendingBefore - pendingAfter;
    const pendingPct = ((pendingDiff / pendingBefore) * 100).toFixed(2) + '%';
    
    const confirmTime = formatTime(tx.confirmationTime);
    
    console.log(`| ${multiplier}x | ${formatPoolCount(pendingBefore)} | ${formatPoolCount(pendingAfter)} | ${formatPoolCount(pendingDiff)} | ${pendingPct} | ${confirmTime} |`);
  });
}

// 生成区块分析
function generateBlockAnalysis(txData) {
  if (!txData || txData.length === 0) return;
  
  // 过滤成功交易
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length === 0) return;
  
  // 按区块号分组
  const byBlock = {};
  successfulTxs.forEach(tx => {
    if (!tx.transactionDetails || !tx.blockInfo) return;
    
    const blockNumber = tx.transactionDetails.blockNumber;
    if (!byBlock[blockNumber]) {
      byBlock[blockNumber] = {
        timestamp: tx.blockInfo.timestamp,
        txCount: tx.blockInfo.transactionCount,
        transactions: []
      };
    }
    
    byBlock[blockNumber].transactions.push({
      index: tx.index,
      multiplier: tx.gasPriceMultiplier,
      confirmTime: tx.confirmationTime
    });
  });
  
  console.log('\n## 区块分析');
  console.log('\n| 区块号 | 区块时间 | 区块内交易数 | 包含我们的交易 |');
  console.log('|--------|----------|--------------|----------------|');
  
  Object.keys(byBlock)
    .sort((a, b) => parseInt(a) - parseInt(b))
    .forEach(blockNumber => {
      const block = byBlock[blockNumber];
      const blockTime = new Date(block.timestamp * 1000).toLocaleString();
      const ourTxs = block.transactions.map(tx => `#${tx.index} (${tx.multiplier}x)`).join(', ');
      
      console.log(`| ${blockNumber} | ${blockTime} | ${block.txCount} | ${ourTxs} |`);
    });
    
  // 计算区块间隔
  const blockNumbers = Object.keys(byBlock).map(Number).sort((a, b) => a - b);
  if (blockNumbers.length > 1) {
    console.log('\n### 区块间隔分析');
    console.log('\n| 起始区块 | 结束区块 | 区块数 | 时间差(秒) | 每区块平均时间(秒) |');
    console.log('|-----------|-----------|--------|------------|--------------------|\n');
    
    for (let i = 1; i < blockNumbers.length; i++) {
      const fromBlock = blockNumbers[i-1];
      const toBlock = blockNumbers[i];
      const blocks = toBlock - fromBlock;
      const fromTimestamp = byBlock[fromBlock].timestamp;
      const toTimestamp = byBlock[toBlock].timestamp;
      const timeDiff = toTimestamp - fromTimestamp;
      const avgBlockTime = (timeDiff / blocks).toFixed(2);
      
      console.log(`| ${fromBlock} | ${toBlock} | ${blocks} | ${timeDiff} | ${avgBlockTime} |`);
    }
    
    // 总体平均区块时间
    const firstBlock = blockNumbers[0];
    const lastBlock = blockNumbers[blockNumbers.length - 1];
    const totalBlocks = lastBlock - firstBlock;
    const totalTime = byBlock[lastBlock].timestamp - byBlock[firstBlock].timestamp;
    const overallAvgBlockTime = (totalTime / totalBlocks).toFixed(2);
    
    console.log(`\n**总体平均区块时间**: ${overallAvgBlockTime} 秒/区块`);
  }
}

// 生成基于Gas价格的效率分析
function generateEfficiencyAnalysis(txData) {
  if (!txData || txData.length === 0) return;
  
  // 过滤成功交易
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length === 0) return;
  
  // 按Gas价格倍数分组
  const byMultiplier = {};
  successfulTxs.forEach(tx => {
    const multiplier = tx.gasPriceMultiplier;
    if (!byMultiplier[multiplier]) {
      byMultiplier[multiplier] = [];
    }
    byMultiplier[multiplier].push(tx);
  });
  
  console.log('\n## Gas价格效率分析');
  console.log('\n| Gas价格倍数 | 平均确认时间 | 时间/价格比 | 区块平均交易数 | 减少交易池的效率 |');
  console.log('|------------|--------------|-------------|----------------|----------------|\n');
  
  const efficiencyData = [];
  
  Object.keys(byMultiplier)
    .forEach(multiplier => {
      const txs = byMultiplier[multiplier];
      const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
      const timePerPrice = (avgTime / 1000) / parseFloat(multiplier);
      
      // 计算区块平均交易数
      const avgBlockTxs = txs.reduce((acc, tx) => acc + (tx.blockInfo ? tx.blockInfo.transactionCount : 0), 0) / txs.length;
      
      // 计算交易池减少效率
      let avgPoolReduction = 0;
      const txsWithPoolData = txs.filter(tx => tx.beforePool && tx.afterConfirmationPool);
      if (txsWithPoolData.length > 0) {
        avgPoolReduction = txsWithPoolData.reduce((acc, tx) => {
          const diff = tx.beforePool.pendingCount - tx.afterConfirmationPool.pendingCount;
          return acc + diff;
        }, 0) / txsWithPoolData.length;
      }
      
      const poolEfficiency = avgPoolReduction / parseFloat(multiplier);
      
      efficiencyData.push({
        multiplier: parseFloat(multiplier),
        avgTime,
        timePerPrice,
        avgBlockTxs,
        avgPoolReduction,
        poolEfficiency
      });
    });
  
  // 按时间/价格比排序（升序）
  efficiencyData.sort((a, b) => a.multiplier - b.multiplier);
  
  efficiencyData.forEach(data => {
    console.log(`| ${data.multiplier}x | ${formatTime(data.avgTime)} | ${data.timePerPrice.toFixed(2)} | ${Math.round(data.avgBlockTxs).toLocaleString()} | ${data.poolEfficiency.toFixed(2)} |`);
  });
  
  // 找出最佳效率的选项
  const bestTimePerPrice = [...efficiencyData].sort((a, b) => a.timePerPrice - b.timePerPrice)[0];
  const bestPoolEfficiency = [...efficiencyData].sort((a, b) => b.poolEfficiency - a.poolEfficiency)[0];
  
  console.log('\n### 最佳Gas价格建议');
  console.log(`\n* **最佳时间效率**: ${bestTimePerPrice.multiplier}x (时间/价格比: ${bestTimePerPrice.timePerPrice.toFixed(2)})`);
  console.log(`* **最佳交易池效率**: ${bestPoolEfficiency.multiplier}x (每单位Gas价格减少交易池: ${bestPoolEfficiency.poolEfficiency.toFixed(2)})`);
}

// 生成总结性分析
function generateSummary(txData) {
  if (!txData || txData.length === 0) return;
  
  // 过滤成功交易
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length === 0) return;
  
  // 按Gas价格倍数分组
  const byMultiplier = {};
  successfulTxs.forEach(tx => {
    const multiplier = tx.gasPriceMultiplier;
    if (!byMultiplier[multiplier]) {
      byMultiplier[multiplier] = [];
    }
    byMultiplier[multiplier].push(tx);
  });
  
  // 计算各种分析指标
  const multiplierStats = Object.keys(byMultiplier).map(multiplier => {
    const txs = byMultiplier[multiplier];
    const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
    const minTime = Math.min(...txs.map(tx => tx.confirmationTime));
    const maxTime = Math.max(...txs.map(tx => tx.confirmationTime));
    
    // 计算交易池减少效率
    let avgPoolReduction = 0;
    const txsWithPoolData = txs.filter(tx => tx.beforePool && tx.afterConfirmationPool);
    if (txsWithPoolData.length > 0) {
      avgPoolReduction = txsWithPoolData.reduce((acc, tx) => {
        const diff = tx.beforePool.pendingCount - tx.afterConfirmationPool.pendingCount;
        return acc + diff;
      }, 0) / txsWithPoolData.length;
    }
    
    return {
      multiplier: parseFloat(multiplier),
      avgTime,
      minTime,
      maxTime,
      timePerPrice: (avgTime / 1000) / parseFloat(multiplier),
      avgPoolReduction,
      poolEfficiency: avgPoolReduction / parseFloat(multiplier),
      txCount: txs.length
    };
  });
  
  // 按倍数排序
  multiplierStats.sort((a, b) => a.multiplier - b.multiplier);
  
  // 找出各类最佳选项
  const fastestOption = [...multiplierStats].sort((a, b) => a.avgTime - b.avgTime)[0];
  const bestEfficiency = [...multiplierStats].sort((a, b) => a.timePerPrice - b.timePerPrice)[0];
  const bestPoolEfficiency = [...multiplierStats].sort((a, b) => b.poolEfficiency - a.poolEfficiency)[0];
  
  // 获取实验范围
  const firstTx = successfulTxs[0];
  const lastTx = successfulTxs[successfulTxs.length - 1];
  const startTime = new Date(firstTx.timestamp).toLocaleString();
  const endTime = new Date(lastTx.timestamp).toLocaleString();
  const experimentDuration = (new Date(lastTx.timestamp) - new Date(firstTx.timestamp)) / 1000;
  
  // 交易池状态变化
  let poolChange = {};
  if (firstTx.beforePool && lastTx.afterConfirmationPool) {
    const initialPending = firstTx.beforePool.pendingCount;
    const finalPending = lastTx.afterConfirmationPool.pendingCount;
    const pendingChange = finalPending - initialPending;
    const pendingPct = ((pendingChange / initialPending) * 100).toFixed(2);
    
    const initialTotal = firstTx.beforePool.totalCount;
    const finalTotal = lastTx.afterConfirmationPool.totalCount;
    const totalChange = finalTotal - initialTotal;
    const totalPct = ((totalChange / initialTotal) * 100).toFixed(2);
    
    poolChange = {
      initialPending,
      finalPending,
      pendingChange,
      pendingPct,
      initialTotal,
      finalTotal,
      totalChange,
      totalPct
    };
  }
  
  console.log('\n## 总结与建议');
  
  console.log('\n### 实验概要');
  console.log(`\n- **实验时间范围**: ${startTime} - ${endTime}`);
  console.log(`- **实验总时长**: ${experimentDuration.toFixed(0)}秒`);
  console.log(`- **交易总数**: ${txData.length}`);
  console.log(`- **成功交易数**: ${successfulTxs.length} (成功率: ${(successfulTxs.length / txData.length * 100).toFixed(2)}%)`);
  console.log(`- **测试的Gas价格倍数**: ${multiplierStats.map(s => s.multiplier + 'x').join(', ')}`);
  
  if (Object.keys(poolChange).length > 0) {
    console.log('\n### 交易池状态变化');
    console.log(`\n- **初始待处理交易**: ${formatPoolCount(poolChange.initialPending)}`);
    console.log(`- **最终待处理交易**: ${formatPoolCount(poolChange.finalPending)}`);
    console.log(`- **变化**: ${formatPoolCount(poolChange.pendingChange)} (${poolChange.pendingPct}%)`);
    console.log(`- **初始总交易**: ${formatPoolCount(poolChange.initialTotal)}`);
    console.log(`- **最终总交易**: ${formatPoolCount(poolChange.finalTotal)}`);
    console.log(`- **变化**: ${formatPoolCount(poolChange.totalChange)} (${poolChange.totalPct}%)`);
  }
  
  console.log('\n### 最佳Gas价格策略');
  console.log('\n| 策略类型 | 推荐Gas价格倍数 | 平均确认时间 | 说明 |');
  console.log('|----------|----------------|--------------|------|');
  console.log(`| 最快确认 | ${fastestOption.multiplier}x | ${formatTime(fastestOption.avgTime)} | 最短确认时间优先 |`);
  console.log(`| 最佳效率 | ${bestEfficiency.multiplier}x | ${formatTime(bestEfficiency.avgTime)} | 时间/价格比最优 |`);
  console.log(`| 交易池效率 | ${bestPoolEfficiency.multiplier}x | ${formatTime(bestPoolEfficiency.avgTime)} | 最有效清理交易池 |`);
  
  // 对不同场景的建议
  console.log('\n### 不同场景的推荐');
  
  // 排序后的选项，按确认时间排序
  const sortedByTime = [...multiplierStats].sort((a, b) => a.avgTime - b.avgTime);
  
  // 急速确认场景
  console.log(`\n- **急速确认**: 使用 ${sortedByTime[0].multiplier}x 倍Gas价格，预期确认时间约 ${formatTime(sortedByTime[0].avgTime)}`);
  
  // 经济型确认场景
  // 找出确认时间在可接受范围内(如15秒内)的最低价格选项
  const economicOptions = multiplierStats.filter(s => s.avgTime < 15000).sort((a, b) => a.multiplier - b.multiplier);
  if (economicOptions.length > 0) {
    console.log(`- **经济型**: 使用 ${economicOptions[0].multiplier}x 倍Gas价格，预期确认时间约 ${formatTime(economicOptions[0].avgTime)}`);
  }
  
  // 平衡型选项
  // 尝试找出时间/价格比较好，且价格适中的选项
  if (multiplierStats.length >= 3) {
    const balancedOptions = [...multiplierStats].sort((a, b) => a.timePerPrice - b.timePerPrice).slice(0, 3);
    const midPriceOption = balancedOptions.sort((a, b) => a.multiplier - b.multiplier)[Math.min(1, balancedOptions.length - 1)];
    console.log(`- **平衡型**: 使用 ${midPriceOption.multiplier}x 倍Gas价格，预期确认时间约 ${formatTime(midPriceOption.avgTime)}`);
  }
  
  // 发现的异常点
  const anomalies = [];
  for (let i = 0; i < multiplierStats.length; i++) {
    for (let j = 0; j < multiplierStats.length; j++) {
      if (multiplierStats[i].multiplier < multiplierStats[j].multiplier && 
          multiplierStats[i].avgTime < multiplierStats[j].avgTime) {
        anomalies.push({
          lower: multiplierStats[i],
          higher: multiplierStats[j]
        });
      }
    }
  }
  
  if (anomalies.length > 0) {
    console.log('\n### 发现的异常');
    console.log('\n以下情况中，较低的Gas价格反而获得了更快的确认时间:');
    
    // 找出最显著的几个异常
    anomalies
      .sort((a, b) => (b.higher.avgTime - b.lower.avgTime) - (a.higher.avgTime - a.lower.avgTime))
      .slice(0, 3)
      .forEach(anomaly => {
        const timeDiff = (anomaly.higher.avgTime - anomaly.lower.avgTime) / 1000;
        console.log(`- ${anomaly.lower.multiplier}x 比 ${anomaly.higher.multiplier}x 快 ${timeDiff.toFixed(2)}秒，尽管Gas价格较低`);
      });
    
    console.log('\n这可能是由于网络负载变化、区块生产者策略或统计偏差导致的。');
  }
}

// 主函数
function main() {
  console.log('正在读取交易数据...');
  const txData = readTxData();
  console.log(`找到 ${txData.length} 条交易记录\n`);
  
  // 重定向输出到字符串
  const originalConsoleLog = console.log;
  let output = '';
  console.log = function(message) {
    output += message + '\n';
    originalConsoleLog(message);
  };
  
  console.log('# XLayer交易实验详细分析报告');
  console.log(`\n生成时间: ${new Date().toLocaleString()}\n`);
  
  console.log('## 交易概览');
  generateOverviewTable(txData);
  
  generateGasPriceAnalysis(txData);
  
  generatePoolChangeAnalysis(txData);
  
  generateBlockAnalysis(txData);
  
  generateEfficiencyAnalysis(txData);
  
  generateSummary(txData);
  
  // 恢复原始的console.log
  console.log = originalConsoleLog;
  
  // 保存到文件
  fs.writeFileSync('./tx_extended_analysis.md', output);
  console.log('\n详细分析报告已保存到 tx_extended_analysis.md');
}

// 执行主函数
main(); 