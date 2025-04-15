package jsonrpc_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"github.com/c2h5oh/datasize"
	mdbx2 "github.com/erigontech/mdbx-go/mdbx"
	"github.com/ledgerwatch/erigon-lib/direct"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/txpool/txpoolcfg"
	types2 "github.com/ledgerwatch/erigon-lib/types"
	txpoolZk "github.com/ledgerwatch/erigon/zk/txpool"
	"math/big"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/wrap"

	"github.com/ledgerwatch/erigon-lib/gointerfaces/sentry"
	"github.com/ledgerwatch/erigon-lib/gointerfaces/txpool"
	"github.com/ledgerwatch/erigon-lib/kv/kvcache"
	"github.com/ledgerwatch/erigon/rpc/rpccfg"
	"github.com/stretchr/testify/require"

	"github.com/ledgerwatch/erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/ledgerwatch/erigon/common/u256"

	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/protocols/eth"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rlp"
	"github.com/ledgerwatch/erigon/turbo/jsonrpc"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
	"github.com/ledgerwatch/erigon/turbo/stages"
	"github.com/ledgerwatch/erigon/turbo/stages/mock"
	"github.com/ledgerwatch/log/v3"
)

func newBaseApiForTest(m *mock.MockSentry) *jsonrpc.BaseAPI {
	agg := m.HistoryV3Components()
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	return jsonrpc.NewBaseApi(nil, stateCache, m.BlockReader, agg, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs)
}

// Do 1 step to start txPool
func oneBlockStep(mockSentry *mock.MockSentry, require *require.Assertions, t *testing.T) {
	chain, err := core.GenerateChain(mockSentry.ChainConfig, mockSentry.Genesis, mockSentry.Engine, mockSentry.DB, 1 /*number of blocks:*/, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{1})
	})
	require.NoError(err)

	// Send NewBlock message
	b, err := rlp.EncodeToBytes(&eth.NewBlockPacket{
		Block: chain.TopBlock,
		TD:    big.NewInt(1), // This is ignored anyway
	})
	require.NoError(err)

	mockSentry.ReceiveWg.Add(1)
	for _, err = range mockSentry.Send(&sentry.InboundMessage{Id: sentry.MessageId_NEW_BLOCK_66, Data: b, PeerId: mockSentry.PeerId}) {
		require.NoError(err)
	}
	// Send all the headers
	b, err = rlp.EncodeToBytes(&eth.BlockHeadersPacket66{
		RequestId:          1,
		BlockHeadersPacket: chain.Headers,
	})
	require.NoError(err)
	mockSentry.ReceiveWg.Add(1)
	for _, err = range mockSentry.Send(&sentry.InboundMessage{Id: sentry.MessageId_BLOCK_HEADERS_66, Data: b, PeerId: mockSentry.PeerId}) {
		require.NoError(err)
	}
	mockSentry.ReceiveWg.Wait() // Wait for all messages to be processed before we proceed

	initialCycle := mock.MockInsertAsInitialCycle
	if err := stages.StageLoopIteration(mockSentry.Ctx, mockSentry.DB, wrap.TxContainer{}, mockSentry.Sync, initialCycle, log.New(), mockSentry.BlockReader, nil, false); err != nil {
		t.Fatal(err)
	}
}

func TestSendRawTransaction(t *testing.T) {
	mockSentry, require := mock.MockWithTxPool(t), require.New(t)

	oneBlockStep(mockSentry, require, t)

	expectedValue := uint64(1234)
	txn, err := types.SignTx(types.NewTransaction(0, common.Address{1}, uint256.NewInt(expectedValue), params.TxGas, uint256.NewInt(10*params.GWei), nil), *types.LatestSignerForChainID(mockSentry.ChainConfig.ChainID), mockSentry.Key)
	require.NoError(err)

	newTxChan := make(chan types2.Announcements)
	ctx := context.Background() // base context
	aclDB, err := txpoolZk.OpenACLDB(ctx, t.TempDir())
	txPoolDB, err := mdbx.NewMDBX(log.New()).Label(kv.TxPoolDB).Path(t.TempDir()).
		WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg { return kv.TxpoolTablesCfg }).
		Flags(func(f uint) uint { return f ^ mdbx2.Durable | mdbx2.SafeNoSync }).
		GrowthStep(16 * datasize.MB).
		SyncPeriod(30 * time.Second).
		Open(ctx)

	chainId := *uint256.NewInt(1337)
	txPool, err := txpoolZk.New(newTxChan, mockSentry.DB, txpoolcfg.DefaultConfig, &ethconfig.Defaults, kvcache.NewDummy(), chainId, new(big.Int).SetUint64(0), new(big.Int).SetUint64(0), aclDB)

	txpoolGrpcServer := txpoolZk.NewGrpcServer(ctx, txPool, txPoolDB, chainId)
	txpoolClient := direct.NewTxPoolClient(txpoolGrpcServer)

	buf := make([]byte, 0)
	writer := bytes.NewBuffer(buf)
	err = txn.EncodeRLP(writer)
	require.NoError(err)
	sender, _ := txn.GetSender()
	req := &txpool.AddRequest{RlpTxs: [][]byte{writer.Bytes()}, DecodedTx: []interface{}{txn}, RecoveredSender: [][20]byte{sender}}
	reply, err := txpoolClient.Add(ctx, req)

	require.Equal([]string([]string{"insufficient funds"}), reply.Errors)

	// TODO: mock the sender with enough fund and send again should succeed
	// TODO: first send succeed and second send should fail with ImportResult_ALREADY_EXISTS
	////send same tx second time and expect error
	//_, err = api.SendRawTransaction(ctx, buf.Bytes())
	//require.NotNil(err)
	//expectedErr := txpool.ImportResult_name[int32(txpool.ImportResult_ALREADY_EXISTS)] + ": " + txpoolcfg.AlreadyKnown.String()
	//require.Equal(expectedErr, err.Error())
	//mockSentry.ReceiveWg.Wait()

}

func TestSendRawTransactionUnprotected(t *testing.T) {
	mockSentry, require := mock.MockWithTxPool(t), require.New(t)
	logger := log.New()

	oneBlockStep(mockSentry, require, t)

	expectedTxValue := uint64(4444)

	// Create a legacy signer pre-155
	unprotectedSigner := types.MakeFrontierSigner()

	txn, err := types.SignTx(types.NewTransaction(0, common.Address{1}, uint256.NewInt(expectedTxValue), params.TxGas, uint256.NewInt(10*params.GWei), nil), *unprotectedSigner, mockSentry.Key)
	require.NoError(err)

	ctx, conn := rpcdaemontest.CreateTestGrpcConn(t, mockSentry)
	txPool := txpool.NewTxpoolClient(conn)
	ff := rpchelper.New(ctx, nil, txPool, txpool.NewMiningClient(conn), func() {}, mockSentry.Log)
	api := jsonrpc.NewEthAPI(newBaseApiForTest(mockSentry), mockSentry.DB, mockSentry.DBSMT, nil, txPool, nil, 5000000, 1e18, 100_000, &ethconfig.Defaults, false, 100_000, 128, logger, nil, 1000)
	api.BadTxAllowance = 1

	// Enable unproteced txs flag
	api.AllowUnprotectedTxs = true

	buf := bytes.NewBuffer(nil)
	err = txn.MarshalBinary(buf)
	require.NoError(err)

	txsCh, id := ff.SubscribePendingTxs(1)
	defer ff.UnsubscribePendingTxs(id)

	txHash, err := api.SendRawTransaction(ctx, buf.Bytes())
	require.NoError(err)

	select {
	case got := <-txsCh:
		require.Equal(expectedTxValue, got[0].GetValue().Uint64())
	case <-time.After(20 * time.Second): // Sometimes the channel times out on github actions
		t.Log("Timeout waiting for txn from channel")
		jsonTx, err := api.GetTransactionByHash(ctx, txHash, nil)
		require.NoError(err)
		jsonTxRPCTransaction, ok := jsonTx.(jsonrpc.RPCTransaction)
		require.True(ok)
		require.Equal(expectedTxValue, jsonTxRPCTransaction.Value.Uint64())
	}
}

func transaction(nonce uint64, gaslimit uint64, key *ecdsa.PrivateKey) types.Transaction {
	return pricedTransaction(nonce, gaslimit, u256.Num1, key)
}

func pricedTransaction(nonce uint64, gaslimit uint64, gasprice *uint256.Int, key *ecdsa.PrivateKey) types.Transaction {
	tx, _ := types.SignTx(types.NewTransaction(nonce, common.Address{}, uint256.NewInt(100), gaslimit, gasprice, nil), *types.LatestSignerForChainID(big.NewInt(1337)), key)
	return tx
}

func Test_RejectLowGasPrice(t *testing.T) {
	cases := map[string]struct {
		txPrice   *big.Int
		lowest    *big.Int
		tolerance float64
		rejected  bool
	}{
		"no tolerance, no reject": {
			txPrice:   big.NewInt(100),
			lowest:    big.NewInt(90),
			tolerance: 0,
			rejected:  false,
		},
		"no tolerance, exact match is allowed": {
			txPrice:   big.NewInt(90),
			lowest:    big.NewInt(90),
			tolerance: 0,
			rejected:  false,
		},
		"no tolerance, rejects underpriced": {
			txPrice:   big.NewInt(80),
			lowest:    big.NewInt(90),
			tolerance: 0,
			rejected:  true,
		},
		"tolerance, no reject": {
			txPrice:   big.NewInt(100),
			lowest:    big.NewInt(90),
			tolerance: 0.1,
			rejected:  false,
		},
		"tolerance, allows normally underpriced through": {
			txPrice:   big.NewInt(85),
			lowest:    big.NewInt(90),
			tolerance: 0.1,
			rejected:  false,
		},
		"tolerance, after applying tx is rejected": {
			txPrice:   big.NewInt(80),
			lowest:    big.NewInt(90),
			tolerance: 0.1,
			rejected:  true,
		},
		"tolerance, after applying an exact match is allowed": {
			txPrice:   big.NewInt(81),
			lowest:    big.NewInt(90),
			tolerance: 0.1,
			rejected:  false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.rejected, jsonrpc.ShouldRejectLowGasPrice(tc.txPrice, tc.lowest, tc.tolerance))
		})
	}

}
