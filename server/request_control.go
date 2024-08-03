// Request controller

package main

import "sync"

// Request controller
type RequestController struct {
	// Mutex for the struct
	mu *sync.Mutex

	// Map (Req type) -> Count
	counts map[string]uint32
}

// Creates instance of RequestController
func CreateRequestController() *RequestController {
	return &RequestController{
		mu:     &sync.Mutex{},
		counts: make(map[string]uint32),
	}
}

// Tries to start a request
// requestType - Request type
// limit - Max number of request for requestType
// Returns true if success, false if the limit was reached
func (rc *RequestController) TryStartRequest(requestType string, limit uint32) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	c := rc.counts[requestType]

	if c >= limit {
		return false
	}

	rc.counts[requestType] = c + 1

	return true
}

// Ends a request
// requestType - Request type
func (rc *RequestController) EndRequest(requestType string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	c := rc.counts[requestType]

	if c == 0 {
		return
	}

	if c == 1 {
		delete(rc.counts, requestType)
		return
	}

	rc.counts[requestType] = c - 1
}

// Returns the current count for a request type
func (rc *RequestController) GetRequestCount(requestType string) uint32 {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.counts[requestType]
}
