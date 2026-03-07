package tracer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"fuku/internal/app/bus"
)

func Test_NewTracer(t *testing.T) {
	tr := NewTracer()

	assert.NotNil(t, tr)
}

func Test_Tracer_Run_StopsOnContextCancel(t *testing.T) {
	tr := NewTracer()

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan bus.Message)

	done := make(chan struct{})

	go func() {
		tr.Run(ctx, ch)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tracer did not stop after context cancel")
	}
}

func Test_Handle_CommandStarted_CreatesTransaction(t *testing.T) {
	tr := &tracer{}
	ctx := context.Background()

	tr.handle(ctx, bus.Message{
		Type:      bus.EventCommandStarted,
		Timestamp: time.Now(),
		Data: bus.CommandStarted{
			Command: "run",
			Profile: "default",
			UI:      true,
		},
	})

	assert.NotNil(t, tr.trace)
	assert.True(t, tr.trace.IsTransaction())
}

func Test_Handle_CommandStarted_InvalidData(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type: bus.EventCommandStarted,
		Data: "invalid",
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_CommandStarted_IgnoresNonRunCommand(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventCommandStarted,
		Timestamp: time.Now(),
		Data: bus.CommandStarted{
			Command: "help",
		},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_ProfileResolved_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventProfileResolved,
		Timestamp: time.Now(),
		Data: bus.ProfileResolved{
			Profile: "default",
			Tiers: []bus.Tier{
				{Name: "foundation", Services: []string{"db", "cache"}},
				{Name: "app", Services: []string{"api"}},
			},
			Duration: 50 * time.Millisecond,
		},
	})

	assert.NotNil(t, tr.trace)
	assert.Len(t, tr.tiers, 2)
}

func Test_Handle_ProfileResolved_InvalidData(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type: bus.EventProfileResolved,
		Data: "invalid",
	})

	assert.NotNil(t, tr.trace)
	assert.Nil(t, tr.tiers)
}

func Test_Handle_ProfileResolved_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventProfileResolved,
		Timestamp: time.Now(),
		Data: bus.ProfileResolved{
			Profile:  "default",
			Duration: 50 * time.Millisecond,
		},
	})

	assert.Nil(t, tr.trace)
	assert.Nil(t, tr.tiers)
}

func Test_Handle_PreflightComplete_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPreflightComplete,
		Timestamp: time.Now(),
		Data:      bus.PreflightComplete{Killed: 1, Duration: 30 * time.Millisecond},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_PreflightComplete_InvalidData(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type: bus.EventPreflightComplete,
		Data: "invalid",
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_PreflightComplete_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPreflightComplete,
		Timestamp: time.Now(),
		Data:      bus.PreflightComplete{Killed: 1, Duration: 30 * time.Millisecond},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_TierReady_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)
	tr.tiers = []bus.Tier{
		{Name: "foundation", Services: []string{"db", "cache"}},
		{Name: "app", Services: []string{"api"}},
	}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventTierReady,
		Timestamp: time.Now(),
		Data: bus.TierReady{
			Name:         "foundation",
			Duration:     100 * time.Millisecond,
			ServiceCount: 2,
		},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_TierReady_InvalidData(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type: bus.EventTierReady,
		Data: "invalid",
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_TierReady_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventTierReady,
		Timestamp: time.Now(),
		Data: bus.TierReady{
			Name:         "foundation",
			Duration:     100 * time.Millisecond,
			ServiceCount: 2,
		},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_WatchTriggered_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventWatchTriggered,
		Timestamp: time.Now(),
		Data:      bus.WatchTriggered{Service: "api", ChangedFiles: []string{"main.go"}},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_WatchTriggered_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventWatchTriggered,
		Timestamp: time.Now(),
		Data:      bus.WatchTriggered{Service: "api"},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_ServiceStop_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.CommandStopService,
		Timestamp: time.Now(),
		Data:      bus.Payload{Name: "api"},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_ServiceStop_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.CommandStopService,
		Timestamp: time.Now(),
		Data:      bus.Payload{Name: "api"},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_ServiceRestart_CreatesSpan(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.CommandRestartService,
		Timestamp: time.Now(),
		Data:      bus.Payload{Name: "api"},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_ServiceRestart_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.CommandRestartService,
		Timestamp: time.Now(),
		Data:      bus.Payload{Name: "api"},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_PhaseChanged_Shutdown(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPhaseChanged,
		Timestamp: time.Now(),
		Data: bus.PhaseChanged{
			Phase:        bus.PhaseStopped,
			Duration:     200 * time.Millisecond,
			ServiceCount: 3,
		},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_PhaseChanged_NoDuration(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPhaseChanged,
		Timestamp: time.Now(),
		Data:      bus.PhaseChanged{Phase: bus.PhaseStopped},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_PhaseChanged_IgnoresNonStopped(t *testing.T) {
	tr := &tracer{}
	startTransaction(tr)

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPhaseChanged,
		Timestamp: time.Now(),
		Data:      bus.PhaseChanged{Phase: bus.PhaseRunning, Duration: time.Second},
	})

	assert.NotNil(t, tr.trace)
}

func Test_Handle_PhaseChanged_NoTransaction(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventPhaseChanged,
		Timestamp: time.Now(),
		Data:      bus.PhaseChanged{Phase: bus.PhaseStopped},
	})

	assert.Nil(t, tr.trace)
}

func Test_Handle_UnhandledEvent(t *testing.T) {
	tr := &tracer{}

	tr.handle(context.Background(), bus.Message{
		Type: bus.EventSignal,
		Data: bus.Signal{Name: "SIGTERM"},
	})

	assert.Nil(t, tr.trace)
}

func Test_Finish_NilTransaction(t *testing.T) {
	tr := &tracer{}

	tr.finish(0)

	assert.Nil(t, tr.trace)
}

func Test_TierPosition(t *testing.T) {
	tr := &tracer{
		tiers: []bus.Tier{
			{Name: "foundation", Services: []string{"db"}},
			{Name: "app", Services: []string{"api"}},
			{Name: "gateway", Services: []string{"nginx"}},
		},
	}

	index, total := tr.tierPosition("app")
	assert.Equal(t, 2, index)
	assert.Equal(t, 3, total)
}

func Test_TierPosition_Unknown(t *testing.T) {
	tr := &tracer{
		tiers: []bus.Tier{
			{Name: "foundation", Services: []string{"db"}},
		},
	}

	index, total := tr.tierPosition("unknown")
	assert.Equal(t, 0, index)
	assert.Equal(t, 1, total)
}

func Test_TierPosition_Empty(t *testing.T) {
	tr := &tracer{}

	index, total := tr.tierPosition("any")
	assert.Equal(t, 0, index)
	assert.Equal(t, 0, total)
}

func Test_Tracer_Run_HandlesMessagesAndChannelClose(t *testing.T) {
	tr := NewTracer()

	ch := make(chan bus.Message, 10)

	ch <- bus.Message{
		Type:      bus.EventCommandStarted,
		Timestamp: time.Now(),
		Data:      bus.CommandStarted{Command: "run", Profile: "default"},
	}

	ch <- bus.Message{
		Type:      bus.EventProfileResolved,
		Timestamp: time.Now(),
		Data: bus.ProfileResolved{
			Profile:  "default",
			Tiers:    []bus.Tier{{Name: "default", Services: []string{"api"}}},
			Duration: 10 * time.Millisecond,
		},
	}

	close(ch)

	done := make(chan struct{})

	go func() {
		tr.Run(context.Background(), ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tracer did not stop after channel close")
	}
}

func startTransaction(tr *tracer) {
	tr.handle(context.Background(), bus.Message{
		Type:      bus.EventCommandStarted,
		Timestamp: time.Now(),
		Data:      bus.CommandStarted{Command: "run"},
	})
}
