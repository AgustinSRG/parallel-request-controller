// Connection

package prc_client

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	simple_rpc_message "github.com/AgustinSRG/go-simple-rpc-message"
	"github.com/gorilla/websocket"
)

// Period to send HEARTBEAT messages to the client
const HEARTBEAT_MSG_PERIOD_SECONDS = 30

// Pending request
type PendingRequest struct {
	// Request type
	requestType string

	// Parallel request limit
	limit uint32
}

// Connection to a PRC server
type Connection struct {
	// Client
	cli *Client

	// Configuration
	config *ClientConfig

	// Mutex for the struct
	mu *sync.Mutex

	// True if the client is connected
	connected bool

	// Socket
	socket *websocket.Conn

	// Wait group to prevent multiple connections
	closeWaitGroup *sync.WaitGroup

	// Pending request to send on connection
	pendingRequests map[uint64]*PendingRequest

	// Pending request counts
	pendingRequestCounts map[string]int
}

func NewConnection(cli *Client, config *ClientConfig) *Connection {
	return &Connection{
		cli:                  cli,
		config:               config,
		mu:                   &sync.Mutex{},
		connected:            false,
		socket:               nil,
		closeWaitGroup:       nil,
		pendingRequests:      make(map[uint64]*PendingRequest),
		pendingRequestCounts: make(map[string]int),
	}
}

// Opens the connection
func (conn *Connection) Connect() {
	conn.mu.Lock()

	if conn.connected {
		return
	}

	conn.connected = true

	conn.mu.Unlock()

	go conn.runConnectionLoop()
}

// Closes the connection
func (conn *Connection) Close() {
	conn.mu.Lock()

	// Clear

	conn.pendingRequests = make(map[uint64]*PendingRequest)

	conn.mu.Unlock()

	// Close

	wg := conn.closeInternal()

	if wg != nil {
		wg.Wait()
	}
}

// Closes (internal)
func (conn *Connection) closeInternal() *sync.WaitGroup {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.connected {
		return conn.closeWaitGroup
	}

	conn.connected = false
	conn.closeWaitGroup = &sync.WaitGroup{}
	conn.closeWaitGroup.Add(1)

	if conn.socket != nil {
		conn.socket.Close()
	}

	return conn.closeWaitGroup
}

// Checks if the client is closed
func (conn *Connection) IsClosed() bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	return !conn.connected
}

// Call after the connection closes
func (conn *Connection) afterClose() {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.closeWaitGroup != nil {
		conn.closeWaitGroup.Done()
		conn.closeWaitGroup = nil
	}

	conn.connected = false
	conn.socket = nil
}

// Call when connected
// Returns false if the connection must be closed
func (conn *Connection) onConnected(socket *websocket.Conn) bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.socket = socket

	// Send pending requests

	for id, req := range conn.pendingRequests {
		msg := simple_rpc_message.RPCMessage{
			Method: "START-REQUEST",
			Params: map[string]string{
				"Request-ID":    fmt.Sprint(id),
				"Request-Type":  req.requestType,
				"Request-Limit": fmt.Sprint(req.limit),
			},
			Body: "",
		}

		conn.socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	}

	// Send pending request counts

	for rType := range conn.pendingRequestCounts {
		msg := simple_rpc_message.RPCMessage{
			Method: "GET-REQUEST-COUNT",
			Params: map[string]string{
				"Request-Type": rType,
			},
			Body: "",
		}

		conn.socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	}

	return conn.connected
}

// Runs connection loop
func (conn *Connection) runConnectionLoop() {
	defer conn.afterClose()

	for {
		closed := conn.IsClosed()

		if closed {
			return
		}

		url, err := conn.config.GetFullConnectionUrl()

		if err != nil {
			return
		}

		socket, _, err := websocket.DefaultDialer.Dial(url, nil)

		if err != nil {
			if conn.config.ErrorHandler != nil {
				conn.config.ErrorHandler.OnConnectionError(err)
			}

			if conn.config.RetryConnectionDelay == 0 {
				time.Sleep(DEFAULT_RETRY_CONNECTION_DELAY)
			} else if conn.config.RetryConnectionDelay > 0 {
				time.Sleep(conn.config.RetryConnectionDelay)
			}

			continue
		}

		// Set connection

		isConnected := conn.onConnected(socket)

		if !isConnected {
			socket.Close()
			return
		}

		go conn.sendHeartbeatMessages(socket)

		// Read messages and close

		conn.readIncomingMessages(socket)
	}
}

// Reads incoming messages
func (conn *Connection) readIncomingMessages(socket *websocket.Conn) {
	defer socket.Close()

	for {
		err := socket.SetReadDeadline(time.Now().Add(HEARTBEAT_MSG_PERIOD_SECONDS * 2 * time.Second))

		if err != nil {
			if !conn.IsClosed() && conn.config.ErrorHandler != nil {
				conn.config.ErrorHandler.OnConnectionError(err)
			}
			return
		}

		mt, message, err := socket.ReadMessage()

		if err != nil {
			if !conn.IsClosed() && conn.config.ErrorHandler != nil {
				conn.config.ErrorHandler.OnConnectionError(err)
			}
			return
		}

		if mt != websocket.TextMessage {
			continue
		}

		parsedMessage := simple_rpc_message.ParseRPCMessage(string(message))

		switch strings.ToUpper(parsedMessage.Method) {
		case "ERROR":
			if conn.config.ErrorHandler != nil {
				conn.config.ErrorHandler.OnServerError(parsedMessage.GetParam("Error-Code"), parsedMessage.GetParam("Error-Message"))
			}
		case "START-REQUEST-ACK":
			conn.ReceiveStartRequestAck(&parsedMessage)
		case "REQUEST-COUNT":
			conn.ReceiveRequestCount(&parsedMessage)
		}
	}
}

// Periodically sends heartbeat messages
func (conn *Connection) sendHeartbeatMessages(socket *websocket.Conn) {
	for {
		time.Sleep(HEARTBEAT_MSG_PERIOD_SECONDS * time.Second)

		// Send heartbeat message
		msg := simple_rpc_message.RPCMessage{
			Method: "HEARTBEAT",
			Params: nil,
			Body:   "",
		}

		err := socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))

		if err != nil {
			return
		}
	}
}

// Send a message to the module
func (conn *Connection) Send(msg *simple_rpc_message.RPCMessage) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.socket != nil {
		conn.socket.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
	}
}

// Sends START-REQUEST message
func (conn *Connection) sendStartRequest(id uint64, rType string, limit uint32) {
	msg := simple_rpc_message.RPCMessage{
		Method: "START-REQUEST",
		Params: map[string]string{
			"Request-ID":    fmt.Sprint(id),
			"Request-Type":  rType,
			"Request-Limit": fmt.Sprint(limit),
		},
		Body: "",
	}

	conn.Send(&msg)
}

// Sends END-REQUEST message
func (conn *Connection) sendEndRequest(id uint64) {
	msg := simple_rpc_message.RPCMessage{
		Method: "END-REQUEST",
		Params: map[string]string{
			"Request-ID": fmt.Sprint(id),
		},
		Body: "",
	}

	conn.Send(&msg)
}

// Sends GET-REQUEST-COUNT message
func (conn *Connection) sendGetRequestCount(rType string) {
	msg := simple_rpc_message.RPCMessage{
		Method: "GET-REQUEST-COUNT",
		Params: map[string]string{
			"Request-Type": rType,
		},
		Body: "",
	}

	conn.Send(&msg)
}

// Starts request, either by sending a START-REQUEST message or waiting for connection
func (conn *Connection) StartRequest(id uint64, rType string, limit uint32) {
	conn.mu.Lock()

	conn.pendingRequests[id] = &PendingRequest{
		requestType: rType,
		limit:       limit,
	}

	conn.mu.Unlock()

	conn.sendStartRequest(id, rType, limit)
}

// Ends a request, by sending the END-REQUEST message
func (conn *Connection) EndRequest(id uint64) {
	conn.mu.Lock()

	delete(conn.pendingRequests, id)

	conn.mu.Unlock()

	conn.sendEndRequest(id)
}

// Receives message: START-REQUEST-ACK
func (conn *Connection) ReceiveStartRequestAck(msg *simple_rpc_message.RPCMessage) {
	idStr := msg.GetParam("Request-ID")

	id, err := strconv.ParseUint(idStr, 10, 64)

	if err != nil {
		if conn.config.ErrorHandler != nil {
			conn.config.ErrorHandler.OnServerError("PROTOCOL_ERROR", "Server send an invalid Request-Id parameter for message START-REQUEST-ACK")
		}
		return
	}

	limitedStr := strings.ToUpper(msg.GetParam("Request-Limit-Reached"))

	limited := limitedStr == "TRUE"

	conn.cli.receiveRequestAck(id, limited)
}

// Sends GET-REQUEST-COUNT message to get the request count for a specific type
func (conn *Connection) GetRequestCount(rType string) {
	conn.mu.Lock()

	c := conn.pendingRequestCounts[rType]

	if c == 0 {
		conn.pendingRequestCounts[rType] = 1
	} else {
		conn.pendingRequestCounts[rType] = c + 1
	}

	conn.mu.Unlock()

	conn.sendGetRequestCount(rType)
}

// Call after a request count is done, either by receiving the response or due to timeout
func (conn *Connection) RequestCountDone(rType string) {
	conn.mu.Lock()

	c := conn.pendingRequestCounts[rType]

	if c > 0 {
		if c == 1 {
			delete(conn.pendingRequestCounts, rType)
		} else {
			conn.pendingRequestCounts[rType] = c - 1
		}

	}

	conn.mu.Unlock()
}

// Receives REQUEST-COUNT message
func (conn *Connection) ReceiveRequestCount(msg *simple_rpc_message.RPCMessage) {
	reqType := msg.GetParam("Request-Type")
	reqCountStr := msg.GetParam("Request-Count")

	reqCount, err := strconv.ParseUint(reqCountStr, 10, 32)

	if err != nil {
		if conn.config.ErrorHandler != nil {
			conn.config.ErrorHandler.OnServerError("PROTOCOL_ERROR", "Server send an invalid Request-Count parameter for message REQUEST-COUNT")
		}
		return
	}

	conn.cli.receiveRequestCount(reqType, uint32(reqCount))
}
