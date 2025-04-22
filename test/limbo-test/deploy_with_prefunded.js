const fs = require('fs');
const path = require('path');
const solc = require('solc');
const { ethers } = require('ethers');

async function main() {
    // 配置连接到本地测试网络
    const provider = new ethers.providers.JsonRpcProvider('http://localhost:8123');
    
    // 使用测试文档中找到的预加载资金的私钥
    const privateKey = '0x815405dddb0e2a99b12af775fd2929e526704e1d1aea6a0b4e74dc33e2f7fcd2';
    const wallet = new ethers.Wallet(privateKey, provider);

    console.log(`使用账户: ${wallet.address}`);
    
    // 在部署前获取一些基本网络信息
    const balance = await wallet.getBalance();
    console.log(`账户余额: ${ethers.utils.formatEther(balance)} ETH`);
    
    const blockNumber = await provider.getBlockNumber();
    console.log(`当前区块高度: ${blockNumber}`);

    // 编译 Solidity 合约
    const contractPath = path.resolve(__dirname, 'BlockhashDemo.sol');
    const contractSource = fs.readFileSync(contractPath, 'utf8');

    // 准备编译器输入
    const input = {
        language: 'Solidity',
        sources: {
            'BlockhashDemo.sol': {
                content: contractSource
            }
        },
        settings: {
            outputSelection: {
                '*': {
                    '*': ['abi', 'evm.bytecode']
                }
            }
        }
    };

    console.log('编译合约...');
    const output = JSON.parse(solc.compile(JSON.stringify(input)));
    
    // 获取编译结果
    const contractOutput = output.contracts['BlockhashDemo.sol']['BlockhashDemo'];
    const abi = contractOutput.abi;
    const bytecode = contractOutput.evm.bytecode.object;

    console.log('部署合约...');
    
    // 创建合约工厂
    const factory = new ethers.ContractFactory(abi, bytecode, wallet);
    
    // 部署合约
    const contract = await factory.deploy();
    await contract.deployed();
    
    console.log(`合约已部署到地址: ${contract.address}`);
    
    // 调用合约的 test 方法来触发 blockhash 操作码
    console.log('调用合约 test() 方法以触发 blockhash 操作码...');
    try {
        const tx = await contract.test({ gasLimit: 500000 });
        console.log(`交易已发送，等待确认: ${tx.hash}`);
        
        const receipt = await tx.wait();
        console.log('交易已确认！');
        console.log(`交易状态: ${receipt.status === 1 ? '成功' : '失败'}`);
        console.log(`Gas 消耗: ${receipt.gasUsed.toString()}`);
    } catch (error) {
        console.log('交易执行失败');
        console.log(`错误信息: ${error.message}`);
        // 这是预期的，因为我们修改了 blockhash 操作码以触发 Limbo
    }
    
    console.log('\n现在应该检查 Sequencer 和 xmonitor 的日志，查看是否成功触发了 Limbo 状态');
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    }); 