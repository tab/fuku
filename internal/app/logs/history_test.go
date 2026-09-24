package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/model"
)

// everything follows every service
func everything(string) bool {
	return true
}

// onlyAPI follows the api service
func onlyAPI(service string) bool {
	return service == "api"
}

func Test_history_replay(t *testing.T) {
	one, two, ten := 1, 2, 10

	tests := []struct {
		name     string
		capacity int
		lines    []model.LogLine
		wants    func(string) bool
		tail     *int
		expected []string
	}{
		{
			name:     "empty history replays nothing",
			capacity: 5,
			wants:    everything,
			expected: []string{},
		},
		{
			name:     "partial fill keeps the push order",
			capacity: 5,
			lines:    []model.LogLine{{Service: "api", Message: "1"}, {Service: "web", Message: "2"}},
			wants:    everything,
			expected: []string{"1", "2"},
		},
		{
			name:     "wraparound drops the oldest",
			capacity: 3,
			lines:    []model.LogLine{{Message: "1"}, {Message: "2"}, {Message: "3"}, {Message: "4"}},
			wants:    everything,
			expected: []string{"2", "3", "4"},
		},
		{
			name:     "filter selects the wanted services",
			capacity: 5,
			lines:    []model.LogLine{{Service: "api", Message: "1"}, {Service: "web", Message: "2"}, {Service: "api", Message: "3"}},
			wants:    onlyAPI,
			expected: []string{"1", "3"},
		},
		{
			name:     "tail keeps the newest",
			capacity: 5,
			lines:    []model.LogLine{{Message: "1"}, {Message: "2"}, {Message: "3"}},
			wants:    everything,
			tail:     &two,
			expected: []string{"2", "3"},
		},
		{
			name:     "tail of one keeps only the newest",
			capacity: 5,
			lines:    []model.LogLine{{Message: "1"}, {Message: "2"}, {Message: "3"}},
			wants:    everything,
			tail:     &one,
			expected: []string{"3"},
		},
		{
			name:     "tail larger than the history returns everything",
			capacity: 5,
			lines:    []model.LogLine{{Message: "1"}, {Message: "2"}},
			wants:    everything,
			tail:     &ten,
			expected: []string{"1", "2"},
		},
		{
			name:     "filter applies before the tail",
			capacity: 5,
			lines:    []model.LogLine{{Service: "api", Message: "1"}, {Service: "web", Message: "2"}, {Service: "api", Message: "3"}, {Service: "web", Message: "4"}, {Service: "api", Message: "5"}},
			wants:    onlyAPI,
			tail:     &two,
			expected: []string{"3", "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHistory(tt.capacity)
			for _, line := range tt.lines {
				h.push(line)
			}

			replayed := h.replay(tt.wants, tt.tail)

			messages := make([]string, 0, len(replayed))
			for _, line := range replayed {
				messages = append(messages, line.Message)
			}

			assert.Equal(t, tt.expected, messages)
		})
	}
}
