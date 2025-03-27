// Fill Transaction Pool Script
// This script sends a large number of transactions to fill the transaction pool
const { Web3 } = require('web3');

// 连接到sequencer节点
const web3 = new Web3('http://localhost:8123');

// 使用私钥创建多个账户（这里使用示例私钥，实际使用时请替换）
const privateKeys = [
  '0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80',
  '0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d',
  '0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a',
  '0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6',
  '0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a'
];

// 添加所有账户到钱包
const accounts = privateKeys.map(pk => {
  const account = web3.eth.accounts.privateKeyToAccount(pk);
  web3.eth.accounts.wallet.add(account);
  return account;
});

// 填充交易池的函数
async function fillTransactionPool(txCount) {
  console.log(`开始填充交易池，计划发送 ${txCount} 个交易...`);
  
  const startTime = new Date();
  const promises = [];
  const batchSize = 100; // 每批发送的交易数量
  const batches = Math.ceil(txCount / batchSize);
  
  for (let batch = 0; batch < batches; batch++) {
    console.log(`处理批次 ${batch + 1}/${batches}`);
    
    const batchPromises = [];
    const currentBatchSize = Math.min(batchSize, txCount - batch * batchSize);
    
    for (let i = 0; i < currentBatchSize; i++) {
      const accountIndex = (batch * batchSize + i) % accounts.length;
      const fromAccount = accounts[accountIndex];
      
      // 获取nonce
      const getNonce = async () => {
        try {
          const nonce = await web3.eth.getTransactionCount(fromAccount.address, 'pending');
          return nonce;
        } catch (error) {
          console.error(`获取nonce失败: ${error.message}`);
          return -1;
        }
      };
      
      const noncePromise = getNonce().then(nonce => {
        if (nonce === -1) return Promise.resolve();
        
        // 创建交易对象 - 使用低gas price
        const tx = {
          from: fromAccount.address,
          to: '0x000000000000000000000000000000000000dEaD', // burn地址
          value: web3.utils.toWei('0.0001', 'ether'),
          gas: 21000,
          gasPrice: web3.utils.toWei('1', 'gwei'), // 低gas price
          nonce: nonce
        };
        
        // 发送交易
        return web3.eth.sendTransaction(tx)
          .then(() => {
            if ((batch * batchSize + i + 1) % 1000 === 0 || batch * batchSize + i + 1 === txCount) {
              console.log(`已发送 ${batch * batchSize + i + 1}/${txCount} 个交易`);
            }
          })
          .catch(error => {
            console.error(`发送交易失败 (${fromAccount.address}, nonce=${nonce}): ${error.message}`);
          });
      });
      
      batchPromises.push(noncePromise);
    }
    
    // 等待当前批次完成
    await Promise.allSettled(batchPromises);
  }
  
  const endTime = new Date();
  const timeTaken = (endTime - startTime) / 1000; // 秒
  
  console.log(`交易池填充完成！`);
  console.log(`总耗时: ${timeTaken} 秒`);
  console.log(`平均每秒发送: ${txCount / timeTaken} 笔交易`);
}

// 默认发送1000个交易，更合理用于测试
const txCount = 1000;
fillTransactionPool(txCount); 