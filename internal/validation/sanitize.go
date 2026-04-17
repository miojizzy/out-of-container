// Package validation 提供命令安全验证功能。
//
// 实现了对命令和参数的沙箱验证，防止shell注入等安全威胁。
package validation

import (
	"errors"
	"strings"
)

// ErrShellMetacharFound 是 shell 元字符错误
var ErrShellMetacharFound = errors.New("shell metacharacters not allowed")

// CheckShellMetacharacters checks if string contains forbidden shell metacharacters
// Forbidden chars: | & ; $ ` < > ( ) and combinations: || && $()
func CheckShellMetacharacters(s string) error {
	// Forbidden characters
	forbidden := "|&;$`<>()"

	for _, ch := range s {
		if strings.ContainsRune(forbidden, ch) {
			return ErrShellMetacharFound
		}
	}

	return nil
}

// CheckCommandSafety validates command field for shell injection
func CheckCommandSafety(command string) error {
	return CheckShellMetacharacters(command)
}

// CheckArgsSafety validates args array
// Args can contain special characters (e.g., "foo&bar.txt")
// But we still check for obvious injection patterns
func CheckArgsSafety(args []string) error {
	// For args, we only check for command substitution patterns
	for _, arg := range args {
		if strings.Contains(arg, "$(") || strings.Contains(arg, "`") {
			return ErrShellMetacharFound
		}
	}
	return nil
}
