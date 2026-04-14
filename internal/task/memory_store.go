package task

import (
	"sync"
	"time"

	"github.com/user/exec-server/internal/models"
)

// MemoryStore 内存任务存储实现
type MemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemoryStore 创建新的内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: make(map[string]*Task),
	}
}

// Save 保存任务
func (s *MemoryStore) Save(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	return nil
}

// Get 获取任务
func (s *MemoryStore) Get(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// Update 更新任务状态和结果
func (s *MemoryStore) Update(taskID string, status TaskStatus, result *models.Result, err *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	task.Status = status
	if status == TaskStatusRunning {
		now := time.Now()
		task.StartedAt = &now
	} else if status == TaskStatusCompleted || status == TaskStatusFailed || status == TaskStatusTimeout {
		now := time.Now()
		task.CompletedAt = &now
		if result != nil {
			task.Result = result
		}
		if err != nil {
			task.Error = err
		}
	}
	return nil
}

// Delete 删除任务
func (s *MemoryStore) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[taskID]; !exists {
		return ErrTaskNotFound
	}
	delete(s.tasks, taskID)
	return nil
}

// GetAll 获取所有任务（用于持久化）
func (s *MemoryStore) GetAll() map[string]*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回深拷贝，防止外部修改
	tasks := make(map[string]*Task, len(s.tasks))
	for k, v := range s.tasks {
		taskCopy := *v
		if v.Result != nil {
			resultCopy := *v.Result
			taskCopy.Result = &resultCopy
		}
		if v.Error != nil {
			errorCopy := *v.Error
			taskCopy.Error = &errorCopy
		}
		tasks[k] = &taskCopy
	}
	return tasks
}

// CleanupExpired 清理过期任务
func (s *MemoryStore) CleanupExpired(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl)

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, task := range s.tasks {
		if task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(s.tasks, id)
		} else if task.CompletedAt == nil && task.CreatedAt.Before(cutoff) {
			// 长时间未完成的挂起任务也清理
			delete(s.tasks, id)
		}
	}
	return nil
}

// ErrTaskNotFound 任务未找到错误
var ErrTaskNotFound = &TaskNotFoundError{}

// TaskNotFoundError 任务未找到错误类型
type TaskNotFoundError struct{}

func (e *TaskNotFoundError) Error() string {
	return "task not found"
}

// IsTaskNotFoundError 检查是否为任务未找到错误
func IsTaskNotFoundError(err error) bool {
	_, ok := err.(*TaskNotFoundError)
	return ok || err == ErrTaskNotFound
}
