package commands

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
)

var StatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Monitor Docker container stats with termui",
	Long:  `Monitor Docker container stats and display CPU, memory, disk, and network stats in sliding line charts.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			log.Fatal("Please provide two container IDs")
		}
		ctx := context.Background()
		csvPath1 := "./container1_stats.csv"
		csvPath2 := "./container2_stats.csv"

		// Monitor first container (no TPS)
		fmt.Println("Monitoring container 1:", args[0])
		err := monitorContainer(ctx, args[0], csvPath1, false)
		if err != nil {
			log.Printf("Error monitoring container 1: %v", err)
		}

		// Monitor second container (with TPS)
		fmt.Println("Monitoring container 2:", args[1])
		err = monitorContainer(ctx, args[1], csvPath2, true)
		if err != nil {
			log.Fatalf("Error monitoring container 2: %v", err)
		}
	},
}

// monitorContainer monitors a single container with optional TPS display
func monitorContainer(ctx context.Context, containerID, csvPath string, showTPS bool) error {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithVersion("1.45"),
	)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}

	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	defer ui.Clear()

	// Create stats view
	lcCPU := widgets.NewPlot()
	lcCPU.Title = "CPU Usage (%) - Last 5 Minutes"
	lcCPU.Data = make([][]float64, 1)
	lcCPU.Data[0] = make([]float64, 30)
	lcCPU.HorizontalScale = 2
	lcCPU.AxesColor = ui.ColorWhite
	lcCPU.LineColors[0] = ui.ColorGreen
	lcCPU.SetRect(0, 0, 68, 20)

	lcMem := widgets.NewPlot()
	lcMem.Title = "Memory Usage (MB) - Last 5 Minutes"
	lcMem.Data = make([][]float64, 1)
	lcMem.Data[0] = make([]float64, 30)
	lcMem.HorizontalScale = 2
	lcMem.AxesColor = ui.ColorWhite
	lcMem.LineColors[0] = ui.ColorYellow
	lcMem.SetRect(78, 0, 146, 20)

	lcDisk := widgets.NewPlot()
	lcDisk.Title = "Disk I/O (MB) - Last 5 Minutes"
	lcDisk.Data = make([][]float64, 2)
	lcDisk.Data[0] = make([]float64, 30)
	lcDisk.Data[1] = make([]float64, 30)
	lcDisk.HorizontalScale = 2
	lcDisk.AxesColor = ui.ColorWhite
	lcDisk.LineColors[0] = ui.ColorBlue
	lcDisk.LineColors[1] = ui.ColorCyan
	lcDisk.SetRect(0, 24, 68, 44)

	lcNet := widgets.NewPlot()
	lcNet.Title = "Network I/O (MB) - Last 5 Minutes"
	lcNet.Data = make([][]float64, 2)
	lcNet.Data[0] = make([]float64, 30)
	lcNet.Data[1] = make([]float64, 30)
	lcNet.HorizontalScale = 2
	lcNet.AxesColor = ui.ColorWhite
	lcNet.LineColors[0] = ui.ColorMagenta
	lcNet.LineColors[1] = ui.ColorRed
	lcNet.SetRect(78, 24, 146, 44)

	// TPS component
	var lcTPS *widgets.Plot
	if showTPS {
		lcTPS = widgets.NewPlot()
		lcTPS.Title = "TPS - Last 5 Minutes"
		lcTPS.Data = make([][]float64, 1)
		lcTPS.Data[0] = make([]float64, 30)
		lcTPS.HorizontalScale = 2
		lcTPS.AxesColor = ui.ColorWhite
		lcTPS.LineColors[0] = ui.ColorWhite
		lcTPS.SetRect(0, 48, 146, 68)
	}

	// Create logs view
	logList := widgets.NewList()
	logList.Title = "Container Logs"
	logList.Rows = []string{}
	logList.TextStyle = ui.NewStyle(ui.ColorWhite)
	logList.WrapText = true
	if showTPS {
		logList.SetRect(0, 0, 146, 40)
	} else {
		logList.SetRect(0, 0, 146, 44)
	}

	// Create key tips area
	keyHint := widgets.NewParagraph()
	keyHint.Text = "t: Toggle view | q/Ctrl+C: Quit"
	keyHint.TextStyle = ui.NewStyle(ui.ColorCyan)
	keyHint.Border = false
	if showTPS {
		keyHint.SetRect(0, 68, 146, 72)
	} else {
		keyHint.SetRect(0, 44, 146, 48)
	}

	xLabels := make([]string, 30)
	startTime := time.Now()
	for i := 0; i < 30; i++ {
		xLabels[i] = fmt.Sprintf("%d", (29-i)*10)
	}
	lcCPU.DataLabels = xLabels
	lcMem.DataLabels = xLabels
	lcDisk.DataLabels = xLabels
	lcNet.DataLabels = xLabels
	if showTPS {
		lcTPS.DataLabels = xLabels
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

	// Open or create CSV file
	csvFile, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	// Write CSV header
	if stat, err := csvFile.Stat(); err == nil && stat.Size() == 0 {
		if showTPS {
			err = csvWriter.Write([]string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx", "TPS"})
		} else {
			err = csvWriter.Write([]string{"Timestamp", "CPUUsage", "MemoryUsage", "DiskRead", "DiskWrite", "NetRx", "NetTx"})
		}
		if err != nil {
			return fmt.Errorf("failed to write CSV header: %v", err)
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

	statsStream, err := cli.ContainerStats(context.Background(), containerID, true)
	if err != nil {
		return fmt.Errorf("failed to get container stats: %v", err)
	}
	defer statsStream.Body.Close()

	logCtx, logCancel := context.WithCancel(context.Background())
	defer logCancel()
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		decoder := json.NewDecoder(statsStream.Body)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		if showStats {
			updateKeyHint()
			if showTPS {
				ui.Render(lcCPU, lcMem, lcDisk, lcNet, lcTPS, keyHint)
			} else {
				ui.Render(lcCPU, lcMem, lcDisk, lcNet, keyHint)
			}
		}

		// TPS regex (example: "TPS: 123.45")
		tpsRegex := regexp.MustCompile(`TPS: (\d+\.\d+|\d+)`)

		for {
			select {
			case <-ctx.Done():
				return
			case <-sigChan:
				return
			case <-ticker.C:
				var stat types.StatsJSON
				if err := decoder.Decode(&stat); err != nil {
					log.Printf("Failed to decode stats: %v", err)
					continue
				}

				cpuDelta := float64(stat.CPUStats.CPUUsage.TotalUsage - stat.PreCPUStats.CPUUsage.TotalUsage)
				systemDelta := float64(stat.CPUStats.SystemUsage - stat.PreCPUStats.SystemUsage)
				cpuCount := float64(stat.CPUStats.OnlineCPUs)
				var cpuUsage float64
				if systemDelta > 0 && cpuDelta > 0 {
					cpuUsage = (cpuDelta / systemDelta) * cpuCount * 100.0
				}

				memoryUsage := float64(stat.MemoryStats.Usage) / 1048576.0

				var diskRead, diskWrite float64
				for _, blk := range stat.BlkioStats.IoServiceBytesRecursive {
					switch blk.Op {
					case "Read":
						diskRead += float64(blk.Value) / 1048576.0
					case "Write":
						diskWrite += float64(blk.Value) / 1048576.0
					}
				}

				var netRx, netTx float64
				for _, net := range stat.Networks {
					netRx += float64(net.RxBytes) / 1048576.0
					netTx += float64(net.TxBytes) / 1048576.0
				}

				// Extract TPS from logs (default to 0 if not found)
				var tps float64
				for i := len(logs) - 1; i >= 0 && i > len(logs)-10; i-- { // Check last 10 lines
					if match := tpsRegex.FindStringSubmatch(logs[i]); match != nil {
						tps, _ = strconv.ParseFloat(match[1], 64)
						break
					}
				}

				// Record stats to CSV
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				var csvRow []string
				if showTPS {
					csvRow = []string{
						timestamp,
						fmt.Sprintf("%.2f", cpuUsage),
						fmt.Sprintf("%.2f", memoryUsage),
						fmt.Sprintf("%.2f", diskRead),
						fmt.Sprintf("%.2f", diskWrite),
						fmt.Sprintf("%.2f", netRx),
						fmt.Sprintf("%.2f", netTx),
						fmt.Sprintf("%.2f", tps),
					}
				} else {
					csvRow = []string{
						timestamp,
						fmt.Sprintf("%.2f", cpuUsage),
						fmt.Sprintf("%.2f", memoryUsage),
						fmt.Sprintf("%.2f", diskRead),
						fmt.Sprintf("%.2f", diskWrite),
						fmt.Sprintf("%.2f", netRx),
						fmt.Sprintf("%.2f", netTx),
					}
				}
				err = csvWriter.Write(csvRow)
				if err != nil {
					log.Printf("Failed to write to CSV: %v", err)
				}
				csvWriter.Flush()

				cpuData = append(cpuData[1:], cpuUsage)
				memData = append(memData[1:], memoryUsage)
				diskReadData = append(diskReadData[1:], diskRead)
				diskWriteData = append(diskWriteData[1:], diskWrite)
				netRxData = append(netRxData[1:], netRx)
				netTxData = append(netTxData[1:], netTx)
				if showTPS {
					tpsData = append(tpsData[1:], tps)
				}

				elapsed := int(time.Since(startTime).Seconds()) / 10 * 10
				for i := 0; i < 30; i++ {
					timeAgo := elapsed - (29-i)*10
					if timeAgo < 0 {
						timeAgo = 0
					}
					xLabels[i] = fmt.Sprintf("%d ago", timeAgo)
				}
				lcCPU.DataLabels = xLabels
				lcMem.DataLabels = xLabels
				lcDisk.DataLabels = xLabels
				lcNet.DataLabels = xLabels
				if showTPS {
					lcTPS.DataLabels = xLabels
				}

				lcCPU.Data[0] = cpuData
				lcMem.Data[0] = memData
				lcDisk.Data[0] = diskReadData
				lcDisk.Data[1] = diskWriteData
				lcNet.Data[0] = netRxData
				lcNet.Data[1] = netTxData
				if showTPS {
					lcTPS.Data[0] = tpsData
				}

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

	go func() {
		scanner := bufio.NewScanner(logReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
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
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Scanner error: %v", err)
		}
	}()

	uiEvents := ui.PollEvents()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
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
		case <-sigChan:
			return nil
		}
	}
}
