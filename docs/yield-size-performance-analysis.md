# Performance Analysis: Impact of yieldSize on Sequencer Transaction Processing

## Simple Process Example: small yieldSize with huge Pay transactions

When a sequencer processes Pay transactions with yieldSize=30:

1. **Transaction Fetching** - Retrieves 30 transactions from the transaction pool
   - Sorts the pool if necessary (independent of yieldSize)
   - Performs 30 database reads (scales with yieldSize)
   - Decodes all 30 transactions (scales with yieldSize)

2. **Transaction Execution** - Processes transactions one-by-one
   - Validates transaction parameters
   - Executes transaction and updates state
   - Monitors ZK counters for overflow

3. **Batch Finalization** - When counters overflow or batch is full
   - Finalizes block with processed transactions
   - Commits the batch
   - Starts a new batch in the next iteration

This process repeats, with each batch containing around 30 Pay transactions, due to huge Pay transactions and ZK system constraints zkCounter.

## Problem Statement

The `yieldSize` parameter in ZK rollup transaction processing determines how many transactions are fetched from the transaction pool in a single batch. Currently, the XLayer sequencer uses a yield size of 1000 transactions, but a typical ZK batch can only fit around 30 transactions due to ZK-specific constraints like proof size limits and counter overflows.

This large mismatch between transactions fetched (1000) and transactions that can fit in a batch (30) creates several inefficiencies in the system. This analysis examines how varying this parameter affects performance, with particular focus on key operations whose time cost scales with yieldSize.

## Technical Components Affected by yieldSize

### 1. Transaction Selection (`bestRead`)

The `bestRead` function in `TxPool` is responsible for selecting the top N transactions (where N = yieldSize) from the pending pool:

```go
func (p *TxPool) bestRead(n uint16, txs *types.TxsRlp, tx kv.Tx, onTopOf, availableGas, availableBlobGas uint64, toSkip mapset.Set[[32]byte]) (bool, int, []*metaTx, error) {
    // ... initial checks ...
    
    best := p.pending.best
    txs.Resize(uint(cmp.Min(int(n), len(best.ms))))
    
    p.pending.EnforceBestInvariants()  // Sort transaction pool - independent of yieldSize
    
    st := time.Now()
    defer func() {
        if count == int(n) {
            log.Info("[txpool] bestRead", "elapsed", time.Since(st), "yieldSize", n)
        }
    }()
    
    // Loop through transactions - scales with yieldSize
    for i := 0; count < int(n) && i < len(best.ms); i++ {
        // For each transaction up to yieldSize:
        rlpTx, sender, isLocal, err := p.getRlpLocked(tx, mt.Tx.IDHash[:])  // Database read - linear cost
        // ... filtering logic ...
    }
}
```

#### Performance Characteristics:

- **Sorting/Organization Cost**: `EnforceBestInvariants()` ensures proper ordering of transactions but is **independent of yieldSize**. This cost depends on the total pool size, not how many transactions we're fetching.

- **Database Read Cost**: The dominant scaling factor with respect to yieldSize. For each candidate transaction being considered (up to yieldSize), a database read operation occurs via `getRlpLocked()`. This creates a **linear relationship** between yieldSize and database I/O operations.

- **Time Complexity**: O(P) for the sorting/organization + O(min(n, P)) for database reads, where:
  - n = yieldSize
  - P = pending pool size
  
- **Critical Path**: When pool size P is large, the number of database reads performed scales directly with yieldSize, making this the most significant variable cost.

- **Memory Usage**: Increases linearly with yieldSize

### 2. Transaction Decoding (`extractTransactionsFromSlot`)

After transactions are selected from the pool, they must be decoded from their RLP-encoded format:

```go
st := time.Now()
yieldedTxs, yieldedIds, toRemove, err := extractTransactionsFromSlot(&slots, executionAt, cfg)
if err != nil {
    return err
}
if len(yieldedTxs) == int(cfg.yieldSize) {
    log.Info("[txpool] extractTransactionsFromSlot", "elapsed", time.Since(st), "yieldedTxs", len(yieldedTxs))
}
```

#### Performance Characteristics:

- **Time Complexity**: O(m) where m is the number of yielded transactions (up to yieldSize)
- **CPU Intensity**: RLP decoding is compute-intensive rather than I/O-bound
- **Memory Allocation**: Each decoded transaction requires memory allocation
- **Scaling Behavior**: Linear with yieldSize

### 3. Transaction Execution and Overflow Detection

Once transactions are fetched and decoded, they are processed one-by-one in the `InnerLoopTransactions` section:

```go
InnerLoopTransactions:
for i, transaction := range batchState.blockState.transactionsForInclusion {
    // Block timer checks...
    
    // Sender recovery and validation...
    
    // The critical overflow check:
    receipt, execResult, txCounters, anyOverflow, err := attemptAddTransaction(
        cfg, sdb, ibs, batchCounters, &blockContext, header, 
        transaction, effectiveGas, batchState.isL1Recovery(), 
        batchState.forkId, l1TreeUpdateIndex, &backupDataSizeChecker, ethBlockGasPool
    )
    
    // Handle overflow case
    if anyOverflow == overflowCounters {
        // Complex handling logic...
        if batchState.reachedOverflowTransactionLimit() || cfg.zk.SealBatchImmediatelyOnOverflow {
            // End the batch and break out of transaction processing
            runLoopBlocks = false
            break OuterLoopTransactions
        }
    }
    
    // Add successful transaction to the block
    batchState.onAddedTransaction(transaction, receipt, execResult, effectiveGas)
}
```

#### Performance Characteristics:

- **Independent of yieldSize**: This phase processes only as many transactions as can fit in a block
- **Early Termination**: Processing stops when counters overflow
- **Typical Processing**: Only ~25-30 transactions are fully processed before overflow occurs

## Practical System Behavior and Overhead

### 1. Transaction Processing Flow

- **Initial Loading**: The system loads up to 1000 transactions into memory at once via `getNextPoolTransactions`.
- **Execution Cutoff**: When a batch overflows (typically around the 30th transaction), processing stops for the current batch.
- **Key Insight**: The system does not process all 1000 transactions - it breaks out of the processing loop when overflow is detected.

### 2. Transaction Pool Management

The code shows that after batch overflow:
```go
// remove mined transactions from the pool
toRemove := append(batchState.blockState.builtBlockElements.txSlots, batchState.blockState.transactionsToDiscard...)
if err := cfg.txPool.RemoveMinedTransactions(ctx, sdb.tx, header.GasLimit, toRemove); err != nil {
    return err
}
```

- **Important**: Only successfully mined transactions and explicitly discarded transactions are removed from the pool.
- **Unused Transactions Handling**: The ~970 transactions that were fetched but not processed remain in the transaction pool.
- **Memory Management**: These unused transactions are implicitly discarded from memory when the function completes, with no explicit cleanup required.

### 3. Batch Boundary Decision Making

The overflow detection logic determines when to end a batch:
```go
if batchState.reachedOverflowTransactionLimit() || cfg.zk.SealBatchImmediatelyOnOverflow {
    log.Info(fmt.Sprintf("[%s] closing batch due to overflow counters", logPrefix), "counters: ", batchState.overflowTransactions, "immediate", cfg.zk.SealBatchImmediatelyOnOverflow)
    runLoopBlocks = false
    // ...
    break OuterLoopTransactions
}
```

- **Design Note**: The system is designed to stop processing quickly after detecting overflow, rather than trying all 1000 transactions.
- **Optimization Opportunity**: This early stopping means the overhead isn't as severe as processing all 1000 transactions, but still involves unnecessary work.

## Performance Implications

### Large yieldSize (e.g., 1000)

1. **Pros**:
   - Fewer total calls to transaction pool
   - More transactions available for immediate processing
   - Can accommodate high-throughput scenarios

2. **Cons**:
   - Much higher memory usage during batch processing
   - **Significantly more database reads** - scales linearly with yieldSize
   - More wasted processing when only ~30 transactions fit in a ZK batch
   - Longer processing time per batch
   - **Repetitive Processing**: Same transactions are repeatedly fetched, decoded, and validated across multiple batches

### Small yieldSize (e.g., 50-60)

1. **Pros**:
   - Lower memory footprint
   - **Fewer database read operations** per batch
   - Less wasted processing
   - Faster per-batch processing time
   - Better aligned with actual batch capacity

2. **Cons**:
   - May need multiple pool queries to fill a batch in some scenarios
   - Potential throughput limitation in high-transaction environments

## Performance Scale Analysis

### Linear Components:
- **Database read operations** in transaction selection scale linearly with yieldSize
- Transaction decoding time scales linearly with yieldSize
- Memory allocation scales linearly with yieldSize

### Fixed-Cost Components:
- Transaction pool sorting/organization through `EnforceBestInvariants()` is independent of yieldSize
- Initial pool state evaluation

### Repeated Work:
- The initial processing (decoding, validation, sender recovery) is repeated for the same transactions across multiple batches when they can't fit in a single batch

## Optimal yieldSize Selection

The optimal yieldSize should:

1. **Match Typical Batch Capacity**: Since ZK batches typically hold ~30 transactions, a yieldSize of 50-60 provides sufficient buffer without excessive waste.

2. **Consider Database I/O Patterns**: Systems with slow storage may particularly benefit from smaller yieldSize values to reduce I/O overhead.

3. **Consider Hardware Resources**: Systems with limited memory may benefit from smaller yieldSize values.

4. **Adapt to Throughput Requirements**: Higher transaction throughput environments might benefit from slightly larger values.

## Recommendations

1. **Reduce yield size**: A more appropriate `yieldSize` would be 50-60 transactions (about 2x the typical batch capacity), providing a buffer without excessive overhead.

2. **Pre-filtering**: Implement lightweight filtering criteria that can be applied at the pool level before transactions are returned by `YieldBest`.

3. **Transient Memory Optimization**: If memory is a constraint, implement a streaming approach where smaller batches of transactions are fetched iteratively.

4. **Transaction History**: Track which transactions have been attempted but failed due to overflow, and prioritize different transactions in subsequent batches.

5. **Adaptive Yield Size**: Dynamically adjust `yieldSize` based on recent batch capacities and current network conditions.

## Conclusion

The analysis suggests that both transaction selection (primarily database reads) and decoding are significant performance factors that scale linearly with yieldSize. Most importantly, database read operations in the transaction selection process have a direct linear relationship with yieldSize.

The current large `yieldSize` of 1000 creates moderate inefficiency in the transaction processing pipeline, though not as severe as it might appear at first glance. While not all 1000 transactions are fully processed when a batch overflows, the system still incurs unnecessary overhead in fetching and initially processing transactions that won't fit in the current batch. These same transactions will likely be re-fetched and re-processed in subsequent batches.

By properly tuning this parameter to match the actual batch capacity (with a small buffer), the system can significantly reduce wasted computational resources and I/O operations while maintaining optimal throughput.

A value of 50-60 for yieldSize appears to be a good starting point for optimization, as it provides enough transactions to fill a batch while minimizing wasted processing. Further fine-tuning should be guided by empirical measurements under various load conditions, particularly focusing on database I/O patterns. 