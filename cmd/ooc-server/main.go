package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/exec-server/internal/auditor"
	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/handlers"
	"github.com/user/exec-server/internal/task"
	"github.com/user/exec-server/internal/whitelist"
	"github.com/user/exec-server/pkg/config"
)

// 版本信息，在构建时通过 -ldflags 注入
var (
	Version   = "dev"
	BuildTime = ""
)

func init() {
	// 如果 BuildTime 未设置，使用当前时间
	if BuildTime == "" {
		BuildTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
}

var (
	configPath = flag.String("config", "", "Path to config file")
	initMode   = flag.Bool("init", false, "Initialize config file")
	version    = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("ooc-server version %s\n", Version)
		fmt.Printf("Build time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Expand config path
	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
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
	defer func() {
		if err := aud.Close(); err != nil {
			log.Printf("Failed to close auditor: %v\n", err)
		}
	}()

	exec := executor.NewExecutor(cfg.Server.TimeoutSeconds, cfg.Server.MaxOutputMB)

	// Initialize task manager
	taskStore := task.NewMemoryStore()
	taskManager := task.NewTaskManager(
		taskStore,
		exec,
		whitelistChecker,
		aud,
		time.Duration(cfg.Server.TaskTTLHours)*time.Hour,
	)
	defer taskManager.Close()

	// Start task cleanup loop
	taskManager.StartCleanupLoop(time.Hour)

	// Setup handlers
	execHandler := handlers.NewExecHandler(
		exec,
		whitelistChecker,
		aud,
		cfg.Server.APIToken,
		cfg.Server.MaxConcurrent,
	)

	whitelistInfoHandler := handlers.NewWhitelistInfoHandler(
		whitelistChecker,
		cfg.Server.APIToken,
	)

	taskHandler := handlers.NewTaskHandler(taskManager)

	mux := http.NewServeMux()
	mux.Handle("/ooc-exec", execHandler)
	mux.Handle("/whitelist-info", whitelistInfoHandler)
	mux.HandleFunc("/health", handlers.HealthHandler)
	mux.HandleFunc("/task", taskHandler.SubmitTask)
	mux.HandleFunc("/task/", taskHandler.GetTaskStatus)

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
		if err := server.Close(); err != nil {
			log.Printf("Server shutdown error: %v\n", err)
		}
	}()

	// Start server
	log.Printf("Server starting on %s", cfg.Server.Listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
