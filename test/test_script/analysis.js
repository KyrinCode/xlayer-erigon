const fs = require('fs');

// 配置
const CONFIG = {
  inputFile: './tx_results_extended.json', // 输入文件路径
  reportFile: './tx_analysis_report.md',    // 输出报告文件路径
};

// 读取交易结果数据
function readResults() {
  try {
    const data = fs.readFileSync(CONFIG.inputFile, 'utf8');
    return JSON.parse(data);
  } catch (error) {
    console.error('读取结果文件失败:', error.message);
    // 尝试读取其他可能的结果文件
    try {
      const data = fs.readFileSync('./tx_results.json', 'utf8');
      return JSON.parse(data);
    } catch (e) {
      console.error('读取替代结果文件失败:', e.message);
      return [];
    }
  }
}

// 分析数据
function analyzeData(results) {
  if (!results || results.length === 0) {
    return {
      success: false,
      message: '没有找到有效的交易数据'
    };
  }

  // 过滤成功的交易
  const successfulTxs = results.filter(r => r.status === 'confirmed');
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

  // 分析各组的平均确认时间
  const multiplierStats = Object.keys(byMultiplier)
    .sort((a, b) => parseFloat(a) - parseFloat(b))
    .map(multiplier => {
      const txs = byMultiplier[multiplier];
      const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
      const minTime = Math.min(...txs.map(tx => tx.confirmationTime));
      const maxTime = Math.max(...txs.map(tx => tx.confirmationTime));
      
      return {
        multiplier: parseFloat(multiplier),
        txCount: txs.length,
        avgTimeMs: avgTime,
        avgTimeSec: avgTime / 1000,
        minTimeMs: minTime,
        minTimeSec: minTime / 1000,
        maxTimeMs: maxTime,
        maxTimeSec: maxTime / 1000,
        txs: txs
      };
    });

  // 交易池变化分析
  const firstTx = successfulTxs[0];
  const lastTx = successfulTxs[successfulTxs.length - 1];
  const poolChange = {
    startTime: new Date(firstTx.timestamp).toLocaleString(),
    endTime: new Date(lastTx.timestamp).toLocaleString(),
    initialPending: firstTx.beforePool.pendingCount,
    initialQueued: firstTx.beforePool.queuedCount,
    initialTotal: firstTx.beforePool.totalCount,
    finalPending: lastTx.afterConfirmationPool.pendingCount,
    finalQueued: lastTx.afterConfirmationPool.queuedCount,
    finalTotal: lastTx.afterConfirmationPool.totalCount,
    pendingChange: lastTx.afterConfirmationPool.pendingCount - firstTx.beforePool.pendingCount,
    queuedChange: lastTx.afterConfirmationPool.queuedCount - firstTx.beforePool.queuedCount,
    totalChange: lastTx.afterConfirmationPool.totalCount - firstTx.beforePool.totalCount,
    pendingChangePercent: ((lastTx.afterConfirmationPool.pendingCount - firstTx.beforePool.pendingCount) / firstTx.beforePool.pendingCount * 100).toFixed(2),
    totalChangePercent: ((lastTx.afterConfirmationPool.totalCount - firstTx.beforePool.totalCount) / firstTx.beforePool.totalCount * 100).toFixed(2)
  };

  // 区块分析
  const blockInfo = successfulTxs.map(tx => ({
    blockNumber: tx.blockInfo.blockNumber,
    timestamp: tx.blockInfo.timestamp,
    dateTime: new Date(tx.blockInfo.timestamp * 1000).toLocaleString(),
    txCount: tx.blockInfo.transactionCount
  }));

  // 区块时间分析
  let blockTimes = [];
  for (let i = 1; i < blockInfo.length; i++) {
    blockTimes.push({
      fromBlock: blockInfo[i-1].blockNumber,
      toBlock: blockInfo[i].blockNumber,
      timeDiff: blockInfo[i].timestamp - blockInfo[i-1].timestamp,
      blocks: blockInfo[i].blockNumber - blockInfo[i-1].blockNumber
    });
  }

  // 平均区块时间
  const avgBlockTime = blockTimes.reduce((acc, bt) => acc + (bt.timeDiff / bt.blocks), 0) / 
                       (blockTimes.length || 1);

  // 汇总统计
  const summary = {
    totalTxs: results.length,
    successfulTxs: successfulTxs.length,
    successRate: (successfulTxs.length / results.length * 100).toFixed(2),
    avgConfirmTime: successfulTxs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / successfulTxs.length,
    minConfirmTime: Math.min(...successfulTxs.map(tx => tx.confirmationTime)),
    maxConfirmTime: Math.max(...successfulTxs.map(tx => tx.confirmationTime)),
    startTime: new Date(successfulTxs[0].timestamp).toLocaleString(),
    endTime: new Date(successfulTxs[successfulTxs.length - 1].timestamp).toLocaleString(),
    duration: (new Date(successfulTxs[successfulTxs.length - 1].timestamp) - new Date(successfulTxs[0].timestamp)) / 1000,
    networkGasPrice: web3Utils.fromWei(successfulTxs[0].beforePool.gasPrice, 'gwei'),
    avgHashGenTime: successfulTxs.reduce((acc, tx) => acc + tx.hashGenerationTime, 0) / successfulTxs.length
  };

  // 生成图表数据
  const chartData = {
    confirmationTimeByMultiplier: multiplierStats.map(stat => ({
      multiplier: stat.multiplier,
      avgTimeSec: stat.avgTimeSec,
      minTimeSec: stat.minTimeSec,
      maxTimeSec: stat.maxTimeSec
    })),
    transactionsByBlock: [...new Set(successfulTxs.map(tx => tx.blockInfo.blockNumber))]
      .sort((a, b) => a - b)
      .map(blockNumber => {
        const txsInBlock = successfulTxs.filter(tx => tx.blockInfo.blockNumber === blockNumber);
        return {
          blockNumber,
          txCount: txsInBlock[0].blockInfo.transactionCount,
          ourTxCount: txsInBlock.length
        };
      })
  };

  return {
    success: true,
    summary,
    multiplierStats,
    poolChange,
    blockInfo,
    blockTimes,
    avgBlockTime,
    chartData
  };
}

// 将Web3单位转换为可读格式的简单实现
const web3Utils = {
  fromWei: (wei, unit) => {
    if (unit === 'gwei') {
      return (Number(wei) / 1e9).toString();
    }
    return wei;
  }
};

// 生成Markdown报告
function generateMarkdownReport(analysis) {
  if (!analysis.success) {
    return `# 交易分析报告\n\n错误: ${analysis.message}\n`;
  }

  const { summary, multiplierStats, poolChange, blockInfo, avgBlockTime, chartData } = analysis;

  let report = `# XLayer交易实验分析报告\n\n`;
  
  report += `## 实验概要\n\n`;
  report += `- 总交易数: ${summary.totalTxs}\n`;
  report += `- 成功交易数: ${summary.successfulTxs} (成功率: ${summary.successRate}%)\n`;
  report += `- 实验开始时间: ${summary.startTime}\n`;
  report += `- 实验结束时间: ${summary.endTime}\n`;
  report += `- 实验总时长: ${summary.duration.toFixed(0)}秒\n`;
  report += `- 网络基础Gas价格: ${summary.networkGasPrice} Gwei\n\n`;
  
  report += `## 交易确认时间分析\n\n`;
  report += `- 平均确认时间: ${(summary.avgConfirmTime/1000).toFixed(2)}秒\n`;
  report += `- 最短确认时间: ${(summary.minConfirmTime/1000).toFixed(2)}秒\n`;
  report += `- 最长确认时间: ${(summary.maxConfirmTime/1000).toFixed(2)}秒\n`;
  report += `- 平均交易哈希生成时间: ${summary.avgHashGenTime.toFixed(2)}毫秒\n\n`;

  report += `### 不同Gas价格倍数的确认时间\n\n`;
  report += `| Gas价格倍数 | 实际Gas价格(Gwei) | 平均确认时间(秒) | 最短确认时间(秒) | 最长确认时间(秒) | 交易数 |\n`;
  report += `|--------------|------------------|-----------------|-----------------|-----------------|--------|\n`;
  
  multiplierStats.forEach(stat => {
    const actualGasPrice = (summary.networkGasPrice * stat.multiplier).toFixed(0);
    report += `| ${stat.multiplier}x | ${actualGasPrice} | ${stat.avgTimeSec.toFixed(2)} | ${stat.minTimeSec.toFixed(2)} | ${stat.maxTimeSec.toFixed(2)} | ${stat.txCount} |\n`;
  });
  
  report += `\n### 确认时间与Gas价格的关系\n\n`;
  report += `\`\`\`\n`;
  report += `// 确认时间与Gas价格倍数的关系图表数据 (可用于绘制图表)\n`;
  report += `const confirmationTimeData = [\n`;
  chartData.confirmationTimeByMultiplier.forEach(data => {
    report += `  { multiplier: ${data.multiplier}, avgTimeSec: ${data.avgTimeSec.toFixed(2)}, minTimeSec: ${data.minTimeSec.toFixed(2)}, maxTimeSec: ${data.maxTimeSec.toFixed(2)} },\n`;
  });
  report += `];\n`;
  report += `\`\`\`\n\n`;
  
  report += `## 交易池状态分析\n\n`;
  report += `- 初始状态 (${poolChange.startTime}):\n`;
  report += `  - 待处理: ${poolChange.initialPending.toLocaleString()}\n`;
  report += `  - 排队中: ${poolChange.initialQueued.toLocaleString()}\n`;
  report += `  - 总计: ${poolChange.initialTotal.toLocaleString()}\n\n`;
  
  report += `- 最终状态 (${poolChange.endTime}):\n`;
  report += `  - 待处理: ${poolChange.finalPending.toLocaleString()}\n`;
  report += `  - 排队中: ${poolChange.finalQueued.toLocaleString()}\n`;
  report += `  - 总计: ${poolChange.finalTotal.toLocaleString()}\n\n`;
  
  report += `- 变化:\n`;
  report += `  - 待处理: ${poolChange.pendingChange.toLocaleString()} (${poolChange.pendingChangePercent}%)\n`;
  report += `  - 排队中: ${poolChange.queuedChange.toLocaleString()}\n`;
  report += `  - 总计: ${poolChange.totalChange.toLocaleString()} (${poolChange.totalChangePercent}%)\n\n`;
  
  report += `## 区块分析\n\n`;
  report += `- 平均区块时间: ${avgBlockTime.toFixed(2)}秒\n\n`;
  
  report += `### 区块详情\n\n`;
  report += `| 区块编号 | 时间 | 区块中的交易数 |\n`;
  report += `|----------|------|----------------|\n`;
  
  blockInfo.forEach(block => {
    report += `| ${block.blockNumber} | ${block.dateTime} | ${block.txCount} |\n`;
  });
  
  report += `\n### 区块间隔\n\n`;
  report += `| 起始区块 | 结束区块 | 区块数 | 时间差(秒) | 每区块平均时间(秒) |\n`;
  report += `|-----------|-----------|--------|------------|--------------------|\n`;
  
  analysis.blockTimes.forEach(bt => {
    report += `| ${bt.fromBlock} | ${bt.toBlock} | ${bt.blocks} | ${bt.timeDiff} | ${(bt.timeDiff / bt.blocks).toFixed(2)} |\n`;
  });
  
  report += `\n## 结论\n\n`;
  
  // 基于数据得出一些结论
  report += `1. **Gas价格影响**:\n`;
  
  // 找出最快确认时间的Gas倍数
  const fastestMultiplier = multiplierStats.reduce((fastest, current) => 
    current.avgTimeSec < fastest.avgTimeSec ? current : fastest, multiplierStats[0]);
  
  report += `   - Gas价格倍数${fastestMultiplier.multiplier}x (${(summary.networkGasPrice * fastestMultiplier.multiplier).toFixed(0)} Gwei)提供了最快的平均确认时间: ${fastestMultiplier.avgTimeSec.toFixed(2)}秒\n`;
  
  // 分析确认时间与Gas价格的整体关系
  const sortedByTime = [...multiplierStats].sort((a, b) => a.avgTimeSec - b.avgTimeSec);
  report += `   - 前三个最快的Gas价格倍数: ${sortedByTime[0].multiplier}x, ${sortedByTime[1].multiplier}x, ${sortedByTime[2].multiplier}x\n`;
  
  // 检查是否有价格更高但确认更慢的情况
  const anomalies = [];
  for (let i = 0; i < multiplierStats.length; i++) {
    for (let j = 0; j < multiplierStats.length; j++) {
      if (multiplierStats[i].multiplier < multiplierStats[j].multiplier && 
          multiplierStats[i].avgTimeSec < multiplierStats[j].avgTimeSec) {
        anomalies.push({
          lower: multiplierStats[i],
          higher: multiplierStats[j]
        });
      }
    }
  }
  
  if (anomalies.length > 0) {
    report += `   - 发现异常: 有些较低的Gas价格反而确认更快，例如:\n`;
    const uniqueAnomalies = [];
    anomalies.slice(0, Math.min(3, anomalies.length)).forEach(anomaly => {
      report += `     * ${anomaly.lower.multiplier}x (${(summary.networkGasPrice * anomaly.lower.multiplier).toFixed(0)} Gwei) 确认时间为 ${anomaly.lower.avgTimeSec.toFixed(2)}秒，但 ${anomaly.higher.multiplier}x (${(summary.networkGasPrice * anomaly.higher.multiplier).toFixed(0)} Gwei) 需要 ${anomaly.higher.avgTimeSec.toFixed(2)}秒\n`;
    });
  }
  
  report += `\n2. **交易池观察**:\n`;
  report += `   - 实验期间交易池大小减少了 ${Math.abs(poolChange.totalChange).toLocaleString()} 笔交易 (${Math.abs(poolChange.totalChangePercent)}%)\n`;
  report += `   - 实验开始时交易池拥堵程度高，有 ${poolChange.initialTotal.toLocaleString()} 笔待处理交易\n`;
  
  report += `\n3. **区块分析**:\n`;
  report += `   - 平均区块产生时间为 ${avgBlockTime.toFixed(2)}秒\n`;
  report += `   - 区块中的交易数量范围: ${Math.min(...blockInfo.map(b => b.txCount)).toLocaleString()} - ${Math.max(...blockInfo.map(b => b.txCount)).toLocaleString()} 笔交易\n`;
  
  report += `\n4. **推荐Gas价格策略**:\n`;
  report += `   - 基于本次实验，最优Gas价格倍数为 **${fastestMultiplier.multiplier}x** (约 ${(summary.networkGasPrice * fastestMultiplier.multiplier).toFixed(0)} Gwei)\n`;
  report += `   - 对于需要快速确认的交易，建议使用 ${sortedByTime[0].multiplier}x-${sortedByTime[2].multiplier}x 的Gas价格倍数\n`;
  report += `   - XLayer网络当前基础Gas价格为 ${summary.networkGasPrice} Gwei\n`;
  
  return report;
}

// 保存报告到文件
function saveReport(report) {
  try {
    fs.writeFileSync(CONFIG.reportFile, report);
    console.log(`分析报告已保存到: ${CONFIG.reportFile}`);
    return true;
  } catch (error) {
    console.error('保存报告失败:', error.message);
    return false;
  }
}

// 主函数
function main() {
  console.log('开始分析交易数据...');
  const results = readResults();
  console.log(`读取到 ${results.length} 条交易记录`);
  
  const analysis = analyzeData(results);
  if (!analysis.success) {
    console.error(analysis.message);
    return;
  }
  
  console.log('生成分析报告...');
  const report = generateMarkdownReport(analysis);
  
  if (saveReport(report)) {
    console.log('报告已生成并保存');
    
    // 输出摘要信息到控制台
    if (analysis.multiplierStats && analysis.multiplierStats.length > 0) {
      console.log('\n=========== 摘要结果 ===========');
      console.log(`总交易数: ${analysis.summary.totalTxs}, 成功交易数: ${analysis.summary.successfulTxs}`);
      console.log(`平均确认时间: ${(analysis.summary.avgConfirmTime/1000).toFixed(2)}秒`);
      
      console.log('\nGas价格与确认时间关系:');
      console.log('Gas倍数\t确认时间(秒)\t实际Gas价格(Gwei)');
      console.log('----------------------------------------');
      
      analysis.multiplierStats.forEach(stat => {
        const actualGasPrice = (analysis.summary.networkGasPrice * stat.multiplier).toFixed(0);
        console.log(`${stat.multiplier}x\t${stat.avgTimeSec.toFixed(2)}\t\t${actualGasPrice}`);
      });
      
      // 找出最快确认时间的Gas倍数
      const fastestMultiplier = analysis.multiplierStats.reduce((fastest, current) => 
        current.avgTimeSec < fastest.avgTimeSec ? current : fastest, analysis.multiplierStats[0]);
        
      console.log(`\n最佳Gas价格倍数: ${fastestMultiplier.multiplier}x (${(analysis.summary.networkGasPrice * fastestMultiplier.multiplier).toFixed(0)} Gwei) 平均确认时间: ${fastestMultiplier.avgTimeSec.toFixed(2)}秒`);
    }
  }
}

// 执行主函数
main(); 