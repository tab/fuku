package services

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
)

func newTestLoader() *Loader {
	return &Loader{Model: spinner.New(), queue: make([]LoaderItem, 0)}
}

func Test_NewServiceFSM_InitialState(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped}
	fsm := newServiceFSM(service, newTestLoader())
	assert.Equal(t, Stopped, fsm.Current())
}

func Test_FSM_Start_Transition(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped}
	fsm := newServiceFSM(service, newTestLoader())

	err := fsm.Event(context.Background(), Start)

	assert.NoError(t, err)
	assert.Equal(t, Starting, fsm.Current())
	assert.Equal(t, StatusStarting, service.Status)
}

func Test_FSM_Started_Transition(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStarting}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)

	err := fsm.Event(context.Background(), Started)

	assert.NoError(t, err)
	assert.Equal(t, Running, fsm.Current())
	assert.Equal(t, StatusRunning, service.Status)
}

func Test_FSM_Stop_Transition(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Started)

	err := fsm.Event(context.Background(), Stop)

	assert.NoError(t, err)
	assert.Equal(t, Stopping, fsm.Current())
	assert.Equal(t, StatusStopping, service.Status)
}

func Test_FSM_Stopped_Transition(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning, Monitor: ServiceMonitor{PID: 1234}}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Started)
	_ = fsm.Event(context.Background(), Stop)

	err := fsm.Event(context.Background(), Stopped)

	assert.NoError(t, err)
	assert.Equal(t, Stopped, fsm.Current())
	assert.Equal(t, StatusStopped, service.Status)
	assert.Equal(t, 0, service.Monitor.PID)
}

func Test_FSM_Restart_From_Running(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Started)

	err := fsm.Event(context.Background(), Restart)

	assert.NoError(t, err)
	assert.Equal(t, Restarting, fsm.Current())
}

func Test_FSM_Restart_From_Failed(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusFailed}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Failed)

	err := fsm.Event(context.Background(), Restart)

	assert.NoError(t, err)
	assert.Equal(t, Restarting, fsm.Current())
}

func Test_FSM_Restart_From_Stopped(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped}
	fsm := newServiceFSM(service, newTestLoader())

	err := fsm.Event(context.Background(), Restart)

	assert.NoError(t, err)
	assert.Equal(t, Restarting, fsm.Current())
}

func Test_FSM_Failed_From_Starting(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStarting}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)

	err := fsm.Event(context.Background(), Failed)

	assert.NoError(t, err)
	assert.Equal(t, Failed, fsm.Current())
	assert.Equal(t, StatusFailed, service.Status)
}

func Test_FSM_Failed_From_Running(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Started)

	err := fsm.Event(context.Background(), Failed)

	assert.NoError(t, err)
	assert.Equal(t, Failed, fsm.Current())
	assert.Equal(t, StatusFailed, service.Status)
}

func Test_FSM_Failed_From_Restarting(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusRunning}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Start)
	_ = fsm.Event(context.Background(), Started)
	_ = fsm.Event(context.Background(), Restart)

	err := fsm.Event(context.Background(), Failed)

	assert.NoError(t, err)
	assert.Equal(t, Failed, fsm.Current())
	assert.Equal(t, StatusFailed, service.Status)
}

func Test_FSM_Start_From_Restarting(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped}
	fsm := newServiceFSM(service, newTestLoader())
	_ = fsm.Event(context.Background(), Restart)
	_ = fsm.Event(context.Background(), Stopped)

	err := fsm.Event(context.Background(), Start)

	assert.NoError(t, err)
	assert.Equal(t, Starting, fsm.Current())
}

func Test_FSM_Invalid_Transition(t *testing.T) {
	service := &ServiceState{Name: "api", Status: StatusStopped}
	fsm := newServiceFSM(service, newTestLoader())

	err := fsm.Event(context.Background(), Stop)

	assert.Error(t, err)
	assert.Equal(t, Stopped, fsm.Current())
}
