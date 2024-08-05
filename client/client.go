// Parallel request controller client

package prc_client

import (
	"errors"
	"sync"
	"time"
)

// Listener for request start ack
type RequestStartAckListener struct {
	// Channel to receive the response
	channel chan bool
}

// Listener for request count
type RequestCountListener struct {
	// Channel to receive the request count
	channel chan uint32
}

// Client for the parallel request controller
type Client struct {
	// Mutex for the struct
	mu *sync.Mutex

	// Configuration
	config *ClientConfig

	// Connections
	connections []*Connection

	// Index to balance the use of the connections
	connectionBalancer int

	// ID for the next request
	nextRequestId uint64

	// Expecting ACKs
	expectingRequestAck map[uint64]*RequestStartAckListener

	// Expecting request counts
	expectingRequestCount map[string]([]*RequestCountListener)
}

// Creates client
func NewClient(config *ClientConfig) *Client {
	connectionsCount := 1

	if config.NumberOfConnections > 1 {
		connectionsCount = config.NumberOfConnections
	}

	connections := make([]*Connection, connectionsCount)

	cli := &Client{
		mu:                    &sync.Mutex{},
		config:                config,
		connections:           connections,
		connectionBalancer:    0,
		nextRequestId:         0,
		expectingRequestAck:   make(map[uint64]*RequestStartAckListener),
		expectingRequestCount: make(map[string][]*RequestCountListener),
	}

	for i := 0; i < len(cli.connections); i++ {
		cli.connections[i] = NewConnection(cli, config)
	}

	return cli
}

// Connects the client
func (cli *Client) Connect() {
	for _, conn := range cli.connections {
		conn.Connect()
	}
}

// Closes all the connections
func (cli *Client) Close() {
	for _, conn := range cli.connections {
		conn.Close()
	}
}

// Gets a connection from the pool
func (cli *Client) getConnectionFromPool() *Connection {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	conn := cli.connections[cli.connectionBalancer]

	cli.connectionBalancer++

	if cli.connectionBalancer >= len(cli.connections) {
		cli.connectionBalancer = 0
	}

	return conn
}

// Gets new unique request ID for this client
func (cli *Client) getNewRequestId() uint64 {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	id := cli.nextRequestId

	cli.nextRequestId++

	return id
}

// Removes an ACK listener
func (cli *Client) removeAckListener(id uint64) {
	cli.mu.Lock()
	defer cli.mu.Unlock()

	delete(cli.expectingRequestAck, id)
}

// Receives request ACK from a connection
func (cli *Client) receiveRequestAck(id uint64, limited bool) {
	var listener *RequestStartAckListener = nil

	cli.mu.Lock()

	listener = cli.expectingRequestAck[id]

	cli.mu.Unlock()

	if listener != nil {
		listener.channel <- limited
	}
}

// Indicates the start of a request
// Parameters:
// - requestType - String to indicate the request type
// - limit - Máximum number of requests allowed to be run in parallel
// Returns:
// - req - Reference to the started request. Keep it to indicate the ending. May be nil in case of error or if the request type reached the limit
// - limited - True if the limit was reached, so the request should be rejected
// - err - An error that prevented the request start indication from completing
func (cli *Client) StartRequest(requestType string, limit uint32) (req *StartedRequest, limited bool, err error) {
	if limit < 1 {
		return nil, true, nil
	}

	if requestType == "" {
		return nil, false, errors.New("invalid request type")
	}

	// Create an ID for the request, and get a connection to the PRC

	id := cli.getNewRequestId()
	conn := cli.getConnectionFromPool()

	// Setup listener for the ACK

	listener := &RequestStartAckListener{
		channel: make(chan bool),
	}

	cli.mu.Lock()

	cli.expectingRequestAck[id] = listener

	cli.mu.Unlock()

	defer cli.removeAckListener(id)

	// Send the start message

	conn.StartRequest(id, requestType, limit)

	// Wait

	timeout := DEFAULT_TIMEOUT

	if cli.config.Timeout > 0 {
		timeout = cli.config.Timeout
	}

	select {
	case limited := <-listener.channel:
		if limited {
			return nil, true, nil
		} else {
			return &StartedRequest{
				id:         id,
				connection: conn,
			}, false, nil
		}
	case <-time.After(timeout):
		conn.EndRequest(id)
		return nil, false, errors.New("timeout")
	}
}

// Receives a request count, calling the listeners
func (cli *Client) receiveRequestCount(rType string, count uint32) {
	var listeners []*RequestCountListener = nil

	cli.mu.Lock()

	listeners = cli.expectingRequestCount[rType]

	delete(cli.expectingRequestCount, rType)

	cli.mu.Unlock()

	if listeners == nil {
		return
	}

	for _, lis := range listeners {
		lis.channel <- count
	}
}

// Clears request count listener on timeout
func (cli *Client) clearRequestCountListener(rType string, listener *RequestCountListener) {
	cli.mu.Lock()

	listeners := cli.expectingRequestCount[rType]

	if listeners != nil {
		newListeners := make([]*RequestCountListener, 0)

		for _, lis := range listeners {
			if lis != listener {
				newListeners = append(newListeners, lis)
			}
		}

		if len(newListeners) > 0 {
			cli.expectingRequestCount[rType] = newListeners
		} else {
			delete(cli.expectingRequestCount, rType)
		}
	}

	cli.mu.Unlock()
}

// Gets the current number of parallel requests of a type
// Parameters:
// - requestType - String to indicate the request type
// Returns:
// - count - Current number of parallel requests of the specified type
// - err - An error that prevented the request count from completing
func (cli *Client) GetRequestCount(requestType string) (count uint32, err error) {
	if requestType == "" {
		return 0, errors.New("invalid request type")
	}

	// Get a connection

	conn := cli.getConnectionFromPool()

	// Setup listener for the ACK

	listener := &RequestCountListener{
		channel: make(chan uint32),
	}

	cli.mu.Lock()

	a := cli.expectingRequestCount[requestType]

	if a == nil {
		cli.expectingRequestCount[requestType] = []*RequestCountListener{listener}
	} else {
		cli.expectingRequestCount[requestType] = append(a, listener)
	}

	cli.mu.Unlock()

	// Send message

	conn.GetRequestCount(requestType)
	defer conn.RequestCountDone(requestType)

	// Wait

	timeout := DEFAULT_TIMEOUT

	if cli.config.Timeout > 0 {
		timeout = cli.config.Timeout
	}

	select {
	case count := <-listener.channel:
		return count, nil
	case <-time.After(timeout):
		cli.clearRequestCountListener(requestType, listener)
		return 0, errors.New("timeout")
	}
}
