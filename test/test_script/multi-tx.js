const Web3 = require('web3');
const fs = require('fs');
const web3 = new Web3('http://localhost:8123'); // 连接到您的XLayer节点

// 配置参数
const CONFIG = {
  // 测试配置
  numberOfTransactions: 20,                 // 要发送的交易数量
  delayBetweenTxs: 5000,                   // 交易之间的延迟（毫秒）
  gasPriceMultipliers: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 3, 4, 5, 7.5, 10, 15, 20], // Gas价格乘数（相对于当前网络价格）
  
  // 账户配置
  privateKey: '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2',
  toAddress: '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3',
  
  // 交易配置
  transactionValue: '0.01',   // 每笔交易金额（ETH/OKB）
  gas: 21000,                 // 标准转账Gas
  
  // 输出配置
  outputFile: './tx_results_extended.json',  // 结果输出文件
};

// 存储所有交易结果
const results = [];

async function getPoolInfo() {
  try {
    // 使用原始的HTTP请求方式获取交易池状态
    const response = await new Promise((resolve, reject) => {
      const request = require('request');
      request({
        url: 'http://localhost:8123',
        method: 'POST',
        json: true,
        body: { jsonrpc: '2.0', method: 'txpool_status', params: [], id: 1 }
      }, (err, res, body) => {
        if (err) reject(err);
        else resolve(body);
      });
    });

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
      afterMempoolPool: null,
      afterConfirmationPool: null,
      blockInfo: null,
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
    
    // 发送交易并等待结果
    return new Promise((resolve, reject) => {
      let txHash;
      web3.eth.sendSignedTransaction(signedTx.rawTransaction)
        .on('transactionHash', async (hash) => {
          txHash = hash;
          console.log(`交易哈希: ${hash}`);
          console.log(`生成交易哈希用时: ${(Date.now() - startTime)}ms`);
          result.transactionHash = hash;
          result.hashGenerationTime = Date.now() - startTime;
          
          // 获取交易被打包前的交易池状态
          console.log('\n交易进入内存池后:');
          const afterMempoolPool = await getPoolInfo();
          console.log(`交易池状态: 待处理: ${afterMempoolPool.pendingCount}, 排队中: ${afterMempoolPool.queuedCount}, 总计: ${afterMempoolPool.totalCount}`);
          console.log(`当前Gas价格: ${web3.utils.fromWei(afterMempoolPool.gasPrice, 'gwei')} Gwei`);
          result.afterMempoolPool = afterMempoolPool;
        })
        .on('receipt', async (receipt) => {
          const confirmationTime = Date.now() - startTime;
          console.log(`\n交易确认! 区块号: ${receipt.blockNumber}`);
          console.log(`总用时: ${confirmationTime}ms`);
          result.confirmationTime = confirmationTime;
          
          // 获取交易被打包后的交易池状态
          console.log('\n交易被打包后:');
          const afterConfirmationPool = await getPoolInfo();
          console.log(`交易池状态: 待处理: ${afterConfirmationPool.pendingCount}, 排队中: ${afterConfirmationPool.queuedCount}, 总计: ${afterConfirmationPool.totalCount}`);
          console.log(`当前Gas价格: ${web3.utils.fromWei(afterConfirmationPool.gasPrice, 'gwei')} Gwei`);
          result.afterConfirmationPool = afterConfirmationPool;
          
          // 获取交易详情
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
          
          result.status = 'confirmed';
          resolve(result);
        })
        .on('error', (error) => {
          console.error('交易错误:', error);
          result.status = 'error';
          result.error = error.message;
          reject(result);
        });
    });
  } catch (error) {
    console.error(`发送交易 #${txIndex} 失败:`, error);
    return {
      index: txIndex,
      gasPriceMultiplier,
      status: 'error',
      error: error.message
    };
  }
}

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function runExperiment() {
  console.log(`开始交易实验: 将发送 ${CONFIG.numberOfTransactions} 笔交易`);
  
  for (let i = 0; i < CONFIG.numberOfTransactions; i++) {
    const multiplier = CONFIG.gasPriceMultipliers[i % CONFIG.gasPriceMultipliers.length];
    try {
      const result = await sendTransaction(multiplier, i + 1);
      results.push(result);
      
      // 保存当前结果
      fs.writeFileSync(CONFIG.outputFile, JSON.stringify(results, null, 2));
      
      // 如果不是最后一笔交易，则等待一段时间
      if (i < CONFIG.numberOfTransactions - 1) {
        console.log(`\n等待 ${CONFIG.delayBetweenTxs / 1000} 秒后发送下一笔交易...`);
        await sleep(CONFIG.delayBetweenTxs);
      }
    } catch (error) {
      console.error(`交易 #${i+1} 失败:`, error);
      results.push(error);
    }
  }
  
  // 实验完成，生成报告
  generateReport();
}

function generateReport() {
  console.log("\n================ 实验结果 ================");
  
  // 计算成功的交易
  const successfulTxs = results.filter(r => r.status === 'confirmed');
  console.log(`成功交易数: ${successfulTxs.length}/${CONFIG.numberOfTransactions}`);
  
  if (successfulTxs.length > 0) {
    // 计算平均确认时间
    const avgConfirmTime = successfulTxs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / successfulTxs.length;
    console.log(`平均确认时间: ${avgConfirmTime.toFixed(2)}ms (${(avgConfirmTime/1000).toFixed(2)}秒)`);
    
    // 按Gas价格倍数分组计算平均确认时间
    const byMultiplier = {};
    successfulTxs.forEach(tx => {
      if (!byMultiplier[tx.gasPriceMultiplier]) {
        byMultiplier[tx.gasPriceMultiplier] = [];
      }
      byMultiplier[tx.gasPriceMultiplier].push(tx);
    });
    
    console.log("\n不同Gas价格倍数的确认时间:");
    console.log("倍数\t平均确认时间(秒)\t交易数");
    console.log("--------------------------------------");
    
    Object.keys(byMultiplier).sort((a, b) => parseFloat(a) - parseFloat(b)).forEach(multiplier => {
      const txs = byMultiplier[multiplier];
      const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
      console.log(`${multiplier}x\t${(avgTime/1000).toFixed(2)}秒\t\t${txs.length}`);
    });
    
    // 计算交易池数据
    if (successfulTxs[0].beforePool) {
      const firstTx = successfulTxs[0];
      const lastTx = successfulTxs[successfulTxs.length - 1];
      
      console.log("\n交易池状态变化:");
      console.log(`初始状态: 待处理: ${firstTx.beforePool.pendingCount}, 排队: ${firstTx.beforePool.queuedCount}, 总计: ${firstTx.beforePool.totalCount}`);
      console.log(`最终状态: 待处理: ${lastTx.afterConfirmationPool.pendingCount}, 排队: ${lastTx.afterConfirmationPool.queuedCount}, 总计: ${lastTx.afterConfirmationPool.totalCount}`);
    }
  }
  
  console.log("\n详细结果已保存到:", CONFIG.outputFile);
}

// 运行实验
runExperiment().catch(console.error); 