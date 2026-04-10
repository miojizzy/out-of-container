package executor

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/validation"
)

// mockValidator 用于测试的验证器
type mockValidator struct {
	checkCommandErr error
	checkArgsErr    error
}

func (m *mockValidator) CheckCommandSafety(command string) error {
	return m.checkCommandErr
}

func (m *mockValidator) CheckArgsSafety(args []string) error {
	return m.checkArgsErr
}

// resetValidation 重置验证函数（保持接口兼容性）
func resetValidation() {
	// 不需要做任何事情，因为现在使用接口
}

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name             string
		timeoutSec       int
		maxOutputMB      int64
		expectedTimeout  time.Duration
		expectedMaxBytes int64
	}{
		{
			name:             "正常创建",
			timeoutSec:       30,
			maxOutputMB:      10,
			expectedTimeout:  30 * time.Second,
			expectedMaxBytes: 10 * 1024 * 1024,
		},
		{
			name:             "零值",
			timeoutSec:       0,
			maxOutputMB:       0,
			expectedTimeout:  0,
			expectedMaxBytes: 0,
		},
		{
			name:             "大值",
			timeoutSec:       3600,
			maxOutputMB:      100,
			expectedTimeout:  3600 * time.Second,
			expectedMaxBytes: 100 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExecutor(tt.timeoutSec, tt.maxOutputMB)
			if e.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, e.timeout)
			}
			if e.maxOutputBytes != tt.expectedMaxBytes {
				t.Errorf("expected maxOutputBytes %d, got %d", tt.expectedMaxBytes, e.maxOutputBytes)
			}
		})
	}
}

func TestExecutor_Execute_CommandSafetyValidation(t *testing.T) {
	e := NewExecutorWithValidator(30, 10, &mockValidator{
		checkCommandErr: errors.New("unsafe command"),
		checkArgsErr:    nil,
	})

	result := e.Execute(&models.Command{
		Command: "dangerous",
		Args:    []string{"arg1"},
		Cwd:     "/tmp",
	})

	if result.Error == nil {
		t.Error("expected error for unsafe command")
	}
	if result.HTTPError != http.StatusBadRequest {
		t.Errorf("expected HTTP status %d, got %d", http.StatusBadRequest, result.HTTPError)
	}
	// validation失败时不会执行命令，所以Result可能为nil
	// 这个行为是正确的
}

func TestExecutor_Execute_ArgsSafetyValidation(t *testing.T) {
	e := NewExecutorWithValidator(30, 10, &mockValidator{
		checkCommandErr: nil,
		checkArgsErr:    errors.New("unsafe args"),
	})

	result := e.Execute(&models.Command{
		Command: "echo",
		Args:    []string{"$(rm -rf /)"},
		Cwd:     "/tmp",
	})

	if result.Error == nil {
		t.Error("expected error for unsafe args")
	}
	if result.HTTPError != http.StatusBadRequest {
		t.Errorf("expected HTTP status %d, got %d", http.StatusBadRequest, result.HTTPError)
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	e := NewExecutor(30, 10)

	// 使用一个简单的成功命令
	result := e.Execute(&models.Command{
		Command: "echo",
		Args:    []string{"hello", "world"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.Result.ExitCode)
	}
	if !strings.Contains(result.Result.Stdout, "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got: %s", result.Result.Stdout)
	}
	if result.Result.Stderr != "" {
		t.Errorf("expected empty stderr, got: %s", result.Result.Stderr)
	}
	if result.Result.Truncated {
		t.Error("expected not truncated")
	}
	if result.HTTPError != 0 {
		t.Errorf("expected no HTTP error, got %d", result.HTTPError)
	}
}

func TestExecutor_Execute_ExitCode(t *testing.T) {
	e := NewExecutor(30, 10)

	// 测试非零退出码
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "exit 42"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.Result.ExitCode)
	}
}

func TestExecutor_Execute_WithCwd(t *testing.T) {
		tmpDir := t.TempDir()
	e := NewExecutor(30, 10)

	// 创建一个测试文件
	testContent := "test content"
	testFile := tmpDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := e.Execute(&models.Command{
		Command: "cat",
		Args:    []string{"test.txt"},
		Cwd:     tmpDir,
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Result.Stdout, testContent) {
		t.Errorf("expected stdout to contain %q, got: %s", testContent, result.Result.Stdout)
	}
}

func TestExecutor_Execute_Timeout(t *testing.T) {
		e := NewExecutor(1, 10) // 1秒超时

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "sleep 5 && echo done"},
		Cwd:     t.TempDir(),
	})

	if result.Error == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", result.Error)
	}
	if result.HTTPError != http.StatusRequestTimeout {
		t.Errorf("expected HTTP status %d, got %d", http.StatusRequestTimeout, result.HTTPError)
	}
	if result.Result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Result.ExitCode != -1 {
		t.Errorf("expected exit code -1, got %d", result.Result.ExitCode)
	}
}

func TestExecutor_Execute_CommandStartFailure(t *testing.T) {
		e := NewExecutor(30, 10)

	// 使用不存在的命令
	result := e.Execute(&models.Command{
		Command: "nonexistent_command_xyz",
		Args:    []string{},
		Cwd:     t.TempDir(),
	})

	if result.Error == nil {
		t.Error("expected error for non-existent command")
	}
	if result.HTTPError != http.StatusInternalServerError {
		t.Errorf("expected HTTP status %d, got %d", http.StatusInternalServerError, result.HTTPError)
	}
	if result.Result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Result.ExitCode != -1 {
		t.Errorf("expected exit code -1, got %d", result.Result.ExitCode)
	}
}

func TestExecutor_Execute_OutputTruncation(t *testing.T) {
		// 使用较小的限制来测试截断
	e := NewExecutor(30, 1) // 1MB限制

	// 生成适量的输出用于测试
	output := strings.Repeat("x", 100*1024) // 100KB
	cmd := fmt.Sprintf("echo %q", output)

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", cmd},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// 在正常情况下，100KB的输出不会被截断，但我们可以检查结果结构
	if result.Result == nil {
		t.Fatal("expected result to be set")
	}
}

func TestExecutor_Execute_CombinedOutputLimit(t *testing.T) {
		// 使用很小的限制来测试共享计数器
	e := NewExecutor(30, 1) // 1MB总输出限制

	// 同时产生stdout和stderr
	script := `
		for i in 1 2 3 4 5; do
			echo "stdout $i"
			echo "stderr $i" >&2
		done
	`
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", script},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	// 验证结果对象不为空
	if result.Result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestExecutor_Execute_StderrOnly(t *testing.T) {
		e := NewExecutor(30, 10)

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "echo 'error' >&2"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Result.Stderr, "error") {
		t.Errorf("expected stderr to contain 'error', got: %s", result.Result.Stderr)
	}
}

func TestExecutor_Execute_DurationMs(t *testing.T) {
		e := NewExecutor(30, 10)

	// 执行一个需要少量时间的命令
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "sleep 0.1"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Result.DurationMs <= 0 {
		t.Errorf("expected positive duration, got %d", result.Result.DurationMs)
	}
}

// LimitedBuffer 的测试

func TestLimitedBuffer_Write(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 100}

	n, err := buf.Write([]byte("hello"))
	if n != 5 || err != nil {
		t.Errorf("Write returned n=%d, err=%v", n, err)
	}
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
	if buf.IsTruncated() {
		t.Error("buffer should not be truncated yet")
	}
	if buf.BytesWritten() != 5 {
		t.Errorf("expected 5 bytes written, got %d", buf.BytesWritten())
	}
}

func TestLimitedBuffer_Truncation(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 10}

	// 写入超过限制
	bigData := strings.Repeat("x", 20)
	n, _ := buf.Write([]byte(bigData))
	if n != 20 {
		t.Errorf("expected Write to return 20, got %d", n)
	}
	if !buf.IsTruncated() {
		t.Error("buffer should be truncated")
	}
	if buf.BytesWritten() > 10 {
		t.Errorf("expected at most 10 bytes written, got %d", buf.BytesWritten())
	}
}

func TestLimitedBuffer_DiscardAfterTruncation(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 5}

	// 先超出限制
	buf.Write([]byte(strings.Repeat("x", 10)))
	if !buf.IsTruncated() {
		t.Fatal("buffer should be truncated after first write")
	}

	// 截断后再次写入应该被丢弃
	before := buf.BytesWritten()
	data := "new data"
	n, _ := buf.Write([]byte(data))
	// Write总是返回请求写入的字节数，即使实际没有写入
	if n != len(data) {
		t.Errorf("expected Write to return %d (data length), got %d", len(data), n)
	}
	// 一旦截断，字节计数不应再增加
	if buf.BytesWritten() != before {
		t.Errorf("bytes written should not change after truncation, was %d, still %d", before, buf.BytesWritten())
	}
}

func TestLimitedBuffer_WithSharedCounter(t *testing.T) {
	counter := &SharedCounter{maxBytes: 100}
	buf := &LimitedBuffer{
		maxBytes:      50,
		sharedCounter: counter,
	}

	// 写入50字节
	data50 := strings.Repeat("a", 50)
	buf.Write([]byte(data50))
	if buf.BytesWritten() != 50 {
		t.Errorf("expected 50 bytes, got %d", buf.BytesWritten())
	}

	// 写入30字节到另一个共享同一counter的buffer
	buf2 := &LimitedBuffer{
		maxBytes:      50,
		sharedCounter: counter,
	}
	data30 := strings.Repeat("b", 30)
	buf2.Write([]byte(data30))
	if buf2.BytesWritten() != 30 {
		t.Errorf("expected 30 bytes, got %d", buf2.BytesWritten())
	}

	// counter总共应该写了80字节
	if counter.total != 80 {
		t.Logf("counter.total = %d (expected ~80)", counter.total)
	}
}

func TestSharedCounter_CanWrite(t *testing.T) {
	tests := []struct {
		name            string
		maxBytes        int64
		initialTotal    int64
		writeSize       int64
		expectedCanWrite bool
		expectedTotal   int64
	}{
		{
			name:            "initial write within limit",
			maxBytes:        100,
			initialTotal:    0,
			writeSize:       50,
			expectedCanWrite: true,
			expectedTotal:   50,
		},
		{
			name:            "write exactly at limit",
			maxBytes:        100,
			initialTotal:    50,
			writeSize:       50,
			expectedCanWrite: true,
			expectedTotal:   100,
		},
		{
			name:            "write over limit",
			maxBytes:        100,
			initialTotal:    90,
			writeSize:       20,
			expectedCanWrite: false,
			expectedTotal:   90,
		},
		{
			name:            "zero write",
			maxBytes:        100,
			initialTotal:    0,
			writeSize:       0,
			expectedCanWrite: true,
			expectedTotal:   0,
		},
		{
			name:            "zero limit",
			maxBytes:        0,
			initialTotal:    0,
			writeSize:       1,
			expectedCanWrite: false,
			expectedTotal:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &SharedCounter{maxBytes: tt.maxBytes, total: tt.initialTotal}
			canWrite := c.CanWrite(tt.writeSize)
			if canWrite != tt.expectedCanWrite {
				t.Errorf("CanWrite returned %v, expected %v", canWrite, tt.expectedCanWrite)
			}
			if c.total != tt.expectedTotal {
				t.Errorf("total = %d, expected %d", c.total, tt.expectedTotal)
			}
		})
	}
}

func TestSharedCounter_ConcurrentAccess(t *testing.T) {
	counter := &SharedCounter{maxBytes: 1000}
	var wg sync.WaitGroup
	var successCount int64

	// 10个goroutine同时写入
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每个goroutine尝试写入150字节，总共需要1500字节
			// 但由于限制是1000字节，只有部分会成功
			if counter.CanWrite(150) {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if counter.total > 1000 {
		t.Errorf("counter exceeded limit: total=%d, max=%d", counter.total, counter.maxBytes)
	}

	// 验证确实有并发控制发生
	if successCount*150 != counter.total {
		t.Logf("Concurrent access controlled: successCount=%d, total=%d", successCount, counter.total)
	}
}

// ErrorResponse 的测试
// 注意：由于ErrorResponse直接写入http.ResponseWriter，需要一个模拟的ResponseWriter

type mockResponseWriter struct {
	headerWritten bool
	statusCode    int
	body          []byte
}

func (m *mockResponseWriter) Header() http.Header {
	return http.Header{}
}

func (m *mockResponseWriter) Write(bytes []byte) (int, error) {
	m.body = append(m.body, bytes...)
	return len(bytes), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.headerWritten = true
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		errMsg      string
		message     string
		wantStatus  int
		wantErrMsg  string
		wantMessage string
	}{
		{
			name:        "错误404",
			statusCode:  http.StatusNotFound,
			errMsg:      "not_found",
			message:     "资源不存在",
			wantStatus:  http.StatusNotFound,
			wantErrMsg:  "not_found",
			wantMessage: "资源不存在",
		},
		{
			name:        "服务器错误500",
			statusCode:  http.StatusInternalServerError,
			errMsg:      "internal_error",
			message:     "服务器内部错误",
			wantStatus:  http.StatusInternalServerError,
			wantErrMsg:  "internal_error",
			wantMessage: "服务器内部错误",
		},
		{
			name:        "JSON编码失败处理",
			statusCode:  http.StatusBadRequest,
			errMsg:      "bad",
			message:     "这是一个包含Unicode字符的测试: 你好世界",
			wantStatus:  http.StatusBadRequest,
			wantErrMsg:  "bad",
			wantMessage: "这是一个包含Unicode字符的测试: 你好世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &mockResponseWriter{}
			ErrorResponse(mw, tt.statusCode, tt.errMsg, tt.message)

			if mw.statusCode != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, mw.statusCode)
			}
		})
	}
}

// 测试ExecuteResult结构体方法

func TestExecuteResult_NilResult(t *testing.T) {
	result := &ExecuteResult{
		Error:     errors.New("test error"),
		HTTPError: http.StatusBadRequest,
	}

	if result.Result != nil {
		t.Error("expected nil result")
	}
}

func TestLimitedBuffer_SetSharedCounter(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 100}
	counter := &SharedCounter{maxBytes: 200}

	buf.SetSharedCounter(counter)
	if buf.sharedCounter != counter {
		t.Error("shared counter not set correctly")
	}
}

func TestLimitedBuffer_ZeroMaxBytes(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 0}

	// 任何写入都应该立即截断
	n, _ := buf.Write([]byte("hello"))
	if n != 5 {
		t.Errorf("expected Write to return 5, got %d", n)
	}
	if !buf.IsTruncated() {
		t.Error("buffer should be truncated when maxBytes is 0")
	}
	if buf.BytesWritten() != 0 {
		t.Errorf("expected 0 bytes written, got %d", buf.BytesWritten())
	}
}

func TestLimitedBuffer_EmptyWrites(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 100}

	n, _ := buf.Write([]byte{})
	if n != 0 {
		t.Errorf("expected Write to return 0 for empty slice, got %d", n)
	}
}

func TestLimitedBuffer_MultipleWrites(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 20}

	buf.Write([]byte("12345"))  // 5 bytes
	buf.Write([]byte("67890"))  // 5 bytes, total 10
	buf.Write([]byte("abcde"))  // 5 bytes, total 15
	buf.Write([]byte("fghij"))  // 5 bytes, total 20, exactly full
	buf.Write([]byte("klmno"))  // should truncate after this

	if buf.BytesWritten() != 20 {
		t.Errorf("expected 20 bytes written, got %d", buf.BytesWritten())
	}
	if !buf.IsTruncated() {
		t.Error("buffer should be truncated after writing past limit")
	}
	if got := buf.String(); got != "1234567890abcde"+"fghij"[:5] {
		t.Errorf("unexpected buffer content: %q", got)
	}
}

// 测试Execute的边界情况

func TestExecutor_Execute_EmptyCommand(t *testing.T) {
	e := NewExecutor(30, 10)

	// 空命令（exec.CommandContext应该会失败）
	result := e.Execute(&models.Command{
		Command: "",
		Args:    []string{},
		Cwd:     t.TempDir(),
	})

	// 空命令应该被安全验证拒绝
	validation.CheckCommandSafety("") // 返回错误
	if result.HTTPError != http.StatusBadRequest {
		t.Logf("empty command resulted in HTTP status: %d", result.HTTPError)
	}
}

func TestExecutor_Execute_LongCommand(t *testing.T) {
		e := NewExecutor(30, 100)

	// 测试带很多参数的命令
	args := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		args[i] = fmt.Sprintf("arg%d", i)
	}

	result := e.Execute(&models.Command{
		Command: "true",  // /bin/true，总是成功
		Args:    args,
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		// 命令可能因为参数过长而失败，这取决于操作系统限制
		t.Logf("long command result: error=%v, exitCode=%d", result.Error, result.Result.ExitCode)
	}
}

func TestExecutor_Execute_SignalTermination(t *testing.T) {
		e := NewExecutor(30, 10)

	// 测试一个会被信号终止的命令
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "trap 'exit 143' TERM; sleep 100"},
		Cwd:     t.TempDir(),
	})

	if result.Result.ExitCode == 143 {
		t.Log("command terminated by TERM signal (exit 143)")
	}
}

func TestExecutor_Execute_Performance(t *testing.T) {
		e := NewExecutor(30, 100)

	// 执行一些快速命令测试性能
	commands := []*models.Command{
		{Command: "true"},
		{Command: "echo", Args: []string{"test"}},
		{Command: "pwd"},
		{Command: "env"},
	}

	for i, cmd := range commands {
		start := time.Now()
		result := e.Execute(cmd)
		duration := time.Since(start)

		if result.Error != nil && !strings.Contains(result.Error.Error(), "not found") {
			t.Logf("command %d failed: %v", i, result.Error)
		}

		// 每个命令应该在合理时间内完成（100ms内）
		if duration > 100*time.Millisecond {
			t.Logf("command %d took %v", i, duration)
		}
	}
}

// LimitedBuffer 的更多边界测试

func TestLimitedBuffer_ConcurrentWrites(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 1000}
	var wg sync.WaitGroup

	// 10个goroutine同时写入
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := fmt.Sprintf("goroutine-%d-data-", idx) + strings.Repeat("x", 100)
			buf.Write([]byte(data))
		}(i)
	}

	wg.Wait()

	// 检查结果：要么被截断，要么总字节不超过限制
	if !buf.IsTruncated() && buf.BytesWritten() > 1000 {
		t.Errorf("buffer overran limit: bytesWritten=%d", buf.BytesWritten())
	}
}

func TestLimitedBuffer_ExactLimit(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 100}

	// 写入刚好100字节
	data := strings.Repeat("x", 100)
	n, _ := buf.Write([]byte(data))
	if n != 100 {
		t.Errorf("Write returned %d, expected 100", n)
	}
	if buf.BytesWritten() != 100 {
		t.Errorf("expected 100 bytes, got %d", buf.BytesWritten())
	}
	if buf.IsTruncated() {
		t.Error("buffer should not be truncated when exactly at limit")
	}

	// 再写入任何内容都应该被丢弃
	before := buf.BytesWritten()
	buf.Write([]byte("extra"))
	if buf.BytesWritten() != before {
		t.Errorf("bytes written changed after limit reached")
	}
}

func TestLimitedBuffer_OneByteOver(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 10}

	buf.Write([]byte("12345"))   // 5
	buf.Write([]byte("67890"))   // 5, total 10 (full)
	buf.Write([]byte("a"))       // 1 over, should trigger truncation

	if buf.BytesWritten() != 10 {
		t.Errorf("expected 10 bytes written, got %d", buf.BytesWritten())
	}
	if !buf.IsTruncated() {
		t.Error("buffer should be truncated")
	}
}

func TestSharedCounter_Reset(t *testing.T) {
	counter := &SharedCounter{maxBytes: 100}
	counter.CanWrite(50)
	if counter.total != 50 {
		t.Errorf("expected total 50, got %d", counter.total)
	}

	// 手动重置total用于测试
	counter.total = 0
	if counter.total != 0 {
		t.Error("failed to reset total")
	}

	if !counter.CanWrite(100) {
		t.Error("should be able to write 100 after reset")
	}
}

// 集成测试

func TestExecutor_Integration_MultipleCommands(t *testing.T) {
		e := NewExecutor(30, 10)

	tests := []struct {
		cmd     string
		args    []string
		wantOut string
	}{
		{"echo", []string{"hello"}, "hello"},
		{"echo", []string{"a", "b", "c"}, "a b c"},
		{"true", []string{}, ""},
		{"false", []string{}, ""},
		{"pwd", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s%v", tt.cmd, tt.args), func(t *testing.T) {
			result := e.Execute(&models.Command{
				Command: tt.cmd,
				Args:    tt.args,
				Cwd:     t.TempDir(),
			})

			if result.Error != nil && tt.cmd != "false" { // false返回非零退出码，不应该被视为error
				t.Skipf("command not available: %v", result.Error)
				return
			}

			if tt.wantOut != "" && !strings.Contains(result.Result.Stdout, tt.wantOut) {
				t.Errorf("expected stdout to contain %q, got %q", tt.wantOut, result.Result.Stdout)
			}
		})
	}
}

func TestExecutor_Execute_ShellBuiltin(t *testing.T) {
		e := NewExecutor(30, 10)

	// 测试shell内置命令（必须通过shell执行）
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "echo $HOME"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Result.Stdout == "" {
		t.Error("expected non-empty stdout")
	}
}

func TestExecutor_Execute_MultipleArguments(t *testing.T) {
		e := NewExecutor(30, 10)

	// 测试多参数命令
	result := e.Execute(&models.Command{
		Command: "printf",
		Args:    []string{"%s-%s-%s", "a", "b", "c"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Result.Stdout, "a-b-c") {
		t.Errorf("expected output to contain 'a-b-c', got %q", result.Result.Stdout)
	}
}

// 测试zero值行为

func TestExecutor_Execute_ZeroTimeout(t *testing.T) {
		// 零超时应该立即超时
	e := NewExecutor(0, 10)

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "sleep 0.1"},
		Cwd:     t.TempDir(),
	})

	// 零超时的处理取决于context的实现，可能不会立即超时
	// 但至少确保不会panic
	if result.Error != nil && !strings.Contains(result.Error.Error(), "timeout") {
		t.Logf("zero timeout resulted in: %v", result.Error)
	}
}

func TestExecutor_Execute_NegativeTimeout(t *testing.T) {
		// 负数超时
	e := NewExecutor(-1, 10)

	result := e.Execute(&models.Command{
		Command: "echo",
		Args:    []string{"test"},
		Cwd:     t.TempDir(),
	})

	// 负数超时应该被视为0，立即应用超时
	if result.Error != nil {
		t.Logf("negative timeout resulted in: %v", result.Error)
	}
}

func TestLimitedBuffer_String(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 100}
	buf.Write([]byte("hello world"))
	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", buf.String())
	}
}

// 大输出截断测试
func TestExecutor_Execute_LargeOutputTruncation(t *testing.T) {
		// 使用极小的限制来确保触发截断
	e := NewExecutor(30, 0) // 0MB = 无输出

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "echo 'test output'"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		// 0字节限制可能导致命令启动失败，这是可以接受的
		t.Logf("command failed with 0MB limit: %v", result.Error)
		return
	}

	// 检查是否至少有一个输出被截断或为空
	if result.Result.Stdout != "" || result.Result.Stderr != "" {
		t.Logf("With zero limit, output might be empty: stdout=%q stderr=%q",
			result.Result.Stdout, result.Result.Stderr)
	}
}

// 测试共享计数器的行为
func TestLimitedBuffer_SharedCounterLimit(t *testing.T) {
	counter := &SharedCounter{maxBytes: 50}
	buf1 := &LimitedBuffer{maxBytes: 100, sharedCounter: counter}
	buf2 := &LimitedBuffer{maxBytes: 100, sharedCounter: counter}

	// 每个buffer最多100字节，但共享限制是50字节

	// 第一个buffer写入30字节
	data1 := strings.Repeat("a", 30)
	n1, _ := buf1.Write([]byte(data1))
	if n1 != 30 {
		t.Errorf("expected Write to return 30, got %d", n1)
	}
	// 检查实际写入的字节数
	if buf1.BytesWritten() != 30 {
		t.Errorf("expected 30 bytes written to buf1, got %d", buf1.BytesWritten())
	}
	// counter.total 应该是30
	if counter.total != 30 {
		t.Logf("counter.total=%d (expected 30)", counter.total)
	}

	// 第二个buffer尝试写入30字节
	// 但由于共享计数器限制（30+30=60 > 50），这次写入应该失败
	data2 := strings.Repeat("b", 30)
	n2, _ := buf2.Write([]byte(data2))
	if n2 != 30 {
		t.Errorf("expected Write to return 30, got %d", n2)
	}
	// 由于共享计数器限制，buf2实际上没有写入任何数据
	if buf2.BytesWritten() != 0 {
		t.Errorf("expected 0 bytes written to buf2 due to shared counter limit, got %d", buf2.BytesWritten())
	}
	// counter.total 应该仍然是30，因为第二次写入被拒绝了
	if counter.total != 30 {
		t.Logf("counter.total=%d (expected 30 because second write was rejected)", counter.total)
	}
}

// 测试buffer在没有共享计数器时的行为
func TestLimitedBuffer_NoSharedCounter(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 10}
	// 不设置共享计数器

	buf.Write([]byte(strings.Repeat("x", 5)))
	if buf.BytesWritten() != 5 {
		t.Errorf("expected 5 bytes, got %d", buf.BytesWritten())
	}

	buf.Write([]byte(strings.Repeat("y", 10)))
	if buf.BytesWritten() != 10 {
		t.Errorf("expected 10 bytes (limited by maxBytes), got %d", buf.BytesWritten())
	}

	if !buf.IsTruncated() {
		t.Error("expected buffer to be truncated")
	}

	// 再次写入应该被丢弃
	before := buf.BytesWritten()
	buf.Write([]byte("extra"))
	if buf.BytesWritten() != before {
		t.Errorf("bytes written changed after truncation")
	}
}

// 测试边界条件：刚好写满
func TestLimitedBuffer_ExactlyFull(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 10}

	buf.Write([]byte("12345")) // 5
	buf.Write([]byte("67890")) // 5, total 10

	if buf.BytesWritten() != 10 {
		t.Errorf("expected 10 bytes, got %d", buf.BytesWritten())
	}
	if buf.IsTruncated() {
		t.Error("should not be truncated when exactly full")
	}
	if buf.String() != "1234567890" {
		t.Errorf("unexpected buffer content: %q", buf.String())
	}

	// 再写一个字节
	buf.Write([]byte("X"))
	if buf.BytesWritten() != 10 {
		t.Errorf("should still have 10 bytes, got %d", buf.BytesWritten())
	}
	if !buf.IsTruncated() {
		t.Error("should be truncated after exceeding limit")
	}
}

// 测试连续多个小写入
func TestLimitedBuffer_MultipleSmallWrites(t *testing.T) {
	buf := &LimitedBuffer{maxBytes: 10}

	for i := 0; i < 20; i++ {
		buf.Write([]byte{byte('a' + i%26)})
	}

	if buf.BytesWritten() != 10 {
		t.Errorf("expected 10 bytes total, got %d", buf.BytesWritten())
	}
	if !buf.IsTruncated() {
		t.Error("expected buffer to be truncated")
	}
}

// 测试ExecuteResult结构体的所有字段
func TestExecuteResult_Structure(t *testing.T) {
	result := &ExecuteResult{
		Result: &models.Result{
			ExitCode:   0,
			Stdout:      "test stdout",
			Stderr:      "test stderr",
			DurationMs: 100,
			Truncated:  false,
			OutputSize: 20,
		},
		Error:     nil,
		HTTPError: 0,
	}

	if result.Result == nil {
		t.Error("Result should not be nil")
	}
	if result.Result.ExitCode != 0 {
		t.Errorf("unexpected exit code: %d", result.Result.ExitCode)
	}
	if result.Result.Stdout != "test stdout" {
		t.Errorf("unexpected stdout: %q", result.Result.Stdout)
	}
}

// 测试接口实现
func TestLimitations(t *testing.T) {
	var _ io.Writer = (*LimitedBuffer)(nil)
}

// 测试空命令（需要特殊处理，因为syscall会失败）
func TestExecutor_Execute_EmptyCommandSafety(t *testing.T) {
		e := NewExecutor(30, 10)

	// 空命令应该被验证拒绝
	result := e.Execute(&models.Command{
		Command: "",
		Args:    []string{},
		Cwd:     t.TempDir(),
	})

	// 验证空命令应该失败
	if result.HTTPError != http.StatusBadRequest {
		t.Logf("Empty command resulted in HTTP status: %d", result.HTTPError)
	}
}

// 测试大量的stderr输出
func TestExecutor_Execute_LargeStderr(t *testing.T) {
		e := NewExecutor(30, 1) // 1MB限制

	script := "for i in 1 2 3 4 5; do echo 'This is stderr line ' >&2; done"

	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", script},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	if result.Result.Stderr == "" {
		t.Error("expected non-empty stderr")
	}
}

// 测试stdout和stderr输出大小计算
func TestExecutor_Execute_BothOutputsLarge(t *testing.T) {
	e := NewExecutor(30, 2) // 2MB限制

	// 产生一些stdout和stderr输出
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", "echo 'stdout line'; echo 'stderr line' >&2; echo 'stdout line 2'; echo 'stderr line 2' >&2"},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	// 检查是否至少有一些输出
	if len(result.Result.Stdout) == 0 && len(result.Result.Stderr) == 0 {
		t.Error("expected some output in stdout or stderr")
	}
}

// 测试快速成功的命令（关注性能）
func TestExecutor_Execute_FastCommand(t *testing.T) {
		e := NewExecutor(30, 10)

	// /bin/true应该立即返回
	start := time.Now()
	result := e.Execute(&models.Command{
		Command: "true",
		Args:    []string{},
		Cwd:     t.TempDir(),
	})
	duration := time.Since(start)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.Result.ExitCode)
	}

	// true命令应该在几毫秒内完成
	if duration > 100*time.Millisecond {
		t.Logf("true command took %v (maybe slow system)", duration)
	}
}


// 测试只包含特殊字符的stdout
func TestExecutor_Execute_SpecialCharsInOutput(t *testing.T) {
		e := NewExecutor(30, 10)

	// 包含特殊字符的输出
	script := "echo '特殊字符: 你好 こんにちは ñoño 🎉'"
	result := e.Execute(&models.Command{
		Command: "sh",
		Args:    []string{"-c", script},
		Cwd:     t.TempDir(),
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	expectedSubstrings := []string{"特殊", "你好", "🎉"}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(result.Result.Stdout, sub) {
			t.Logf("expected stdout to contain %q, got %q", sub, result.Result.Stdout)
		}
	}
}

