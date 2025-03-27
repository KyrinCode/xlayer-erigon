const Web3 = require('web3');
const web3 = new Web3('http://localhost:8123'); // 连接到您的XLayer节点

async function checkBalance(address) {
  const balance = await web3.eth.getBalance(address);
  const balanceInEther = web3.utils.fromWei(balance, 'ether');
  console.log(`地址 ${address} 的余额: ${balanceInEther} OKB`);
  return balance;
}

async function sendTransaction() {
  // 私钥（请确保在测试网使用）
  const privateKey = '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2';
  
  // 获取账户
  const account = web3.eth.accounts.privateKeyToAccount(privateKey);
  const fromAddress = account.address;
  
  // 接收地址
  const toAddress = '0x39aeea40C5c6dbCCEB28088e7A8805E112bc7DD3';
  
  // 检查接收地址余额
  console.log('\n检查接收地址余额:');
  await checkBalance(toAddress);
  
  // 检查发送地址余额
  console.log('\n检查发送地址余额:');
  await checkBalance(fromAddress);
  
  // 获取当前nonce
  const nonce = await web3.eth.getTransactionCount(fromAddress, 'pending');
  
  // 交易参数
  const txParams = {
    from: fromAddress,
    to: toAddress,
    value: web3.utils.toWei('1000', 'ether'), // 发送1000 OKB
    gas: 21000, // 标准转账gas
    gasPrice: web3.utils.toWei('100', 'gwei'), // 非常高的gas价格 100 Gwei
    nonce: nonce
  };
  
  // 签名交易
  const signedTx = await web3.eth.accounts.signTransaction(txParams, privateKey);
  
  console.log('开始发送交易...');
  const startTime = Date.now();
  
  // 发送交易
  web3.eth.sendSignedTransaction(signedTx.rawTransaction)
    .on('transactionHash', (hash) => {
      console.log(`交易哈希: ${hash}`);
      console.log(`生成交易哈希用时: ${(Date.now() - startTime)}ms`);
    })
    .on('receipt', async (receipt) => {
      console.log(`交易确认! 区块号: ${receipt.blockNumber}`);
      console.log(`总用时: ${(Date.now() - startTime)}ms`);
      
      // 交易确认后检查余额
      console.log('\n交易确认后检查余额:');
      console.log('检查接收地址余额:');
      await checkBalance(toAddress);
      console.log('\n检查发送地址余额:');
      await checkBalance(fromAddress);
    })
    .on('error', (error) => {
      console.error('交易错误:', error);
    });
}

sendTransaction().catch(console.error);