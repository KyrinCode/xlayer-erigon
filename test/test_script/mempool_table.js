const fs = require('fs');

// 读取交易结果数据
function readTxData() {
  try {
    // 首先尝试读取tx_results.json
    const data = fs.readFileSync('./tx_results.json', 'utf8');
    return JSON.parse(data);
  } catch (err) {
    console.error('读取tx_results.json失败:', err.message);
    
    // 尝试读取扩展版本的文件
    try {
      const data = fs.readFileSync('./tx_results_extended.json', 'utf8');
      return JSON.parse(data);
    } catch (err2) {
      console.error('读取替代结果文件失败:', err2.message);
      return [];
    }
  }
}

// 格式化时间（毫秒转换为秒）
function formatTime(ms) {
  return (ms / 1000).toFixed(2) + '秒';
}

// 格式化Gas价格
function formatGasPrice(wei) {
  return (BigInt(wei) / BigInt(1000000000)).toString();
}

// 生成交易池状态表格
function generateMempoolTable(txData) {
  if (!txData || txData.length === 0) {
    console.log('没有找到交易数据');
    return;
  }

  // 表头 - 包含交易池信息
  console.log('| 序号 | Gas价格倍数 | 实际Gas价格(Gwei) | 确认时间 | 区块号 | 区块内交易数 | 交易池待处理 | 交易池排队 | 交易池总数 | 状态 |');
  console.log('|------|------------|-------------------|----------|--------|--------------|--------------|------------|------------|------|');
  
  // 表格内容
  txData.forEach(tx => {
    const index = tx.index;
    const multiplier = tx.gasPriceMultiplier;
    
    // 获取Gas价格
    let gasPrice = '未知';
    if (tx.transactionDetails && tx.transactionDetails.gasPrice) {
      gasPrice = (BigInt(tx.transactionDetails.gasPrice) / BigInt(1000000000)).toString();
    }
    
    const confirmTime = tx.confirmationTime ? formatTime(tx.confirmationTime) : '未确认';
    const blockNumber = tx.transactionDetails ? tx.transactionDetails.blockNumber : '未知';
    
    let txCountInBlock = '未知';
    if (tx.blockInfo && tx.blockInfo.transactionCount) {
      txCountInBlock = tx.blockInfo.transactionCount;
    }
    
    // 交易池状态 - 使用交易前的状态
    const pendingBefore = tx.beforePool ? tx.beforePool.pendingCount : '未知';
    const queuedBefore = tx.beforePool ? tx.beforePool.queuedCount : '未知';
    const totalBefore = tx.beforePool ? tx.beforePool.totalCount : '未知';
    
    const status = tx.status;
    
    console.log(`| ${index} | ${multiplier}x | ${gasPrice} | ${confirmTime} | ${blockNumber} | ${txCountInBlock} | ${pendingBefore} | ${queuedBefore} | ${totalBefore} | ${status} |`);
  });
  
  // 汇总交易池数据变化
  if (txData.length > 0) {
    const firstTx = txData[0];
    const lastTx = txData[txData.length - 1];
    
    if (firstTx.beforePool && lastTx.afterConfirmationPool) {
      const initialPending = firstTx.beforePool.pendingCount;
      const finalPending = lastTx.afterConfirmationPool.pendingCount;
      const pendingChange = finalPending - initialPending;
      const pendingPct = ((pendingChange / initialPending) * 100).toFixed(2);
      
      const initialTotal = firstTx.beforePool.totalCount;
      const finalTotal = lastTx.afterConfirmationPool.totalCount;
      const totalChange = finalTotal - initialTotal;
      const totalPct = ((totalChange / initialTotal) * 100).toFixed(2);
      
      console.log('\n### 交易池状态变化');
      console.log(`- 初始待处理交易: ${initialPending.toLocaleString()}`);
      console.log(`- 最终待处理交易: ${finalPending.toLocaleString()}`);
      console.log(`- 变化: ${pendingChange.toLocaleString()} (${pendingPct}%)`);
      console.log(`- 初始总交易: ${initialTotal.toLocaleString()}`);
      console.log(`- 最终总交易: ${finalTotal.toLocaleString()}`);
      console.log(`- 变化: ${totalChange.toLocaleString()} (${totalPct}%)`);
    }
  }
  
  // 生成每笔交易后的交易池变化表格
  console.log('\n### 交易池状态详细变化');
  console.log('| 序号 | Gas价格倍数 | 交易前待处理 | 交易前总数 | 交易后待处理 | 交易后总数 | 区块打包后待处理 | 区块打包后总数 | 变化百分比 |');
  console.log('|------|------------|--------------|------------|--------------|------------|------------------|---------------|------------|');
  
  txData.forEach(tx => {
    const index = tx.index;
    const multiplier = tx.gasPriceMultiplier;
    
    // 交易前的交易池状态
    const pendingBefore = tx.beforePool ? tx.beforePool.pendingCount : '未知';
    const totalBefore = tx.beforePool ? tx.beforePool.totalCount : '未知';
    
    // 交易进入内存池后状态
    const pendingAfterMempool = tx.afterMempoolPool ? tx.afterMempoolPool.pendingCount : '未知';
    const totalAfterMempool = tx.afterMempoolPool ? tx.afterMempoolPool.totalCount : '未知';
    
    // 交易被确认后状态
    const pendingAfterConfirm = tx.afterConfirmationPool ? tx.afterConfirmationPool.pendingCount : '未知';
    const totalAfterConfirm = tx.afterConfirmationPool ? tx.afterConfirmationPool.totalCount : '未知';
    
    // 计算变化百分比
    let changePct = '未知';
    if (typeof pendingBefore === 'number' && typeof pendingAfterConfirm === 'number') {
      const change = pendingAfterConfirm - pendingBefore;
      changePct = ((change / pendingBefore) * 100).toFixed(2) + '%';
    }
    
    console.log(`| ${index} | ${multiplier}x | ${pendingBefore} | ${totalBefore} | ${pendingAfterMempool} | ${totalAfterMempool} | ${pendingAfterConfirm} | ${totalAfterConfirm} | ${changePct} |`);
  });
}

// 主函数
function main() {
  console.log('正在读取交易数据...');
  const txData = readTxData();
  console.log(`找到 ${txData.length} 条交易记录\n`);
  
  console.log('### 包含交易池状态的交易详情表格');
  generateMempoolTable(txData);
  
  // 保存到文件
  const output = fs.createWriteStream('./mempool_analysis.md');
  
  // 备份原始的console.log
  const originalLog = console.log;
  
  // 重定向console.log到文件
  console.log = function(message) {
    output.write(message + '\n');
    originalLog(message);
  };
  
  console.log('# XLayer交易池状态分析');
  console.log('生成时间: ' + new Date().toLocaleString());
  console.log('\n## 交易池状态变化表格');
  generateMempoolTable(txData);
  
  // 恢复原始的console.log
  console.log = originalLog;
  
  output.end();
  console.log('\n交易池状态分析已保存到 mempool_analysis.md');
}

// 执行主函数
main();