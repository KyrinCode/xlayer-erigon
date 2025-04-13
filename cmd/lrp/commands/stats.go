package commands

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// var StatsCmd = &cobra.Command{
// 	Use:   "stats [batchStart] [batchEnd]",
// 	Short: "Display the test report for a specified batch range",
// 	Long:  `Specify batchStart and batchEnd as arguments to display the top 5 historical test reports for that batch range in a table.`,
// 	Args:  cobra.ExactArgs(2),
// 	Run: func(cmd *cobra.Command, args []string) {
// 		// Parse batchStart and batchEnd from arguments
// 		batchStart, err := strconv.Atoi(args[0])
// 		if err != nil {
// 			log.Fatalf("Invalid batchStart: %v", err)
// 		}
// 		batchEnd, err := strconv.Atoi(args[1])
// 		if err != nil {
// 			log.Fatalf("Invalid batchEnd: %v", err)
// 		}

// 		// Call showHistoryReport with the parsed batch range
// 		err = showHistoryReport(path, batchStart, batchEnd)
// 		if err != nil {
// 			log.Fatalf("Failed to display test report: %v", err)
// 		}
// 	},
// }

func monitorContainer(ctx context.Context, containerID, csvPath string, sampleIntv time.Duration, showTPS bool) error {
	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	// Initialize TermUI
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	defer ui.Clear()

	// Set time window title suffix
	totalIntv := 30 * sampleIntv
	titleSuffix := fmt.Sprintf("%.2f Minutes", totalIntv.Minutes())

	lcCPU := widgets.NewPlot()
	lcCPU.Title = "CPU Usage (%) - Last " + titleSuffix
	lcCPU.Data = make([][]float64, 1)
	lcCPU.Data[0] = make([]float64, 30)
	lcCPU.HorizontalScale = 2
	lcCPU.AxesColor = ui.ColorWhite
	lcCPU.LineColors[0] = ui.ColorGreen

	lcMem := widgets.NewPlot()
	lcMem.Title = "Memory Usage (MiB) - Last " + titleSuffix
	lcMem.Data = make([][]float64, 1)
	lcMem.Data[0] = make([]float64, 30)
	lcMem.HorizontalScale = 2
	lcMem.AxesColor = ui.ColorWhite
	lcMem.LineColors[0] = ui.ColorYellow

	lcDisk := widgets.NewPlot()
	lcDisk.Title = "Disk I/O (MiB) - Last " + titleSuffix
	lcDisk.Data = make([][]float64, 2)
	lcDisk.Data[0] = make([]float64, 30)
	lcDisk.Data[1] = make([]float64, 30)
	lcDisk.HorizontalScale = 2
	lcDisk.AxesColor = ui.ColorWhite
	lcDisk.LineColors[0] = ui.ColorBlue
	lcDisk.LineColors[1] = ui.ColorCyan

	lcNet := widgets.NewPlot()
	lcNet.Title = "Network I/O (MiB) - Last " + titleSuffix
	lcNet.Data = make([][]float64, 2)
	lcNet.Data[0] = make([]float64, 30)
	lcNet.Data[1] = make([]float64, 30)
	lcNet.HorizontalScale = 2
	lcNet.AxesColor = ui.ColorWhite
	lcNet.LineColors[0] = ui.ColorMagenta
	lcNet.LineColors[1] = ui.ColorRed

	var lcTPS *widgets.Plot
	if showTPS {
		lcTPS = widgets.NewPlot()
		lcTPS.Title = "TPS by Batch Number (Latest 30 Batches)"
		lcTPS.Data = make([][]float64, 1)
		lcTPS.Data[0] = make([]float64, 30)
		lcTPS.HorizontalScale = 2
		lcTPS.AxesColor = ui.ColorWhite
		lcTPS.LineColors[0] = ui.ColorWhite
	}

	logList := widgets.NewList()
	logList.Title = "Container Logs"
	logList.Rows = []string{}
	logList.TextStyle = ui.NewStyle(ui.ColorWhite)
	logList.WrapText = true

	titleBar := widgets.NewParagraph()
	titleBar.Text = fmt.Sprintf("Container: %s - Monitoring Stats & Logs", containerID)
	titleBar.TextStyle = ui.NewStyle(ui.ColorYellow, ui.ColorBlack, ui.ModifierBold)
	titleBar.Border = true

	keyHint := widgets.NewParagraph()
	keyHint.Text = "t: Toggle view | q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = true

	// Data storage
	xLabels := make([]string, 30)
	batchLabels := make([]string, 30)
	startTime := time.Now()
	for i := 0; i < 30; i++ {
		xLabels[i] = fmt.Sprintf("%d", (29-i)*10)
		batchLabels[i] = ""
	}
	lcCPU.DataLabels = xLabels
	lcMem.DataLabels = xLabels
	lcDisk.DataLabels = xLabels
	lcNet.DataLabels = xLabels
	if showTPS {
		lcTPS.DataLabels = batchLabels
	}

	cpuData := make([]float64, 30)
	memData := make([]float64, 30)
	diskWriteData := make([]float64, 30)
	diskReadData := make([]float64, 30)
	netRxData := make([]float64, 30)
	netTxData := make([]float64, 30)
	tpsData := make([]float64, 30)
	logs := make([]string, 0, 1000)

	// State variables
	showStats := true
	scrollOffset := 0

	// Initialize CSV file
	csvFile, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open main CSV file: %v", err)
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	if stat, err := csvFile.Stat(); err == nil && stat.Size() == 0 {
		err = csvWriter.Write([]string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx"})
		if err != nil {
			return fmt.Errorf("failed to write main CSV header: %v", err)
		}
	}

	var tpsCSVFile *os.File
	var tpsCSVWriter *csv.Writer
	if showTPS {
		tpsCSVPath := csvPath[:len(csvPath)-4] + "-tps.csv"
		tpsCSVFile, err = os.OpenFile(tpsCSVPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open TPS CSV file: %v", err)
		}
		defer tpsCSVFile.Close()

		tpsCSVWriter = csv.NewWriter(tpsCSVFile)
		defer tpsCSVWriter.Flush()

		if stat, err := tpsCSVFile.Stat(); err == nil && stat.Size() == 0 {
			err = tpsCSVWriter.Write([]string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx", "Batch", "TxCount", "Duration", "TPS"})
			if err != nil {
				return fmt.Errorf("failed to write TPS CSV header: %v", err)
			}
		}
	}

	// Update log display function
	updateLogDisplay := func(maxHeight int) {
		totalLines := len(logs)
		if totalLines == 0 {
			logList.Rows = []string{"No logs available yet"}
			return
		}
		maxVisibleLines := maxHeight - 2
		if maxVisibleLines < 1 {
			maxVisibleLines = 1
		}
		if totalLines > maxVisibleLines {
			scrollOffset = totalLines - maxVisibleLines
		} else {
			scrollOffset = 0
		}
		start := scrollOffset
		end := start + maxVisibleLines
		if end > totalLines {
			end = totalLines
		}
		logList.Rows = logs[start:end]
	}

	// Update key hint
	updateKeyHint := func() {
		if showStats {
			keyHint.Text = "t: Toggle to Logs | q/Ctrl+C: Quit"
		} else {
			keyHint.Text = "t: Toggle to Stats | q/Ctrl+C: Quit"
		}
	}

	// Adjust UI layout
	resizeUI := func(width, height int) {
		const minWidth, minHeight = 60, 20
		if width < minWidth || height < minHeight {
			return
		}

		titleBar.SetRect(0, 0, width, 3)
		keyHint.SetRect(0, height-3, width, height)

		if showStats {
			availableHeight := height - 6
			halfWidth := width / 2
			halfHeight := availableHeight / 2

			lcCPU.SetRect(0, 3, halfWidth, 3+halfHeight)
			lcMem.SetRect(halfWidth, 3, width, 3+halfHeight)
			lcDisk.SetRect(0, 3+halfHeight, halfWidth, height-3)
			lcNet.SetRect(halfWidth, 3+halfHeight, width, height-3)

			if showTPS {
				thirdWidth := width / 3
				lcTPS.SetRect(0, 3, thirdWidth, height-3)
				lcCPU.SetRect(thirdWidth, 3, 2*thirdWidth, 3+availableHeight/2)
				lcMem.SetRect(thirdWidth, 3+availableHeight/2, 2*thirdWidth, height-3)
				lcDisk.SetRect(2*thirdWidth, 3+availableHeight/2, width, height-3)
				lcNet.SetRect(2*thirdWidth, 3, width, 3+availableHeight/2)
			}
		} else {
			logList.SetRect(0, 3, width, height-3)
			updateLogDisplay(height - 6)
		}
	}

	w, h := ui.TerminalDimensions()
	resizeUI(w, h)

	// Create context for logs and stats streams
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()

	statsStream, err := cli.ContainerStats(logCtx, containerID, true)
	if err != nil {
		return fmt.Errorf("failed to get container stats: %v", err)
	}
	defer statsStream.Body.Close()

	logReader, err := cli.ContainerLogs(logCtx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
		Timestamps: true,
	})
	if err != nil {
		return fmt.Errorf("failed to get container logs: %v", err)
	}
	defer logReader.Close()

	// Stats processing
	statsDoneChan := make(chan struct{})
	type statsHolder struct {
		sync.Mutex
		stats types.StatsJSON
	}
	currentStats := &statsHolder{}

	go func() {
		decoder := json.NewDecoder(statsStream.Body)
		for {
			var stats types.StatsJSON
			if err := decoder.Decode(&stats); err != nil {
				log.Printf("Stream decode error: %v", err)
				return
			}
			currentStats.Lock()
			currentStats.stats = stats
			currentStats.Unlock()
		}
	}()

	// Update stats data
	updateStats := func() {
		currentStats.Lock()
		stat := currentStats.stats
		currentStats.Unlock()

		var (
			cpuUsage, memoryUsage float64
			diskRead, diskWrite   float64
			netRx, netTx          float64
		)

		cpuUsage = calculateCPUUsage(stat.CPUStats, stat.PreCPUStats)
		memoryUsage = calculateMemoryUsage(stat.MemoryStats)
		diskRead, diskWrite = calculateBlockIO(stat.BlkioStats)

		for _, net := range stat.Networks {
			netRx += float64(net.RxBytes) / 1024 / 1024
			netTx += float64(net.TxBytes) / 1024 / 1024
		}

		cpuData = append(cpuData[1:], cpuUsage)
		memData = append(memData[1:], memoryUsage)
		diskReadData = append(diskReadData[1:], diskRead)
		diskWriteData = append(diskWriteData[1:], diskWrite)
		netRxData = append(netRxData[1:], netRx)
		netTxData = append(netTxData[1:], netTx)

		elapsed := int(time.Since(startTime).Seconds()) / 10 * 10
		for i := 0; i < 30; i++ {
			timeAgo := elapsed - (29-i)*10
			if timeAgo < 0 {
				timeAgo = 0
			}
			xLabels[i] = fmt.Sprintf("%d", timeAgo)
		}
		lcCPU.DataLabels = xLabels
		lcMem.DataLabels = xLabels
		lcDisk.DataLabels = xLabels
		lcNet.DataLabels = xLabels

		lcCPU.Data[0] = cpuData
		lcMem.Data[0] = memData
		lcDisk.Data[0] = diskReadData
		lcDisk.Data[1] = diskWriteData
		lcNet.Data[0] = netRxData
		lcNet.Data[1] = netTxData

		timestamp := time.Now().Format("2006-01-02 15:04:05")
		csvRow := []string{
			timestamp,
			fmt.Sprintf("%.2f", cpuUsage),
			fmt.Sprintf("%.2f", memoryUsage),
			fmt.Sprintf("%.2f", diskRead),
			fmt.Sprintf("%.2f", diskWrite),
			fmt.Sprintf("%.2f", netRx),
			fmt.Sprintf("%.2f", netTx),
		}
		if err := csvWriter.Write(csvRow); err != nil {
			log.Printf("Failed to write periodic main CSV: %v", err)
		}
		csvWriter.Flush()
	}

	// Display stats immediately on start
	go func() {
		for currentStats.stats.CPUStats.CPUUsage.TotalUsage == 0 {
			time.Sleep(50 * time.Millisecond)
		}
		updateStats()
		if showTPS {
			ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
		} else {
			ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, keyHint)
		}
	}()

	// Periodically update stats
	go func() {
		defer close(statsDoneChan)

		ticker := time.NewTicker(sampleIntv)
		defer ticker.Stop()

		for {
			select {
			case <-logCtx.Done():
				return
			case <-ticker.C:
				updateStats()
				if showStats {
					if showTPS {
						ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
					} else {
						ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, keyHint)
					}
				}
			}
		}
	}()

	// Log processing
	logDoneChan := make(chan struct{})
	go func() {
		defer close(logDoneChan)

		scanner := bufio.NewScanner(logReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		batchRegex := regexp.MustCompile(`Batch<(\d+)>`)
		durationRegex := regexp.MustCompile(`TotalDuration-batch<(\d+)ms>`)
		txRegex := regexp.MustCompile(`Txs<(\d+)>`)

		for {
			select {
			case <-logCtx.Done():
				return
			default:
				if !scanner.Scan() {
					if err := scanner.Err(); err != nil && err != io.EOF {
						log.Printf("Scanner error: %v", err)
					}
					return
				}
				line := scanner.Text()
				if len(line) > 8 {
					line = line[8:]
					if len(logs) >= 1000 {
						logs = logs[1:]
						if scrollOffset > 0 {
							scrollOffset--
						}
					}
					logs = append(logs, line)
					if !showStats {
						_, h := ui.TerminalDimensions()
						updateLogDisplay(h - 6)
						ui.Clear()
						ui.Render(titleBar, logList, keyHint)
					}

					if showTPS && strings.Contains(line, "Batch") && strings.Contains(line, "TotalDuration") && strings.Contains(line, "Tx") {
						batchMatch := batchRegex.FindStringSubmatch(line)
						durationMatch := durationRegex.FindStringSubmatch(line)
						txMatch := txRegex.FindStringSubmatch(line)

						if batchMatch != nil && durationMatch != nil && txMatch != nil {
							batchNo, _ := strconv.Atoi(batchMatch[1])
							durationMs, _ := strconv.Atoi(durationMatch[1])
							txCount, _ := strconv.Atoi(txMatch[1])

							var instantTPS float64
							if durationMs > 0 {
								instantTPS = float64(txCount*1000) / float64(durationMs)
							}

							tpsData = append(tpsData[1:], instantTPS)
							batchLabels = append(batchLabels[1:], fmt.Sprintf("%d", batchNo))
							lcTPS.Data[0] = tpsData
							lcTPS.DataLabels = batchLabels

							timestamp := time.Now().Format("2006-01-02 15:04:05")
							tpsCSVRow := []string{
								timestamp,
								fmt.Sprintf("%.2f", cpuData[len(cpuData)-1]),
								fmt.Sprintf("%.2f", memData[len(memData)-1]),
								fmt.Sprintf("%.2f", diskReadData[len(diskReadData)-1]),
								fmt.Sprintf("%.2f", diskWriteData[len(diskWriteData)-1]),
								fmt.Sprintf("%.2f", netRxData[len(netRxData)-1]),
								fmt.Sprintf("%.2f", netTxData[len(netTxData)-1]),
								fmt.Sprintf("%d", batchNo),
								fmt.Sprintf("%d", txCount),
								fmt.Sprintf("%d", durationMs),
								fmt.Sprintf("%.2f", instantTPS),
							}
							if err := tpsCSVWriter.Write(tpsCSVRow); err != nil {
								log.Printf("Failed to write TPS CSV: %v", err)
							}
							tpsCSVWriter.Flush()

							if showStats {
								ui.Clear()
								ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
							}
						}
					}
				}
			}
		}
	}()

	uiEvents := ui.PollEvents()
mainLoop:
	for {
		select {
		case <-ctx.Done():
			logCancel()
			<-logDoneChan
			<-statsDoneChan
			break mainLoop
		case e := <-uiEvents:
			switch e.Type {
			case ui.KeyboardEvent:
				switch e.ID {
				case "q", "<C-c>":
					logCancel()
					<-logDoneChan
					<-statsDoneChan
					return fmt.Errorf("capture a quit signal, program interrupted")
				case "t":
					showStats = !showStats
					ui.Clear()
					updateKeyHint()
					w, h := ui.TerminalDimensions()
					resizeUI(w, h)
					if showStats {
						if showTPS {
							ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
						} else {
							ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, keyHint)
						}
					} else {
						totalLines := len(logs)
						maxVisibleLines := h - 6
						if totalLines > maxVisibleLines {
							scrollOffset = totalLines - maxVisibleLines
						} else {
							scrollOffset = 0
						}
						updateLogDisplay(h - 6)
						ui.Render(titleBar, logList, keyHint)
					}
				}
			case ui.ResizeEvent:
				payload := e.Payload.(ui.Resize)
				resizeUI(payload.Width, payload.Height)
				ui.Clear()
				if showStats {
					if showTPS {
						ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
					} else {
						ui.Render(titleBar, lcCPU, lcMem, lcDisk, lcNet, keyHint)
					}
				} else {
					totalLines := len(logs)
					maxVisibleLines := payload.Height - 6
					if totalLines > maxVisibleLines {
						scrollOffset = totalLines - maxVisibleLines
					} else {
						scrollOffset = 0
					}
					updateLogDisplay(payload.Height - 6)
					ui.Render(titleBar, logList, keyHint)
				}
			}
		}
	}
	return nil
}

func calculateMemoryUsage(memoryStats types.MemoryStats) float64 {
	usage := float64(memoryStats.Usage)
	if inactiveFile, ok := memoryStats.Stats["inactive_file"]; ok && inactiveFile > 0 {
		activeMemory := usage - float64(inactiveFile)
		if activeMemory > 0 {
			return activeMemory / 1024 / 1024
		}
	}
	if rss, ok := memoryStats.Stats["rss"]; ok && rss > 0 {
		return float64(rss) / 1024 / 1024
	}
	if totalRss, ok := memoryStats.Stats["total_rss"]; ok && totalRss > 0 {
		return float64(totalRss) / 1024 / 1024
	}
	activeAnon, anonOk := memoryStats.Stats["active_anon"]
	activeFile, fileOk := memoryStats.Stats["active_file"]
	if anonOk && fileOk && (activeAnon > 0 || activeFile > 0) {
		return float64(activeAnon+activeFile) / 1024 / 1024
	}
	return usage / 1024 / 1024
}

func calculateCPUUsage(curCPUStats, preCPUStats types.CPUStats) float64 {
	cpuDelta := float64(curCPUStats.CPUUsage.TotalUsage - preCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(curCPUStats.SystemUsage - preCPUStats.SystemUsage)
	cpuCount := float64(curCPUStats.OnlineCPUs)
	if systemDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / systemDelta) * cpuCount * 100.0
	}
	return 0.0
}

func calculateBlockIO(blkioStats types.BlkioStats) (rx float64, tx float64) {
	for _, blk := range blkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(blk.Op) {
		case "read":
			rx += float64(blk.Value) / 1024 / 1024
		case "write":
			tx += float64(blk.Value) / 1024 / 1024
		}
	}
	return rx, tx
}

// TestResult defines the structure for storing test result data
type TestResult struct {
	FromBatch           int       `json:"from_batch"`
	ToBatch             int       `json:"to_batch"`
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	TotalTxCount        int       `json:"total_tx_count"`
	Duration            float64   `json:"duration"`
	AverageTPS          float64   `json:"average_tps"`
	StateRootMismatch   bool      `json:"state_root_mismatch"`
	MismatchBlockHeight int       `json:"mismatch_block_height"`
	GitCommitID         string    `json:"git_commit_id"`
	TestTime            time.Time `json:"test_time"`
}

// showReport generates and displays the current replay report with history below
func showReport(path, workDir, commitID string) error {
	replayTPSCSV := filepath.Join(workDir, "replay-container-stats-tps.csv")
	replayLog := filepath.Join(workDir, "replay.log")

	// Open CSV file
	file, err := os.Open(replayTPSCSV)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	// Read CSV data
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %v", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV file is empty or invalid")
	}

	// Validate CSV header
	header := records[0]
	expectedHeader := []string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx", "Batch", "TxCount", "Duration", "TPS"}
	if len(header) != len(expectedHeader) {
		return fmt.Errorf("invalid CSV header")
	}

	var startTime, endTime time.Time
	var totalTxCount int
	var firstNonZeroBatch, lastNonZeroBatch int

	// Process CSV rows
	for i, row := range records[1:] {
		timestamp, err := time.Parse("2006-01-02 15:04:05", row[0])
		if err != nil {
			log.Printf("Failed to parse timestamp in row %d: %v", i+1, err)
			continue
		}

		batchNo, err := strconv.Atoi(row[7])
		if err != nil {
			log.Printf("Failed to parse Batch in row %d: %v", i+1, err)
			continue
		}

		txCount, err := strconv.Atoi(row[8])
		if err != nil {
			log.Printf("Failed to parse TxCount in row %d: %v", i+1, err)
			continue
		}

		totalTxCount += txCount

		if batchNo > 0 && startTime.IsZero() {
			startTime = timestamp
			firstNonZeroBatch = batchNo
		}
		if batchNo > 0 {
			endTime = timestamp
			lastNonZeroBatch = batchNo
		}
	}

	// Calculate duration and TPS
	duration := endTime.Sub(startTime).Seconds()
	avgTPS := float64(totalTxCount) / duration
	if duration <= 0 {
		avgTPS = 0
	}

	// Check state root mismatch
	stateRootMismatch, mismatchBlockHeight := checkStateRootMismatch(replayLog)

	// Create current test result
	currentResult := TestResult{
		FromBatch:           firstNonZeroBatch,
		ToBatch:             lastNonZeroBatch,
		StartTime:           startTime,
		EndTime:             endTime,
		TotalTxCount:        totalTxCount,
		Duration:            duration,
		AverageTPS:          avgTPS,
		StateRootMismatch:   stateRootMismatch,
		MismatchBlockHeight: mismatchBlockHeight,
		GitCommitID:         commitID,
		TestTime:            time.Now(),
	}

	// Determine batch range file
	batchRangeFile := filepath.Join(path, utils.HISTORY_FOLDER, fmt.Sprintf("batch_%d-%d.json", firstNonZeroBatch, lastNonZeroBatch))

	// Save current result to batch-specific file
	err = saveTestResult(batchRangeFile, currentResult)
	if err != nil {
		log.Printf("Failed to save test result: %v", err)
	}

	// Load history results
	historyResults, err := loadTestHistory(batchRangeFile)
	if err != nil {
		log.Printf("Failed to load history: %v", err)
		historyResults = []TestResult{}
	}
	log.Printf("Loaded %d historical results", len(historyResults))

	// Initialize termui
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	var misMatchBlockResult string
	if mismatchBlockHeight == 0 {
		misMatchBlockResult = "N/A"
	} else {
		misMatchBlockResult = fmt.Sprintf("%d", mismatchBlockHeight)
	}

	// Create current result table
	currentTable := widgets.NewTable()
	currentTable.Title = "Replay Report"
	currentTable.Rows = [][]string{
		{"From Batch:", fmt.Sprintf("%d", firstNonZeroBatch)},
		{"To Batch:", fmt.Sprintf("%d", lastNonZeroBatch)},
		{"Git Commit ID:", commitID},
		{"Start Time:", startTime.Format("2006-01-02 15:04:05")},
		{"End Time:", endTime.Format("2006-01-02 15:04:05")},
		{"Total Transactions:", fmt.Sprintf("%d", totalTxCount)},
		{"Replay Duration:", fmt.Sprintf("%.2f seconds", duration)},
		{"Average TPS:", fmt.Sprintf("%.2f", avgTPS)},
		{"State Root Mismatch:", fmt.Sprintf("%t", stateRootMismatch)},
		{"Mismatch Block Height:", misMatchBlockResult},
	}
	if !stateRootMismatch {
		currentTable.Rows[8][1] = "N/A"
	}
	currentTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	currentTable.RowSeparator = true
	currentTable.BorderStyle = ui.NewStyle(ui.ColorCyan)

	// Create history table
	p := message.NewPrinter(language.English)
	historyTable := widgets.NewTable()
	historyTable.Title = fmt.Sprintf("Historical Results (Top 3 TPS for Batch %d-%d)", firstNonZeroBatch, lastNonZeroBatch)
	historyTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	historyTable.RowSeparator = true
	historyTable.BorderStyle = ui.NewStyle(ui.ColorCyan)
	historyTable.Rows = [][]string{
		{"Batch Range", "Start Time", "End Time", "Tx Count", "Duration (s)", "TPS", "State Root Mismatch", "Mismatch Block Height", "Git Commit ID"},
	}
	sort.Slice(historyResults, func(i, j int) bool {
		return historyResults[i].AverageTPS > historyResults[j].AverageTPS
	})
	maxHistory := 5 // Show up to 3 historical entries (4 rows total with header)
	historyCount := 0
	for i, result := range historyResults {
		if historyCount >= maxHistory {
			break
		}
		// Only skip if this is the exact current result (last added)
		if i == len(historyResults)-1 && result.TestTime.Equal(currentResult.TestTime) && result.GitCommitID == currentResult.GitCommitID {
			log.Printf("Skipping current result: TestTime=%v, GitCommitID=%s", result.TestTime, result.GitCommitID)
			continue
		}
		var misMatchBlockResult string
		if result.MismatchBlockHeight == 0 {
			misMatchBlockResult = "N/A"
		} else {
			misMatchBlockResult = fmt.Sprintf("%d", result.MismatchBlockHeight)
		}

		row := []string{
			fmt.Sprintf("%d - %d", result.FromBatch, result.ToBatch),
			result.GitCommitID,
			result.StartTime.Format("2006-01-02 15:04:05"),
			result.EndTime.Format("2006-01-02 15:04:05"),
			p.Sprintf("%d", result.TotalTxCount),
			p.Sprintf("%.2f", result.Duration),
			p.Sprintf("%.2f", result.AverageTPS),
			fmt.Sprintf("%t", result.StateRootMismatch),
			misMatchBlockResult,
		}
		historyTable.Rows = append(historyTable.Rows, row)
		historyCount++
		log.Printf("Added history row %d: %v", historyCount, row)
	}
	log.Printf("History table has %d rows (including header)", len(historyTable.Rows))

	// Create quit hint
	keyHint := widgets.NewParagraph()
	keyHint.Text = "q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = false

	// Function to update table sizes based on terminal dimensions
	updateLayout := func() {
		termWidth, termHeight := ui.TerminalDimensions()

		// Current table: Half width, full height for 10 rows
		currentFullHeight := len(currentTable.Rows)*2 + 1 // 10 rows * 2 (with separator) + 1 for title
		currentHeight := currentFullHeight
		minHistoryHeight := 6 // Header + 3 rows + title + border
		minKeyHeight := 2     // Key hint
		minTotalHeight := currentFullHeight + minHistoryHeight + minKeyHeight
		if termHeight < minTotalHeight {
			// Allocate 60% to current table if terminal is too small
			currentHeight = int(float64(termHeight) * 0.6)
			if currentHeight < 4 {
				currentHeight = 4
			}
			if currentHeight > currentFullHeight {
				currentHeight = currentFullHeight
			}
		}
		currentTable.SetRect(0, 0, termWidth/2, currentHeight)

		// History table: Full width, ensure enough height for 3 rows + header
		historyFullHeight := (len(historyTable.Rows)+1)*2 + 1
		historyHeight := historyFullHeight
		availableHeight := termHeight - currentHeight - minKeyHeight
		if availableHeight < minHistoryHeight {
			historyHeight = minHistoryHeight
		} else if historyHeight > availableHeight {
			historyHeight = availableHeight
		}
		historyTable.SetRect(0, currentHeight+2, termWidth, currentHeight+historyHeight)

		// Key hint: Below history table
		keyHint.SetRect(0, currentHeight+historyHeight, termWidth/2, currentHeight+historyHeight+4)

		ui.Render(currentTable, historyTable, keyHint)
	}

	// Initial layout
	updateLayout()

	// Event loop with resize handling
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			case "<Resize>":
				ui.Clear()
				updateLayout()
			}
		}
	}
}

// showHistoryReport displays historical results for a specific batch range
func showHistoryReport(path string, fromBatch, toBatch int) error {
	// Initialize termui
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	// Determine batch range file
	batchRangeFile := filepath.Join(path, fmt.Sprintf("batch_%d-%d.json", fromBatch, toBatch))

	// Load history results
	historyResults, err := loadTestHistory(batchRangeFile)
	if err != nil {
		log.Printf("Failed to load history: %v", err)
		historyResults = []TestResult{}
	}

	// Create history table
	p := message.NewPrinter(language.English)
	historyTable := widgets.NewTable()
	historyTable.Title = fmt.Sprintf("Historical Results (Top 3 TPS for Batch %d-%d)", fromBatch, toBatch)
	historyTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	historyTable.RowSeparator = true
	historyTable.BorderStyle = ui.NewStyle(ui.ColorCyan)
	historyTable.Rows = [][]string{
		{"Batch Range", "Git Commit ID", "Start Time", "End Time", "Tx Count", "Duration (s)", "TPS", "State Root Mismatch", "Mismatch Block Height"},
	}
	sort.Slice(historyResults, func(i, j int) bool {
		return historyResults[i].AverageTPS > historyResults[j].AverageTPS
	})
	maxHistory := 5 // Show up to 5 historical entries (4 rows total with header)
	historyCount := 0
	for _, result := range historyResults {
		if historyCount >= maxHistory {
			break
		}
		var misMatchBlockResult string
		if result.MismatchBlockHeight == 0 {
			misMatchBlockResult = "N/A"
		} else {
			misMatchBlockResult = fmt.Sprintf("%d", result.MismatchBlockHeight)
		}
		row := []string{
			fmt.Sprintf("%d - %d", result.FromBatch, result.ToBatch),
			result.GitCommitID,
			result.StartTime.Format("2006-01-02 15:04:05"),
			result.EndTime.Format("2006-01-02 15:04:05"),
			p.Sprintf("%d", result.TotalTxCount),
			p.Sprintf("%.2f", result.Duration),
			p.Sprintf("%.2f", result.AverageTPS),
			fmt.Sprintf("%t", result.StateRootMismatch),
			misMatchBlockResult,
		}
		historyTable.Rows = append(historyTable.Rows, row)
		historyCount++
	}

	// Create quit hint
	keyHint := widgets.NewParagraph()
	keyHint.Text = "q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = false

	// Function to update table size based on terminal dimensions
	updateLayout := func() {
		termWidth, termHeight := ui.TerminalDimensions()

		// History table: Full width, height based on rows, capped by terminal height
		historyHeight := (len(historyTable.Rows) * 2) + 1 // +2 for title and border
		if historyHeight > termHeight-2 {                 // Leave space for key hint
			historyHeight = termHeight - 2
		}
		if historyHeight < 4 { // Minimum height to show title and header
			historyHeight = 4
		}
		historyTable.SetRect(0, 0, termWidth, historyHeight)

		// Key hint: Below history table
		keyHint.SetRect(0, historyHeight+2, termWidth/2, historyHeight+6)

		ui.Render(historyTable, keyHint)
	}

	// Initial layout
	updateLayout()

	// Event loop with resize handling
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			case "<Resize>":
				ui.Clear()
				updateLayout()
			}
		}
	}
}

// checkStateRootMismatch checks for state root mismatch in the log file
func checkStateRootMismatch(logPath string) (bool, int) {
	file, err := os.Open(logPath)
	if err != nil {
		log.Printf("Failed to open log file %s: %v", logPath, err)
		return false, 0
	}
	defer file.Close()

	re := regexp.MustCompile(`State root mismatch of block (\d+) after resequencing`)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			blockNumber, err := strconv.Atoi(matches[1])
			if err != nil {
				log.Printf("Failed to parse block number: %v", err)
				continue
			}
			return true, blockNumber
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading log file %s: %v", logPath, err)
	}
	return false, 0
}

// saveTestResult saves a test result to a batch-specific file
func saveTestResult(filePath string, result TestResult) error {
	var history []TestResult

	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %v", dir, err)
	}

	// Load existing history if file exists
	if _, err := os.Stat(filePath); err == nil {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read history file: %v", err)
		}
		if err := json.Unmarshal(data, &history); err != nil {
			return fmt.Errorf("failed to unmarshal history: %v", err)
		}
	} else if !os.IsNotExist(err) {
		// If the error is not "file does not exist," return it
		return fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}

	// Append new result
	history = append(history, result)

	// Write back to file
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %v", err)
	}
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write to file %s: %v", filePath, err)
	}
	return nil
}

// loadTestHistory loads test history from a batch-specific file
func loadTestHistory(filePath string) ([]TestResult, error) {
	var history []TestResult
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return history, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read history file: %v", err)
	}
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history: %v", err)
	}
	return history, nil
}
