const Web3 = require('web3');
const web3 = new Web3('http://localhost:8123');

// 账户信息
const privateKey = '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2';
const account = web3.eth.accounts.privateKeyToAccount(privateKey);
const toAddress = '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3';

// 测试函数
async function testTransactionTime() {
  console.log('开始交易确认时间测试...');
  console.log(`发送地址: ${account.address}`);
  console.log(`接收地址: ${toAddress}`);
  
  try {
    // 获取当前nonce和Gas价格
    const nonce = await web3.eth.getTransactionCount(account.address);
    const gasPrice = await web3.eth.getGasPrice();
    console.log(`当前nonce: ${nonce}`);
    console.log(`当前Gas价格: ${web3.utils.fromWei(gasPrice, 'gwei')} Gwei`);
    
    // 创建交易
    const txParams = {
      from: account.address,
      to: toAddress,
      value: web3.utils.toWei('0.01', 'ether'),
      gas: 21000,
      gasPrice: gasPrice,
      nonce: nonce
    };
    
    console.log('准备签名交易...');
    
    // 签名交易
    const signedTx = await web3.eth.accounts.signTransaction(txParams, privateKey);
    console.log('交易已签名，准备发送...');
    
    // 记录开始时间
    const startTime = Date.now();
    
    // 发送签名交易并等待确认
    const receipt = await new Promise((resolve, reject) => {
      web3.eth.sendSignedTransaction(signedTx.rawTransaction)
        .on('transactionHash', (hash) => {
          const hashTime = Date.now() - startTime;
          console.log(`交易哈希: ${hash}`);
          console.log(`获取交易哈希用时: ${hashTime}ms`);
        })
        .on('receipt', (receipt) => {
          const confirmTime = Date.now() - startTime;
          console.log(`交易已确认! 区块号: ${receipt.blockNumber}`);
          console.log(`确认用时: ${confirmTime}ms (${(confirmTime/1000).toFixed(2)}秒)`);
          resolve(receipt);
        })
        .on('error', (error) => {
          console.error('交易错误:', error);
          reject(error);
        });
    });
    
    // 获取区块信息
    const block = await web3.eth.getBlock(receipt.blockNumber);
    console.log('\n区块信息:');
    console.log(`区块时间: ${new Date(block.timestamp * 1000).toLocaleString()}`);
    console.log(`区块中的交易数量: ${block.transactions.length}`);
    
    // 查询前一个区块信息
    if (receipt.blockNumber > 1) {
      const prevBlock = await web3.eth.getBlock(receipt.blockNumber - 1);
      const blockInterval = block.timestamp - prevBlock.timestamp;
      console.log(`区块间隔: ${blockInterval}秒`);
    }
    
    console.log('\n测试完成!');
  } catch (error) {
    console.error('测试过程中发生错误:', error);
  }
}

// 运行测试
testTransactionTime(); 