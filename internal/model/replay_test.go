package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ReplayOptions_Bounded(t *testing.T) {
	tail := 10

	tests := []struct {
		name     string
		options  ReplayOptions
		expected bool
	}{
		{
			name:     "no options",
			options:  ReplayOptions{},
			expected: false,
		},
		{
			name:     "tail only",
			options:  ReplayOptions{Tail: &tail},
			expected: true,
		},
		{
			name:     "no-follow only",
			options:  ReplayOptions{NoFollow: true},
			expected: true,
		},
		{
			name:     "tail and no-follow",
			options:  ReplayOptions{Tail: &tail, NoFollow: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.options.Bounded())
		})
	}
}

func Test_ReplayOptions_Equal(t *testing.T) {
	ten := 10
	anotherTen := 10
	twenty := 20

	tests := []struct {
		name     string
		options  ReplayOptions
		other    ReplayOptions
		expected bool
	}{
		{
			name:     "both empty",
			options:  ReplayOptions{},
			other:    ReplayOptions{},
			expected: true,
		},
		{
			name:     "same tail through different pointers",
			options:  ReplayOptions{Tail: &ten, NoFollow: true},
			other:    ReplayOptions{Tail: &anotherTen, NoFollow: true},
			expected: true,
		},
		{
			name:     "different tail",
			options:  ReplayOptions{Tail: &ten},
			other:    ReplayOptions{Tail: &twenty},
			expected: false,
		},
		{
			name:     "tail missing on one side",
			options:  ReplayOptions{Tail: &ten},
			other:    ReplayOptions{},
			expected: false,
		},
		{
			name:     "different no-follow",
			options:  ReplayOptions{NoFollow: true},
			other:    ReplayOptions{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.options.Equal(tt.other))
		})
	}
}
