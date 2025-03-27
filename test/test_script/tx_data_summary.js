const fs = require('fs');

// 尝试读取交易结果文件
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
      console.error('读取tx_results_extended.json失败:', err2.message);
      return [];
    }
  }
}

// 格式化时间（毫秒转换为秒）
function formatTime(ms) {
  return (ms / 1000).toFixed(2) + '秒';
}

// 生成表格数据
function generateTableData(txData) {
  if (!txData || txData.length === 0) {
    console.log('没有找到交易数据');
    return;
  }

  // 表头
  console.log('| 序号 | Gas价格倍数 | 实际Gas价格(Gwei) | 确认时间 | 区块号 | 区块内交易数 | 状态 |');
  console.log('|------|------------|-------------------|----------|--------|--------------|------|');
  
  // 表格内容
  txData.forEach(tx => {
    const index = tx.index;
    const multiplier = tx.gasPriceMultiplier;
    
    // 从交易详情中获取Gas价格
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
    
    const status = tx.status;
    
    console.log(`| ${index} | ${multiplier}x | ${gasPrice} | ${confirmTime} | ${blockNumber} | ${txCountInBlock} | ${status} |`);
  });
  
  // 汇总信息
  const successfulTxs = txData.filter(tx => tx.status === 'confirmed');
  if (successfulTxs.length > 0) {
    const avgConfirmTime = successfulTxs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / successfulTxs.length;
    
    console.log('\n### 汇总数据');
    console.log(`- 总交易数: ${txData.length}`);
    console.log(`- 成功交易数: ${successfulTxs.length}`);
    console.log(`- 平均确认时间: ${formatTime(avgConfirmTime)}`);
    
    // 按Gas价格倍数分组的确认时间
    console.log('\n### 不同Gas价格倍数的确认时间');
    console.log('| Gas价格倍数 | 平均确认时间 | 交易数 |');
    console.log('|------------|--------------|--------|');
    
    // 创建一个按倍数分组的map
    const byMultiplier = {};
    successfulTxs.forEach(tx => {
      if (!byMultiplier[tx.gasPriceMultiplier]) {
        byMultiplier[tx.gasPriceMultiplier] = [];
      }
      byMultiplier[tx.gasPriceMultiplier].push(tx);
    });
    
    // 输出每组的平均确认时间
    Object.keys(byMultiplier)
      .sort((a, b) => parseFloat(a) - parseFloat(b))
      .forEach(multiplier => {
        const txs = byMultiplier[multiplier];
        const avgTime = txs.reduce((acc, tx) => acc + tx.confirmationTime, 0) / txs.length;
        console.log(`| ${multiplier}x | ${formatTime(avgTime)} | ${txs.length} |`);
      });
  }
}

// 主函数
function main() {
  console.log('正在读取交易数据...');
  const txData = readTxData();
  console.log(`找到 ${txData.length} 条交易记录\n`);
  
  console.log('### 交易详情表格');
  generateTableData(txData);
  
  // 保存到文件
  const output = fs.createWriteStream('./tx_summary_table.md');
  
  // 备份原始的console.log
  const originalLog = console.log;
  
  // 重定向console.log到文件
  console.log = function(message) {
    output.write(message + '\n');
    originalLog(message);
  };
  
  console.log('# XLayer交易实验数据摘要');
  console.log('生成时间: ' + new Date().toLocaleString());
  console.log('\n## 交易数据表格');
  generateTableData(txData);
  
  // 恢复原始的console.log
  console.log = originalLog;
  
  output.end();
  console.log('\n表格数据已保存到 tx_summary_table.md');
}

// 执行主函数
main(); 