# Limbo 测试

本目录包含用于触发 zkEVM Limbo 状态的测试工具。

## 前提条件

1. 已经修改 `xlayer-erigon/core/vm/instructions_zkevm.go` 中的 `opBlockhash_zkevm` 函数
2. 已经更新 `xlayer-erigon/test/config/test.erigon.seq.config.yaml` 配置:
   - `zkevm.executor-mock: false`
   - `zkevm.executor-urls: xlayer-executor:50071` 
   - `zkevm.limbo: true`
3. 已启动测试环境: Sequencer, Executor 和其他必要的服务

## 安装依赖

```bash
cd xlayer-erigon/test/limbo-test
npm install
```

## 运行测试

```bash
node deploy_with_prefunded.js
```

这个脚本会:
1. 使用预加载资金的账户部署 `BlockhashDemo` 合约
2. 调用合约的 `test()` 方法触发我们修改过的 `blockhash` 操作码
3. 由于操作码被修改为返回 `ErrExecutionReverted`，会导致 Sequencer 和 Executor 状态不一致
4. Executor 验证失败时，Sequencer 会进入 Limbo 状态

## 验证 Limbo 状态

运行测试后，可以通过以下方式验证 Limbo 状态是否触发:

1. 检查 Sequencer 日志:
   ```bash
   docker logs xlayer-seq
   ```
   
   应该能看到类似以下内容:
   ```
   [Limbo Test] BLOCKHASH opcode disabled...
   identified an invalid batch with number XXX
   adding transaction to limbo hash=0x...
   ```

2. 检查 xmonitor 是否检测到 Limbo:
   ```bash
   docker logs xmonitor
   ```
   
   应该能看到 Limbo 计数增加以及告警信息。

3. 使用 RPC 直接查询 Limbo 状态:
   ```bash
   curl -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"txpool_limbo","params":[],"id":1}' http://localhost:8123
   ``` 