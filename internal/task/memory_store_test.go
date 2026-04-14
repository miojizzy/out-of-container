package task

import (
	"testing"
	"time"

	"github.com/user/exec-server/internal/models"
)

func TestMemoryStore_Save(t *testing.T) {
	store := NewMemoryStore()
	cmd := &models.Command{
		Command: "echo",
		Args:    []string{"hello"},
		Cwd:     "/tmp",
	}
	task := NewTask(cmd)

	// 保存任务
	if err := store.Save(task); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证任务已保存
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if loaded == nil {
		t.Error("Expected task to be loaded, got nil")
		return
	}
	if loaded.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, loaded.ID)
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()
	cmd := &models.Command{
		Command: "ls",
		Args:    []string{"-la"},
		Cwd:     "/home",
	}
	task := NewTask(cmd)

	// 保存任务
	if err := store.Save(task); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 获取存在的任务
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if loaded.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, loaded.ID)
	}

	// 获取不存在的任务
	_, err = store.Get("non-existent-id")
	if !IsTaskNotFoundError(err) {
		t.Errorf("Expected TaskNotFoundError, got %v", err)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	cmd := &models.Command{
		Command: "sleep",
		Args:    []string{"5"},
		Cwd:     "/tmp",
	}
	task := NewTask(cmd)

	// 保存任务
	if err := store.Save(task); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 更新为 running 状态
	if err := store.Update(task.ID, TaskStatusRunning, nil, nil); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证状态更新
	loaded, err := store.Get(task.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if loaded.Status != TaskStatusRunning {
		t.Errorf("Expected status 'running', got %s", loaded.Status)
	}
	if loaded.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}

	// 更新为 completed 状态
	result := &models.Result{
		ExitCode:   0,
		Stdout:     "success\n",
		Stderr:     "",
		DurationMs: 5000,
		Truncated:  false,
	}
	if err := store.Update(task.ID, TaskStatusCompleted, result, nil); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证状态和结果更新
	loaded, err = store.Get(task.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if loaded.Status != TaskStatusCompleted {
		t.Errorf("Expected status 'completed', got %s", loaded.Status)
	}
	if loaded.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
	if loaded.Result == nil {
		t.Error("Expected Result to be set")
	}
	if loaded.Result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", loaded.Result.ExitCode)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	cmd := &models.Command{
		Command: "pwd",
		Args:    []string{},
		Cwd:     "/",
	}
	task := NewTask(cmd)

	// 保存任务
	if err := store.Save(task); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 删除任务
	if err := store.Delete(task.ID); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证任务已被删除
	_, err := store.Get(task.ID)
	if !IsTaskNotFoundError(err) {
		t.Errorf("Expected TaskNotFoundError, got %v", err)
	}

	// 删除不存在的任务
	if err := store.Delete("non-existent-id"); err != nil {
		if !IsTaskNotFoundError(err) {
			t.Errorf("Expected TaskNotFoundError, got %v", err)
		}
	}
}

func TestMemoryStore_GetAll(t *testing.T) {
	store := NewMemoryStore()
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

	// 保存两个任务
	if err := store.Save(task1); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if err := store.Save(task2); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 获取所有任务
	allTasks := store.GetAll()

	// 验证任务数量
	if len(allTasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(allTasks))
	}

	// 验证任务内容
	if allTasks[task1.ID] == nil {
		t.Error("Expected task1 to be in map")
	}
	if allTasks[task2.ID] == nil {
		t.Error("Expected task2 to be in map")
	}

	// 验证返回的是副本，修改不会影响原存储
	allTasks[task1.ID].Command = "changed"
	loaded, _ := store.Get(task1.ID)
	if loaded.Command != "echo" {
		t.Errorf("Expected original command 'echo', got %s", loaded.Command)
	}
}

func TestMemoryStore_CleanupExpired(t *testing.T) {
	store := NewMemoryStore()
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

	// 保存任务
	if err := store.Save(task1); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if err := store.Save(task2); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 模拟任务1已完成，任务2还在运行
	task1.Status = TaskStatusCompleted
	task1.CompletedAt = &time.Time{}
	task2.Status = TaskStatusRunning

	// 清理1秒前的任务
	if err := store.CleanupExpired(1 * time.Second); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证已完成的任务被清理
	_, err := store.Get(task1.ID)
	if !IsTaskNotFoundError(err) {
		t.Errorf("Expected TaskNotFoundError for completed task, got %v", err)
	}

	// 验证正在运行的任务未被清理
	loaded, err := store.Get(task2.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if loaded == nil {
		t.Error("Expected task2 to still exist")
	}

	// 模拟长时间未完成的挂起任务
	task3 := NewTask(&models.Command{
		Command: "sleep",
		Args:    []string{"1000"},
		Cwd:     "/tmp",
	})
	// 设置创建时间为1小时之前
	task3.CreatedAt = time.Now().Add(-1 * time.Hour)
	if err := store.Save(task3); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 清理10分钟前的任务
	if err := store.CleanupExpired(10 * time.Minute); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证长时间未完成的挂起任务被清理
	_, err = store.Get(task3.ID)
	if !IsTaskNotFoundError(err) {
		t.Errorf("Expected TaskNotFoundError for pending task, got %v", err)
	}
}

func TestTaskNotFoundError(t *testing.T) {
	err := ErrTaskNotFound

	if err.Error() != "task not found" {
		t.Errorf("Expected error message 'task not found', got %s", err.Error())
	}

	if !IsTaskNotFoundError(err) {
		t.Error("Expected IsTaskNotFoundError to return true")
	}

	if !IsTaskNotFoundError(&TaskNotFoundError{}) {
		t.Error("Expected IsTaskNotFoundError to return true for *TaskNotFoundError")
	}
}
