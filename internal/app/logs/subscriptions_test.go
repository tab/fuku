package logs

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"fuku/internal/model"
)

// Message bodies shared by the replay tests
const (
	apiFirst  = "api-1"
	apiSecond = "api-2"
	apiThird  = "api-3"
	webFirst  = "web-1"
	webSecond = "web-2"
)

// drain collects the queued messages, reporting whether the queue is closed
func drain(sub *Subscription) ([]string, bool) {
	messages := make([]string, 0)

	for {
		select {
		case line, ok := <-sub.Lines():
			if !ok {
				return messages, true
			}

			messages = append(messages, line.Message)
		default:
			return messages, false
		}
	}
}

func Test_NewHub(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	h := NewHub(Options{Buffer: 10, History: 50}, log)

	assert.Equal(t, 60, h.queue)
	assert.Len(t, h.history.lines, 50)
	assert.Empty(t, h.subs)
}

func Test_Hub_Subscribe(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	one, two := 1, 2

	seed := []model.LogLine{
		{Service: "api", Message: apiFirst},
		{Service: "web", Message: webFirst},
		{Service: "api", Message: apiSecond},
		{Service: "web", Message: webSecond},
		{Service: "api", Message: apiThird},
	}

	tests := []struct {
		name     string
		services []string
		replay   model.ReplayOptions
		live     []model.LogLine
		expected []string
		closed   bool
	}{
		{
			name:     "no filter replays everything and follows",
			live:     []model.LogLine{{Service: "db", Message: "db-1"}},
			expected: []string{apiFirst, webFirst, apiSecond, webSecond, apiThird, "db-1"},
		},
		{
			name:     "filter selects the replay and the live lines",
			services: []string{"api"},
			live:     []model.LogLine{{Service: "web", Message: "web-3"}, {Service: "api", Message: "api-4"}},
			expected: []string{apiFirst, apiSecond, apiThird, "api-4"},
		},
		{
			name:     "tail keeps the newest replay lines before the live ones",
			replay:   model.ReplayOptions{Tail: &two},
			live:     []model.LogLine{{Service: "api", Message: "api-4"}},
			expected: []string{webSecond, apiThird, "api-4"},
		},
		{
			name:     "no follow replays and closes",
			services: []string{"api"},
			replay:   model.ReplayOptions{Tail: &one, NoFollow: true},
			live:     []model.LogLine{{Service: "api", Message: "api-4"}},
			expected: []string{apiThird},
			closed:   true,
		},
		{
			name:     "no follow with nothing matching closes empty",
			services: []string{"db"},
			replay:   model.ReplayOptions{NoFollow: true},
			live:     []model.LogLine{{Service: "db", Message: "db-1"}},
			expected: []string{},
			closed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHub(Options{Buffer: 10, History: 50}, log)
			for _, line := range seed {
				h.Broadcast(line.Service, line.Message)
			}

			sub := h.Subscribe(tt.services, tt.replay)

			for _, line := range tt.live {
				h.Broadcast(line.Service, line.Message)
			}

			messages, closed := drain(sub)

			assert.Equal(t, tt.expected, messages)
			assert.Equal(t, tt.closed, closed)
		})
	}
}

func Test_Hub_Unsubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLog := NewMockLogger(ctrl)

	tests := []struct {
		name      string
		before    func(h *Hub, sub *Subscription)
		broadcast int
		queued    int
	}{
		{
			name:      "closes the queue",
			before:    func(*Hub, *Subscription) {},
			broadcast: 1,
			queued:    1,
		},
		{
			name: "reports the lines the full queue dropped",
			before: func(*Hub, *Subscription) {
				mockLog.EXPECT().Warn("Dropped 3 log messages (buffer full)")
			},
			broadcast: 5,
			queued:    2,
		},
		{
			name: "ignores a subscription that already ended",
			before: func(h *Hub, sub *Subscription) {
				h.Unsubscribe(sub)
			},
			broadcast: 1,
			queued:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHub(Options{Buffer: 1, History: 1}, mockLog)
			sub := h.Subscribe(nil, model.ReplayOptions{})

			for i := range tt.broadcast {
				h.Broadcast("api", fmt.Sprintf("msg-%d", i))
			}

			tt.before(h, sub)

			h.Unsubscribe(sub)

			messages, closed := drain(sub)

			assert.True(t, closed)
			assert.Len(t, messages, tt.queued)
			assert.Empty(t, h.subs)
		})
	}
}

func Test_Hub_Subscribe_LiveLinesFollowTheReplayInOrder(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	const (
		producers = 4
		perWorker = 250
	)

	h := NewHub(Options{Buffer: producers * perWorker, History: 100}, log)

	var (
		seq   int
		seqMu sync.Mutex
		wg    sync.WaitGroup
	)

	publish := func() {
		seqMu.Lock()
		defer seqMu.Unlock()

		seq++

		h.Broadcast("api", strconv.Itoa(seq))
	}

	for range producers {
		wg.Go(func() {
			for range perWorker {
				publish()
			}
		})
	}

	sub := h.Subscribe(nil, model.ReplayOptions{})

	wg.Wait()
	h.Unsubscribe(sub)

	messages, closed := drain(sub)

	require.True(t, closed)
	require.NotEmpty(t, messages)

	first, err := strconv.Atoi(messages[0])
	require.NoError(t, err)

	for i, message := range messages {
		assert.Equal(t, strconv.Itoa(first+i), message)
	}

	assert.Equal(t, strconv.Itoa(producers*perWorker), messages[len(messages)-1])
}
