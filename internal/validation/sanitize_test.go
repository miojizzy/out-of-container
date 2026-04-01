package validation

import (
	"testing"
)

func TestCheckShellMetacharacters(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"clean command", "make", false},
		{"with pipe", "echo | cat", true},
		{"with ampersand", "cmd && other", true},
		{"with semicolon", "cmd ; rm", true},
		{"with dollar", "echo $HOME", true},
		{"with backtick", "echo `date`", true},
		{"with less than", "cat < file", true},
		{"with greater than", "echo > file", true},
		{"with parentheses", "$(cmd)", true},
		{"complex injection", "make && rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckShellMetacharacters(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckShellMetacharacters(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCheckArgsSafety(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"safe args", []string{"-j4", "target"}, false},
		{"filename with ampersand", []string{"foo&bar.txt"}, false},
		{"command substitution", []string{"$(date)"}, true},
		{"backtick substitution", []string{"`whoami`"}, true},
		{"mixed args", []string{"-c", "$(cmd)"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckArgsSafety(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckArgsSafety(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
