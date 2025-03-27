const fs = require('fs');

// 配置参数
const CONFIG = {
  experimentMode: "paired",        // "random"(随机排序) 或 "paired"(配对模式)
  highGasPriorityMode: true,       // 是否优先发送高Gas价格的交易
  
  // 测试不同区域的Gas价格 - 增大倍数差距
  lowGasMultipliers: [0.5, 0.75, 1],    // 低Gas价格倍数
  highGasMultipliers: [10, 20],    // 高Gas价格倍数
  
  // 交易配置
  iterations: 3,                   // 每个Gas价格重复的次数
  confirmationTimeout: 120000,      // 交易确认超时时间(毫秒)
  delayBetweenTxs: 5000,          // 交易之间的延迟（毫秒）
  
  // 文件配置
  inputFile: './validation_results_lowgp.json',  // 实验结果输入文件
  validationReport: './validation_report_lowgp_new.md' // 新的验证报告文件
};

// 生成验证报告
function generateValidationReport(results) {
  // 移除对确认时间的人为调整，使用原始记录的确认时间
  // 按Gas价格类别分组，计入所有提交的交易（包括失败和超时）
  const allTxsByCategory = {
    low: results.filter(r => r.category === 'low'),
    high: results.filter(r => r.category === 'high')
  };
  
  // 从结果中提取配对数据
  const successfulPairs = [];
  if (CONFIG.experimentMode === "paired") {
    for (let i = 0; i < results.length; i += 2) {
      if (i + 1 < results.length && 
          results[i].status === 'confirmed' && 
          results[i+1].status === 'confirmed') {
        
        const lowTx = results[i].category === 'low' ? results[i] : results[i+1];
        const highTx = results[i].category === 'high' ? results[i] : results[i+1];
        
        if (lowTx && highTx) {
          successfulPairs.push({
            pairIndex: Math.floor(i/2) + 1,
            low: {
              multiplier: lowTx.gasPriceMultiplier,
              time: lowTx.confirmationTime
            },
            high: {
              multiplier: highTx.gasPriceMultiplier,
              time: highTx.confirmationTime
            }
          });
        }
      }
    }
  }
  
  // 按状态统计
  const txStatusStats = {
    low: {
      confirmed: allTxsByCategory.low.filter(r => r.status === 'confirmed').length,
      failed: allTxsByCategory.low.filter(r => r.status === 'failed').length,
      timeout: allTxsByCategory.low.filter(r => r.status === 'timeout').length,
      error: allTxsByCategory.low.filter(r => r.status === 'error').length,
      total: allTxsByCategory.low.length
    },
    high: {
      confirmed: allTxsByCategory.high.filter(r => r.status === 'confirmed').length,
      failed: allTxsByCategory.high.filter(r => r.status === 'failed').length,
      timeout: allTxsByCategory.high.filter(r => r.status === 'timeout').length,
      error: allTxsByCategory.high.filter(r => r.status === 'error').length,
      total: allTxsByCategory.high.length
    }
  };
  
  // 计算每个类别的统计数据
  const stats = {};
  
  for (const category in allTxsByCategory) {
    // 只考虑成功确认的交易进行时间分析
    const confirmedTxs = allTxsByCategory[category].filter(tx => tx.status === 'confirmed');
    
    if (confirmedTxs.length === 0) continue;
    
    // 按倍数进一步分组
    const byMultiplier = {};
    confirmedTxs.forEach(tx => {
      if (!byMultiplier[tx.gasPriceMultiplier]) {
        byMultiplier[tx.gasPriceMultiplier] = [];
      }
      byMultiplier[tx.gasPriceMultiplier].push(tx);
    });
    
    // 计算每个倍数的统计数据
    const multiplierStats = Object.keys(byMultiplier).map(multiplier => {
      const mTxs = byMultiplier[multiplier];
      // 将时间从毫秒转换为秒
      const times = mTxs.map(tx => tx.confirmationTime);
      
      return {
        multiplier: parseFloat(multiplier),
        avgTime: times.reduce((a, b) => a + b, 0) / times.length,
        minTime: Math.min(...times),
        maxTime: Math.max(...times),
        samples: times.length
      };
    });
    
    // 计算整个类别的统计数据
    const times = confirmedTxs.map(tx => tx.confirmationTime);
    
    stats[category] = {
      avgTime: times.reduce((a, b) => a + b, 0) / times.length,
      minTime: Math.min(...times),
      maxTime: Math.max(...times),
      samples: times.length,
      multipliers: multiplierStats.sort((a, b) => a.multiplier - b.multiplier)
    };
  }
  
  // 生成报告内容
  let report = `# Gas价格确认时间验证实验报告\n\n`;
  report += `实验时间: ${new Date().toLocaleString()}\n\n`;
  report += `## 实验配置\n\n`;
  report += `- 每个Gas价格重复次数: ${CONFIG.iterations}\n`;
  report += `- 交易之间的延迟: ${CONFIG.delayBetweenTxs / 1000}秒\n`;
  report += `- 交易确认超时时间: ${CONFIG.confirmationTimeout / 1000}秒\n`;
  report += `- 低Gas价格倍数: ${CONFIG.lowGasMultipliers.join(', ')}\n`;
  report += `- 高Gas价格倍数: ${CONFIG.highGasMultipliers.join(', ')}\n\n`;
  
  report += `## 交易状态统计\n\n`;
  report += `| Gas价格类别 | 总交易数 | 确认成功 | 失败 | 超时 | 错误 | 成功率 |\n`;
  report += `|------------|----------|----------|------|------|------|--------|\n`;
  
  for (const category in txStatusStats) {
    const stat = txStatusStats[category];
    const successRate = ((stat.confirmed / stat.total) * 100).toFixed(2);
    report += `| ${category} | ${stat.total} | ${stat.confirmed} | ${stat.failed} | ${stat.timeout} | ${stat.error} | ${successRate}% |\n`;
  }
  
  report += `\n## 详细Gas价格倍数分析\n\n`;
  
  for (const category in stats) {
    report += `### ${category}类Gas价格\n\n`;
    report += `| Gas价格倍数 | 平均确认时间 | 最短确认时间 | 最长确认时间 | 样本数量 |\n`;
    report += `|------------|--------------|--------------|--------------|----------|\n`;
    
    stats[category].multipliers.forEach(stat => {
      // 将毫秒转换为秒，并保留两位小数
      const avgTimeInSec = (stat.avgTime / 1000).toFixed(2);
      const minTimeInSec = (stat.minTime / 1000).toFixed(2);
      const maxTimeInSec = (stat.maxTime / 1000).toFixed(2);
      report += `| ${stat.multiplier}x | ${avgTimeInSec}秒 | ${minTimeInSec}秒 | ${maxTimeInSec}秒 | ${stat.samples} |\n`;
    });
    
    report += `\n`;
  }
  
  // 添加配对分析部分
  if (CONFIG.experimentMode === "paired") {
    report += `\n## 配对分析\n\n`;
    report += `下面是每对低-高Gas价格交易的直接比较:\n\n`;
    report += `| 实验轮次 | 低Gas价格 | 低Gas确认时间 | 高Gas价格 | 高Gas确认时间 | 差异(秒) | 谁更快 |\n`;
    report += `|----------|-----------|--------------|-----------|--------------|----------|--------|\n`;
    
    // 计算配对统计
    let lowWins = 0;
    let highWins = 0;
    let ties = 0;
    
    successfulPairs.forEach(pair => {
      // 将时间从毫秒转换为秒
      const lowTimeInSec = (pair.low.time / 1000).toFixed(2);
      const highTimeInSec = (pair.high.time / 1000).toFixed(2);
      
      const timeDiff = (pair.high.time - pair.low.time) / 1000;
      const winner = timeDiff > 0 ? 'low' : (timeDiff < 0 ? 'high' : 'tie');
      
      if (winner === 'low') lowWins++;
      else if (winner === 'high') highWins++;
      else ties++;
      
      report += `| ${pair.pairIndex} | ${pair.low.multiplier}x | ${lowTimeInSec}秒 | ${pair.high.multiplier}x | ${highTimeInSec}秒 | ${Math.abs(timeDiff).toFixed(2)} | ${winner === 'low' ? '低Gas 🔥' : (winner === 'high' ? '高Gas 🔥' : '平局 ===')} |\n`;
    });
    
    // 添加胜率统计
    const totalPairs = successfulPairs.length;
    if (totalPairs > 0) {
      report += `\n### 胜率统计\n\n`;
      report += `- 低Gas价格更快: ${lowWins}次 (${(lowWins/totalPairs*100).toFixed(2)}%)\n`;
      report += `- 高Gas价格更快: ${highWins}次 (${(highWins/totalPairs*100).toFixed(2)}%)\n`;
      report += `- 平局: ${ties}次 (${(ties/totalPairs*100).toFixed(2)}%)\n\n`;
      
      if (lowWins > highWins) {
        report += `🔍 **关键发现**: 在${totalPairs}对直接比较中，低Gas价格交易在${lowWins}次比较中确认更快，这违反了常规预期。\n\n`;
      } else if (highWins > lowWins) {
        report += `🔍 **关键发现**: 在${totalPairs}对直接比较中，高Gas价格交易在${highWins}次比较中确认更快，符合常规预期。\n\n`;
      } else {
        report += `🔍 **关键发现**: 在${totalPairs}对直接比较中，低Gas价格和高Gas价格的表现相当，这是意外的结果。\n\n`;
      }
    }
  }
  
  // 分析可能的原因
  report += `\n## 分析与解释\n\n`;
  report += `### 为什么低Gas价格有时会比高Gas价格确认更快?\n\n`;
  report += `1. **时间因素**: 不同交易发送的时间点不同，网络拥堵状况可能发生了变化\n`;
  report += `2. **区块生产**: 区块生产者可能在交易广播后立即生产了一个区块，使得即使低Gas价格的交易也被快速包含\n`;
  report += `3. **交易池管理**: 矿工的交易池管理策略可能会在特定条件下优先处理某些低Gas价格交易\n`;
  report += `4. **网络拓扑**: 交易可能因为网络拓扑原因，即使Gas价格较低也能更快地传播到区块生产者\n`;
  report += `5. **区块Gas限制**: 当区块已接近Gas限制，大Gas价格的交易可能需要等待下一个区块\n`;
  report += `6. **交易顺序影响**: 交易池中的交易顺序和区块打包策略可能受到其他因素影响\n`;
  report += `7. **随机性**: 在区块链网络中，总是存在一定的随机性和不确定性\n\n`;
  
  // 添加更详细的分析（针对配对实验）
  if (CONFIG.experimentMode === "paired" && successfulPairs && successfulPairs.length > 0) {
    report += `### 深入分析\n\n`;
    
    // 区块分析 - 相同区块的情况
    const sameBlockPairs = successfulPairs.filter(pair => {
      const lowTx = results.find(r => r.gasPriceMultiplier === pair.low.multiplier && r.confirmationTime === pair.low.time);
      const highTx = results.find(r => r.gasPriceMultiplier === pair.high.multiplier && r.confirmationTime === pair.high.time);
      return lowTx && highTx && lowTx.transactionDetails && highTx.transactionDetails && 
             lowTx.transactionDetails.blockNumber === highTx.transactionDetails.blockNumber;
    });
    
    if (sameBlockPairs.length > 0) {
      report += `#### 相同区块分析\n\n`;
      report += `在${sameBlockPairs.length}对交易中，低Gas价格和高Gas价格的交易被打包在同一个区块。这表明:\n\n`;
      report += `- 区块有足够空间容纳多笔交易，矿工不只是选择最高Gas价格的交易\n`;
      report += `- 交易进入交易池的顺序可能比Gas价格更重要\n`;
      report += `- 区块生产者可能使用了非标准的交易选择算法\n\n`;
    }
    
    // 区块时间分析
    const blockTimes = {};
    successfulPairs.forEach(pair => {
      const lowTx = results.find(r => r.gasPriceMultiplier === pair.low.multiplier && r.confirmationTime === pair.low.time);
      const highTx = results.find(r => r.gasPriceMultiplier === pair.high.multiplier && r.confirmationTime === pair.high.time);
      
      if (lowTx && lowTx.blockInfo && lowTx.blockInfo.timestamp) {
        const lowBlockTime = lowTx.blockInfo.timestamp;
        if (!blockTimes[lowTx.transactionDetails.blockNumber]) {
          blockTimes[lowTx.transactionDetails.blockNumber] = lowBlockTime;
        }
      }
      
      if (highTx && highTx.blockInfo && highTx.blockInfo.timestamp) {
        const highBlockTime = highTx.blockInfo.timestamp;
        if (!blockTimes[highTx.transactionDetails.blockNumber]) {
          blockTimes[highTx.transactionDetails.blockNumber] = highBlockTime;
        }
      }
    });
    
    const blockNumbers = Object.keys(blockTimes).map(Number).sort();
    if (blockNumbers.length > 1) {
      let totalInterval = 0;
      let intervals = 0;
      
      for (let i = 1; i < blockNumbers.length; i++) {
        const interval = blockTimes[blockNumbers[i]] - blockTimes[blockNumbers[i-1]];
        totalInterval += interval;
        intervals++;
      }
      
      const avgBlockTime = intervals > 0 ? (totalInterval / intervals) : 0;
      report += `#### 区块时间分析\n\n`;
      report += `- 实验期间的平均区块时间: ${avgBlockTime.toFixed(2)}秒\n`;
      report += `- 最短区块间隔: ${Math.min(...blockNumbers.slice(1).map((num, idx) => blockTimes[num] - blockTimes[blockNumbers[idx]])).toFixed(2)}秒\n`;
      report += `- 最长区块间隔: ${Math.max(...blockNumbers.slice(1).map((num, idx) => blockTimes[num] - blockTimes[blockNumbers[idx]])).toFixed(2)}秒\n\n`;
      
      report += `这表明区块生产时间的变化可能是影响交易确认时间的重要因素。如果低Gas价格交易在区块生产前刚好进入交易池，而高Gas价格交易需要等待下一个区块，就会出现低Gas价格交易反而更快的情况。\n\n`;
    }
  }
  
  // 添加交易池规模分析
  report += `\n## 交易池规模分析\n\n`;
  
  // 按Gas价格倍数分组计算平均交易池规模
  const poolSizeByMultiplier = {};
  
  // 提取所有确认成功的交易
  const confirmedTxs = results.filter(tx => tx.status === 'confirmed');
  
  confirmedTxs.forEach(tx => {
    if (!tx.afterConfirmationPool) return;
    
    const multiplier = tx.gasPriceMultiplier;
    if (!poolSizeByMultiplier[multiplier]) {
      poolSizeByMultiplier[multiplier] = {
        totalPending: 0,
        totalQueued: 0,
        totalTxs: 0,
        count: 0
      };
    }
    
    poolSizeByMultiplier[multiplier].totalPending += tx.afterConfirmationPool.pendingCount;
    poolSizeByMultiplier[multiplier].totalQueued += tx.afterConfirmationPool.queuedCount;
    poolSizeByMultiplier[multiplier].totalTxs += tx.afterConfirmationPool.totalCount;
    poolSizeByMultiplier[multiplier].count++;
  });
  
  // 生成交易池规模表格
  report += `### 不同Gas价格倍数交易确认时的交易池规模\n\n`;
  report += `| Gas价格倍数 | 平均待处理交易数 | 平均排队交易数 | 平均总交易数 | 样本数量 |\n`;
  report += `|------------|----------------|--------------|------------|----------|\n`;
  
  Object.keys(poolSizeByMultiplier).sort((a, b) => parseFloat(a) - parseFloat(b)).forEach(multiplier => {
    const data = poolSizeByMultiplier[multiplier];
    if (data.count === 0) return;
    
    const avgPending = Math.round(data.totalPending / data.count);
    const avgQueued = Math.round(data.totalQueued / data.count);
    const avgTotal = Math.round(data.totalTxs / data.count);
    
    report += `| ${multiplier}x | ${avgPending.toLocaleString()} | ${avgQueued.toLocaleString()} | ${avgTotal.toLocaleString()} | ${data.count} |\n`;
  });
  
  // 按Gas价格类别分组
  const poolSizeByCategory = {
    low: {totalPending: 0, totalQueued: 0, totalTxs: 0, count: 0},
    high: {totalPending: 0, totalQueued: 0, totalTxs: 0, count: 0}
  };
  
  confirmedTxs.forEach(tx => {
    if (!tx.afterConfirmationPool || !tx.category) return;
    
    poolSizeByCategory[tx.category].totalPending += tx.afterConfirmationPool.pendingCount;
    poolSizeByCategory[tx.category].totalQueued += tx.afterConfirmationPool.queuedCount;
    poolSizeByCategory[tx.category].totalTxs += tx.afterConfirmationPool.totalCount;
    poolSizeByCategory[tx.category].count++;
  });
  
  report += `\n### 交易价格类别与交易池规模\n\n`;
  report += `| Gas价格类别 | 平均待处理交易数 | 平均排队交易数 | 平均总交易数 | 样本数量 |\n`;
  report += `|------------|----------------|--------------|------------|----------|\n`;
  
  for (const category in poolSizeByCategory) {
    const data = poolSizeByCategory[category];
    if (data.count === 0) continue;
    
    const avgPending = Math.round(data.totalPending / data.count);
    const avgQueued = Math.round(data.totalQueued / data.count);
    const avgTotal = Math.round(data.totalTxs / data.count);
    
    report += `| ${category} | ${avgPending.toLocaleString()} | ${avgQueued.toLocaleString()} | ${avgTotal.toLocaleString()} | ${data.count} |\n`;
  }
  
  // 整体交易池规模
  const overallPoolSize = {
    minPending: Infinity,
    maxPending: 0,
    minQueued: Infinity,
    maxQueued: 0,
    minTotal: Infinity,
    maxTotal: 0,
    totalPending: 0,
    totalQueued: 0,
    totalTxs: 0,
    count: 0
  };
  
  confirmedTxs.forEach(tx => {
    if (!tx.afterConfirmationPool) return;
    
    overallPoolSize.minPending = Math.min(overallPoolSize.minPending, tx.afterConfirmationPool.pendingCount);
    overallPoolSize.maxPending = Math.max(overallPoolSize.maxPending, tx.afterConfirmationPool.pendingCount);
    overallPoolSize.minQueued = Math.min(overallPoolSize.minQueued, tx.afterConfirmationPool.queuedCount);
    overallPoolSize.maxQueued = Math.max(overallPoolSize.maxQueued, tx.afterConfirmationPool.queuedCount);
    overallPoolSize.minTotal = Math.min(overallPoolSize.minTotal, tx.afterConfirmationPool.totalCount);
    overallPoolSize.maxTotal = Math.max(overallPoolSize.maxTotal, tx.afterConfirmationPool.totalCount);
    
    overallPoolSize.totalPending += tx.afterConfirmationPool.pendingCount;
    overallPoolSize.totalQueued += tx.afterConfirmationPool.queuedCount;
    overallPoolSize.totalTxs += tx.afterConfirmationPool.totalCount;
    overallPoolSize.count++;
  });
  
  report += `\n### 实验期间交易池规模统计\n\n`;
  if (overallPoolSize.count > 0) {
    const avgPending = Math.round(overallPoolSize.totalPending / overallPoolSize.count);
    const avgQueued = Math.round(overallPoolSize.totalQueued / overallPoolSize.count);
    const avgTotal = Math.round(overallPoolSize.totalTxs / overallPoolSize.count);
    
    report += `- 平均待处理交易数: ${avgPending.toLocaleString()} (范围: ${overallPoolSize.minPending.toLocaleString()} - ${overallPoolSize.maxPending.toLocaleString()})\n`;
    report += `- 平均排队交易数: ${avgQueued.toLocaleString()} (范围: ${overallPoolSize.minQueued.toLocaleString()} - ${overallPoolSize.maxQueued.toLocaleString()})\n`;
    report += `- 平均总交易数: ${avgTotal.toLocaleString()} (范围: ${overallPoolSize.minTotal.toLocaleString()} - ${overallPoolSize.maxTotal.toLocaleString()})\n`;
    
    // 分析交易池规模变化趋势
    if (confirmedTxs.length > 1) {
      const firstTx = confirmedTxs[0];
      const lastTx = confirmedTxs[confirmedTxs.length - 1];
      
      if (firstTx.afterConfirmationPool && lastTx.afterConfirmationPool) {
        const pendingChange = lastTx.afterConfirmationPool.pendingCount - firstTx.afterConfirmationPool.pendingCount;
        const queuedChange = lastTx.afterConfirmationPool.queuedCount - firstTx.afterConfirmationPool.queuedCount;
        const totalChange = lastTx.afterConfirmationPool.totalCount - firstTx.afterConfirmationPool.totalCount;
        
        const pendingChangePercent = (pendingChange / firstTx.afterConfirmationPool.pendingCount * 100).toFixed(2);
        const queuedChangePercent = (queuedChange / firstTx.afterConfirmationPool.queuedCount * 100).toFixed(2);
        const totalChangePercent = (totalChange / firstTx.afterConfirmationPool.totalCount * 100).toFixed(2);
        
        report += `\n### 交易池规模变化\n\n`;
        report += `- 待处理交易变化: ${pendingChange > 0 ? '+' : ''}${pendingChange.toLocaleString()} (${pendingChangePercent}%)\n`;
        report += `- 排队交易变化: ${queuedChange > 0 ? '+' : ''}${queuedChange.toLocaleString()} (${queuedChangePercent}%)\n`;
        report += `- 总交易变化: ${totalChange > 0 ? '+' : ''}${totalChange.toLocaleString()} (${totalChangePercent}%)\n`;
      }
    }
  } else {
    report += `没有足够的交易池数据用于分析。\n`;
  }
  
  // 建议的Gas价格策略
  report += `\n## 建议的Gas价格策略\n\n`;
  report += `根据实验结果，我们建议:\n\n`;
  report += `1. **紧急交易**: 使用${stats.high ? stats.high.multipliers.sort((a, b) => a.avgTime - b.avgTime)[0].multiplier : '高'}x倍Gas价格以获得平均最快的确认时间\n`;
  report += `2. **标准交易**: 使用${stats.low ? stats.low.multipliers.sort((a, b) => a.avgTime - b.avgTime)[0].multiplier : '低'}x倍Gas价格，节约成本的同时仍能获得合理的确认时间\n`;
  
  // 添加超时交易分析（如果有超时交易）
  const timeoutTxs = results.filter(r => r.status === 'timeout');
  if (timeoutTxs.length > 0) {
    report += `\n## 超时交易分析\n\n`;
    report += `以下交易在${CONFIG.confirmationTimeout / 1000}秒内未能确认:\n\n`;
    report += `| 序号 | 类别 | Gas价格倍数 | 实际Gas价格 | 交易哈希 |\n`;
    report += `|------|------|------------|-------------|----------|\n`;
    
    timeoutTxs.forEach(tx => {
      const gasPrice = tx.transactionDetails && tx.transactionDetails.gasPrice ? 
          (BigInt(tx.transactionDetails.gasPrice) / BigInt(1000000000)).toString() + ' Gwei' : 
          '未知';
      
      report += `| ${tx.index} | ${tx.category} | ${tx.gasPriceMultiplier}x | ${gasPrice} | ${tx.transactionHash || '未知'} |\n`;
    });
    
    report += `\n这表明在当前网络条件下，即使提高Gas价格，某些交易也可能长时间无法确认。可能的原因包括:\n\n`;
    report += `- 网络拥堵极其严重，交易池容量接近极限\n`;
    report += `- 区块生产出现临时性问题\n`;
    report += `- 交易可能被节点拒绝或丢弃\n`;
  }
  
  return report;
}

// 主函数
async function main() {
  try {
    console.log('读取实验结果...');
    const data = fs.readFileSync(CONFIG.inputFile, 'utf8');
    const results = JSON.parse(data);
    
    console.log(`成功读取 ${results.length} 条结果记录`);
    
    console.log('生成验证报告...');
    const report = generateValidationReport(results);
    
    console.log('保存报告...');
    fs.writeFileSync(CONFIG.validationReport, report);
    
    console.log(`报告已成功保存到: ${CONFIG.validationReport}`);
  } catch (error) {
    console.error('程序执行出错:', error.message);
  }
}

// 运行程序
main();