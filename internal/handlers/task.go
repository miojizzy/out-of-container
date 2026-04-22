package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/task"
)

// TaskHandler handles task-related endpoints
type TaskHandler struct {
	taskManager *task.Manager
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(tm *task.Manager) *TaskHandler {
	return &TaskHandler{
		taskManager: tm,
	}
}

// SubmitTask handles POST /task
func (h *TaskHandler) SubmitTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var cmd models.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "invalid request: failed to parse JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if cmd.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}
	if cmd.Cwd == "" {
		http.Error(w, "cwd is required", http.StatusBadRequest)
		return
	}

	// Extract token prefix from Authorization header
	token := r.Header.Get("Authorization")
	tokenPrefix := ""
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	if len(token) > 8 {
		tokenPrefix = token[:8]
	} else if len(token) > 0 {
		tokenPrefix = token
	}

	// Submit task with token prefix for audit logging
	newTask, err := h.taskManager.SubmitTask(&cmd, tokenPrefix)
	if err != nil {
		if err.Error() == "command not in whitelist" {
			http.Error(w, "command not in whitelist", http.StatusForbidden)
			return
		}
		if err.Error() == "cwd not allowed" {
			http.Error(w, "cwd not allowed", http.StatusForbidden)
			return
		}
		http.Error(w, "failed to submit task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response
	response := &task.SubmitResponse{
		TaskID:    newTask.ID,
		Status:    newTask.Status,
		Message:   "task submitted successfully",
		CreatedAt: newTask.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "encoding_error", "failed to encode response")
		return
	}
}

// GetTaskStatus handles GET /task/{task_id}
func (h *TaskHandler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL
	taskID := r.URL.Path[len("/task/"):] // remove "/task/" prefix
	if taskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}

	// Get task status
	taskObj, err := h.taskManager.GetStatus(taskID)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	// Build response
	response := &task.TaskStatusResponse{
		TaskID:    taskObj.ID,
		Status:    taskObj.Status,
		CreatedAt: taskObj.CreatedAt,
	}

	if taskObj.StartedAt != nil {
		response.StartedAt = taskObj.StartedAt
	}
	if taskObj.CompletedAt != nil {
		response.CompletedAt = taskObj.CompletedAt
		response.DurationMs = new(int64)
		*response.DurationMs = taskObj.CompletedAt.Sub(*taskObj.StartedAt).Milliseconds()
	}

	if taskObj.Result != nil {
		response.ExitCode = &taskObj.Result.ExitCode
		response.Stdout = &taskObj.Result.Stdout
		response.Stderr = &taskObj.Result.Stderr
		response.OutputSize = &taskObj.Result.OutputSize
		response.Truncated = &taskObj.Result.Truncated
	}

	if taskObj.Error != nil {
		response.Error = taskObj.Error
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "encoding_error", "failed to encode response")
		return
	}
}
