package e2e

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"strings"
	"testing"
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
