package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewTracker(t *testing.T) {
	trackerInstance := NewTracker()
	assert.NotNil(t, trackerInstance)

	instance, ok := trackerInstance.(*tracker)
	assert.True(t, ok)
	assert.NotNil(t, instance)
	assert.NotNil(t, instance.results)
	assert.Equal(t, 0, len(instance.results))
}

func Test_Add(t *testing.T) {
	trackerInstance := NewTracker()

	firstResult := trackerInstance.Add("test-service")
	assert.NotNil(t, firstResult)

	secondResult := trackerInstance.Add("test-service")
	assert.NotNil(t, secondResult)
}

func Test_Add_MultipleServices(t *testing.T) {
	trackerInstance := NewTracker()

	result1 := trackerInstance.Add("service-1")
	result2 := trackerInstance.Add("service-2")
	result3 := trackerInstance.Add("service-3")

	assert.NotNil(t, result1)
	assert.NotNil(t, result2)
	assert.NotNil(t, result3)
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	trackerInstance := NewTracker()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			r := trackerInstance.Add("service-" + string(rune('0'+id)))
			assert.NotNil(t, r)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func Test_ServiceNaming(t *testing.T) {
	trackerInstance := NewTracker()

	tests := []string{
		"simple-service",
		"service_with_underscores",
		"service-123",
		"service.with.dots",
		"service@special$chars",
		"",
	}

	for _, name := range tests {
		r := trackerInstance.Add(name)
		assert.NotNil(t, r)
		assert.Equal(t, name, r.GetName())
	}
}
