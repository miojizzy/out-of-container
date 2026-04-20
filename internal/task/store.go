package task

import (
	"time"

	"github.com/user/exec-server/internal/models"
)

// Store 定义任务存储接口
type Store interface {
	// Save 保存任务
	Save(task *Task) error

	// Get 获取任务
	Get(taskID string) (*Task, error)

	// Update 更新任务状态和结果
	Update(taskID string, status Status, result *models.Result, err *string) error

	// Delete 删除任务
	Delete(taskID string) error

	// GetAll 获取所有任务（用于持久化）
	GetAll() map[string]*Task

	// CleanupExpired 清理过期任务
	CleanupExpired(ttl time.Duration) error
}
