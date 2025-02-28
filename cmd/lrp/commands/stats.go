package commands

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
)

var StatsCmd = &cobra.Command{
	Use:   "stats [file]",
	Short: "Specify a file to display the test report",
	Long:  `Specify a file path as an argument to display a test report in a table.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		err := showReport(filePath)
		if err != nil {
			log.Fatalf("Failed to display test report: %v", err)
		}
	},
}

func monitorContainer(ctx context.Context, containerID, csvPath, tpsCSVPath string, sampleIntv time.Duration, showTPS bool) error {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithVersion("1.45"),
	)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}
	defer cli.Close()

	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	defer ui.Clear()

	totalIntv := 30 * sampleIntv
	titleSuffix := fmt.Sprintf("%.2f Minutes", totalIntv.Minutes())

	// UI components
	lcCPU := widgets.NewPlot()
	lcCPU.Title = "CPU Usage (%) - Last " + titleSuffix
	lcCPU.Data = make([][]float64, 1)
	lcCPU.Data[0] = make([]float64, 30)
	lcCPU.HorizontalScale = 2
	lcCPU.AxesColor = ui.ColorWhite
	lcCPU.LineColors[0] = ui.ColorGreen
	lcCPU.SetRect(0, 0, 68, 20)

	lcMem := widgets.NewPlot()
	lcMem.Title = "Memory Usage (MiB) - Last" + titleSuffix
	lcMem.Data = make([][]float64, 1)
	lcMem.Data[0] = make([]float64, 30)
	lcMem.HorizontalScale = 2
	lcMem.AxesColor = ui.ColorWhite
	lcMem.LineColors[0] = ui.ColorYellow
	lcMem.SetRect(78, 0, 146, 20)

	lcDisk := widgets.NewPlot()
	lcDisk.Title = "Disk I/O (MiB) - Last " + titleSuffix
	lcDisk.Data = make([][]float64, 2)
	lcDisk.Data[0] = make([]float64, 30)
	lcDisk.Data[1] = make([]float64, 30)
	lcDisk.HorizontalScale = 2
	lcDisk.AxesColor = ui.ColorWhite
	lcDisk.LineColors[0] = ui.ColorBlue
	lcDisk.LineColors[1] = ui.ColorCyan
	lcDisk.SetRect(0, 24, 68, 44)

	lcNet := widgets.NewPlot()
	lcNet.Title = "Network I/O (MiB) - " + titleSuffix
	lcNet.Data = make([][]float64, 2)
	lcNet.Data[0] = make([]float64, 30)
	lcNet.Data[1] = make([]float64, 30)
	lcNet.HorizontalScale = 2
	lcNet.AxesColor = ui.ColorWhite
	lcNet.LineColors[0] = ui.ColorMagenta
	lcNet.LineColors[1] = ui.ColorRed
	lcNet.SetRect(78, 24, 146, 44)

	var lcTPS *widgets.Plot
	if showTPS {
		lcTPS = widgets.NewPlot()
		lcTPS.Title = "TPS by Batch Number (Latest 30 Batches)"
		lcTPS.Data = make([][]float64, 1)
		lcTPS.Data[0] = make([]float64, 30)
		lcTPS.HorizontalScale = 2
		lcTPS.AxesColor = ui.ColorWhite
		lcTPS.LineColors[0] = ui.ColorWhite
		lcTPS.SetRect(156, 0, 224, 20)
	}

	logList := widgets.NewList()
	logList.Title = "Container Logs"
	logList.Rows = []string{}
	logList.TextStyle = ui.NewStyle(ui.ColorWhite)
	logList.WrapText = true
	logList.SetRect(0, 0, 146, 44)

	keyHint := widgets.NewParagraph()
	keyHint.Text = "t: Toggle view | q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = false
	if showTPS {
		keyHint.SetRect(0, 68, 224, 72)
	} else {
		keyHint.SetRect(0, 44, 146, 48)
	}

	// Data slices
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

	showStats := true
	const maxVisibleLines = 30
	scrollOffset := 0

	// Main CSV setup (for non-TPS data)
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

	// TPS CSV setup (only for showTPS)
	var tpsCSVFile *os.File
	var tpsCSVWriter *csv.Writer
	if showTPS {
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

	updateLogDisplay := func() {
		totalLines := len(logs)
		if totalLines == 0 {
			logList.Rows = []string{"No logs available yet"}
			return
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		if totalLines > maxVisibleLines && scrollOffset > totalLines-maxVisibleLines {
			scrollOffset = totalLines - maxVisibleLines
		}
		start := scrollOffset
		end := start + maxVisibleLines
		if end > totalLines {
			end = totalLines
		}
		logList.Rows = logs[start:end]
	}

	updateKeyHint := func() {
		if showStats {
			keyHint.Text = "t: Toggle to Logs | q/Ctrl+C: Quit"
		} else {
			keyHint.Text = "t: Toggle to Stats | Up/Down: Scroll | q/Ctrl+C: Quit"
		}
	}

	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()

	// Use logCtx for both stats and logs to ensure synchronized cancellation
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

	// Stats goroutine
	statsDoneChan := make(chan struct{})
	go func() {
		defer close(statsDoneChan)

		decoder := json.NewDecoder(statsStream.Body)
		ticker := time.NewTicker(sampleIntv)
		defer ticker.Stop()

		for {
			select {
			case <-logCtx.Done():
				return
			case <-ticker.C:
				var stat types.StatsJSON
				if err := decoder.Decode(&stat); err != nil {
					if logCtx.Err() != nil { // Context canceled, exit silently
						return
					}
					log.Printf("Failed to decode stats: %v", err)
					continue
				}

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

				// Update data
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

				// Write to main CSV periodically (non-TPS data)
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

				// Render stats
				if showStats {
					if showTPS {
						ui.Render(lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
					} else {
						ui.Render(lcCPU, lcMem, lcDisk, lcNet, keyHint)
					}
				}
			}
		}
	}()

	// Log and TPS goroutine (real-time updates for showTPS)
	logDoneChan := make(chan struct{})
	go func() {
		defer close(logDoneChan)

		scanner := bufio.NewScanner(logReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		// TPS regex patterns
		batchRegex := regexp.MustCompile(`Batch<(\d+)>`)
		durationRegex := regexp.MustCompile(`TotalDuration<(\d+)ms>`)
		txRegex := regexp.MustCompile(`Tx<(\d+)>`)

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
					updateLogDisplay()

					if !showStats {
						ui.Render(logList, keyHint)
					}

					// Parse TPS-related fields if showTPS is enabled
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

							// Update TPS data with automatic sliding
							tpsData = append(tpsData[1:], instantTPS)
							batchLabels = append(batchLabels[1:], fmt.Sprintf("%d", batchNo))
							lcTPS.Data[0] = tpsData
							lcTPS.DataLabels = batchLabels

							// Write to TPS CSV with full data
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

							// Real-time render when TPS updates
							if showStats {
								ui.Render(lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
							}
						}
					}
				}
			}
		}
	}()

	// Event loop
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
				if showStats {
					if showTPS {
						ui.Render(lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
					} else {
						ui.Render(lcCPU, lcMem, lcDisk, lcNet, keyHint)
					}
				} else {
					if len(logs) <= maxVisibleLines {
						scrollOffset = 0
					} else {
						scrollOffset = len(logs) - maxVisibleLines
					}
					updateLogDisplay()
					ui.Render(logList, keyHint)
				}
			case "<Up>":
				if !showStats {
					scrollOffset--
					updateLogDisplay()
					ui.Clear()
					ui.Render(logList, keyHint)
				}
			case "<Down>":
				if !showStats {
					scrollOffset++
					updateLogDisplay()
					ui.Clear()
					ui.Render(logList, keyHint)
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

// showReport displays a report based on CSV data using termui
func showReport(csvPath string) error {
	// Open CSV file
	file, err := os.Open(csvPath)
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

	if len(records) < 2 { // At least header + 1 row
		return fmt.Errorf("CSV file is empty or invalid")
	}

	// Parse header
	header := records[0]
	expectedHeader := []string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx", "Batch", "TxCount", "Duration", "TPS"}
	if len(header) != len(expectedHeader) {
		return fmt.Errorf("invalid CSV header")
	}

	// Variables for computation
	var startTime, endTime time.Time
	var totalTxCount int
	var firstNonZeroBatch, lastNonZeroBatch int

	// Process data rows
	for i, row := range records[1:] {
		// Parse Timestamp
		timestamp, err := time.Parse("2006-01-02 15:04:05", row[0])
		if err != nil {
			log.Printf("Failed to parse timestamp in row %d: %v", i+1, err)
			continue
		}

		// Parse Batch
		batchNo, err := strconv.Atoi(row[7])
		if err != nil {
			log.Printf("Failed to parse Batch in row %d: %v", i+1, err)
			continue
		}

		// Parse TxCount
		txCount, err := strconv.Atoi(row[8])
		if err != nil {
			log.Printf("Failed to parse TxCount in row %d: %v", i+1, err)
			continue
		}

		// Update totalTxCount
		totalTxCount += txCount

		// Find startTime (first non-zero batch)
		if batchNo > 0 && startTime.IsZero() {
			startTime = timestamp
			firstNonZeroBatch = batchNo
		}

		// Update endTime (last non-zero batch)
		if batchNo > 0 {
			endTime = timestamp
			lastNonZeroBatch = batchNo
		}
	}

	// Calculate duration and average TPS
	duration := endTime.Sub(startTime).Seconds()
	avgTPS := float64(totalTxCount) / duration
	if duration <= 0 {
		avgTPS = 0
	}
	fmt.Println(totalTxCount)

	// Initialize termui
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	defer ui.Clear()

	// Create report table
	table := widgets.NewTable()
	table.Title = "Replay Report"
	table.Rows = [][]string{
		{"From Batch:", fmt.Sprintf("%d", firstNonZeroBatch)},
		{"To Batch:", fmt.Sprintf("%d", lastNonZeroBatch)},
		{"Start Time:", startTime.Format("2006-01-02 15:04:05")},
		{"End Time:", endTime.Format("2006-01-02 15:04:05")},
		{"Total Transactions:", fmt.Sprintf("%d", totalTxCount)},
		{"Replay Duration:", fmt.Sprintf("%.2f seconds", duration)},
		{"Average TPS:", fmt.Sprintf("%.2f", avgTPS)},
	}
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.RowSeparator = true
	table.BorderStyle = ui.NewStyle(ui.ColorCyan)
	table.SetRect(0, 0, 50, 15)

	// Create key hint
	keyHint := widgets.NewParagraph()
	keyHint.Text = "q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = false
	keyHint.SetRect(0, 30, 50, 27)

	// Render initial UI
	ui.Render(table, keyHint)

	// Event loop
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			}
		}
	}
}
