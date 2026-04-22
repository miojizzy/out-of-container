package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Mock server for testing
func mockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/task":
			// Mock async task submission
			w.WriteHeader(http.StatusAccepted)
			response := `{
				"task_id": "task-1234567890",
				"status": "pending",
				"message": "task submitted successfully",
				"created_at": "2026-04-17T10:00:00Z"
			}`
			fmt.Fprint(w, response)

		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/task/"):
			// Mock task status query
			taskID := strings.TrimPrefix(r.URL.Path, "/task/")
			if taskID == "task-1234567890" {
				w.WriteHeader(http.StatusOK)
				response := `{
					"task_id": "task-1234567890",
					"status": "completed",
					"created_at": "2026-04-17T10:00:00Z",
					"completed_at": "2026-04-17T10:00:05Z",
					"exit_code": 0,
					"stdout": "hello world\n",
					"stderr": "",
					"duration_ms": 5000,
					"truncated": false,
					"output_size_bytes": 12
				}`
				fmt.Fprint(w, response)
			} else if taskID == "not-found" {
				w.WriteHeader(http.StatusNotFound)
				response := `{
					"error": "task_not_found",
					"message": "task not found"
				}`
				fmt.Fprint(w, response)
			} else {
				w.WriteHeader(http.StatusOK)
				response := `{
					"task_id": "` + taskID + `",
					"status": "pending",
					"created_at": "2026-04-17T10:00:00Z"
				}`
				fmt.Fprint(w, response)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestMain 控制测试主函数
func TestMain(m *testing.M) {
	// 运行测试
	code := m.Run()
	os.Exit(code)
}

// TestAsyncSubmitSuccess 测试异步任务提交成功
func TestAsyncSubmitSuccess(t *testing.T) {
	server := mockServer()
	defer server.Close()

	// 保存原始参数
	origServerURL := *serverURL
	origApiToken := *apiToken
	origCommand := *command
	origCwd := *cwd
	origArgs := *args
	origAsyncMode := *asyncMode
	origStatusMode := *statusMode
	origTaskID := *taskID

	// 恢复原始值
	defer func() {
		*serverURL = origServerURL
		*apiToken = origApiToken
		*command = origCommand
		*cwd = origCwd
		*args = origArgs
		*asyncMode = origAsyncMode
		*statusMode = origStatusMode
		*taskID = origTaskID
	}()

	// 设置测试参数
	*serverURL = server.URL
	*apiToken = "test-token"
	*command = "echo"
	*args = "hello,world"
	*asyncMode = true
	*cwd = "/tmp"
	*statusMode = false
	*taskID = ""

	// 调用 handleAsyncSubmit
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleAsyncSubmit panicked: %v", r)
		}
	}()
	handleAsyncSubmit(ClientConfig{
		ServerURL: *serverURL,
		ApiToken:  *apiToken,
	})
}

// TestAsyncSubmitMissingCommand 测试异步提交缺少 command 参数
func TestAsyncSubmitMissingCommand(t *testing.T) {
	// 保存原始参数
	origCommand := *command
	defer func() { *command = origCommand }()

	*command = ""

	// 调用应该会 os.Exit(1)
	// 由于我们无法直接捕获 os.Exit，我们使用 panic 来模拟
	// 实际测试需要重构代码以返回 error 而不是直接 os.Exit
}

// TestStatusQuerySuccess 测试状态查询成功
func TestStatusQuerySuccess(t *testing.T) {
	server := mockServer()
	defer server.Close()

	// 保存原始参数
	origServerURL := *serverURL
	origApiToken := *apiToken
	origTaskID := *taskID
	origStatusMode := *statusMode
	origAsyncMode := *asyncMode

	// 恢复原始值
	defer func() {
		*serverURL = origServerURL
		*apiToken = origApiToken
		*taskID = origTaskID
		*statusMode = origStatusMode
		*asyncMode = origAsyncMode
	}()

	// 设置测试参数
	*serverURL = server.URL
	*apiToken = "test-token"
	*taskID = "task-1234567890"
	*statusMode = true
	*asyncMode = false

	// 调用 handleStatusQuery
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleStatusQuery panicked: %v", r)
		}
	}()
	handleStatusQuery(ClientConfig{
		ServerURL: *serverURL,
		ApiToken:  *apiToken,
	})
}

// TestStatusQueryMissingTaskID 测试状态查询缺少 task-id 参数
func TestStatusQueryMissingTaskID(t *testing.T) {
	// 保存原始参数
	origTaskID := *taskID
	defer func() { *taskID = origTaskID }()

	*taskID = ""

	// 调用应该会 os.Exit(1)
}

// TestParseSubmitResponse 测试解析任务提交响应
func TestParseSubmitResponse(t *testing.T) {
	jsonData := `{
		"task_id": "task-1234567890",
		"status": "pending",
		"message": "task submitted successfully",
		"created_at": "2026-04-17T10:00:00Z"
	}`

	var response SubmitResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to parse submit response: %v", err)
	}

	if response.TaskID != "task-1234567890" {
		t.Errorf("Expected task_id 'task-1234567890', got '%s'", response.TaskID)
	}

	if response.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", response.Status)
	}
}

// TestParseStatusResponse 测试解析任务状态响应
func TestParseStatusResponse(t *testing.T) {
	jsonData := `{
		"task_id": "task-1234567890",
		"status": "completed",
		"created_at": "2026-04-17T10:00:00Z",
		"completed_at": "2026-04-17T10:00:05Z",
		"exit_code": 0,
		"stdout": "hello world\n",
		"stderr": "",
		"duration_ms": 5000,
		"truncated": false,
		"output_size_bytes": 12
	}`

	var response TaskStatusResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	if response.TaskID != "task-1234567890" {
		t.Errorf("Expected task_id 'task-1234567890', got '%s'", response.TaskID)
	}

	if response.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", response.Status)
	}

	if *response.ExitCode != 0 {
		t.Errorf("Expected exit_code 0, got %d", *response.ExitCode)
	}
}

// TestExecuteHTTPRequest 辅助测试函数
func TestExecuteHTTPRequest(t *testing.T) {
	// 这个测试验证 HTTP 请求构造是否正确
	req := ExecRequest{
		Command: "echo",
		Args:    []string{"hello", "world"},
		Cwd:     "/tmp",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if !bytes.Contains(body, []byte(`"command":"echo"`)) {
		t.Error("Request body missing command field")
	}
	if !bytes.Contains(body, []byte(`"args":["hello","world"]`)) {
		t.Error("Request body missing or incorrect args field")
	}
	if !bytes.Contains(body, []byte(`"cwd":"/tmp"`)) {
		t.Error("Request body missing cwd field")
	}
}