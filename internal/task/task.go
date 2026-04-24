package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/user/exec-server/internal/models"
)

// TaskStatus 任务状态
type TaskStatus string

// TaskStatusPending 任务处于等待执行状态
// TaskStatusRunning 任务正在执行中
// TaskStatusCompleted 任务已成功完成
// TaskStatusFailed 任务执行失败
// TaskStatusTimeout 任务因超时而终止
const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimeout   TaskStatus = "timeout"
)

// ManagerInterface 定义任务管理器接口，用于解耦 handlers 包
// 以避免导入循环
// 其他包应该依赖此接口，而不是直接依赖 Manager
type ManagerInterface interface {
	SubmitTask(cmd *models.Command, tokenPrefix string) (*Task, error)
	GetStatus(taskID string) (*Task, error)
	Close()
}

// Task 任务对象，用于异步执行
type Task struct {
	ID          string         `json:"task_id"`
	Command     string         `json:"command"`
	Args        []string       `json:"args,omitempty"`
	Cwd         string         `json:"cwd"`
	Status      TaskStatus     `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Result      *models.Result `json:"result,omitempty"`
	Error       *string        `json:"error,omitempty"`
	TokenPrefix string         `json:"token_prefix,omitempty"`
}

// SubmitResponse 任务提交响应
type SubmitResponse struct {
	TaskID    string     `json:"task_id"`
	Status    TaskStatus `json:"status"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	Stdout      *string    `json:"stdout,omitempty"`
	Stderr      *string    `json:"stderr,omitempty"`
	DurationMs  *int64     `json:"duration_ms,omitempty"`
	Truncated   *bool      `json:"truncated,omitempty"`
	OutputSize  *int64     `json:"output_size_bytes,omitempty"`
	Error       *string    `json:"error,omitempty"`
}

// NewTask 创建新任务
func NewTask(cmd *models.Command, tokenPrefix string) *Task {
	return &Task{
		ID:          generateTaskID(),
		Command:     cmd.Command,
		Args:        cmd.Args,
		Cwd:         cmd.Cwd,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		TokenPrefix: tokenPrefix,
	}
}

// generateTaskID 生成唯一任务ID
func generateTaskID() string {
	return fmt.Sprintf("task-%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}
