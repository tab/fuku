package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fuku/internal/adapters/cli"
	"fuku/internal/adapters/eventlog"
	"fuku/internal/bootstrap/lifecycle"
)

func Test_newStopParticipants(t *testing.T) {
	observers := observerParams{Recorder: &eventlog.Recorder{}, Announcer: &cli.Announcer{}}
	stop := &cli.Stop{}

	participants := newStopParticipants(observers, stop)

	assert.Nil(t, participants.Guard)
	assert.Equal(t, []lifecycle.Consumer{observers.Recorder}, participants.Consumers)
	assert.Equal(t, []lifecycle.Producer{observers.Announcer}, participants.Producers)
	assert.Equal(t, stop, participants.Command)
}
