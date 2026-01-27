package runner

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_NewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)

	instance, ok := r.(*registry)
	assert.True(t, ok)
	assert.NotNil(t, instance.processes)
	assert.NotNil(t, instance.detached)
}

func Test_Registry_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "default")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.False(t, lookup.Detached)
	assert.Equal(t, mockProc, lookup.Proc)
	assert.Equal(t, "default", lookup.Tier)
}

func Test_Registry_Get_NotFound(t *testing.T) {
	reg := NewRegistry()

	lookup := reg.Get("nonexistent")
	assert.False(t, lookup.Exists)
	assert.Equal(t, "", lookup.Tier)
}

func Test_Registry_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")

	snapshot := reg.SnapshotReverse()
	assert.Len(t, snapshot, 2)
}

func Test_Registry_Remove_FromProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "platform")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)

	result := reg.Remove("test-service", mockProc)
	assert.True(t, result.Removed)
	assert.True(t, result.UnexpectedExit)
	assert.Equal(t, "platform", result.Tier)

	lookup = reg.Get("test-service")
	assert.False(t, lookup.Exists)
}

func Test_Registry_Remove_FromDetached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "platform")
	reg.Detach("test-service")

	result := reg.Remove("test-service", mockProc)
	assert.True(t, result.Removed)
	assert.False(t, result.UnexpectedExit)
	assert.Equal(t, "platform", result.Tier)

	lookup := reg.Get("test-service")
	assert.False(t, lookup.Exists)
}

func Test_Registry_Remove_Nonexistent(t *testing.T) {
	reg := NewRegistry()

	result := reg.Remove("nonexistent", nil)
	assert.False(t, result.Removed)
	assert.False(t, result.UnexpectedExit)
}

func Test_Registry_Remove_WrongProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc1, "default")

	result := reg.Remove("test-service", mockProc2)
	assert.False(t, result.Removed)

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
}

func Test_Registry_Wait(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")

	waitDone := make(chan struct{})

	go func() {
		reg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		t.Fatal("Wait() returned before all processes completed")
	case <-time.After(10 * time.Millisecond):
	}

	reg.Remove("service1", mockProc1)
	reg.Remove("service2", mockProc2)

	select {
	case <-waitDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() did not return after all processes completed")
	}
}

func Test_Registry_ConcurrentAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reg := NewRegistry()
	numServices := 10
	procs := make([]Process, numServices)

	var addWg sync.WaitGroup

	for i := 0; i < numServices; i++ {
		mockProc := NewMockProcess(ctrl)
		procs[i] = mockProc

		serviceName := fmt.Sprintf("service-%d", i)

		addWg.Add(1)

		go func(name string, proc Process) {
			defer addWg.Done()

			reg.Add(name, proc, "default")
		}(serviceName, mockProc)
	}

	addWg.Wait()

	var accessWg sync.WaitGroup

	for i := 0; i < 5; i++ {
		accessWg.Add(2)

		go func() {
			defer accessWg.Done()

			_ = reg.Get("service-0")
		}()

		go func() {
			defer accessWg.Done()

			reg.SnapshotReverse()
		}()
	}

	accessWg.Wait()

	for i := 0; i < numServices; i++ {
		serviceName := fmt.Sprintf("service-%d", i)
		reg.Remove(serviceName, procs[i])
	}

	waitDone := make(chan struct{})

	go func() {
		reg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait() did not return after concurrent operations")
	}
}

func Test_Registry_Detach_RemovesFromMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "default")

	reg.Detach("test-service")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.True(t, lookup.Detached)
}

func Test_Registry_Detach_Nonexistent(t *testing.T) {
	reg := NewRegistry()
	reg.Detach("nonexistent")
}

func Test_Registry_Remove_ChecksPointerIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", mockProc1, "default")

	reg.Detach("test-service")
	reg.Add("test-service", mockProc2, "default")

	result := reg.Remove("test-service", mockProc1)
	assert.False(t, result.Removed, "Should not remove old process")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.Equal(t, mockProc2, lookup.Proc)
}

func Test_Registry_RestartRaceCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oldProc := NewMockProcess(ctrl)
	newProc := NewMockProcess(ctrl)

	reg := NewRegistry()
	reg.Add("test-service", oldProc, "default")

	reg.Detach("test-service")
	reg.Add("test-service", newProc, "default")

	result := reg.Remove("test-service", oldProc)
	assert.False(t, result.Removed, "Old process should not be removed from active processes")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.False(t, lookup.Detached)
	assert.Equal(t, newProc, lookup.Proc)
}

func Test_Registry_MarkRestarting(t *testing.T) {
	reg := NewRegistry()

	assert.False(t, reg.IsRestarting("test-service"))

	reg.MarkRestarting("test-service")

	assert.True(t, reg.IsRestarting("test-service"))
}

func Test_Registry_ClearRestarting(t *testing.T) {
	reg := NewRegistry()

	reg.MarkRestarting("test-service")
	assert.True(t, reg.IsRestarting("test-service"))

	reg.ClearRestarting("test-service")
	assert.False(t, reg.IsRestarting("test-service"))
}

func Test_Registry_Add_KeepsRestarting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)

	reg := NewRegistry()

	reg.MarkRestarting("test-service")
	assert.True(t, reg.IsRestarting("test-service"))

	reg.Add("test-service", mockProc, "default")
	assert.True(t, reg.IsRestarting("test-service"), "Add should not clear restarting flag")
}

func Test_Registry_RestartingIndependent(t *testing.T) {
	reg := NewRegistry()

	reg.MarkRestarting("service-a")
	reg.MarkRestarting("service-b")

	assert.True(t, reg.IsRestarting("service-a"))
	assert.True(t, reg.IsRestarting("service-b"))
	assert.False(t, reg.IsRestarting("service-c"))

	reg.ClearRestarting("service-a")

	assert.False(t, reg.IsRestarting("service-a"))
	assert.True(t, reg.IsRestarting("service-b"))
}
