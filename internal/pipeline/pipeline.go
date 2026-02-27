package pipeline

import (
	"context"
	"docs_organiser/internal/ai"
	"docs_organiser/internal/extractor"
	"docs_organiser/internal/fileops"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Pipeline struct {
	SourceDir    string
	DestDir      string
	AI           *ai.MLXEngine
	Workers      int
	ExtractLimit int

	// Progress counters
	TotalFiles     int32
	ProcessedFiles int32
	FailedFiles    int32
}

type FileJob struct {
	Path string
}

func NewPipeline(src, dst string, aiEngine *ai.MLXEngine, workers, extractLimit int) *Pipeline {
	if workers <= 0 {
		workers = 5
	}
	if extractLimit <= 0 {
		extractLimit = 100000
	}
	return &Pipeline{
		SourceDir:    src,
		DestDir:      dst,
		AI:           aiEngine,
		Workers:      workers,
		ExtractLimit: extractLimit,
	}
}

func (p *Pipeline) Run(ctx context.Context) error {
	// Step 0: Dynamic Category Discovery
	discoveredCategories, err := p.discoverCategories()
	if err != nil {
		log.Printf("[!] Warning: Category discovery failed: %v. Using defaults.", err)
	} else if len(discoveredCategories) > 0 {
		log.Printf("[*] Discovered %d categories in %s", len(discoveredCategories), p.DestDir)
		p.AI.SetCategories(discoveredCategories)
	}

	jobs := make(chan FileJob, p.Workers*2)
	var wg sync.WaitGroup

	// Step 1: Start workers
	for i := 0; i < p.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[!] Worker panicked: %v", r)
					atomic.AddInt32(&p.FailedFiles, 1)
					p.updateProgressDisplay()
				}
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}

					// Log the file being processed to identify "killer files"
					log.Printf("[*] Processing: %s", filepath.Base(job.Path))

					// Use a per-file timeout to prevent hanging workers
					fileCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
					p.processFile(fileCtx, job.Path)
					cancel()

					p.updateProgressDisplay()

					// Periodically suggest memory release to the OS
					if atomic.LoadInt32(&p.ProcessedFiles)%10 == 0 {
						debug.FreeOSMemory()
					}
				}
			}
		}()
	}

	// Step 2: Scan and feed jobs in a stream
	fmt.Println("[*] Scanning source directory...")
	err = filepath.Walk(p.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".pdf" || ext == ".txt" || ext == ".md" {
				atomic.AddInt32(&p.TotalFiles, 1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case jobs <- FileJob{Path: path}:
				}
			}
		}
		return nil
	})

	// Close jobs channel after scanning is done
	close(jobs)

	// Step 3: Wait for workers to finish
	wg.Wait()
	fmt.Println() // New line after final progress

	if err != nil && err != context.Canceled {
		return err
	}
	return ctx.Err()
}

func (p *Pipeline) processFile(ctx context.Context, path string) {
	text, err := extractor.ExtractText(path, p.ExtractLimit)
	if err != nil {
		log.Printf("[!] Failed to extract text from %s: %v", filepath.Base(path), err)
		atomic.AddInt32(&p.FailedFiles, 1)
		return
	}

	if ctx.Err() != nil {
		return
	}

	result, err := p.AI.Categorize(ctx, text)

	targetFolder := "Misc"
	targetName := ai.SanitizeFilename(filepath.Base(path))

	if err == nil {
		targetFolder = result.Category
		targetName = result.Title + filepath.Ext(path)
	}

	finalDestDir := filepath.Join(p.DestDir, targetFolder)

	if err := fileops.MoveFile(path, finalDestDir, targetName); err != nil {
		log.Printf("[!] Failed to move %s to %s/%s: %v", filepath.Base(path), targetFolder, targetName, err)
		atomic.AddInt32(&p.FailedFiles, 1)
	} else {
		atomic.AddInt32(&p.ProcessedFiles, 1)
	}
}

func (p *Pipeline) discoverCategories() ([]string, error) {
	var categories []string
	maxDepth := 3

	err := filepath.WalkDir(p.DestDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		// Skip the root DestDir itself
		if path == p.DestDir {
			return nil
		}

		// Calculate depth relative to DestDir
		rel, err := filepath.Rel(p.DestDir, path)
		if err != nil {
			return nil
		}

		segments := strings.Split(rel, string(os.PathSeparator))
		if len(segments) > maxDepth {
			return filepath.SkipDir
		}

		// Skip hidden directories
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		// Use forward slash uniformly for AI consistency
		categories = append(categories, filepath.ToSlash(rel))
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Always include "Misc" if not already there
	hasMisc := false
	for _, c := range categories {
		if c == "Misc" {
			hasMisc = true
			break
		}
	}
	if !hasMisc {
		categories = append(categories, "Misc")
	}

	return categories, nil
}

func (p *Pipeline) updateProgressDisplay() {
	processed := atomic.LoadInt32(&p.ProcessedFiles)
	failed := atomic.LoadInt32(&p.FailedFiles)
	total := p.TotalFiles
	completed := processed + failed

	percentage := float64(completed) / float64(total) * 100
	// Using \r to refresh the same line for a clean terminal experience
	fmt.Printf("\r[Progress] %d/%d files (%.1f%%) | Success: %d | Failed: %d   ",
		completed, total, percentage, processed, failed)
}

func (p *Pipeline) GetSummary() string {
	return fmt.Sprintf("\nSummary:\n- Total Files:     %d\n- Successfully Moved: %d\n- Failed/Skipped:     %d\n",
		p.TotalFiles, atomic.LoadInt32(&p.ProcessedFiles), atomic.LoadInt32(&p.FailedFiles))
}
