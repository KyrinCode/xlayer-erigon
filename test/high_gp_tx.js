// High Gas Price Transaction Test
// This script sends a transaction with very high gas price to ensure it gets prioritized
const { Web3 } = require('web3');
const fs = require('fs');

// 连接到sequencer节点
const web3 = new Web3('http://localhost:8123');

// 使用私钥创建账户（这里使用示例私钥，实际使用时请替换）
const privateKey = '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80'; // 示例私钥
const account = web3.eth.accounts.privateKeyToAccount(privateKey);
web3.eth.accounts.wallet.add(account);

// 包装函数，添加重试功能
async function withRetry(fn, retries = 3, delay = 1000) {
  let lastError;
  for (let i = 0; i < retries; i++) {
    try {
      return await fn();
    } catch (error) {
      console.error(`尝试 ${i + 1}/${retries} 失败: ${error.message}`);
      lastError = error;
      if (i < retries - 1) {
        console.log(`等待 ${delay}ms 后重试...`);
        await new Promise(resolve => setTimeout(resolve, delay));
      }
    }
  }
  throw lastError;
}

// 查询交易池状态
async function checkTxpoolStatus() {
  try {
    const status = await web3.eth.provider.request({
      method: 'txpool_status',
      params: [],
    });
    console.log('交易池状态:', status);
    return status;
  } catch (error) {
    console.error('查询交易池状态失败:', error.message);
    return { pending: '0x0', queued: '0x0' };
  }
}

// 获取当前Gas Price并提高100倍
async function sendHighPriorityTx() {
  try {
    console.log('开始准备高优先级交易...');
    
    // 检查交易池状态
    await checkTxpoolStatus();
    
    // 获取当前gasPrice
    const gasPrice = await withRetry(() => web3.eth.getGasPrice());
    console.log(`当前Gas Price: ${gasPrice} wei`);
    
    // 计算新的gasPrice (提高1000倍)
    const highGasPrice = BigInt(gasPrice) * 1000n;
    console.log(`设置高Gas Price: ${highGasPrice} wei`);
    
    // 获取nonce
    const nonce = await withRetry(() => web3.eth.getTransactionCount(account.address));
    console.log(`账户 ${account.address} 的Nonce: ${nonce}`);
    
    // 记录发送时间
    const startTime = new Date();
    console.log(`开始发送交易: ${startTime.toISOString()}`);
    
    // 发送交易
    const tx = {
      from: account.address,
      to: '0x70997970c51812dc3a010c7d01b50e0d17dc79c8', // 示例接收地址
      value: web3.utils.toWei('0.001', 'ether'),
      gas: 21000,
      gasPrice: highGasPrice.toString(),
      nonce: nonce
    };
    
    console.log('交易详情:', JSON.stringify(tx, null, 2));
    
    const receipt = await withRetry(() => web3.eth.sendTransaction(tx).then(rec => {
      console.log(`交易已提交，哈希: ${rec.transactionHash}`);
      return web3.eth.getTransactionReceipt(rec.transactionHash);
    }));
    
    // 记录确认时间
    const endTime = new Date();
    const timeTaken = (endTime - startTime) / 1000; // 秒
    
    console.log(`交易已确认！`);
    console.log(`交易哈希: ${receipt.transactionHash}`);
    console.log(`区块号: ${receipt.blockNumber}`);
    console.log(`打包耗时: ${timeTaken} 秒`);
    console.log('交易详情:', receipt);
    
    // 将结果写入日志文件
    const logData = {
      txHash: receipt.transactionHash,
      blockNumber: receipt.blockNumber,
      gasPrice: highGasPrice.toString(),
      timeTaken: timeTaken,
      timestamp: new Date().toISOString()
    };
    
    fs.writeFileSync('./high_gp_result.json', JSON.stringify(logData, null, 2));
    console.log('测试结果已保存到 high_gp_result.json');
    
  } catch (error) {
    console.error('发送交易失败:', error.message);
  }
}

// 执行测试
sendHighPriorityTx(); 