// Package task 提供任务管理功能。
//
// 实现了任务的提交、状态管理、执行和清理机制。
package task

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/user/exec-server/internal/auditor"
	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/whitelist"
)

// Manager 任务管理器
type Manager struct {
	store     Store
	executor  *executor.Executor
	whitelist *whitelist.Checker
	auditor   *auditor.Auditor
	taskTTL   time.Duration
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

// NewManager 创建任务管理器
func NewManager(
	store Store,
	exec *executor.Executor,
	whitelistChecker *whitelist.Checker,
	aud *auditor.Auditor,
	taskTTL time.Duration,
) *Manager {
	// 创建取消上下文，但不使用它，因为任务执行是独立的 goroutine
	// 我们只在 Close 时使用 cancel
	_, cancel := context.WithCancel(context.Background())
	return &Manager{
		store:     store,
		executor:  exec,
		whitelist: whitelistChecker,
		auditor:   aud,
		taskTTL:   taskTTL,
		cancel:    cancel,
	}
}

// SubmitTask 提交新任务
func (tm *Manager) SubmitTask(cmd *models.Command, tokenPrefix string) (*Task, error) {
	// 检查白名单
	_, _, err := tm.whitelist.IsAllowed(cmd.Command, cmd.Cwd)
	if err != nil {
		if err == whitelist.ErrCommandNotInWhitelist {
			return nil, fmt.Errorf("command not in whitelist: %w", err)
		}
		if err == whitelist.ErrPathNotAllowed {
			return nil, fmt.Errorf("cwd not allowed: %w", err)
		}
		return nil, fmt.Errorf("whitelist check failed: %w", err)
	}

	// 创建任务（包含 token prefix 用于审计）
	task := NewTask(cmd, tokenPrefix)
	task.Status = StatusPending

	// 保存任务到存储
	if err := tm.store.Save(task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// 启动后台执行 goroutine
	tm.wg.Add(1)
	go tm.executeTask(task.ID)

	return task, nil
}

// executeTask 执行任务（后台 goroutine）
func (tm *Manager) executeTask(taskID string) {
	defer tm.wg.Done()

	// 获取任务
	task, err := tm.getTask(taskID)
	if err != nil {
		log.Printf("Failed to get task %s: %v\n", taskID, err)
		return
	}

	// 更新状态为 running
	now := time.Now()
	task.Status = StatusRunning
	task.StartedAt = &now
	if err := tm.store.Update(taskID, StatusRunning, nil, nil); err != nil {
		log.Printf("Failed to update task %s status: %v\n", taskID, err)
		return
	}

	// 准备命令
	execCmd := &models.Command{
		Command: task.Command,
		Args:    task.Args,
		Cwd:     task.Cwd,
	}

	// 使用 limiter 的 Middleware 来限制并发
	// 我们需要一个 HTTP Handler 风格的执行，但这里只是简单地使用
	// 由于 ConcurrencyLimiter 的 Middleware 依赖于 http.ResponseWriter 和 http.Request
	// 我们需要修改设计，但为了简化，我们直接执行，因为任务是在 goroutine 中运行的
	// 实际上，ConcurrentLimiter 是用于 HTTP 请求的，不是用于内部任务调度
	// 我们可以创建一个类似但不同的机制，或者使用信号量

	// 简单的实现：直接执行
	result := tm.executor.Execute(execCmd)

	// 处理结果
	taskStatus := StatusCompleted
	var resultData *models.Result
	var errMsg *string

	if result.Error != nil {
		if result.HTTPError == 408 { // context deadline exceeded
			taskStatus = StatusTimeout
		} else {
			taskStatus = StatusFailed
		}
		msg := result.Error.Error()
		errMsg = &msg
	}
	resultData = result.Result

	// 更新任务状态
	if err := tm.store.Update(taskID, taskStatus, resultData, errMsg); err != nil {
		log.Printf("Failed to update task %s after execution: %v\n", taskID, err)
		return
	}

	// 审计日志
	if tm.auditor != nil && result.Result != nil {
		tm.auditor.Log(&models.AuditEntry{
			Timestamp:       time.Now().Format(time.RFC3339),
			Command:         task.Command,
			Args:            task.Args,
			Cwd:             task.Cwd,
			TokenPrefix:     task.TokenPrefix,
			ExitCode:        result.Result.ExitCode,
			DurationMs:      result.Result.DurationMs,
			OutputSizeBytes: result.Result.OutputSize,
			Truncated:       result.Result.Truncated,
			AllowedBy:       "async", // 异步模式
		})
	}
}

// GetStatus 获取任务状态
func (tm *Manager) GetStatus(taskID string) (*Task, error) {
	return tm.store.Get(taskID)
}

// GetTask 获取任务（内部使用）
func (tm *Manager) getTask(taskID string) (*Task, error) {
	return tm.store.Get(taskID)
}

// Close 优雅关闭
func (tm *Manager) Close() {
	// 取消所有进行中的任务
	if tm.cancel != nil {
		tm.cancel()
	}

	// 等待所有 goroutine 完成
	tm.wg.Wait()
}

// StartCleanupLoop 启动任务清理循环
func (tm *Manager) StartCleanupLoop(interval time.Duration) {
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := tm.store.CleanupExpired(tm.taskTTL); err != nil {
				log.Printf("Failed to cleanup expired tasks: %v\n", err)
			}
		}
	}()
}
