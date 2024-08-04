// Request controller tests

package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type AtomicCounter struct {
	mu    *sync.Mutex
	count int
}

func CreateAtomicCounter() *AtomicCounter {
	return &AtomicCounter{
		mu:    &sync.Mutex{},
		count: 0,
	}
}

func (ac *AtomicCounter) Get() int {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	return ac.count
}

func (ac *AtomicCounter) Increment() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.count += 1
}

func testStartRequest(wg *sync.WaitGroup, ac *AtomicCounter, requestController *RequestController, rType string, limit uint32) {
	defer wg.Done()

	r := requestController.TryStartRequest(rType, limit)

	if r {
		ac.Increment()
	}
}

func TestRequestController(t *testing.T) {
	requestController := CreateRequestController()
	ac := CreateAtomicCounter()
	wg := &sync.WaitGroup{}
	wg.Add(3)

	rType := "test-type"
	limit := uint32(2)

	// Start requests

	go testStartRequest(wg, ac, requestController, rType, limit)
	go testStartRequest(wg, ac, requestController, rType, limit)
	go testStartRequest(wg, ac, requestController, rType, limit)

	wg.Wait()

	assert.Equal(t, ac.Get(), 2)
	assert.Equal(t, requestController.GetRequestCount(rType), uint32(2))

	// End requests

	requestController.EndRequest(rType)
	assert.Equal(t, requestController.GetRequestCount(rType), uint32(1))

	requestController.EndRequest(rType)
	assert.Equal(t, requestController.GetRequestCount(rType), uint32(0))

	requestController.EndRequest(rType)
	assert.Equal(t, requestController.GetRequestCount(rType), uint32(0))
}
