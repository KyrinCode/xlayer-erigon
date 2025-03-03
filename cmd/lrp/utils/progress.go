package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

// CopyProgress stores the copy progress
type CopyProgress struct {
	Title       string
	TotalBytes  int64
	CopiedBytes int64
	Mu          sync.Mutex
}

func (p *CopyProgress) Progress(srcPath, dstPath string) error {
	// Validate source path
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("source path %s does not exist", srcPath)
	}

	// Adjust dstPath to include the source folder name
	dstPath = filepath.Join(dstPath, filepath.Base(srcPath))
	if fileInfo, _ := os.Stat(dstPath); fileInfo != nil {
		fmt.Println("The source data is already copied done")
		return nil
	}

	// Calculate total size
	totalBytes, err := getDirSize(srcPath)
	if err != nil {
		fmt.Println("Failed to calculate folder size:", err)
		return err
	}

	// Create progress object
	p.TotalBytes = totalBytes

	// Progress and done channels
	progressChan := make(chan int64)
	doneChan := make(chan struct{})

	// Start copy Goroutine
	go copyDir(srcPath, dstPath, progressChan, doneChan)

	if err := ui.Init(); err != nil {
		fmt.Println("Failed to initialize termui", err)
		return err
	}
	defer ui.Close()

	// Create progress bar
	gauge := widgets.NewGauge()
	gauge.Title = p.Title
	gauge.Percent = 0
	gauge.BarColor = ui.ColorGreen
	gauge.SetRect(0, 0, 70, 5)

	// Render initial UI
	ui.Render(gauge)

	// Update UI
	go func() {
		for bytes := range progressChan {
			p.Mu.Lock()
			p.CopiedBytes += bytes
			percent := int(float64(p.CopiedBytes) / float64(p.TotalBytes) * 100)
			p.Mu.Unlock()

			gauge.Percent = percent
			ui.Render(gauge)
		}
	}()

	// Event loop for TermUI
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			if e.ID == "q" || e.ID == "<C-c>" {
				return fmt.Errorf("copy has been stopped")
			}
		case <-doneChan:
			fmt.Println("Copy completed")
			return nil
		}
	}
}

// getDirSize calculates the total size of the folder
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// copyDir copies the folder and updates the progress
func copyDir(src, dst string, progressChan chan<- int64, doneChan chan<- struct{}) {
	defer close(doneChan)
	defer close(progressChan)

	err := filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, srcPath)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		return copyFile(srcPath, dstPath, progressChan)
	})
	if err != nil {
		fmt.Println("Failed to copy folder:", err)
	}
}

// copyFile copies a single file and updates the progress
func copyFile(src, dst string, progressChan chan<- int64) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy in chunks and update progress
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := dstFile.Write(buf[:n]); err != nil {
			return err
		}

		// Update progress
		progressChan <- int64(n)
	}

	return nil
}
