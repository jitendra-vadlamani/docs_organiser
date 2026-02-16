package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"docs_organiser/internal/ai"
	"docs_organiser/internal/pipeline"
)

func main() {
	// Parse CLI flags
	sourceDir := flag.String("src", "", "Source directory to scan for files")
	destDir := flag.String("dst", "", "Destination directory to move files into")
	apiURL := flag.String("api", "http://localhost:8080/v1", "URL of the MLX server")
	modelName := flag.String("model", "mlx-community/Llama-3.2-1B-Instruct-4bit", "Model name to use in API requests")
	workers := flag.Int("workers", 5, "Number of concurrent workers")
	flag.Parse()

	// Validate flags
	if *sourceDir == "" || *destDir == "" {
		fmt.Println("Usage: docs_organiser -src <source_dir> -dst <dest_dir> [-api <url>] [-model <name>] [-workers <n>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("=== MLX File Mover (Production Ready) ===")
	fmt.Printf("Source:      %s\n", *sourceDir)
	fmt.Printf("Destination: %s\n", *destDir)
	fmt.Printf("API URL:     %s\n", *apiURL)
	fmt.Printf("Model:       %s\n", *modelName)
	fmt.Printf("Workers:     %d\n", *workers)
	fmt.Println("-----------------------------------------")

	// Initialize AI Engine
	fmt.Println("[*] Initializing AI Engine...")
	aiEngine, err := ai.NewMLXEngine(*apiURL, *modelName)
	if err != nil {
		log.Fatalf("Failed to initialize AI engine: %v", err)
	}
	fmt.Println("[+] AI Engine initialized.")

	// Initialize and Run Pipeline
	p := pipeline.NewPipeline(*sourceDir, *destDir, aiEngine, *workers)

	fmt.Println("[*] Starting processing pipeline...")
	startTime := time.Now()

	// Run pipeline with context
	if err := p.Run(ctx); err != nil {
		if err == context.Canceled {
			fmt.Println("\n[!] Pipeline stopped by user (Ctrl+C).")
		} else {
			fmt.Printf("\n[!] Pipeline finished with error: %v\n", err)
		}
	}

	duration := time.Since(startTime)
	fmt.Println("-----------------------------------------")
	fmt.Print(p.GetSummary())
	fmt.Printf("[+] Processing complete in %v\n", duration)
}
