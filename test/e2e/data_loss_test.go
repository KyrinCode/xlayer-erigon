package e2e

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"github.com/stretchr/testify/require"
)

const nonceFile = "../data/nonce.txt"
const maxLoop = 10000
const stopBatch = 5

// before doFinishBlockAndUpdateState (before RemoveMinedTransactions)
// txs are still in txpool, data is lost in chaindata and not written to datastream yet
func TestDataLoss_1_Step1_ModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
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

	// Insert lose data logic before doFinishBlockAndUpdateState
	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "doFinishBlockAndUpdateState") {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before doFinishBlockAndUpdateState")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestDataLoss_1_Step3_RecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))`

	blockRegex := regexp.MustCompile(`\n?` + regexp.QuoteMeta(blockToInsert) + `\n?`)
	updatedContent := blockRegex.ReplaceAllString(content, "\n")

	importRegex := regexp.MustCompile(`\s*"os",?\s*\n?`)
	updatedContent = importRegex.ReplaceAllString(updatedContent, "")

	importMultiFixRegex := regexp.MustCompile(`import \(\n?"([^"]+)"`)
	updatedContent = importMultiFixRegex.ReplaceAllString(updatedContent, "import (\n\t\"$1\"")

	importSpacingFixRegex := regexp.MustCompile(`\n"([^"]+)"`)
	updatedContent = importSpacingFixRegex.ReplaceAllString(updatedContent, "\n\t\"$1\"")

	importSingleFixRegex := regexp.MustCompile(`import \(\s*\n\t?"([^"]+)"\s*\n\)`)
	updatedContent = importSingleFixRegex.ReplaceAllString(updatedContent, "import \"$1\"")

	// Write the recovered content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

// before 1st CommitAndStart (before RemoveMinedTransactions)
// txs are still in txpool, data is lost in chaindata and not written to datastream yet
func TestDataLoss_2_Step1_ModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
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

	// Insert lose data logic before the 1st CommitAndStart
	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the 1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	count := 0
	for i, line := range lines {
		if strings.Contains(line, "sdb.CommitAndStart()") {
			count++
		}
		if count == 1 && !inserted {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before the 1st CommitAndStart")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestDataLoss_2_Step3_RecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the 1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	blockRegex := regexp.MustCompile(`\n?` + regexp.QuoteMeta(blockToInsert) + `\n?`)
	updatedContent := blockRegex.ReplaceAllString(content, "\n")

	importRegex := regexp.MustCompile(`\s*"os",?\s*\n?`)
	updatedContent = importRegex.ReplaceAllString(updatedContent, "")

	importMultiFixRegex := regexp.MustCompile(`import \(\n?"([^"]+)"`)
	updatedContent = importMultiFixRegex.ReplaceAllString(updatedContent, "import (\n\t\"$1\"")

	importSpacingFixRegex := regexp.MustCompile(`\n"([^"]+)"`)
	updatedContent = importSpacingFixRegex.ReplaceAllString(updatedContent, "\n\t\"$1\"")

	importSingleFixRegex := regexp.MustCompile(`import \(\s*\n\t?"([^"]+)"\s*\n\)`)
	updatedContent = importSingleFixRegex.ReplaceAllString(updatedContent, "import \"$1\"")

	// Write the recovered content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

// before 2nd CommitAndStart (after RemoveMinedTransactions and updateStreamAndCheckRollback)
// txs are removed from txpool, data is lost in chaindata
// assuming current block having been written to ds
func TestDataLoss_3_Step1_ModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
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

	// Insert lose data logic before the 2nd CommitAndStart
	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the 2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	count := 0
	for i, line := range lines {
		if strings.Contains(line, "sdb.CommitAndStart()") {
			count++
		}
		if count == 2 && !inserted {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before the 2nd CommitAndStart")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestDataLoss_3_Step3_RecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the 2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	blockRegex := regexp.MustCompile(`\n?` + regexp.QuoteMeta(blockToInsert) + `\n?`)
	updatedContent := blockRegex.ReplaceAllString(content, "\n")

	importRegex := regexp.MustCompile(`\s*"os",?\s*\n?`)
	updatedContent = importRegex.ReplaceAllString(updatedContent, "")

	importMultiFixRegex := regexp.MustCompile(`import \(\n?"([^"]+)"`)
	updatedContent = importMultiFixRegex.ReplaceAllString(updatedContent, "import (\n\t\"$1\"")

	importSpacingFixRegex := regexp.MustCompile(`\n"([^"]+)"`)
	updatedContent = importSpacingFixRegex.ReplaceAllString(updatedContent, "\n\t\"$1\"")

	importSingleFixRegex := regexp.MustCompile(`import \(\s*\n\t?"([^"]+)"\s*\n\)`)
	updatedContent = importSingleFixRegex.ReplaceAllString(updatedContent, "import \"$1\"")

	// Write the recovered content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

// before the last sdb.tx.Commit (after RemoveMinedTransactions and updateStreamAndCheckRollback)
// txs are removed from txpool, data is lost in chaindata
// assuming current block having been written to ds
func TestDataLoss_4_Step1_ModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
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

	// Insert lose data logic after the 'Finish batch' log
	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "Finish batch") {
			lines = append(lines[:i+1], append([]string{blockToInsert}, lines[i+1:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted after the 'Finish batch' log")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestDataLoss_4_Step3_RecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before the last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))`

	blockRegex := regexp.MustCompile(`\n?` + regexp.QuoteMeta(blockToInsert) + `\n?`)
	updatedContent := blockRegex.ReplaceAllString(content, "\n")

	importRegex := regexp.MustCompile(`\s*"os",?\s*\n?`)
	updatedContent = importRegex.ReplaceAllString(updatedContent, "")

	importMultiFixRegex := regexp.MustCompile(`import \(\n?"([^"]+)"`)
	updatedContent = importMultiFixRegex.ReplaceAllString(updatedContent, "import (\n\t\"$1\"")

	importSpacingFixRegex := regexp.MustCompile(`\n"([^"]+)"`)
	updatedContent = importSpacingFixRegex.ReplaceAllString(updatedContent, "\n\t\"$1\"")

	importSingleFixRegex := regexp.MustCompile(`import \(\s*\n\t?"([^"]+)"\s*\n\)`)
	updatedContent = importSingleFixRegex.ReplaceAllString(updatedContent, "import \"$1\"")

	// Write the recovered content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

// between RemoveMinedTransactions and updateStreamAndCheckRollback
// txs are removed from txpool, data is not written to datastream yet
func TestDataLoss_5_Step1_ModifyCode(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
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

	// Insert lose data logic before updateStreamAndCheckRollback
	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before updateStreamAndCheckRollback:%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("updateStreamAndCheckRollback:%v,%v", batchState.batchNumber, block.Number))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "updateStreamAndCheckRollback(") {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}
	require.True(t, inserted, "Expected the block to be inserted before updateStreamAndCheckRollback")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestDataLoss_5_Step3_RecoverCode(t *testing.T) {
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file for recovery:", err)
	}
	content := string(data)

	blockToInsert := `
// For data loss
if batchState.batchNumber == 5 {
	log.Info(fmt.Sprintf("Stop before updateStreamAndCheckRollback:%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("updateStreamAndCheckRollback:%v,%v", batchState.batchNumber, block.Number))`

	blockRegex := regexp.MustCompile(`\n?` + regexp.QuoteMeta(blockToInsert) + `\n?`)
	updatedContent := blockRegex.ReplaceAllString(content, "\n")

	importRegex := regexp.MustCompile(`\s*"os",?\s*\n?`)
	updatedContent = importRegex.ReplaceAllString(updatedContent, "")

	importMultiFixRegex := regexp.MustCompile(`import \(\n?"([^"]+)"`)
	updatedContent = importMultiFixRegex.ReplaceAllString(updatedContent, "import (\n\t\"$1\"")

	importSpacingFixRegex := regexp.MustCompile(`\n"([^"]+)"`)
	updatedContent = importSpacingFixRegex.ReplaceAllString(updatedContent, "\n\t\"$1\"")

	importSingleFixRegex := regexp.MustCompile(`import \(\s*\n\t?"([^"]+)"\s*\n\)`)
	updatedContent = importSingleFixRegex.ReplaceAllString(updatedContent, "import \"$1\"")

	// Write the recovered content back to the file
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatal("Error writing recovered file:", err)
	}
}

func TestDataLoss_Step2_CheckSeqStop(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	for i := 1; i < maxLoop; i++ {
		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		nonce, err := client.PendingNonceAt(ctx, from)
		// log.Info(fmt.Sprintf("Nonce: %v", nonce))
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(uint64(i) * 10 * encoding.Gwei),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
		batchNum, err := operations.GetBatchNumber()
		log.Info(fmt.Sprintf("Cur Batch Number: %v, nonce :%v", batchNum, nonce))
		require.NoError(t, err)
		if batchNum == uint64(stopBatch-1) {
			log.Info(fmt.Sprintf("Cur Batch Number: %v, nonce :%v", batchNum, nonce))
			err = writeNonce(nonce)
			require.NoError(t, err)
			break
		}
	}

	require.NoError(t, err)

	batchNum, err := operations.GetBatchNumber()
	require.NoError(t, err)
	require.Equal(t, batchNum, uint64(stopBatch-1))
}

func TestDataLoss_Step4_CheckVerify(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	var nonce uint64
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var rNonce uint64
	rNonce, err = readNonce()
	require.NoError(t, err)
	require.Equal(t, nonce, rNonce+1)
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
	require.NoError(t, err)
	var txs []*types.Transaction
	txs = append(txs, &signedTx)
	log.Info(fmt.Sprintf("signedTx nonce: %v", signedTx.GetNonce()))
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
	err = writeNonce(nonce)
	require.NoError(t, err)
}

func writeNonce(nonce uint64) error {
	data := strconv.FormatUint(nonce, 10)
	return os.WriteFile(nonceFile, []byte(data), 0644)
}

func readNonce() (uint64, error) {
	data, err := os.ReadFile(nonceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0, nil
	}

	nonce, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}
