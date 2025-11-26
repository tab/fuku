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
	doneChan := make(chan struct{})

	mockProc.EXPECT().Done().Return(doneChan).AnyTimes()

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "default")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.False(t, lookup.Detached)
	assert.Equal(t, mockProc, lookup.Proc)

	close(doneChan)

	assert.Eventually(t, func() bool {
		return !reg.Get("test-service").Exists
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Registry_Get_NotFound(t *testing.T) {
	reg := NewRegistry()

	lookup := reg.Get("nonexistent")
	assert.False(t, lookup.Exists)
}

func Test_Registry_Snapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)
	doneChan1 := make(chan struct{})
	doneChan2 := make(chan struct{})

	mockProc1.EXPECT().Done().Return(doneChan1).AnyTimes()
	mockProc2.EXPECT().Done().Return(doneChan2).AnyTimes()

	reg := NewRegistry()
	reg.Add("service1", mockProc1, "default")
	reg.Add("service2", mockProc2, "default")

	snapshot := reg.SnapshotReverse()
	assert.Len(t, snapshot, 2)

	close(doneChan1)
	close(doneChan2)
}

func Test_Registry_Remove(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc := NewMockProcess(ctrl)
	doneChan := make(chan struct{})

	mockProc.EXPECT().Done().Return(doneChan).AnyTimes()

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "default")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)

	close(doneChan)

	assert.Eventually(t, func() bool {
		return !reg.Get("test-service").Exists
	}, 100*time.Millisecond, 5*time.Millisecond)
}

func Test_Registry_Remove_Nonexistent(t *testing.T) {
	reg := NewRegistry()
	reg.Detach("nonexistent")
}

func Test_Registry_Wait(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	mockProc2 := NewMockProcess(ctrl)
	doneChan1 := make(chan struct{})
	doneChan2 := make(chan struct{})

	mockProc1.EXPECT().Done().Return(doneChan1).AnyTimes()
	mockProc2.EXPECT().Done().Return(doneChan2).AnyTimes()

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

	close(doneChan1)
	close(doneChan2)

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
	doneChans := make([]chan struct{}, numServices)

	var addWg sync.WaitGroup

	for i := 0; i < numServices; i++ {
		mockProc := NewMockProcess(ctrl)
		doneChan := make(chan struct{})
		doneChans[i] = doneChan

		mockProc.EXPECT().Done().Return(doneChan).AnyTimes()

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

	for _, ch := range doneChans {
		close(ch)
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
	doneChan := make(chan struct{})
	mockProc.EXPECT().Done().Return(doneChan).AnyTimes()

	reg := NewRegistry()
	reg.Add("test-service", mockProc, "default")

	reg.Detach("test-service")

	lookup := reg.Get("test-service")
	assert.True(t, lookup.Exists)
	assert.True(t, lookup.Detached)

	close(doneChan)
}

func Test_Registry_RemoveAndDone_ChecksPointerIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProc1 := NewMockProcess(ctrl)
	doneChan1 := make(chan struct{})
	mockProc1.EXPECT().Done().Return(doneChan1).AnyTimes()

	mockProc2 := NewMockProcess(ctrl)
	doneChan2 := make(chan struct{})
	mockProc2.EXPECT().Done().Return(doneChan2).AnyTimes()

	reg := NewRegistry()
	reg.Add("test-service", mockProc1, "default")

	reg.Detach("test-service")
	reg.Add("test-service", mockProc2, "default")

	close(doneChan1)

	assert.Eventually(t, func() bool {
		lookup := reg.Get("test-service")

		return lookup.Exists && !lookup.Detached && lookup.Proc == mockProc2
	}, 100*time.Millisecond, 5*time.Millisecond, "New process should still be in registry after old process exits")

	close(doneChan2)
}

func Test_Registry_RestartRaceCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	oldProc := NewMockProcess(ctrl)
	oldDone := make(chan struct{})
	oldProc.EXPECT().Done().Return(oldDone).AnyTimes()

	newProc := NewMockProcess(ctrl)
	newDone := make(chan struct{})
	newProc.EXPECT().Done().Return(newDone).AnyTimes()

	reg := NewRegistry()
	reg.Add("test-service", oldProc, "default")

	reg.Detach("test-service")

	reg.Add("test-service", newProc, "default")

	close(oldDone)

	assert.Eventually(t, func() bool {
		lookup := reg.Get("test-service")

		return lookup.Exists && !lookup.Detached && lookup.Proc == newProc
	}, 100*time.Millisecond, 5*time.Millisecond, "New process should still be in registry after old process exits")

	close(newDone)
}
