package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/user/exec-server/internal/auth"
	"github.com/user/exec-server/internal/whitelist"
)

// WhitelistInfoResponse 返回白名单信息的结构体
type WhitelistInfoResponse struct {
	// LiteralCommands 返回字面量匹配的命令列表
	LiteralCommands []string `json:"literal_commands"`
	// RegexCommands 返回正则表达式匹配的命令列表
	RegexCommands []string `json:"regex_commands"`
	// AllowedPaths 返回允许的工作目录路径列表
	AllowedPaths []string `json:"allowed_paths"`
	// ReloadIntervalSeconds 返回配置重载间隔（秒）
	ReloadIntervalSeconds int `json:"reload_interval_seconds"`
}

// WhitelistInfoHandler 处理白名单信息发现请求
type WhitelistInfoHandler struct {
	whitelist *whitelist.Checker
	auth      *auth.AuthMiddleware
}

// NewWhitelistInfoHandler 创建新的白名单信息处理器
func NewWhitelistInfoHandler(
	whitelistChecker *whitelist.Checker,
	apiToken string,
) *WhitelistInfoHandler {
	return &WhitelistInfoHandler{
		whitelist: whitelistChecker,
		auth:      auth.NewAuthMiddleware(apiToken),
	}
}

// ServeHTTP 处理白名单信息请求
func (h *WhitelistInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 应用认证中间件
	handler := h.auth.Middleware(h.handle)
	handler(w, r)
}

// handle 是实际的请求处理函数
func (h *WhitelistInfoHandler) handle(w http.ResponseWriter, r *http.Request) {
	// 仅允许 GET 方法
	if r.Method != http.MethodGet {
		ErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed")
		return
	}

	// 获取白名单配置
	config := h.whitelist.GetConfig()
	if config == nil {
		ErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to get whitelist configuration")
		return
	}

	// 构建响应
	resp := WhitelistInfoResponse{
		LiteralCommands:       config.Whitelist.LiteralCommands,
		RegexCommands:         config.Whitelist.RegexCommands,
		AllowedPaths:          config.Whitelist.AllowedPaths,
		ReloadIntervalSeconds: config.Whitelist.ReloadIntervalSeconds,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 编码并发送响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// 注意：此时响应可能已经部分写入，但这是最后一步
		// 在发送响应头后出错只能记录日志
		// 这里不需要特殊处理，因为客户端会收到不完整的响应
	}
}
