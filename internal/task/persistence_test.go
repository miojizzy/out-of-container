package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/user/exec-server/internal/models"
)

func TestPersistenceManager_StartStop(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tasks.json")

	store := NewMemoryStore()
	config := PersistenceConfig{
		Enabled:       true,
		FilePath:      tmpFile,
		RestoreOnLoad: false,
		SaveInterval:  "1m",
	}

	pm := NewPersistenceManager(store, config)

	// 启动
	if err := pm.Start(); err != nil {
		t.Fatalf("Failed to start persistence manager: %v", err)
	}
	defer pm.Stop()

	time.Sleep(100 * time.Millisecond)

	// 验证正在运行
	if !pm.running {
		t.Error("Expected persistence manager to be running")
	}

	// 停止
	pm.Stop()

	if pm.running {
		t.Error("Expected persistence manager to be stopped")
	}
}

func TestPersistenceManager_SaveRestore(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tasks.json")

	store := NewMemoryStore()

	// 添加一些任务
	cmd1 := &models.Command{
		Command: "echo",
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}
	cmd2 := &models.Command{
		Command: "ls",
		Args:    []string{"-la"},
		Cwd:     "/home",
	}
	task1 := NewTask(cmd1)
	task2 := NewTask(cmd2)

	if err := store.Save(task1); err != nil {
		t.Fatalf("Failed to save task1: %v", err)
	}
	if err := store.Save(task2); err != nil {
		t.Fatalf("Failed to save task2: %v", err)
	}

	// 更新 task1 状态
	if err := store.Update(task1.ID, TaskStatusCompleted, &models.Result{
		ExitCode:   0,
		Stdout:     "hello\n",
		Stderr:     "",
		DurationMs: 100,
		Truncated:  false,
	}, nil); err != nil {
		t.Fatalf("Failed to update task1: %v", err)
	}

	// 创建持久化管理器并手动保存
	config := PersistenceConfig{
		Enabled:       true,
		FilePath:      tmpFile,
		RestoreOnLoad: false,
		SaveInterval:  "5m",
	}
	pm := NewPersistenceManager(store, config)

	// 手动保存
	if err := pm.save(); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// 验证文件存在并包含数据
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatal("Saved file does not exist")
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	var savedTasks map[string]*Task
	if err := json.Unmarshal(content, &savedTasks); err != nil {
		t.Fatalf("Failed to unmarshal saved tasks: %v", err)
	}

	if len(savedTasks) != 2 {
		t.Errorf("Expected 2 tasks in file, got %d", len(savedTasks))
	}

	// 验证 task1 数据
	if savedTask1, ok := savedTasks[task1.ID]; ok {
		if savedTask1.Status != TaskStatusCompleted {
			t.Errorf("Expected task1 status 'completed', got %s", savedTask1.Status)
		}
		if savedTask1.Result == nil {
			t.Error("Expected task1 to have result")
		} else if savedTask1.Result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", savedTask1.Result.ExitCode)
		}
	} else {
		t.Error("Task1 not found in saved data")
	}

	// 创建新的存储并恢复
	newStore := NewMemoryStore()
	pm2 := NewPersistenceManager(newStore, config)

	// 手动恢复
	if err := pm2.restore(); err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// 验证恢复的任务
	restoredTask1, err := newStore.Get(task1.ID)
	if err != nil {
		t.Fatalf("Failed to get task1 after restore: %v", err)
	}
	if restoredTask1.Status != TaskStatusCompleted {
		t.Errorf("Expected task1 status 'completed', got %s", restoredTask1.Status)
	}
}

func TestPersistenceManager_RestoreMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nonexistent.json")

	store := NewMemoryStore()
	config := PersistenceConfig{
		Enabled:       true,
		FilePath:      tmpFile,
		RestoreOnLoad: true,
		SaveInterval:  "5m",
	}
	pm := NewPersistenceManager(store, config)

	// 恢复应该成功，因为文件不存在
	if err := pm.restore(); err != nil {
		t.Fatalf("Expected no error for missing file, got %v", err)
	}
}

func TestPersistenceManager_SaveEmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.json")

	store := NewMemoryStore()
	config := PersistenceConfig{
		Enabled:       true,
		FilePath:      tmpFile,
		RestoreOnLoad: false,
		SaveInterval:  "5m",
	}
	pm := NewPersistenceManager(store, config)

	// 保存空存储
	if err := pm.save(); err != nil {
		t.Fatalf("Failed to save empty store: %v", err)
	}

	// 验证文件存在且包含空 map
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if string(content) != "{}" && string(content) != "null" {
		t.Errorf("Expected empty JSON object or null, got %s", content)
	}
}

func TestPersistenceManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "concurrent.json")

	store := NewMemoryStore()
	config := PersistenceConfig{
		Enabled:       true,
		FilePath:      tmpFile,
		RestoreOnLoad: false,
		SaveInterval:  "1m",
	}
	pm := NewPersistenceManager(store, config)

	// 启动后台保存
	if err := pm.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer pm.Stop()

	// 并发添加任务
	const numGoroutines = 5
	const tasksPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				cmd := &models.Command{
					Command: fmt.Sprintf("cmd-%d-%d", idx, j),
					Args:    []string{},
					Cwd:     "/tmp",
				}
				task := NewTask(cmd)
				if err := store.Save(task); err != nil && err != ErrTaskNotFound {
					t.Errorf("Failed to save task: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// 等待后台保存
	time.Sleep(200 * time.Millisecond)

	// 验证所有任务都已保存
	allTasks := store.GetAll()
	expectedTasks := numGoroutines * tasksPerGoroutine
	if len(allTasks) != expectedTasks {
		t.Errorf("Expected %d tasks, got %d", expectedTasks, len(allTasks))
	}

	// logger for completed goroutines
}

func TestPersistenceManager_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "disabled.json")

	store := NewMemoryStore()
	config := PersistenceConfig{
		Enabled:       false,
		FilePath:      tmpFile,
		RestoreOnLoad: false,
		SaveInterval:  "1m",
	}
	pm := NewPersistenceManager(store, config)

	// 启动应该成功但不做任何事
	if err := pm.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	defer pm.Stop()

	if pm.running {
		t.Error("Expected persistence manager not to be running when disabled")
	}
}