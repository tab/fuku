package registry

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"fuku/internal/app/process"
)

func Test_New(t *testing.T) {
	r := New()
	assert.NotNil(t, r)

	instance, ok := r.(*registry)
	assert.True(t, ok)
	assert.NotNil(t, instance.processes)
	assert.NotNil(t, instance.detached)
}

func Test_Registry_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("test-service", mockProc, "default")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.False(t, lookup.Detached)
	assert.Equal(t, mockProc, lookup.Proc)
	assert.Equal(t, "default", lookup.Tier)
}

func Test_Registry_Get_NotFound(t *testing.T) {
	reg := New()

	lookup := reg.Get("nonexistent")
	assert.False(t, lookup.Exists)
	assert.Equal(t, "", lookup.Tier)
}

func Test_Registry_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")

	snapshot := reg.SnapshotReverse()
	assert.Len(t, snapshot, 2)
}

func Test_Registry_Remove_FromProcesses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := process.NewMockProcess(ctrl)

	reg := New()
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

	mockProc := process.NewMockProcess(ctrl)

	reg := New()
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
	reg := New()

	result := reg.Remove("nonexistent", nil)
	assert.False(t, result.Removed)
	assert.False(t, result.UnexpectedExit)
}

func Test_Registry_Remove_WrongProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("test-service", mockProc1, "default")

	result := reg.Remove("test-service", mockProc2)
	assert.False(t, result.Removed)

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
}

func Test_Registry_Wait(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)

	reg := New()
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

	reg := New()
	numServices := 10
	procs := make([]process.Process, numServices)

	var addWg sync.WaitGroup

	for i := 0; i < numServices; i++ {
		mockProc := process.NewMockProcess(ctrl)
		procs[i] = mockProc

		serviceName := fmt.Sprintf("service-%d", i)

		addWg.Add(1)

		go func(name string, proc process.Process) {
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

	mockProc := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("test-service", mockProc, "default")

	reg.Detach("test-service")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.True(t, lookup.Detached)
}

func Test_Registry_Detach_Nonexistent(t *testing.T) {
	reg := New()
	reg.Detach("nonexistent")
}

func Test_Registry_Remove_ChecksPointerIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)

	reg := New()
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

	oldProc := process.NewMockProcess(ctrl)
	newProc := process.NewMockProcess(ctrl)

	reg := New()
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

func Test_Registry_SnapshotReverse_IncludesDetached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")
	reg.Detach("service1")

	snapshot := reg.SnapshotReverse()
	assert.Len(t, snapshot, 2)
	assert.Equal(t, mockProc2, snapshot[0])
	assert.Equal(t, mockProc1, snapshot[1])
}

func Test_Registry_SnapshotReverse_ReturnsReverseOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := process.NewMockProcess(ctrl)
	mockProc2 := process.NewMockProcess(ctrl)
	mockProc3 := process.NewMockProcess(ctrl)

	reg := New()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")
	reg.Add("service3", mockProc3, "default")

	snapshot := reg.SnapshotReverse()
	assert.Len(t, snapshot, 3)
	assert.Equal(t, mockProc3, snapshot[0])
	assert.Equal(t, mockProc2, snapshot[1])
	assert.Equal(t, mockProc1, snapshot[2])
}
