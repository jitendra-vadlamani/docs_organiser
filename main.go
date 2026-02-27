package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"docs_organiser/internal/ai"
	"docs_organiser/internal/config"
	"docs_organiser/internal/pipeline"
)

func main() {
	// Load configuration using Viper
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate required fields
	if cfg.SourceDir == "" || cfg.DestDir == "" {
		fmt.Println("Usage: docs_organiser -src <source_dir> -dst <dest_dir> [-api <url>] [-model <name>] [-ctx <n>] [-workers <n>] [-limit <n>]")
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("=== MLX File Mover (Production Ready) ===")
	fmt.Printf("Source:      %s\n", cfg.SourceDir)
	fmt.Printf("Destination: %s\n", cfg.DestDir)
	fmt.Printf("API URL:     %s\n", cfg.APIURL)
	fmt.Printf("Model:       %s\n", cfg.ModelName)
	fmt.Printf("Context:     %d tokens\n", cfg.ContextWindow)
	fmt.Printf("Limit:       %d characters\n", cfg.ExtractLimit)
	fmt.Printf("Workers:     %d\n", cfg.Workers)
	fmt.Println("-----------------------------------------")

	// Initialize AI Engine
	fmt.Println("[*] Initializing AI Engine...")
	aiEngine, err := ai.NewMLXEngine(cfg.APIURL, cfg.ModelName, cfg.ContextWindow, cfg.Encoding)
	if err != nil {
		log.Fatalf("Failed to initialize AI engine: %v", err)
	}
	if len(cfg.Categories) > 0 {
		aiEngine.SetCategories(cfg.Categories)
		fmt.Printf("[*] Using %d manual categories from config.\n", len(cfg.Categories))
	}
	fmt.Println("[+] AI Engine initialized.")

	// Initialize and Run Pipeline
	p := pipeline.NewPipeline(cfg.SourceDir, cfg.DestDir, aiEngine, cfg.Workers, cfg.ExtractLimit)

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
