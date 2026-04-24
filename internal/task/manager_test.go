package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/whitelist"
)

// setupTestWhitelist 创建测试用白检查器的辅助函数
func setupTestWhitelist(allowedCmds []string, allowedPaths []string) (*whitelist.Checker, string, error) {
	// 创建临时配置文件
	tmpDir, err := os.MkdirTemp("", "task-manager-test")
	if err != nil {
		return nil, "", err
	}

	// 创建临时白名单目录
	pathsDir := filepath.Join(tmpDir, "paths")
	if err := os.MkdirAll(pathsDir, 0755); err != nil {
		return nil, "", err
	}

	// 写入配置文件
	configPath := filepath.Join(tmpDir, "config.yaml")
	configData := []byte(`
server:
  listen: "0.0.0.0:8080"
  timeout_seconds: 30
  max_output_mb: 10
  max_concurrent: 5
  api_token: "test-token-123456789012345678901234"
  task_ttl_hours: 24
whitelist:
  literal_commands:`)
	for _, cmd := range allowedCmds {
		configData = append(configData, []byte("\n    - \""+cmd+"\"")...)
	}
	configData = append(configData, []byte("\n  allowed_paths:")...)
	for _, path := range allowedPaths {
		configData = append(configData, []byte("\n    - \""+path+"\"")...)
	}
	configData = append(configData, []byte(`
audit:
  enabled: false
`)...)

	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, "", err
	}

	// 创建白检查器
	checker, err := whitelist.NewChecker(configPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, "", err
	}

	return checker, tmpDir, nil
}

// cleanupTestWhitelist 清理测试资源
func cleanupTestWhitelist(tmpDir string) {
	_ = os.RemoveAll(tmpDir)
}

func TestManager_SubmitTask(t *testing.T) {
	store := NewMemoryStore()
	exec := executor.NewExecutor(30, 10)

	// 创建白检查器
	whitelistChecker, tmpDir, err := setupTestWhitelist([]string{"echo", "ls"}, []string{"/tmp", "/home"})
	if err != nil {
		t.Fatalf("Failed to setup whitelist: %v", err)
	}
	defer cleanupTestWhitelist(tmpDir)

	tm := NewManager(store, exec, whitelistChecker, nil, 24*time.Hour)

	cmd := &models.Command{
		Command: "echo",
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}

	// 测试成功提交
	submittedTask, err := tm.SubmitTask(cmd, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if submittedTask == nil {
		t.Fatal("Expected task to be returned")
	}

	// 从存储中获取任务检查状态（避免数据竞争）
	task, err := store.Get(submittedTask.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task.Status != TaskStatusPending {
		t.Errorf("Expected status 'pending', got %s", task.Status)
	}

	// 验证任务已保存
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if loaded.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, loaded.ID)
	}

	// 等待任务执行完成
	time.Sleep(200 * time.Millisecond)

	// 检查任务状态
	loaded, err = store.Get(task.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded.Status != TaskStatusCompleted {
		t.Errorf("Expected status 'completed', got %s", loaded.Status)
	}

	if loaded.Result == nil {
		t.Error("Expected result to be set")
	}
}

func TestManager_SubmitTask_WhitelistDenied(t *testing.T) {
	store := NewMemoryStore()
	exec := executor.NewExecutor(30, 10)

	// 创建一个严格的白名单配置
	whitelistChecker, tmpDir, err := setupTestWhitelist([]string{"ls"}, []string{"/tmp"})
	if err != nil {
		t.Fatalf("Failed to setup whitelist: %v", err)
	}
	defer cleanupTestWhitelist(tmpDir)

	tm := NewManager(store, exec, whitelistChecker, nil, 24*time.Hour)

	cmd := &models.Command{
		Command: "echo", // 不在白名单中
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}

	task, err := tm.SubmitTask(cmd, "")
	if task != nil {
		t.Error("Expected nil task for denied command")
	}

	if err == nil {
		t.Fatal("Expected error for denied command")
	}
}

func TestManager_GetStatus(t *testing.T) {
	store := NewMemoryStore()
	exec := executor.NewExecutor(30, 10)

	whitelistChecker, tmpDir, err := setupTestWhitelist([]string{"echo"}, []string{"/tmp"})
	if err != nil {
		t.Fatalf("Failed to setup whitelist: %v", err)
	}
	defer cleanupTestWhitelist(tmpDir)

	tm := NewManager(store, exec, whitelistChecker, nil, 24*time.Hour)

	cmd := &models.Command{
		Command: "echo",
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}

	// 提交任务
	task, err := tm.SubmitTask(cmd, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 获取任务状态
	loaded, err := tm.GetStatus(task.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if loaded.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, loaded.ID)
	}
}

func TestManager_Close(t *testing.T) {
	store := NewMemoryStore()
	exec := executor.NewExecutor(30, 10)

	whitelistChecker, tmpDir, err := setupTestWhitelist([]string{"sleep"}, []string{"/tmp"})
	if err != nil {
		t.Fatalf("Failed to setup whitelist: %v", err)
	}
	defer cleanupTestWhitelist(tmpDir)

	tm := NewManager(store, exec, whitelistChecker, nil, 24*time.Hour)

	cmd := &models.Command{
		Command: "sleep",
		Args:    []string{"1"},
		Cwd:     "/tmp",
	}

	// 提交任务
	_, err = tm.SubmitTask(cmd, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 关闭管理器
	tm.Close()

	// 验证所有 goroutine 已完成
	// 这个测试只是验证 Close 不会 panic
}

func TestManager_StartCleanupLoop(t *testing.T) {
	store := NewMemoryStore()
	exec := executor.NewExecutor(30, 10)

	whitelistChecker, tmpDir, err := setupTestWhitelist([]string{"echo"}, []string{"/tmp"})
	if err != nil {
		t.Fatalf("Failed to setup whitelist: %v", err)
	}
	defer cleanupTestWhitelist(tmpDir)

	// 设置 TTL 为 2 秒，清理间隔为 500 毫秒
	tm := NewManager(store, exec, whitelistChecker, nil, 2*time.Second)

	// 直接在存储中创建任务，不通过 Manager 执行
	cmd := &models.Command{
		Command: "echo",
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}
	task := NewTask(cmd, "")
	task.Status = TaskStatusPending

	if err := store.Save(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// 启动清理循环（每 500 毫秒清理一次）
	tm.StartCleanupLoop(500 * time.Millisecond)

	// 等待 1 秒（任务还在 TTL 内，不应被清理）
	time.Sleep(1 * time.Second)

	// 检查任务是否还存在（应该还在）
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if loaded == nil {
		t.Error("Expected task to still exist within TTL")
	}

	// 更新任务为已完成
	if err := store.Update(task.ID, TaskStatusCompleted, &models.Result{
		ExitCode:   0,
		Stdout:     "success\n",
		Stderr:     "",
		DurationMs: 100,
		Truncated:  false,
	}, nil); err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}

	// 等待 3 秒（从完成时间算起超过 2 秒 TTL）
	// 这确保了从 CompletedAt 开始的时间已经超过了 TTL
	time.Sleep(3 * time.Second)

	// 检查任务是否被清理
	loaded, err = store.Get(task.ID)
	if err == nil && loaded != nil {
		t.Error("Expected task to be cleaned up after TTL")
	}
}
