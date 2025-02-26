package e2e

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/stretchr/testify/require"
)

func TestModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert loss data logic after the 4th `if` block
	blockToInsert := `
// For loss database data
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	count := 0
	for i, line := range lines {
		if strings.Contains(line, "if !batchState.isL1Recovery() {") {
			count++
		}
		if count == 4 && !inserted {
			lines = append(lines[:i+1], append([]string{blockToInsert}, lines[i+1:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted after the 4th 'if' statement")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestRecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For loss database data
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`
	updatedContent := strings.Replace(content, blockToInsert, "", -1)
	updatedContent = strings.Replace(updatedContent, "\"os\"", "", -1)

	// Write the recovered content back to the file
	err = ioutil.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

func TestCheckSeqStop(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	for i := 0; i < 60; i++ {
		batchNum, err := operations.GetBatchNumber()
		require.NoError(t, err)
		require.LessOrEqual(t, batchNum, uint64(5))
		time.Sleep(1 * time.Second)
	}
}

func TestCheckVerify(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if !testVerified {
		return
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   21000,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.NewInt(10 * encoding.Gwei),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	var txs []*types.Transaction
	txs = append(txs, &signedTx)
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
}
