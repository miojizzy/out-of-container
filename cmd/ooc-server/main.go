// Package main 提供 exec-server HTTP 服务器。
//
// ooc-server 是容器外命令执行系统的服务器端，负责接收执行请求、
// 验证白名单、审计日志、并发控制和任务管理。
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
	os.Exit(run())
}

// run 是主逻辑入口，返回退出码。将逻辑提取到独立函数以确保 defer 正常执行。
func run() int {
	flag.Parse()

	// 显示版本信息
	if *version {
		log.Printf("ooc-server version %s", Version)
		log.Printf("Build time: %s", BuildTime)
		return 0
	}

	// Expand config path
	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get user home directory: %v\n", err)
			return 1
		}
		*configPath = home + "/.config/ooc-server/config.yaml"
	}

	loader := config.NewLoader(*configPath)

	// Handle init mode
	if *initMode {
		if err := loader.InitConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
			return 1
		}
		return 0
	}

	// Load config
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	// Initialize components
	whitelistChecker, err := whitelist.NewChecker(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize whitelist checker: %v\n", err)
		return 1
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
	taskManager := task.NewManager(
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
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		return 1
	}

	return 0
}
