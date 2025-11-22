package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HighlightLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"error with word boundary", "error occurred"},
		{"ERROR uppercase with word boundary", "ERROR occurred"},
		{"fatal with word boundary", "fatal error"},
		{"err abbreviation", "err: connection failed"},
		{"warning with word boundary", "warning: deprecated"},
		{"warn abbreviation", "warn: check this"},
		{"info with word boundary", "info message"},
		{"inf abbreviation", "inf: starting"},
		{"debug with word boundary", "debug enabled"},
		{"error in brackets", "[ERROR] failed"},
		{"error with optional brackets", "ERROR failed"},
		{"warning in brackets", "[WARN] deprecated"},
		{"info in brackets", "[INFO] started"},
		{"debug in brackets", "[DEBUG] trace"},
		{"level=error format", "level=error msg=failed"},
		{"level=fatal format", "level=fatal msg=crash"},
		{"level=warning format", "level=warning msg=deprecated"},
		{"level=warn format", "level=warn msg=check"},
		{"level=info format", "level=info msg=started"},
		{"level=debug format", "level=debug msg=trace"},
		{"mixed case error", "Error in processing"},
		{"mixed case warning", "Warning: deprecated API"},
		{"mixed case info", "Info: service started"},
		{"UUID format", "request-id: 550e8400-e29b-41d4-a716-446655440000"},
		{"multiple UUIDs", "id1: 550e8400-e29b-41d4-a716-446655440000 id2: 123e4567-e89b-12d3-a456-426614174000"},
		{"no keywords", "plain log message"},
		{"no UUID", "plain message without uuid"},
		{"error and UUID", "ERROR request 550e8400-e29b-41d4-a716-446655440000 failed"},
		{"multiple levels", "ERROR: info level=debug msg=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightLogLevel(tt.input)
			assert.NotEmpty(t, result)
			assert.True(t, len(result) >= len(tt.input), "highlighted output should not be shorter than input")
		})
	}
}

func Test_HighlightLogLevel_PreservesUppercase(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain string
	}{
		{"lowercase error becomes uppercase", "error occurred", "ERROR"},
		{"lowercase warning becomes uppercase", "warning message", "WARNING"},
		{"lowercase info becomes uppercase", "info message", "INFO"},
		{"lowercase debug becomes uppercase", "debug trace", "DEBUG"},
		{"mixed case Error becomes uppercase", "Error occurred", "ERROR"},
		{"level=error value becomes uppercase", "level=error msg=test", "level=ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightLogLevel(tt.input)
			assert.Contains(t, result, tt.shouldContain)
		})
	}
}

func Test_HighlightLogLevel_NoChange(t *testing.T) {
	h := newHighlighter()

	tests := []struct {
		name  string
		input string
	}{
		{"no keywords", "plain log message"},
		{"no UUID", "message without identifiers"},
		{"numbers only", "123 456 789"},
		{"partial UUID", "550e8400-e29b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.highlight(tt.input)
			plainInput := stripANSI(tt.input)
			plainResult := stripANSI(result)
			assert.Equal(t, plainInput, plainResult, "content without ANSI should match when no highlighting applied")
		})
	}
}

func Test_Highlighter_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"error occurred", "ERROR"},
		{"ERROR occurred", "ERROR"},
		{"Error occurred", "ERROR"},
		{"eRRoR occurred", "ERROR"},
		{"warning message", "WARNING"},
		{"WARNING message", "WARNING"},
		{"WaRnInG message", "WARNING"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := highlightLogLevel(tt.input)
			assert.Contains(t, result, tt.want)
		})
	}
}

func stripANSI(s string) string {
	result := make([]rune, 0, len(s))
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}

		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}

			continue
		}

		result = append(result, r)
	}

	return string(result)
}
