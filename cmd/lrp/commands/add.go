package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/ledgerwatch/erigon/cmd/lrp/utils"
	"github.com/spf13/cobra"
)

type BatchRangeOption struct {
	BatchFrom      int       `json:"batch_from"`
	BatchTo        int       `json:"batch_to"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	LastSelectedAt time.Time `json:"last_selected_at,omitempty"`
}

var AddCmd = &cobra.Command{
	Use:   "add",
	Short: "Interactively add a new batch range option",
	Long:  `Prompts the user to input batchFrom, batchTo, and description, then saves the record to a file.`,
	Run: func(cmd *cobra.Command, args []string) {
		scanner := bufio.NewScanner(os.Stdin)

		// Prompt for batchFrom
		fmt.Print("Enter batchFrom: ")
		scanner.Scan()
		batchFromStr := scanner.Text()
		batchFrom, err := strconv.Atoi(batchFromStr)
		if err != nil {
			log.Fatalf("Invalid batchFrom: %v", err)
		}

		// Prompt for batchTo
		fmt.Print("Enter batchTo: ")
		scanner.Scan()
		batchToStr := scanner.Text()
		batchTo, err := strconv.Atoi(batchToStr)
		if err != nil {
			log.Fatalf("Invalid batchTo: %v", err)
		}

		// Prompt for description
		fmt.Print("Enter description: ")
		scanner.Scan()
		description := scanner.Text()

		// Create new record
		option := BatchRangeOption{
			BatchFrom:      batchFrom,
			BatchTo:        batchTo,
			Description:    description,
			CreatedAt:      time.Now(),
			LastSelectedAt: time.Time{}, // Zero value until selected
		}

		// Save to file
		err = saveBatchRangeOption(filepath.Join(path, utils.OPTIONS_JSON), option)
		if err != nil {
			log.Fatalf("Failed to save batch range option: %v", err)
		}
		fmt.Println("Batch range option added successfully!")
	},
}

// saveBatchRangeOption appends a new option to the JSON file
func saveBatchRangeOption(filePath string, option BatchRangeOption) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	var options []BatchRangeOption
	data, err := ioutil.ReadFile(filePath)
	if err == nil {
		if err := json.Unmarshal(data, &options); err != nil {
			return fmt.Errorf("failed to unmarshal options: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read file: %v", err)
	}

	options = append(options, option)

	// Save back to file
	data, err = json.MarshalIndent(options, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal options: %v", err)
	}
	return ioutil.WriteFile(filePath, data, 0644)
}

// loadTestOptions loads all options from the JSON file
func loadTestOptions(filePath string) ([]BatchRangeOption, error) {
	var options []BatchRangeOption
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return options, nil
		}
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	if err := json.Unmarshal(data, &options); err != nil {
		return nil, fmt.Errorf("failed to unmarshal options: %v", err)
	}
	return options, nil
}

// selectBatchRange displays options and returns the selected one
func selectBatchRange(path string) (*BatchRangeOption, error) {
	// Initialize termui
	if err := ui.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	filePath := filepath.Join(path, utils.OPTIONS_JSON)
	options, err := loadTestOptions(filePath)
	if err != nil {
		return nil, err
	}

	// If no options, return nil without showing UI
	if len(options) == 0 {
		log.Println("No batch range options available")
		return nil, nil
	}

	// Create list widget
	list := widgets.NewList()
	list.Title = "Batch Range Options (Select with Enter, Quit with q/Ctrl+C)"
	list.TextStyle = ui.NewStyle(ui.ColorWhite)
	list.SelectedRowStyle = ui.NewStyle(ui.ColorBlack, ui.ColorYellow)
	list.BorderStyle = ui.NewStyle(ui.ColorCyan)

	// Sort options by LastSelectedAt (descending), falling back to CreatedAt if never selected
	sort.Slice(options, func(i, j int) bool {
		timeI := options[i].LastSelectedAt
		if timeI.IsZero() {
			timeI = options[i].CreatedAt
		}
		timeJ := options[j].LastSelectedAt
		if timeJ.IsZero() {
			timeJ = options[j].CreatedAt
		}
		return timeI.After(timeJ)
	})

	// Populate list items
	list.Rows = make([]string, len(options))
	for i, r := range options {
		lastSelected := "Never"
		if !r.LastSelectedAt.IsZero() {
			lastSelected = r.LastSelectedAt.Format("2006-01-02 15:04:05")
		}
		list.Rows[i] = fmt.Sprintf("Batch %d-%d | %s | Created: %s | Last Selected: %s",
			r.BatchFrom, r.BatchTo, r.Description, r.CreatedAt.Format("2006-01-02 15:04:05"), lastSelected)
	}

	// Create instruction paragraph
	instructions := widgets.NewParagraph()
	instructions.Text = "Use ↑/↓ to navigate, Enter to select, q/Ctrl+C to quit"
	instructions.TextStyle = ui.NewStyle(ui.ColorCyan)
	instructions.Border = false

	// Update layout based on terminal size
	updateLayout := func() {
		termWidth, termHeight := ui.TerminalDimensions()
		listHeight := termHeight - 3
		if listHeight < 5 {
			listHeight = 5
		}
		list.SetRect(0, 0, termWidth, listHeight)
		instructions.SetRect(0, listHeight, termWidth, listHeight+3)
		ui.Render(list, instructions)
	}

	// Initial layout
	updateLayout()

	// Event loop
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil, nil // Quit without selecting
			case "<Up>":
				list.ScrollUp()
				ui.Render(list, instructions)
			case "<Down>":
				list.ScrollDown()
				ui.Render(list, instructions)
			case "<Enter>":
				if len(options) > 0 && list.SelectedRow >= 0 && list.SelectedRow < len(options) {
					// Update LastSelectedAt for the selected option
					options[list.SelectedRow].LastSelectedAt = time.Now()
					selected := options[list.SelectedRow]

					// Save updated options
					data, err := json.MarshalIndent(options, "", "  ")
					if err != nil {
						log.Printf("Failed to marshal records: %v", err)
					} else if err = ioutil.WriteFile(filePath, data, 0644); err != nil {
						log.Printf("Failed to save updated options: %v", err)
					}

					// Return the selected option and exit
					return &selected, nil
				}
			case "<Resize>":
				ui.Clear()
				updateLayout()
			}
		}
	}
}
