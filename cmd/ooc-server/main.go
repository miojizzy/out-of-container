package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/exec-server/internal/auditor"
	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/handlers"
	"github.com/user/exec-server/internal/whitelist"
	"github.com/user/exec-server/pkg/config"
)

var (
	configPath = flag.String("config", "", "Path to config file")
	initMode   = flag.Bool("init", false, "Initialize config file")
)

func main() {
	flag.Parse()

	// Expand config path
	if *configPath == "" {
		home, _ := os.UserHomeDir()
		*configPath = home + "/.config/ooc-server/config.yaml"
	}

	loader := config.NewLoader(*configPath)

	// Handle init mode
	if *initMode {
		if err := loader.InitConfig(); err != nil {
			log.Fatalf("Failed to initialize config: %v", err)
		}
		return
	}

	// Load config
	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize components
	whitelistChecker, err := whitelist.NewChecker(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize whitelist checker: %v", err)
	}

	aud := auditor.NewAuditor(
		cfg.Audit.LogFile,
		cfg.Audit.RotationMaxMB,
		cfg.Audit.RotationCount,
	)
	defer aud.Close()

	exec := executor.NewExecutor(cfg.Server.TimeoutSeconds, cfg.Server.MaxOutputMB)

	// Setup handlers
	execHandler := handlers.NewExecHandler(
		exec,
		whitelistChecker,
		aud,
		cfg.Server.ApiToken,
		cfg.Server.MaxConcurrent,
	)

	mux := http.NewServeMux()
	mux.Handle("/ooc-exec", execHandler)
	mux.HandleFunc("/health", handlers.HealthHandler)

	// Setup graceful shutdown
	server := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: mux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		server.Close()
	}()

	// Start server
	log.Printf("Server starting on %s", cfg.Server.Listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
