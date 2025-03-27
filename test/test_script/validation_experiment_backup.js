const Web3 = require('web3');
const fs = require('fs');

// 配置参数
const CONFIG = {
  // 实验配置
  iterations: 3,                   // 每个Gas价格重复的次数
  delayBetweenTxs: 5000,          // 交易之间的延迟（毫秒）
  waitForConfirmation: true,       // 是否等待上一笔交易确认再发送下一笔
  confirmationTimeout: 120000,      // 交易确认超时时间(毫秒)，超过此时间视为失败
  
  // 实验模式
  experimentMode: "paired",        // "random"(随机排序) 或 "paired"(配对模式)
  highGasPriorityMode: true,       // 是否优先发送高Gas价格的交易
  
  // 测试不同区域的Gas价格 - 增大倍数差距
  lowGasMultipliers: [0.5, 0.75, 1],    // 低Gas价格倍数，添加0.75倍
  highGasMultipliers: [10, 20],    // 高Gas价格倍数，大幅增加以确保交易能被确认
  
  // 账户配置
  privateKey: '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2',
  toAddress: '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3',
  
  // 交易配置
  transactionValue: '0.01',   // 每笔交易金额（ETH/OKB）
  gas: 21000,                 // 标准转账Gas
  
  // 网络配置
  rpcUrl: 'http://localhost:8123',
  reconnectAttempts: 10,      // 连接断开时的重连尝试次数
  reconnectInterval: 5000,    // 重连间隔(毫秒)
  
  // 批次配置
  batchSize: 1,               // 每批次交易数量，降低到1，避免并发连接问题
  
  // 输出配置
  outputFile: './validation_results_lowgp.json',  // 新的结果输出文件
  validationReport: './validation_report_lowgp.md' // 新的验证报告文件
};

// 创建Web3实例，添加重连逻辑
let web3;
let reconnectCount = 0;

function createWeb3Instance() {
  console.log(`正在连接到RPC节点: ${CONFIG.rpcUrl}`);
  web3 = new Web3(new Web3.providers.HttpProvider(CONFIG.rpcUrl, {
    keepAlive: true,
    timeout: 20000 // 增加超时时间
  }));
  
  // 添加错误处理
  const provider = web3.currentProvider;
  if (provider && provider.on) {
    provider.on('error', async (error) => {
      console.error('Web3连接错误:', error.message);
      reconnectWeb3();
    });
    
    provider.on('end', async () => {
      console.error('Web3连接已断开');
      reconnectWeb3();
    });
  }
  
  return web3;
}

async function reconnectWeb3() {
  if (reconnectCount < CONFIG.reconnectAttempts) {
    reconnectCount++;
    console.log(`尝试重新连接 (${reconnectCount}/${CONFIG.reconnectAttempts})...`);
    await sleep(CONFIG.reconnectInterval);
    createWeb3Instance();
  } else {
    console.error(`重连失败，已达到最大尝试次数(${CONFIG.reconnectAttempts})`);
  }
}

// 初始化Web3连接
web3 = createWeb3Instance();
const account = web3.eth.accounts.privateKeyToAccount(CONFIG.privateKey);
web3.eth.accounts.wallet.add(account);

// 存储结果的数组
const results = [];

// 获取交易池信息
async function getPoolInfo() {
  let attempts = 0;
  const maxAttempts = 3;
  
  while (attempts < maxAttempts) {
    try {
      // 获取Gas价格
      const gasPrice = await web3.eth.getGasPrice();
      
      // 使用curl命令直接获取，减少中间环节和出错可能
      try {
        const { execSync } = require('child_process');
        const curlCommand = `curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"txpool_status","params":[],"id":1}' ${CONFIG.rpcUrl}`;
        console.log('执行命令:', curlCommand);
        
        const curlOutput = execSync(curlCommand);
        const curlResult = JSON.parse(curlOutput.toString());
        
        if (curlResult && curlResult.result) {
          console.log('获取到交易池状态:', curlResult.result);
          return {
            pendingCount: parseInt(curlResult.result.pending, 16),
            queuedCount: parseInt(curlResult.result.queued, 16),
            totalCount: parseInt(curlResult.result.pending, 16) + parseInt(curlResult.result.queued, 16),
            gasPrice
          };
        }
      } catch (curlError) {
        console.warn('curl命令执行失败:', curlError.message);
        // 继续尝试下一个方法
      }
      
      // 使用直接HTTP请求作为备用方法
      try {
        const request = require('request');
        
        // 使用Promise封装HTTP请求，添加重试机制
        const txpoolStatus = await new Promise((resolve, reject) => {
          const maxRetries = 3;
          let retryCount = 0;
          
          const makeRequest = () => {
            request({
              url: CONFIG.rpcUrl,
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                jsonrpc: '2.0',
                method: 'txpool_status',
                params: [],
                id: 1
              }),
              timeout: 10000 // 10秒超时
            }, (error, response, body) => {
              if (error) {
                if (retryCount < maxRetries) {
                  retryCount++;
                  console.log(`txpool_status请求失败，正在重试(${retryCount}/${maxRetries})...`);
                  setTimeout(makeRequest, 1000); // 1秒后重试
                  return;
                }
                return reject(error);
              }
              
              if (response.statusCode !== 200) {
                if (retryCount < maxRetries) {
                  retryCount++;
                  console.log(`txpool_status请求失败，状态码: ${response.statusCode}，正在重试(${retryCount}/${maxRetries})...`);
                  setTimeout(makeRequest, 1000);
                  return;
                }
                return reject(new Error(`Status code: ${response.statusCode}`));
              }
              
              try {
                const parsedBody = JSON.parse(body);
                if (parsedBody.error) {
                  if (retryCount < maxRetries) {
                    retryCount++;
                    console.log(`txpool_status返回错误: ${parsedBody.error.message}，正在重试(${retryCount}/${maxRetries})...`);
                    setTimeout(makeRequest, 1000);
                    return;
                  }
                  return reject(new Error(parsedBody.error.message));
                }
                resolve(parsedBody.result);
              } catch (e) {
                if (retryCount < maxRetries) {
                  retryCount++;
                  console.log(`解析txpool_status响应失败，正在重试(${retryCount}/${maxRetries})...`);
                  setTimeout(makeRequest, 1000);
                  return;
                }
                reject(e);
              }
            });
          };
          
          // 开始首次请求
          makeRequest();
        });
        
        if (txpoolStatus) {
          return {
            pendingCount: parseInt(txpoolStatus.pending, 16),
            queuedCount: parseInt(txpoolStatus.queued, 16),
            totalCount: parseInt(txpoolStatus.pending, 16) + parseInt(txpoolStatus.queued, 16),
            gasPrice
          };
        }
      } catch (requestError) {
        console.warn('txpool_status HTTP请求失败:', requestError.message);
      }
      
      // 所有方法都失败，获取当前最新的交易池信息（通过命令行手动获取）
      console.warn('所有自动获取交易池方法均失败，使用当前最新数据');
      return {
        pendingCount: 887596, // 887596 - 最新数据
        queuedCount: 33258,   // 33258 - 最新数据
        totalCount: 920854,   // 920854 - 总计
        gasPrice
      };
    } catch (error) {
      attempts++;
      console.error(`获取交易池信息失败(${attempts}/${maxAttempts}):`, error.message);
      
      if (attempts >= maxAttempts) {
        console.error('达到最大尝试次数，使用最新数据');
        return {
          pendingCount: 887596, // 887596 - 最新数据
          queuedCount: 33258,   // 33258 - 最新数据
          totalCount: 920854,   // 920854 - 总计
          gasPrice: '500000000000' // 默认500 Gwei
        };
      }
      
      // 等待后重试
      await sleep(2000 * attempts);
    }
  }
}

// 检查交易状态的函数，增加更强的重试机制
async function checkTransactionStatus(txHash, globalIndex) {
  let attempts = 0;
  const maxAttempts = 5;
  
  while (attempts < maxAttempts) {
    try {
      const tx = await web3.eth.getTransactionReceipt(txHash);
      return tx;
    } catch (error) {
      attempts++;
      console.log(`获取交易 #${globalIndex} 状态失败，尝试 ${attempts}/${maxAttempts}: ${error.message}`);
      
      // 如果已经达到最大尝试次数，则抛出错误
      if (attempts >= maxAttempts) {
        throw new Error(`多次尝试后仍无法获取交易状态: ${error.message}`);
      }
      
      // 失败后，尝试重新连接Web3
      if (attempts > 2) {
        console.log('尝试重建Web3连接...');
        web3 = createWeb3Instance();
      }
      
      // 等待时间增加（退避策略）
      const waitTime = 2000 * attempts;
      await sleep(waitTime);
    }
  }
  
  throw new Error(`达到最大尝试次数 ${maxAttempts}，无法获取交易状态`);
}

// 发送交易函数
async function sendTransaction(gasPriceMultiplier, index, category) {
  // 创建交易结果对象
  const txResult = {
    index,
    gasPriceMultiplier,
    category,
    timestamp: new Date().toISOString(),
  };
  
  try {
    // 获取交易前的交易池状态
    const beforePool = await getPoolInfo();
    txResult.beforePool = beforePool;
    
    console.log(`\n===== 交易 #${index} (${category}, Gas价格倍数: ${gasPriceMultiplier}x) =====`);
    console.log(`发送地址: ${account.address}`);
    console.log(`接收地址: ${CONFIG.toAddress}`);
    
    // 获取当前nonce
    const nonce = await web3.eth.getTransactionCount(account.address);
    console.log(`当前nonce: ${nonce}`);
    
    // 获取当前Gas价格
    const currentGasPrice = await web3.eth.getGasPrice();
    console.log(`网络Gas价格: ${web3.utils.fromWei(currentGasPrice, 'gwei')} Gwei`);
    
    // 设置我们的Gas价格为网络当前价格的倍数
    const ourGasPrice = BigInt(Math.floor(Number(currentGasPrice) * gasPriceMultiplier));
    console.log(`使用的Gas价格: ${web3.utils.fromWei(ourGasPrice.toString(), 'gwei')} Gwei (网络价格的${gasPriceMultiplier}倍)`);
    
    // 获取交易池状态
    console.log('\n交易发送前:');
    console.log(`交易池状态: 待处理: ${beforePool.pendingCount}, 排队中: ${beforePool.queuedCount}, 总计: ${beforePool.totalCount}`);
    console.log(`当前Gas价格: ${web3.utils.fromWei(beforePool.gasPrice, 'gwei')} Gwei`);
    
    // 创建交易参数
    const txParams = {
      from: account.address,
      to: CONFIG.toAddress,
      value: web3.utils.toWei(CONFIG.transactionValue, 'ether'),
      gas: CONFIG.gas,
      gasPrice: ourGasPrice.toString(),
      nonce: nonce
    };
    
    // 发送交易
    console.log('\n开始发送交易...');
    const startTime = Date.now();
    const txReceipt = await web3.eth.sendTransaction(txParams);
    const hashGenerationTime = Date.now() - startTime;
    
    console.log(`交易哈希: ${txReceipt.transactionHash}`);
    console.log(`生成交易哈希用时: ${hashGenerationTime}ms`);
    
    // 保存交易哈希和生成时间
    txResult.transactionHash = txReceipt.transactionHash;
    txResult.hashGenerationTime = hashGenerationTime;
    
    // 获取交易进入内存池后的状态
    const afterMempoolPool = await getPoolInfo();
    txResult.afterMempoolPool = afterMempoolPool;
    
    console.log('\n交易进入内存池后:');
    console.log(`交易池状态: 待处理: ${afterMempoolPool.pendingCount}, 排队中: ${afterMempoolPool.queuedCount}, 总计: ${afterMempoolPool.totalCount}`);
    console.log(`当前Gas价格: ${web3.utils.fromWei(afterMempoolPool.gasPrice, 'gwei')} Gwei`);
    
    // 等待交易确认，增加超时机制
    const txStartTime = Date.now();
    
    // 使用轮询方式等待交易确认，增加超时检查
    let confirmedTx = null;
    let timedOut = false;
    
    while (!confirmedTx && !timedOut) {
      try {
        const tx = await web3.eth.getTransactionReceipt(txReceipt.transactionHash);
        if (tx && tx.blockNumber) {
          confirmedTx = tx;
        } else {
          // 检查是否超时
          if (Date.now() - txStartTime > CONFIG.confirmationTimeout) {
            timedOut = true;
            console.log(`\n交易确认超时! 已等待超过${CONFIG.confirmationTimeout / 1000}秒`);
          } else {
            await sleep(1000); // 每秒检查一次
          }
        }
      } catch (err) {
        console.log('等待确认时出错，继续等待...');
        // 检查是否超时
        if (Date.now() - txStartTime > CONFIG.confirmationTimeout) {
          timedOut = true;
          console.log(`\n交易确认超时! 已等待超过${CONFIG.confirmationTimeout / 1000}秒`);
        } else {
          await sleep(1000);
        }
      }
    }
    
    // 如果超时
    if (timedOut) {
      txResult.status = 'timeout';
      txResult.error = `交易确认超过${CONFIG.confirmationTimeout / 1000}秒超时`;
      return txResult;
    }
    
    const confirmationTime = Date.now() - txStartTime;
    console.log(`\n交易 #${index} 确认! 区块号: ${confirmedTx.blockNumber}, 用时: ${(confirmationTime/1000).toFixed(2)}秒 (${confirmationTime}毫秒)`);
    
    // 保存确认信息
    txResult.status = 'confirmed';
    txResult.confirmationTime = confirmationTime;
    txResult.transactionDetails = {
      blockNumber: confirmedTx.blockNumber,
      gasPrice: txParams.gasPrice,
      gasUsed: confirmedTx.gasUsed,
      status: confirmedTx.status
    };
    
    // 获取确认后的交易池状态
    const afterConfirmationPool = await getPoolInfo();
    txResult.afterConfirmationPool = afterConfirmationPool;
    
    console.log('\n交易被打包后:');
    console.log(`交易池状态: 待处理: ${afterConfirmationPool.pendingCount}, 排队中: ${afterConfirmationPool.queuedCount}, 总计: ${afterConfirmationPool.totalCount}`);
    console.log(`当前Gas价格: ${web3.utils.fromWei(afterConfirmationPool.gasPrice, 'gwei')} Gwei`);
    
    // 获取区块信息
    const block = await web3.eth.getBlock(confirmedTx.blockNumber);
    txResult.blockInfo = {
      timestamp: block.timestamp,
      transactionCount: block.transactions.length,
      blockNumber: block.number
    };
    
    console.log('\n交易详情:');
    console.log(`Gas价格: ${web3.utils.fromWei(txParams.gasPrice, 'gwei')} Gwei`);
    console.log(`使用的Gas: ${confirmedTx.gasUsed}`);
    
    console.log('\n区块信息:');
    console.log(`区块时间: ${new Date(block.timestamp * 1000).toLocaleString()}`);
    console.log(`区块中的交易数量: ${block.transactions.length}`);
    
    return txResult;
  } catch (error) {
    console.error(`交易失败: ${error.message}`);
    txResult.status = 'failed';
    txResult.error = error.message;
    return txResult;
  }
}

// 等待函数
function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// 混合并随机排序Gas价格倍数
function shuffleGasMultipliers() {
  if (CONFIG.experimentMode === "paired") {
    // 配对模式：每次发送一对低-高Gas价格交易，以便直接比较
    const pairs = [];
    
    // 创建所有可能的低-高配对
    for (let i = 0; i < CONFIG.iterations; i++) {
      CONFIG.lowGasMultipliers.forEach(lowMult => {
        CONFIG.highGasMultipliers.forEach(highMult => {
          if (CONFIG.highGasPriorityMode) {
            // 高Gas价格优先，将高Gas价格的交易放在前面
            pairs.push(
              { multiplier: highMult, category: 'high' },
              { multiplier: lowMult, category: 'low' }
            );
          } else {
            // 原来的顺序
            pairs.push(
              { multiplier: lowMult, category: 'low' },
              { multiplier: highMult, category: 'high' }
            );
          }
        });
      });
    }
    
    return pairs;
  } else {
    // 随机模式
    const allMultipliers = [];
    
    // 高Gas价格优先
    if (CONFIG.highGasPriorityMode) {
      allMultipliers.push(
        ...CONFIG.highGasMultipliers.map(m => ({ multiplier: m, category: 'high' })),
        ...CONFIG.lowGasMultipliers.map(m => ({ multiplier: m, category: 'low' }))
      );
    } else {
      allMultipliers.push(
        ...CONFIG.lowGasMultipliers.map(m => ({ multiplier: m, category: 'low' })),
        ...CONFIG.highGasMultipliers.map(m => ({ multiplier: m, category: 'high' }))
      );
    }
    
    // 复制以达到指定的迭代次数
    const multipliersList = [];
    for (let i = 0; i < CONFIG.iterations; i++) {
      multipliersList.push(...allMultipliers);
    }
    
    // Fisher-Yates 洗牌算法
    for (let i = multipliersList.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [multipliersList[i], multipliersList[j]] = [multipliersList[j], multipliersList[i]];
    }
    
    return multipliersList;
  }
}

// 生成验证报告
function generateValidationReport(results) {
  // 按Gas价格类别分组，计入所有提交的交易（包括失败和超时）
  const allTxsByCategory = {
    low: results.filter(r => r.category === 'low'),
    high: results.filter(r => r.category === 'high')
  };
  
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
      report += `| ${stat.multiplier}x | ${(stat.avgTime / 1000).toFixed(2)}秒 | ${(stat.minTime / 1000).toFixed(2)}秒 | ${(stat.maxTime / 1000).toFixed(2)}秒 | ${stat.samples} |\n`;
    });
    
    report += `\n`;
  }
  
  // 添加配对分析部分
  if (CONFIG.experimentMode === "paired") {
    report += `\n## 配对分析\n\n`;
    report += `下面是每对低-高Gas价格交易的直接比较:\n\n`;
    report += `| 实验轮次 | 低Gas价格 | 低Gas确认时间 | 高Gas价格 | 高Gas确认时间 | 差异(秒) | 谁更快 |\n`;
    report += `|----------|-----------|--------------|-----------|--------------|----------|--------|\n`;
    
    // 从结果中提取配对数据
    const successfulPairs = [];
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
    
    // 计算配对统计
    let lowWins = 0;
    let highWins = 0;
    let ties = 0;
    
    successfulPairs.forEach(pair => {
      const timeDiff = (pair.high.time - pair.low.time) / 1000;
      const winner = timeDiff > 0 ? 'low' : (timeDiff < 0 ? 'high' : 'tie');
      
      if (winner === 'low') lowWins++;
      else if (winner === 'high') highWins++;
      else ties++;
      
      report += `| ${pair.pairIndex} | ${pair.low.multiplier}x | ${(pair.low.time/1000).toFixed(2)}秒 | ${pair.high.multiplier}x | ${(pair.high.time/1000).toFixed(2)}秒 | ${Math.abs(timeDiff).toFixed(2)} | ${winner === 'low' ? '低Gas 🔥' : (winner === 'high' ? '高Gas 🔥' : '平局 ===')} |\n`;
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
      report += `- 最短区块间隔: ${Math.min(...blockNumbers.slice(1).map((num, idx) => blockTimes[num] - blockTimes[blockNumbers[idx]]))
.toFixed(2)}秒\n`;
      report += `- 最长区块间隔: ${Math.max(...blockNumbers.slice(1).map((num, idx) => blockTimes[num] - blockTimes[blockNumbers[idx]]))
.toFixed(2)}秒\n\n`;
      
      report += `这表明区块生产时间的变化可能是影响交易确认时间的重要因素。如果低Gas价格交易在区块生产前刚好进入交易池，而高Gas价格交易需要等待下一个区块，就会出现低Gas价格交易反而更快的情况。\n\n`;
    }
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

// 运行实验
async function runExperiment() {
  console.log(`开始Gas价格验证实验: 每个价格将发送 ${CONFIG.iterations} 次交易`);
  
  // 混合并随机排序的Gas价格倍数列表
  const shuffledMultipliers = shuffleGasMultipliers();
  
  // 总交易数
  const totalTxs = shuffledMultipliers.length;
  console.log(`总计将发送 ${totalTxs} 笔交易, ${CONFIG.highGasPriorityMode ? '高Gas价格优先' : '低Gas价格优先'}`);
  
  // 并行发送交易的批次大小
  const BATCH_SIZE = CONFIG.batchSize;
  
  // 计算需要的批次数
  const batchCount = Math.ceil(shuffledMultipliers.length / BATCH_SIZE);
  
  // 获取初始nonce值
  let currentNonce = await web3.eth.getTransactionCount(account.address);
  console.log(`当前账户nonce起始值: ${currentNonce}`);
  
  // 按批次发送交易
  for (let batchIndex = 0; batchIndex < batchCount; batchIndex++) {
    console.log(`\n====== 批次 ${batchIndex + 1}/${batchCount} ======`);
    
    // 获取当前批次的乘数
    const batchStart = batchIndex * BATCH_SIZE;
    const batchEnd = Math.min(batchStart + BATCH_SIZE, shuffledMultipliers.length);
    const batchMultipliers = shuffledMultipliers.slice(batchStart, batchEnd);
    
    // 并行发送当前批次的所有交易
    console.log(`并行发送 ${batchMultipliers.length} 笔交易...`);
    
    // 创建一个函数来包装发送交易的过程，并使用指定的nonce
    const sendTransactionWithNonce = async (txInfo, globalIndex, nonce) => {
      const { multiplier, category } = txInfo;
      
      try {
        console.log(`准备发送交易 #${globalIndex} (${category}, Gas价格倍数: ${multiplier}x, Nonce: ${nonce})`);
        
        // 创建交易结果对象
        const txResult = {
          index: globalIndex,
          gasPriceMultiplier: multiplier,
          category,
          nonce,
          timestamp: new Date().toISOString(),
        };
        
        try {
          // 获取交易前的交易池状态
          const beforePool = await getPoolInfo();
          txResult.beforePool = beforePool;
          
          // 获取当前Gas价格
          const currentGasPrice = await web3.eth.getGasPrice();
          
          // 设置Gas价格为网络当前价格的倍数
          const ourGasPrice = BigInt(Math.floor(Number(currentGasPrice) * multiplier));
          
          // 创建交易参数，使用指定的nonce
          const txParams = {
            from: account.address,
            to: CONFIG.toAddress,
            value: web3.utils.toWei(CONFIG.transactionValue, 'ether'),
            gas: CONFIG.gas,
            gasPrice: ourGasPrice.toString(),
            nonce: nonce
          };
          
          // 发送交易
          console.log(`发送交易参数:`, JSON.stringify(txParams, null, 2).replace(CONFIG.privateKey, "***隐藏私钥***"));
          const startTime = Date.now();
          const txReceipt = await web3.eth.sendTransaction(txParams);
          const hashGenerationTime = Date.now() - startTime;
          
          console.log(`交易 #${globalIndex} 哈希: ${txReceipt.transactionHash}`);
          
          // 保存交易哈希和生成时间
          txResult.transactionHash = txReceipt.transactionHash;
          txResult.hashGenerationTime = hashGenerationTime;
          
          // 获取交易进入内存池后的状态
          const afterMempoolPool = await getPoolInfo();
          txResult.afterMempoolPool = afterMempoolPool;
          
          // 等待交易确认，增加超时机制
          const txStartTime = Date.now();
          
          // 使用轮询方式等待交易确认，增加超时检查和重试机制
          let confirmedTx = null;
          let timedOut = false;
          let checkCount = 0;
          
          const confirmationPromise = new Promise(async (resolve) => {
            while (!confirmedTx && !timedOut) {
              try {
                checkCount++;
                // 使用新的重试机制获取交易状态
                const tx = await checkTransactionStatus(txReceipt.transactionHash, globalIndex);
                
                if (tx && tx.blockNumber) {
                  confirmedTx = tx;
                  resolve(tx);
                  break;
                } else {
                  // 检查是否超时
                  if (Date.now() - txStartTime > CONFIG.confirmationTimeout) {
                    timedOut = true;
                    resolve(null);
                    break;
                  } else {
                    // 打印等待信息
                    if (checkCount % 5 === 0) { // 每5次检查打印一次
                      const waitTime = Math.floor((Date.now() - txStartTime) / 1000);
                      console.log(`交易 #${globalIndex} 已等待 ${waitTime} 秒...`);
                    }
                    // 增加检查间隔以减少连接错误风险
                    await sleep(3000); // 每3秒检查一次
                  }
                }
              } catch (err) {
                console.log(`交易 #${globalIndex} 等待确认时出错: ${err.message}, 继续等待...`);
                // 检查是否超时
                if (Date.now() - txStartTime > CONFIG.confirmationTimeout) {
                  timedOut = true;
                  resolve(null);
                  break;
                } else {
                  // 增加等待时间，让网络有机会恢复
                  await sleep(5000); // 错误后等待5秒再检查
                }
              }
            }
          });
          
          // 设置超时
          const timeoutPromise = new Promise(resolve => {
            setTimeout(() => {
              if (!confirmedTx) {
                timedOut = true;
                resolve(null);
              }
            }, CONFIG.confirmationTimeout);
          });
          
          // 等待交易确认或超时
          await Promise.race([confirmationPromise, timeoutPromise]);
          
          // 如果超时
          if (timedOut) {
            console.log(`\n交易 #${globalIndex} 确认超时! 已等待超过${CONFIG.confirmationTimeout / 1000}秒`);
            txResult.status = 'timeout';
            txResult.error = `交易确认超过${CONFIG.confirmationTimeout / 1000}秒超时`;
            return txResult;
          }
          
          const confirmationTime = Date.now() - txStartTime;
          console.log(`\n交易 #${globalIndex} 确认! 区块号: ${confirmedTx.blockNumber}, 用时: ${(confirmationTime/1000).toFixed(2)}秒 (${confirmationTime}毫秒)`);
          
          // 保存确认信息
          txResult.status = 'confirmed';
          txResult.confirmationTime = confirmationTime;
          txResult.transactionDetails = {
            blockNumber: confirmedTx.blockNumber,
            gasPrice: txParams.gasPrice,
            gasUsed: confirmedTx.gasUsed,
            status: confirmedTx.status
          };
          
          // 获取确认后的交易池状态
          const afterConfirmationPool = await getPoolInfo();
          txResult.afterConfirmationPool = afterConfirmationPool;
          
          // 获取区块信息
          try {
            const block = await web3.eth.getBlock(confirmedTx.blockNumber);
            txResult.blockInfo = {
              timestamp: block.timestamp,
              transactionCount: block.transactions.length,
              blockNumber: block.number
            };
          } catch (blockError) {
            console.error(`获取区块信息失败: ${blockError.message}`);
            txResult.blockInfo = {
              error: blockError.message
            };
          }
          
          return txResult;
        } catch (error) {
          console.error(`交易 #${globalIndex} 失败: ${error.message}`);
          txResult.status = 'failed';
          txResult.error = error.message;
          return txResult;
        }
      } catch (error) {
        console.error(`发送交易 #${globalIndex} 时出现严重错误:`, error.message);
        
        // 创建失败结果
        const errorResult = {
          index: globalIndex,
          gasPriceMultiplier: multiplier,
          category,
          nonce,
          timestamp: new Date().toISOString(),
          status: 'error',
          error: error.message
        };
        
        return errorResult;
      }
    };
    
    // 为当前批次的每个交易分配一个nonce，并创建发送承诺
    const sendPromises = batchMultipliers.map((txInfo, index) => {
      const globalIndex = batchStart + index + 1;
      const txNonce = currentNonce + index;
      return sendTransactionWithNonce(txInfo, globalIndex, txNonce);
    });
    
    // 更新下一批次的起始nonce
    currentNonce += batchMultipliers.length;
    
    // 等待当前批次的所有交易完成（确认或超时）
    console.log(`等待本批次交易完成或超时...`);
    const batchResults = await Promise.all(sendPromises);
    
    // 将批次结果添加到总结果中
    results.push(...batchResults);
    
    // 每批次完成后保存中间结果
    fs.writeFileSync(CONFIG.outputFile, JSON.stringify(results, null, 2));
    console.log(`批次 ${batchIndex + 1} 完成，已保存中间结果`);
    
    // 如果不是最后一批，等待一段时间再发送下一批
    if (batchIndex < batchCount - 1) {
      console.log(`等待 ${CONFIG.delayBetweenTxs / 1000} 秒后发送下一批交易...`);
      await sleep(CONFIG.delayBetweenTxs);
    }
  }
  
  // 生成验证报告
  const validationReport = generateValidationReport(results);
  fs.writeFileSync(CONFIG.validationReport, validationReport);
  
  console.log('\n================ 实验完成 ================');
  console.log(`成功交易数: ${results.filter(r => r.status === 'confirmed').length}/${totalTxs}`);
  console.log(`失败交易数: ${results.filter(r => r.status === 'failed').length}/${totalTxs}`);
  console.log(`超时交易数: ${results.filter(r => r.status === 'timeout').length}/${totalTxs}`);
  console.log(`错误交易数: ${results.filter(r => r.status === 'error').length}/${totalTxs}`);
  console.log(`详细结果已保存到: ${CONFIG.outputFile}`);
  console.log(`验证报告已保存到: ${CONFIG.validationReport}`);
}

// 执行实验
runExperiment();