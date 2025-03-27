const Web3 = require('web3');
const fs = require('fs');
const web3 = new Web3('http://localhost:8124'); // 连接到您的XLayer节点

// 配置参数
const CONFIG = {
  // 测试配置
  iterationsPerGP: 2,                      // 每个Gas价格倍数发送的交易次数
  delayBetweenTxs: 5000,                   // 交易之间的延迟（毫秒）
  confirmationTimeout: 180000,              // 交易确认超时时间（毫秒）- 增加到3分钟
  
  // Gas价格配置
  highGasPriceFirst: true,                  // 是否优先发送高Gas价格的交易
  gasPriceMultipliers: [1, 50, 200, 500],   // Gas价格乘数（相对于当前网络价格）- 大幅增加
  
  // 账户配置
  privateKey: '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2',
  toAddress: '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3',
  
  // 交易配置
  transactionValue: '0.01',   // 每笔交易金额（ETH/OKB）
  gas: 21000,                 // 标准转账Gas
  
  // 输出配置
  outputFile: './gp_comparison_results_extreme.json',  // 结果输出文件
  reportFile: './gp_comparison_report_extreme.md',     // 报告文件
};

// 存储所有交易结果
const results = [];

async function getPoolInfo() {
  try {
    // 使用curl命令获取交易池状态
    const { execSync } = require('child_process');
    const curlCommand = `curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"txpool_status","params":[],"id":1}' ${CONFIG.rpcUrl || 'http://localhost:8124'}`;
    
    const curlOutput = execSync(curlCommand);
    const response = JSON.parse(curlOutput.toString());

    if (response.error) {
      console.error('获取交易池状态失败:', response.error.message);
      return { pendingCount: 'N/A', queuedCount: 'N/A' };
    }

    const pendingCount = parseInt(response.result.pending, 16);
    const queuedCount = parseInt(response.result.queued, 16);
    
    // 获取当前Gas价格建议
    const gasPrice = await web3.eth.getGasPrice();
    
    return { 
      pendingCount, 
      queuedCount, 
      totalCount: pendingCount + queuedCount,
      gasPrice
    };
  } catch (error) {
    console.error('获取交易池信息失败:', error.message);
    return { pendingCount: 'N/A', queuedCount: 'N/A', totalCount: 'N/A', gasPrice: 'N/A' };
  }
}

async function sendTransaction(gasPriceMultiplier, txIndex) {
  try {
    // 结果记录对象
    const result = {
      index: txIndex,
      gasPriceMultiplier,
      timestamp: new Date().toISOString(),
      beforePool: null,
      transactionDetails: null,
      confirmationTime: null,
      status: 'pending'
    };
    
    // 私钥配置
    const privateKey = CONFIG.privateKey;
    
    // 获取账户信息
    const account = web3.eth.accounts.privateKeyToAccount(privateKey);
    const fromAddress = account.address;
    
    // 接收地址
    const toAddress = CONFIG.toAddress;
    
    console.log(`\n===== 交易 #${txIndex} (Gas价格倍数: ${gasPriceMultiplier}x) =====`);
    console.log(`发送地址: ${fromAddress}`);
    console.log(`接收地址: ${toAddress}`);
    
    // 获取当前nonce
    const nonce = await web3.eth.getTransactionCount(fromAddress, 'pending');
    console.log(`当前nonce: ${nonce}`);
    
    // 获取当前Gas价格
    const currentGasPrice = await web3.eth.getGasPrice();
    console.log(`网络Gas价格: ${web3.utils.fromWei(currentGasPrice, 'gwei')} Gwei`);
    
    // 设置我们的Gas价格为网络当前价格的倍数
    const ourGasPrice = BigInt(Math.floor(Number(currentGasPrice) * gasPriceMultiplier));
    console.log(`使用的Gas价格: ${web3.utils.fromWei(ourGasPrice.toString(), 'gwei')} Gwei (网络价格的${gasPriceMultiplier}倍)`);
    
    // 交易参数
    const txParams = {
      from: fromAddress,
      to: toAddress,
      value: web3.utils.toWei(CONFIG.transactionValue, 'ether'),
      gas: CONFIG.gas,
      gasPrice: ourGasPrice.toString(), 
      nonce: nonce
    };
    
    // 获取交易池状态（发送前）
    console.log('\n交易发送前:');
    const beforePool = await getPoolInfo();
    console.log(`交易池状态: 待处理: ${beforePool.pendingCount}, 排队中: ${beforePool.queuedCount}, 总计: ${beforePool.totalCount}`);
    console.log(`当前Gas价格: ${web3.utils.fromWei(beforePool.gasPrice, 'gwei')} Gwei`);
    result.beforePool = beforePool;
    
    // 签名交易
    const signedTx = await web3.eth.accounts.signTransaction(txParams, privateKey);
    
    console.log('\n开始发送交易...');
    const startTime = Date.now();
    
    // 添加超时处理
    return new Promise((resolve) => {
      // 设置超时处理
      const timeoutId = setTimeout(() => {
        console.log(`\n交易 #${txIndex} 确认超时! 已等待超过${CONFIG.confirmationTimeout / 1000}秒`);
        result.status = 'timeout';
        result.error = `交易确认超时，已等待${CONFIG.confirmationTimeout / 1000}秒`;
        resolve(result);
      }, CONFIG.confirmationTimeout);
      
      // 交易发送和监听
      let txHash;
      web3.eth.sendSignedTransaction(signedTx.rawTransaction)
        .on('transactionHash', async (hash) => {
          txHash = hash;
          console.log(`交易哈希: ${hash}`);
          console.log(`生成交易哈希用时: ${(Date.now() - startTime)}ms`);
          result.transactionHash = hash;
          result.hashGenerationTime = Date.now() - startTime;
        })
        .on('receipt', async (receipt) => {
          clearTimeout(timeoutId); // 清除超时处理
          
          const confirmationTime = Date.now() - startTime;
          console.log(`\n交易确认! 区块号: ${receipt.blockNumber}`);
          console.log(`总用时: ${confirmationTime}ms`);
          result.confirmationTime = confirmationTime;
          
          // 获取交易详情
          try {
            const tx = await web3.eth.getTransaction(txHash);
            console.log('\n交易详情:');
            console.log(`Gas价格: ${web3.utils.fromWei(tx.gasPrice, 'gwei')} Gwei`);
            console.log(`使用的Gas: ${receipt.gasUsed}`);
            result.transactionDetails = {
              blockNumber: receipt.blockNumber,
              gasPrice: tx.gasPrice,
              gasUsed: receipt.gasUsed,
              status: receipt.status
            };
            
            // 获取区块信息
            const block = await web3.eth.getBlock(receipt.blockNumber);
            console.log('\n区块信息:');
            console.log(`区块时间: ${new Date(block.timestamp * 1000).toLocaleString()}`);
            console.log(`区块中的交易数量: ${block.transactions.length}`);
            result.blockInfo = {
              timestamp: block.timestamp,
              transactionCount: block.transactions.length,
              blockNumber: receipt.blockNumber
            };
          } catch (err) {
            console.error('获取交易或区块详情失败:', err.message);
          }
          
          result.status = 'confirmed';
          resolve(result);
        })
        .on('error', (error) => {
          clearTimeout(timeoutId); // 清除超时处理
          console.error('交易错误:', error);
          result.status = 'error';
          result.error = error.message;
          resolve(result);
        });
    });
  } catch (error) {
    console.error(`发送交易 #${txIndex} 失败:`, error);
    return {
      index: txIndex,
      gasPriceMultiplier,
      status: 'error',
      error: error.message,
      timestamp: new Date().toISOString()
    };
  }
}

// 等待函数
async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// 运行实验
async function runExperiment() {
  // 获取交易顺序
  let multipliers = [];
  let totalTxs = 0;
  
  // 根据配置生成交易列表
  CONFIG.gasPriceMultipliers.forEach(multiplier => {
    for (let i = 0; i < CONFIG.iterationsPerGP; i++) {
      multipliers.push(multiplier);
      totalTxs++;
    }
  });
  
  // 根据配置决定是否先发送高Gas价格交易
  if (CONFIG.highGasPriceFirst) {
    multipliers.sort((a, b) => b - a); // 从高到低排序
  }
  
  console.log(`开始Gas价格比较实验: 每个Gas价格将发送 ${CONFIG.iterationsPerGP} 次交易`);
  console.log(`测试的Gas价格倍数: ${CONFIG.gasPriceMultipliers.join(', ')}`);
  console.log(`总计将发送 ${totalTxs} 笔交易`);
  console.log(`交易顺序: ${CONFIG.highGasPriceFirst ? '从高到低' : '原始顺序'}`);
  
  // 逐个发送交易
  for (let i = 0; i < multipliers.length; i++) {
    try {
      const multiplier = multipliers[i];
      const result = await sendTransaction(multiplier, i + 1);
      results.push(result);
      
      // 保存中间结果
      fs.writeFileSync(CONFIG.outputFile, JSON.stringify(results, null, 2));
      
      // 如果不是最后一笔交易，等待延迟时间
      if (i < multipliers.length - 1) {
        console.log(`等待 ${CONFIG.delayBetweenTxs / 1000} 秒后发送下一笔交易...`);
        await sleep(CONFIG.delayBetweenTxs);
      }
    } catch (error) {
      console.error(`发送交易 #${i+1} 时出现未预期的错误:`, error);
      
      // 记录错误并继续
      results.push({
        index: i + 1,
        gasPriceMultiplier: multipliers[i],
        status: 'critical_error',
        error: error.message,
        timestamp: new Date().toISOString()
      });
      
      // 保存中间结果
      fs.writeFileSync(CONFIG.outputFile, JSON.stringify(results, null, 2));
      
      // 短暂休息后继续
      await sleep(3000);
    }
  }
  
  // 生成报告
  const report = generateReport();
  fs.writeFileSync(CONFIG.reportFile, report);
  
  console.log('\n================ 实验完成 ================');
  console.log(`成功交易数: ${results.filter(r => r.status === 'confirmed').length}/${totalTxs}`);
  console.log(`失败交易数: ${results.filter(r => r.status === 'error').length}/${totalTxs}`);
  console.log(`超时交易数: ${results.filter(r => r.status === 'timeout').length}/${totalTxs}`);
  console.log(`详细结果已保存到: ${CONFIG.outputFile}`);
  console.log(`分析报告已保存到: ${CONFIG.reportFile}`);
}

// 生成报告
function generateReport() {
  // 按Gas价格倍数分组
  const byMultiplier = {};
  
  // 按交易状态分组
  const byStatus = {
    confirmed: results.filter(r => r.status === 'confirmed'),
    error: results.filter(r => r.status === 'error'),
    timeout: results.filter(r => r.status === 'timeout')
  };
  
  // 只考虑确认成功的交易
  byStatus.confirmed.forEach(tx => {
    if (!byMultiplier[tx.gasPriceMultiplier]) {
      byMultiplier[tx.gasPriceMultiplier] = [];
    }
    byMultiplier[tx.gasPriceMultiplier].push(tx);
  });
  
  // 计算每个Gas价格倍数的统计数据
  const stats = Object.keys(byMultiplier).map(multiplier => {
    const txs = byMultiplier[multiplier];
    const times = txs.map(tx => tx.confirmationTime);
    
    return {
      multiplier: parseFloat(multiplier),
      avgTime: times.length > 0 ? times.reduce((a, b) => a + b, 0) / times.length : 'N/A',
      minTime: times.length > 0 ? Math.min(...times) : 'N/A',
      maxTime: times.length > 0 ? Math.max(...times) : 'N/A',
      samples: txs.length,
      successRate: (txs.length / CONFIG.iterationsPerGP * 100).toFixed(2)
    };
  }).sort((a, b) => a.multiplier - b.multiplier);
  
  // 生成报告内容
  let report = `# Gas价格对交易确认时间的影响分析\n\n`;
  report += `实验时间: ${new Date().toLocaleString()}\n\n`;
  
  report += `## 实验配置\n\n`;
  report += `- 每个Gas价格发送的交易次数: ${CONFIG.iterationsPerGP}\n`;
  report += `- 交易之间的延迟: ${CONFIG.delayBetweenTxs / 1000} 秒\n`;
  report += `- 交易确认超时: ${CONFIG.confirmationTimeout / 1000} 秒\n`;
  report += `- 测试的Gas价格倍数: ${CONFIG.gasPriceMultipliers.join(', ')}\n`;
  report += `- 交易顺序: ${CONFIG.highGasPriceFirst ? '从高到低' : '原始顺序'}\n\n`;
  
  report += `## 交易结果概览\n\n`;
  report += `- 总交易数: ${results.length}\n`;
  report += `- 成功交易数: ${byStatus.confirmed.length}\n`;
  report += `- 失败交易数: ${byStatus.error.length}\n`;
  report += `- 超时交易数: ${byStatus.timeout.length}\n\n`;
  
  report += `## Gas价格与确认时间的关系\n\n`;
  report += `| Gas价格倍数 | 平均确认时间 | 最短确认时间 | 最长确认时间 | 成功率 | 样本数 |\n`;
  report += `|------------|----------------|----------------|----------------|---------|--------|\n`;
  
  stats.forEach(stat => {
    let avgTime = stat.avgTime === 'N/A' ? 'N/A' : `${(stat.avgTime / 1000).toFixed(2)}秒`;
    let minTime = stat.minTime === 'N/A' ? 'N/A' : `${(stat.minTime / 1000).toFixed(2)}秒`;
    let maxTime = stat.maxTime === 'N/A' ? 'N/A' : `${(stat.maxTime / 1000).toFixed(2)}秒`;
    
    report += `| ${stat.multiplier}x | ${avgTime} | ${minTime} | ${maxTime} | ${stat.successRate}% | ${stat.samples}/${CONFIG.iterationsPerGP} |\n`;
  });
  
  report += `\n## 发现与分析\n\n`;
  
  // 找出确认时间最短的Gas价格倍数
  let fastestGP = stats.length > 0 ? stats.reduce((fastest, current) => {
    if (current.avgTime === 'N/A') return fastest;
    if (fastest.avgTime === 'N/A') return current;
    return current.avgTime < fastest.avgTime ? current : fastest;
  }, { avgTime: 'N/A' }) : null;
  
  // 成功率最高的Gas价格倍数
  let mostReliableGP = stats.length > 0 ? stats.reduce((reliable, current) => {
    return parseFloat(current.successRate) > parseFloat(reliable.successRate) ? current : reliable;
  }, { successRate: '0' }) : null;
  
  if (fastestGP && fastestGP.avgTime !== 'N/A') {
    report += `1. **最快确认时间**: Gas价格倍数为 **${fastestGP.multiplier}x** 时，平均确认时间为 **${(fastestGP.avgTime / 1000).toFixed(2)}秒**。\n`;
  }
  
  if (mostReliableGP) {
    report += `2. **最高成功率**: Gas价格倍数为 **${mostReliableGP.multiplier}x** 时，成功率为 **${mostReliableGP.successRate}%**。\n`;
  }
  
  // 根据统计结果提供最优Gas价格建议
  report += `\n## 最优Gas价格建议\n\n`;
  
  if (stats.length > 0) {
    // 找到一个平衡点：成功率高且确认速度快的Gas价格
    const balancedGP = stats.filter(s => s.avgTime !== 'N/A' && parseFloat(s.successRate) > 80)
      .sort((a, b) => a.avgTime - b.avgTime)[0];
    
    if (balancedGP) {
      report += `- **推荐Gas价格倍数**: ${balancedGP.multiplier}x\n`;
      report += `- **预期确认时间**: ${(balancedGP.avgTime / 1000).toFixed(2)}秒\n`;
      report += `- **成功率**: ${balancedGP.successRate}%\n`;
    } else {
      report += `- 根据当前实验数据，无法提供可靠的Gas价格建议。建议增加实验样本量或调整Gas价格范围。\n`;
    }
  } else {
    report += `- 没有足够的成功交易数据来提供可靠的Gas价格建议。\n`;
  }
  
  report += `\n## 交易池状态\n\n`;
  
  // 获取交易池状态
  const poolStats = {};
  results.forEach(tx => {
    if (tx.beforePool && tx.beforePool.pendingCount !== 'N/A') {
      poolStats.pendingCount = tx.beforePool.pendingCount;
      poolStats.queuedCount = tx.beforePool.queuedCount;
      poolStats.totalCount = tx.beforePool.totalCount;
    }
  });
  
  if (poolStats.pendingCount) {
    report += `实验期间的交易池状态:\n`;
    report += `- 待处理交易: 约 ${poolStats.pendingCount} 笔\n`;
    report += `- 排队交易: 约 ${poolStats.queuedCount} 笔\n`;
    report += `- 总计: 约 ${poolStats.totalCount} 笔\n`;
  } else {
    report += `未能获取有效的交易池状态数据。\n`;
  }
  
  return report;
}

// 执行实验
runExperiment(); 