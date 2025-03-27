const Web3 = require('web3');
const web3 = new Web3('http://localhost:8123'); // 连接到您的XLayer节点

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
      return { pendingCount: 'N/A', queuedCount: 'N/A', gasPrice: 'N/A' };
    }

    const pendingCount = parseInt(response.result.pending, 16);
    const queuedCount = parseInt(response.result.queued, 16);
    console.log(`交易池状态: 待处理: ${pendingCount}, 排队中: ${queuedCount}, 总计: ${pendingCount + queuedCount}`);
    
    // 获取当前Gas价格建议
    const gasPrice = await web3.eth.getGasPrice();
    console.log(`当前Gas价格: ${web3.utils.fromWei(gasPrice, 'gwei')} Gwei`);
    
    return { pendingCount, queuedCount, gasPrice };
  } catch (error) {
    console.error('获取交易池信息失败:', error.message);
    return { pendingCount: 'N/A', queuedCount: 'N/A', gasPrice: 'N/A' };
  }
}

async function sendTransaction() {
  // 私钥（请确保在测试网使用）
  const privateKey = '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2';
  
  // 获取账户
  const account = web3.eth.accounts.privateKeyToAccount(privateKey);
  const fromAddress = account.address;
  
  // 接收地址
  const toAddress = '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3';
  
  console.log(`发送地址: ${fromAddress}`);
  console.log(`接收地址: ${toAddress}`);
  
  // 获取当前nonce
  const nonce = await web3.eth.getTransactionCount(fromAddress, 'pending');
  console.log(`当前nonce: ${nonce}`);
  
  // 获取当前Gas价格
  const currentGasPrice = await web3.eth.getGasPrice();
  console.log(`网络Gas价格: ${web3.utils.fromWei(currentGasPrice, 'gwei')} Gwei`);
  
  // 设置我们的Gas价格为网络当前价格的2倍
  const ourGasPrice = BigInt(currentGasPrice) * BigInt(2);
  console.log(`我们使用的Gas价格: ${web3.utils.fromWei(ourGasPrice.toString(), 'gwei')} Gwei (网络价格的2倍)`);
  
  // 交易参数
  const txParams = {
    from: fromAddress,
    to: toAddress,
    value: web3.utils.toWei('0.01', 'ether'), // 发送0.01 OKB
    gas: 21000, // 标准转账gas
    gasPrice: ourGasPrice.toString(), 
    nonce: nonce
  };
  
  // 获取交易池状态（发送前）
  console.log('\n交易发送前:');
  await getPoolInfo();
  
  // 签名交易
  const signedTx = await web3.eth.accounts.signTransaction(txParams, privateKey);
  
  console.log('\n开始发送交易...');
  const startTime = Date.now();
  
  // 发送交易
  let txHash;
  await web3.eth.sendSignedTransaction(signedTx.rawTransaction)
    .on('transactionHash', async (hash) => {
      txHash = hash;
      console.log(`交易哈希: ${hash}`);
      console.log(`生成交易哈希用时: ${(Date.now() - startTime)}ms`);
      
      // 获取交易被打包前的交易池状态
      console.log('\n交易进入内存池后:');
      await getPoolInfo();
    })
    .on('receipt', async (receipt) => {
      console.log(`\n交易确认! 区块号: ${receipt.blockNumber}`);
      console.log(`总用时: ${(Date.now() - startTime)}ms`);
      
      // 获取交易被打包后的交易池状态
      console.log('\n交易被打包后:');
      await getPoolInfo();
      
      // 获取交易详情
      const tx = await web3.eth.getTransaction(txHash);
      console.log('\n交易详情:');
      console.log(`Gas价格: ${web3.utils.fromWei(tx.gasPrice, 'gwei')} Gwei`);
      console.log(`使用的Gas: ${receipt.gasUsed}`);
      
      // 获取区块信息
      const block = await web3.eth.getBlock(receipt.blockNumber);
      console.log('\n区块信息:');
      console.log(`区块时间: ${new Date(block.timestamp * 1000).toLocaleString()}`);
      console.log(`区块中的交易数量: ${block.transactions.length}`);
    })
    .on('error', (error) => {
      console.error('交易错误:', error);
    });
}

sendTransaction().catch(console.error);
