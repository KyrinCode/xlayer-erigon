const fs = require('fs');

// 配置
const CONFIG = {
  inputFile: './tx_results.json',
  reportFile: './gas_price_analysis.md',
  chartDataFile: './chart_data.js'
};

// 读取交易数据
function readTxData() {
  try {
    const data = fs.readFileSync(CONFIG.inputFile, 'utf8');
    return JSON.parse(data);
  } catch (err) {
    console.error('读取交易数据失败:', err.message);
    
    // 尝试读取其他可能的数据文件
    try {
      const data = fs.readFileSync('./tx_results_extended.json', 'utf8');
      return JSON.parse(data);
    } catch (err2) {
      console.error('读取扩展交易数据失败:', err2.message);
      return [];
    }
  }
}

// 分析交易数据
function analyzeData(txData) {
  // 过滤成功的交易
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length === 0) {
    return {
      success: false,
      message: '没有成功的交易记录'
    };
  }
  
  // 按Gas价格倍数分组
  const byMultiplier = {};
  successfulTxs.forEach(tx => {
    if (!byMultiplier[tx.gasPriceMultiplier]) {
      byMultiplier[tx.gasPriceMultiplier] = [];
    }
    byMultiplier[tx.gasPriceMultiplier].push(tx);
  });
  
  // 计算每组的统计数据
  const multiplierStats = Object.keys(byMultiplier)
    .sort((a, b) => parseFloat(a) - parseFloat(b))
    .map(multiplier => {
      const txs = byMultiplier[multiplier];
      const confirmTimes = txs.map(tx => tx.confirmationTime);
      
      // 确认时间统计
      const avgConfirmTime = confirmTimes.reduce((acc, time) => acc + time, 0) / confirmTimes.length;
      const minConfirmTime = Math.min(...confirmTimes);
      const maxConfirmTime = Math.max(...confirmTimes);
      const medianConfirmTime = calculateMedian(confirmTimes);
      
      // 计算交易费用
      const txFees = txs.map(tx => {
        if (tx.transactionDetails) {
          const gasPrice = BigInt(tx.transactionDetails.gasPrice);
          const gasUsed = BigInt(tx.transactionDetails.gasUsed);
          return Number(gasPrice * gasUsed);
        }
        return 0;
      });
      
      const avgTxFee = txFees.reduce((acc, fee) => acc + fee, 0) / txFees.length;
      
      // 实际Gas价格
      const actualGasPrice = txs[0].transactionDetails ? 
                            BigInt(txs[0].transactionDetails.gasPrice) / BigInt(1000000000) : 0;
      
      return {
        multiplier: parseFloat(multiplier),
        txCount: txs.length,
        confirmTimes: {
          avg: avgConfirmTime,
          min: minConfirmTime,
          max: maxConfirmTime,
          median: medianConfirmTime,
          raw: confirmTimes
        },
        fees: {
          avg: avgTxFee,
          avgInGwei: avgTxFee / 1e9
        },
        actualGasPrice,
        blockNumbers: txs.map(tx => tx.transactionDetails ? tx.transactionDetails.blockNumber : null),
        txsInBlock: txs.map(tx => tx.blockInfo ? tx.blockInfo.transactionCount : 0)
      };
    });
  
  // 获取交易池信息变化
  const firstTx = successfulTxs[0];
  const lastTx = successfulTxs[successfulTxs.length - 1];
  
  const poolInfo = {
    initial: firstTx.beforePool,
    final: lastTx.afterConfirmationPool,
    change: {
      pending: lastTx.afterConfirmationPool.pendingCount - firstTx.beforePool.pendingCount,
      queued: lastTx.afterConfirmationPool.queuedCount - firstTx.beforePool.queuedCount,
      total: lastTx.afterConfirmationPool.totalCount - firstTx.beforePool.totalCount
    }
  };
  
  // 区块信息分析
  const blockInfo = {};
  successfulTxs.forEach(tx => {
    if (tx.blockInfo) {
      const blockNumber = tx.transactionDetails.blockNumber;
      if (!blockInfo[blockNumber]) {
        blockInfo[blockNumber] = {
          timestamp: tx.blockInfo.timestamp,
          txCount: tx.blockInfo.transactionCount,
          ourTxs: []
        };
      }
      blockInfo[blockNumber].ourTxs.push({
        index: tx.index,
        multiplier: tx.gasPriceMultiplier,
        confirmTime: tx.confirmationTime
      });
    }
  });
  
  // 计算最佳Gas价格策略
  const bestByTime = [...multiplierStats].sort((a, b) => a.confirmTimes.avg - b.confirmTimes.avg);
  const bestByEfficiency = [...multiplierStats].map(stat => {
    return {
      ...stat,
      efficiency: stat.confirmTimes.avg / stat.multiplier // 时间/价格比率越低越好
    };
  }).sort((a, b) => a.efficiency - b.efficiency);
  
  return {
    success: true,
    txCount: txData.length,
    successfulTxCount: successfulTxs.length,
    multiplierStats,
    poolInfo,
    blockInfo,
    bestStrategies: {
      byTime: bestByTime[0],
      byEfficiency: bestByEfficiency[0]
    },
    allTxs: successfulTxs
  };
}

// 计算中位数
function calculateMedian(numbers) {
  const sorted = [...numbers].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  
  if (sorted.length % 2 === 0) {
    return (sorted[middle - 1] + sorted[middle]) / 2;
  }
  
  return sorted[middle];
}

// 格式化时间
function formatTime(ms) {
  return (ms / 1000).toFixed(2);
}

// 格式化Wei为Gwei
function weiToGwei(wei) {
  return (Number(wei) / 1e9).toFixed(2);
}

// 生成Markdown报告
function generateReport(analysis) {
  if (!analysis.success) {
    return `# Gas价格分析报告\n\n错误: ${analysis.message}\n`;
  }
  
  let report = `# XLayer Gas价格分析报告\n\n`;
  report += `生成时间: ${new Date().toLocaleString()}\n\n`;
  
  report += `## 实验概要\n\n`;
  report += `- 总交易数: ${analysis.txCount}\n`;
  report += `- 成功交易数: ${analysis.successfulTxCount}\n`;
  report += `- 成功率: ${(analysis.successfulTxCount / analysis.txCount * 100).toFixed(2)}%\n\n`;
  
  report += `## Gas价格与确认时间关系\n\n`;
  report += `| Gas价格倍数 | 实际Gas价格(Gwei) | 平均确认时间(秒) | 中位数确认时间(秒) | 最短确认时间(秒) | 最长确认时间(秒) | 交易数 |\n`;
  report += `|------------|-------------------|-----------------|-------------------|-----------------|-----------------|--------|\n`;
  
  analysis.multiplierStats.forEach(stat => {
    report += `| ${stat.multiplier}x | ${stat.actualGasPrice} | ${formatTime(stat.confirmTimes.avg)} | ${formatTime(stat.confirmTimes.median)} | ${formatTime(stat.confirmTimes.min)} | ${formatTime(stat.confirmTimes.max)} | ${stat.txCount} |\n`;
  });
  
  report += `\n### 确认时间与Gas价格的关系图表\n\n`;
  report += `![Gas价格与确认时间关系](./gas_price_vs_time.png)\n\n`;
  report += `*注: 可以使用chart_data.js中的数据生成上图*\n\n`;
  
  report += `## 最佳Gas价格策略\n\n`;
  
  // 最快确认时间策略
  const bestTimeStrategy = analysis.bestStrategies.byTime;
  report += `### 1. 最快确认策略\n\n`;
  report += `- **推荐Gas价格倍数**: ${bestTimeStrategy.multiplier}x (${bestTimeStrategy.actualGasPrice} Gwei)\n`;
  report += `- **预期确认时间**: ${formatTime(bestTimeStrategy.confirmTimes.avg)}秒\n`;
  report += `- **时间范围**: ${formatTime(bestTimeStrategy.confirmTimes.min)}-${formatTime(bestTimeStrategy.confirmTimes.max)}秒\n\n`;
  
  // 最佳效率策略
  const bestEfficiencyStrategy = analysis.bestStrategies.byEfficiency;
  report += `### 2. 最佳效率策略 (时间/价格比最优)\n\n`;
  report += `- **推荐Gas价格倍数**: ${bestEfficiencyStrategy.multiplier}x (${bestEfficiencyStrategy.actualGasPrice} Gwei)\n`;
  report += `- **预期确认时间**: ${formatTime(bestEfficiencyStrategy.confirmTimes.avg)}秒\n`;
  report += `- **效率指数**: ${bestEfficiencyStrategy.efficiency.toFixed(2)}\n\n`;
  
  // 不同场景的推荐
  report += `### 3. 不同场景推荐\n\n`;
  
  // 按确认时间排序
  const sortedByTime = [...analysis.multiplierStats].sort((a, b) => a.confirmTimes.avg - b.confirmTimes.avg);
  
  report += `- **急速确认 (最快)**: ${sortedByTime[0].multiplier}x (${sortedByTime[0].actualGasPrice} Gwei) - 预期 ${formatTime(sortedByTime[0].confirmTimes.avg)}秒\n`;
  
  // 找出中间效率的选项
  const mediumIndex = Math.floor(sortedByTime.length / 2);
  if (sortedByTime.length > 2) {
    report += `- **平衡速度/成本**: ${sortedByTime[mediumIndex].multiplier}x (${sortedByTime[mediumIndex].actualGasPrice} Gwei) - 预期 ${formatTime(sortedByTime[mediumIndex].confirmTimes.avg)}秒\n`;
  }
  
  // 最低成本但仍然合理时间的选项
  const reasonableOptions = sortedByTime.filter(opt => opt.confirmTimes.avg < 15000); // 15秒以内
  if (reasonableOptions.length > 0) {
    const lowestCost = reasonableOptions.sort((a, b) => a.multiplier - b.multiplier)[0];
    report += `- **经济型 (合理时间内最低成本)**: ${lowestCost.multiplier}x (${lowestCost.actualGasPrice} Gwei) - 预期 ${formatTime(lowestCost.confirmTimes.avg)}秒\n\n`;
  }
  
  // 异常分析 - 是否有Gas价格更高但确认更慢的情况
  const anomalies = [];
  for (let i = 0; i < analysis.multiplierStats.length; i++) {
    for (let j = 0; j < analysis.multiplierStats.length; j++) {
      const statA = analysis.multiplierStats[i];
      const statB = analysis.multiplierStats[j];
      
      if (statA.multiplier < statB.multiplier && 
          statA.confirmTimes.avg < statB.confirmTimes.avg) {
        anomalies.push({
          lower: statA,
          higher: statB
        });
      }
    }
  }
  
  if (anomalies.length > 0) {
    report += `## Gas价格异常分析\n\n`;
    report += `在测试中发现一些较低的Gas价格反而获得了更快的确认时间，这可能是由网络负载、矿工策略或交易池状态变化导致的。\n\n`;
    report += `### 价格/时间异常\n\n`;
    report += `| 低Gas倍数 | 低Gas时间(秒) | 高Gas倍数 | 高Gas时间(秒) | 时间差(秒) |\n`;
    report += `|-----------|---------------|-----------|---------------|------------|\n`;
    
    // 只显示前5个最显著的异常
    anomalies
      .sort((a, b) => (b.higher.confirmTimes.avg - b.lower.confirmTimes.avg) - (a.higher.confirmTimes.avg - a.lower.confirmTimes.avg))
      .slice(0, 5)
      .forEach(anomaly => {
        const timeDiff = (anomaly.higher.confirmTimes.avg - anomaly.lower.confirmTimes.avg) / 1000;
        report += `| ${anomaly.lower.multiplier}x (${anomaly.lower.actualGasPrice} Gwei) | ${formatTime(anomaly.lower.confirmTimes.avg)} | ${anomaly.higher.multiplier}x (${anomaly.higher.actualGasPrice} Gwei) | ${formatTime(anomaly.higher.confirmTimes.avg)} | ${timeDiff.toFixed(2)} |\n`;
      });
    
    report += `\n可能的原因：\n`;
    report += `- 区块生产者在不同时间点的负载变化\n`;
    report += `- 交易池拥堵状态的变化\n`;
    report += `- 交易被打包时的网络状态差异\n`;
    report += `- 小样本量导致的统计偏差\n\n`;
  }
  
  report += `## 交易池状态分析\n\n`;
  const poolStartTime = new Date(analysis.allTxs[0].timestamp).toLocaleString();
  const poolEndTime = new Date(analysis.allTxs[analysis.allTxs.length - 1].timestamp).toLocaleString();
  
  report += `### 交易池状态变化 (${poolStartTime} → ${poolEndTime})\n\n`;
  report += `| 状态 | 初始值 | 最终值 | 变化数量 | 变化百分比 |\n`;
  report += `|------|--------|--------|----------|------------|\n`;
  
  const pendingChange = analysis.poolInfo.change.pending;
  const pendingPct = (pendingChange / analysis.poolInfo.initial.pendingCount * 100).toFixed(2);
  report += `| 待处理交易 | ${analysis.poolInfo.initial.pendingCount} | ${analysis.poolInfo.final.pendingCount} | ${pendingChange} | ${pendingPct}% |\n`;
  
  const queuedChange = analysis.poolInfo.change.queued;
  const queuedPct = (queuedChange / analysis.poolInfo.initial.queuedCount * 100).toFixed(2);
  report += `| 排队交易 | ${analysis.poolInfo.initial.queuedCount} | ${analysis.poolInfo.final.queuedCount} | ${queuedChange} | ${queuedPct}% |\n`;
  
  const totalChange = analysis.poolInfo.change.total;
  const totalPct = (totalChange / analysis.poolInfo.initial.totalCount * 100).toFixed(2);
  report += `| 总交易数 | ${analysis.poolInfo.initial.totalCount} | ${analysis.poolInfo.final.totalCount} | ${totalChange} | ${totalPct}% |\n\n`;
  
  // 区块分析
  report += `## 区块分析\n\n`;
  report += `### 交易所在区块详情\n\n`;
  report += `| 区块号 | 区块时间 | 区块内交易数 | 包含我们的交易 |\n`;
  report += `|--------|----------|--------------|----------------|\n`;
  
  Object.keys(analysis.blockInfo)
    .sort((a, b) => parseInt(a) - parseInt(b))
    .forEach(blockNumber => {
      const block = analysis.blockInfo[blockNumber];
      const blockTime = new Date(block.timestamp * 1000).toLocaleString();
      report += `| ${blockNumber} | ${blockTime} | ${block.txCount} | ${block.ourTxs.map(tx => `#${tx.index} (${tx.multiplier}x)`).join(', ')} |\n`;
    });
  
  report += `\n## 结论与建议\n\n`;
  
  // 最终建议
  report += `根据测试结果，我们可以得出以下结论：\n\n`;
  
  // 基于按时间排序的结果给出建议
  report += `1. **最佳Gas价格策略**：\n`;
  report += `   - 如果追求最快确认速度，推荐使用 **${sortedByTime[0].multiplier}x** (${sortedByTime[0].actualGasPrice} Gwei) 的Gas价格，预期确认时间约 ${formatTime(sortedByTime[0].confirmTimes.avg)}秒\n`;
  
  if (bestEfficiencyStrategy.multiplier !== sortedByTime[0].multiplier) {
    report += `   - 如果考虑成本效益平衡，推荐使用 **${bestEfficiencyStrategy.multiplier}x** (${bestEfficiencyStrategy.actualGasPrice} Gwei) 的Gas价格，预期确认时间约 ${formatTime(bestEfficiencyStrategy.confirmTimes.avg)}秒\n`;
  }
  
  // 总结网络状况
  report += `\n2. **网络状况**：\n`;
  report += `   - 当前XLayer网络基础Gas价格为 ${weiToGwei(analysis.allTxs[0].beforePool.gasPrice)} Gwei\n`;
  report += `   - 交易池拥堵状态：${analysis.poolInfo.initial.totalCount > 100000 ? '严重拥堵' : (analysis.poolInfo.initial.totalCount > 10000 ? '中度拥堵' : '轻度拥堵')}\n`;
  
  // 最后的观察
  report += `\n3. **其他观察**：\n`;
  
  // 检查确认时间和Gas价格是否存在明显的相关性
  const correlationDetected = checkCorrelation(analysis.multiplierStats);
  if (correlationDetected) {
    report += `   - Gas价格与确认时间呈${correlationDetected.type}相关，提高Gas价格能${correlationDetected.effect}提高交易确认速度\n`;
  } else {
    report += `   - 在测试范围内，Gas价格与确认时间没有显示出强相关性，可能受网络状况和区块生产影响较大\n`;
  }
  
  // 计算平均区块时间
  const blockNumbers = Object.keys(analysis.blockInfo).map(n => parseInt(n)).sort();
  let avgBlockTime = 0;
  if (blockNumbers.length > 1) {
    const firstBlock = analysis.blockInfo[blockNumbers[0]];
    const lastBlock = analysis.blockInfo[blockNumbers[blockNumbers.length - 1]];
    const timeDiff = lastBlock.timestamp - firstBlock.timestamp;
    const blockDiff = blockNumbers[blockNumbers.length - 1] - blockNumbers[0];
    avgBlockTime = timeDiff / blockDiff;
    
    report += `   - 测试期间平均区块时间约为 ${avgBlockTime.toFixed(2)}秒\n`;
  }
  
  return report;
}

// 检查Gas价格和确认时间的相关性
function checkCorrelation(multiplierStats) {
  if (multiplierStats.length < 3) return null;
  
  // 准备数据点
  const dataPoints = multiplierStats.map(stat => ({
    multiplier: stat.multiplier,
    time: stat.confirmTimes.avg
  }));
  
  // 计算相关系数
  const n = dataPoints.length;
  const sumX = dataPoints.reduce((acc, point) => acc + point.multiplier, 0);
  const sumY = dataPoints.reduce((acc, point) => acc + point.time, 0);
  const sumXY = dataPoints.reduce((acc, point) => acc + point.multiplier * point.time, 0);
  const sumX2 = dataPoints.reduce((acc, point) => acc + point.multiplier * point.multiplier, 0);
  const sumY2 = dataPoints.reduce((acc, point) => acc + point.time * point.time, 0);
  
  const r = (n * sumXY - sumX * sumY) / 
            Math.sqrt((n * sumX2 - sumX * sumX) * (n * sumY2 - sumY * sumY));
  
  // 解释相关系数
  if (r < -0.5) {
    return { type: '明显负', effect: '显著' };
  } else if (r < -0.3) {
    return { type: '中度负', effect: '适度' };
  } else if (r > 0.5) {
    return { type: '明显正', effect: '反而降低' }; // 这是异常情况，通常高Gas应该更快
  } else if (r > 0.3) {
    return { type: '中度正', effect: '反而轻微降低' };
  }
  
  return null; // 无明显相关性
}

// 生成图表数据
function generateChartData(analysis) {
  const chartData = {
    // 确认时间与Gas价格对比图表数据
    confirmationTime: {
      labels: analysis.multiplierStats.map(stat => `${stat.multiplier}x`),
      datasets: [
        {
          label: '平均确认时间(秒)',
          data: analysis.multiplierStats.map(stat => formatTime(stat.confirmTimes.avg))
        },
        {
          label: '最短确认时间(秒)',
          data: analysis.multiplierStats.map(stat => formatTime(stat.confirmTimes.min))
        },
        {
          label: '最长确认时间(秒)',
          data: analysis.multiplierStats.map(stat => formatTime(stat.confirmTimes.max))
        }
      ]
    },
    
    // 实际Gas价格与期望值对比
    gasPrice: {
      labels: analysis.multiplierStats.map(stat => `${stat.multiplier}x`),
      expected: analysis.multiplierStats.map(stat => parseFloat(weiToGwei(analysis.allTxs[0].beforePool.gasPrice)) * stat.multiplier),
      actual: analysis.multiplierStats.map(stat => Number(stat.actualGasPrice))
    },
    
    // 时间与价格散点图数据
    scatterData: analysis.multiplierStats.map(stat => ({
      x: stat.multiplier,
      y: formatTime(stat.confirmTimes.avg),
      r: 5 + stat.txCount * 2, // 气泡大小基于交易数
      label: `${stat.multiplier}x (${stat.actualGasPrice} Gwei)`
    }))
  };
  
  return `// XLayer Gas价格分析图表数据
// 生成时间: ${new Date().toLocaleString()}

const chartData = ${JSON.stringify(chartData, null, 2)};

// 使用示例:
/*
// 使用Chart.js创建图表
const ctx = document.getElementById('confirmationTimeChart').getContext('2d');
new Chart(ctx, {
  type: 'bar',
  data: {
    labels: chartData.confirmationTime.labels,
    datasets: chartData.confirmationTime.datasets
  },
  options: {
    responsive: true,
    title: {
      display: true,
      text: 'Gas价格倍数与确认时间关系'
    }
  }
});

// 创建散点图
const scatterCtx = document.getElementById('scatterChart').getContext('2d');
new Chart(scatterCtx, {
  type: 'bubble',
  data: {
    datasets: [{
      label: 'Gas价格vs确认时间',
      data: chartData.scatterData
    }]
  },
  options: {
    scales: {
      x: {
        title: {
          display: true,
          text: 'Gas价格倍数'
        }
      },
      y: {
        title: {
          display: true,
          text: '确认时间(秒)'
        }
      }
    }
  }
});
*/
`;
}

// 保存文件
function saveFile(filename, content) {
  try {
    fs.writeFileSync(filename, content);
    return true;
  } catch (err) {
    console.error(`保存文件 ${filename} 失败:`, err.message);
    return false;
  }
}

// 主函数
function main() {
  console.log('开始分析Gas价格数据...');
  
  const txData = readTxData();
  console.log(`读取到 ${txData.length} 条交易记录`);
  
  if (txData.length === 0) {
    console.error('没有找到有效的交易数据');
    return;
  }
  
  const analysis = analyzeData(txData);
  if (!analysis.success) {
    console.error(analysis.message);
    return;
  }
  
  console.log('生成分析报告...');
  const report = generateReport(analysis);
  
  if (saveFile(CONFIG.reportFile, report)) {
    console.log(`分析报告已保存到 ${CONFIG.reportFile}`);
  }
  
  console.log('生成图表数据...');
  const chartData = generateChartData(analysis);
  
  if (saveFile(CONFIG.chartDataFile, chartData)) {
    console.log(`图表数据已保存到 ${CONFIG.chartDataFile}`);
  }
  
  // 输出关键结论到控制台
  console.log('\n主要发现:');
  
  const bestTime = analysis.bestStrategies.byTime;
  console.log(`1. 最快确认 Gas 价格: ${bestTime.multiplier}x (${bestTime.actualGasPrice} Gwei) - 确认时间: ${formatTime(bestTime.confirmTimes.avg)}秒`);
  
  const bestEfficiency = analysis.bestStrategies.byEfficiency;
  console.log(`2. 最佳效率 Gas 价格: ${bestEfficiency.multiplier}x (${bestEfficiency.actualGasPrice} Gwei) - 确认时间: ${formatTime(bestEfficiency.confirmTimes.avg)}秒`);
  
  console.log('3. 详细分析请查看生成的报告文件');
}

// 执行主函数
main(); 