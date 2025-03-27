const Web3 = require('web3');
const fs = require('fs');
const web3 = new Web3('http://localhost:8123');

// 配置参数
const CONFIG = {
  // 测试的Gas价格倍数
  gasPriceMultipliers: [1, 10, 50, 100, 200, 500],
  
  // 每个倍数测试次数
  iterations: 1,
  
  // 账户配置
  privateKey: '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2',
  toAddress: '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3',
  
  // 交易配置
  transactionValue: '0.01',   // 每笔交易金额（ETH/OKB）
  gas: 21000,                 // 标准转账Gas
  
  // 输出配置
  outputFile: './simple_gp_results.json'
};

// 获取交易池信息
async function getPoolInfo() {
  try {
    const { execSync } = require('child_process');
    const command = `curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"txpool_status","params":[],"id":1}' http://localhost:8123`;
    const result = JSON.parse(execSync(command).toString());
    
    const pendingCount = parseInt(result.result.pending, 16);
    const queuedCount = parseInt(result.result.queued, 16);
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

// 发送一笔交易并等待确认
async function sendTransaction(gasPriceMultiplier) {
  // 配置账户
  const account = web3.eth.accounts.privateKeyToAccount(CONFIG.privateKey);
  web3.eth.accounts.wallet.add(account);
  
  console.log(`\n===== 发送交易 (Gas价格倍数: ${gasPriceMultiplier}x) =====`);
  
  // 获取当前nonce
  const nonce = await web3.eth.getTransactionCount(account.address, 'pending');
  console.log(`当前nonce: ${nonce}`);
  
  // 获取当前Gas价格
  const currentGasPrice = await web3.eth.getGasPrice();
  console.log(`网络Gas价格: ${web3.utils.fromWei(currentGasPrice, 'gwei')} Gwei`);
  
  // 设置我们的Gas价格
  const ourGasPrice = BigInt(Math.floor(Number(currentGasPrice) * gasPriceMultiplier));
  console.log(`使用的Gas价格: ${web3.utils.fromWei(ourGasPrice.toString(), 'gwei')} Gwei (${gasPriceMultiplier}倍)`);
  
  // 获取交易池状态
  const poolInfo = await getPoolInfo();
  console.log(`交易池状态: 待处理: ${poolInfo.pendingCount}, 排队: ${poolInfo.queuedCount}, 总计: ${poolInfo.totalCount}`);
  
  // 创建交易参数
  const txParams = {
    from: account.address,
    to: CONFIG.toAddress,
    value: web3.utils.toWei(CONFIG.transactionValue, 'ether'),
    gas: CONFIG.gas,
    gasPrice: ourGasPrice.toString(),
    nonce: nonce
  };
  
  // 记录结果
  const result = {
    gasPriceMultiplier,
    timestamp: new Date().toISOString(),
    poolState: poolInfo,
    nonce,
    status: 'pending'
  };
  
  try {
    // 发送交易
    console.log("\n开始发送交易...");
    const startTime = Date.now();
    
    // 使用Promise方式发送交易并等待确认
    const receipt = await new Promise((resolve, reject) => {
      web3.eth.sendTransaction(txParams)
        .on('transactionHash', (hash) => {
          console.log(`交易哈希: ${hash}`);
          console.log(`生成交易哈希用时: ${(Date.now() - startTime)}ms`);
          result.transactionHash = hash;
          result.hashTime = Date.now() - startTime;
        })
        .on('receipt', (receipt) => {
          resolve(receipt);
        })
        .on('error', (error) => {
          reject(error);
        });
    });
    
    // 计算确认时间
    const confirmTime = Date.now() - startTime;
    console.log(`交易确认! 区块号: ${receipt.blockNumber}, 用时: ${(confirmTime/1000).toFixed(2)}秒 (${confirmTime}毫秒)`);
    
    // 记录结果
    result.status = 'confirmed';
    result.confirmationTime = confirmTime;
    result.blockNumber = receipt.blockNumber;
    result.gasUsed = receipt.gasUsed;
    
    return result;
  } catch (error) {
    console.error(`交易失败: ${error.message}`);
    result.status = 'failed';
    result.error = error.message;
    return result;
  }
}

// 主函数
async function main() {
  console.log(`开始测试不同Gas价格倍数的交易确认时间`);
  console.log(`将测试以下Gas价格倍数: ${CONFIG.gasPriceMultipliers.join(', ')}`);
  
  const results = [];
  
  // 按Gas价格倍数从高到低排序（可选）
  const sortedMultipliers = [...CONFIG.gasPriceMultipliers].sort((a, b) => b - a);
  
  // 每个倍数发送多次
  for (const multiplier of sortedMultipliers) {
    for (let i = 0; i < CONFIG.iterations; i++) {
      try {
        // 发送交易并等待确认
        const result = await sendTransaction(multiplier);
        results.push(result);
        
        // 保存结果
        fs.writeFileSync(CONFIG.outputFile, JSON.stringify(results, null, 2));
        
        // 简单的休息以避免连续发送
        if (i < CONFIG.iterations - 1 || multiplier !== sortedMultipliers[sortedMultipliers.length - 1]) {
          console.log(`等待3秒后继续...`);
          await new Promise(resolve => setTimeout(resolve, 3000));
        }
      } catch (error) {
        console.error(`测试Gas价格倍数 ${multiplier}x 失败:`, error);
      }
    }
  }
  
  // 生成报告
  console.log(`\n===== 测试结果 =====`);
  
  // 按Gas价格倍数分组计算确认时间
  const groupedResults = {};
  results.filter(r => r.status === 'confirmed').forEach(r => {
    if (!groupedResults[r.gasPriceMultiplier]) {
      groupedResults[r.gasPriceMultiplier] = [];
    }
    groupedResults[r.gasPriceMultiplier].push(r);
  });
  
  // 打印报告
  console.log(`Gas价格倍数\t确认时间(秒)\t交易哈希时间(毫秒)\t区块号`);
  console.log(`-----------------------------------------------------------------`);
  
  Object.keys(groupedResults).sort((a, b) => Number(a) - Number(b)).forEach(multiplier => {
    const txs = groupedResults[multiplier];
    txs.forEach(tx => {
      console.log(`${tx.gasPriceMultiplier}x\t\t${(tx.confirmationTime/1000).toFixed(2)}\t\t${tx.hashTime}\t\t${tx.blockNumber}`);
    });
  });
  
  console.log(`\n结果已保存到: ${CONFIG.outputFile}`);
}

// 执行
main().catch(console.error); 