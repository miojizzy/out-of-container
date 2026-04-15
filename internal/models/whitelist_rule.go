package models

import "regexp"

// WhitelistRule defines the interface for whitelist matching
type WhitelistRule interface {
	// Match checks if the command matches this rule
	Match(command string) (bool, error)
	// Type returns the rule type ("literal" or "regex")
	Type() string
}

// LiteralRule matches commands by exact string comparison
type LiteralRule struct {
	command string
}

// NewLiteralRule creates a new literal matching rule
func NewLiteralRule(command string) *LiteralRule {
	return &LiteralRule{command: command}
}

// Match checks if the command exactly matches the rule's command
func (r *LiteralRule) Match(command string) (bool, error) {
	return command == r.command, nil
}

// Type returns the rule type as "literal"
func (r *LiteralRule) Type() string {
	return "literal"
}

// RegexRule matches commands using regular expression
type RegexRule struct {
	pattern *regexp.Regexp
	regex   string
}

// NewRegexRule creates a new regex matching rule
func NewRegexRule(regex string) (*RegexRule, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return &RegexRule{pattern: re, regex: regex}, nil
}

func (r *RegexRule) Match(command string) (bool, error) {
	return r.pattern.MatchString(command), nil
}

func (r *RegexRule) Type() string {
	return "regex"
}
