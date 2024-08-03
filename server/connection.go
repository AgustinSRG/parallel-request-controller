// Connection handler

package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	simple_rpc_message "github.com/AgustinSRG/go-simple-rpc-message"
	"github.com/gorilla/websocket"
)

// Period to send HEARTBEAT messages to the client
const HEARTBEAT_MSG_PERIOD_SECONDS = 30

// Max time with no HEARTBEAT messages to consider the connection dead
const HEARTBEAT_TIMEOUT_MS = 2 * HEARTBEAT_MSG_PERIOD_SECONDS * 1000

// Connection handler
type ConnectionHandler struct {
	// Connection id
	id uint64

	// Connection
	connection *websocket.Conn

	// HTTP server
	server *HttpServer

	// Request controller
	requestController *RequestController

	// Mutex for the struct
	mu *sync.Mutex

	// Timestamp: Last time a HEARTBEAT message was received
	lastHeartbeat int64

	// True if closed
	closed bool

	// Mutex for the requests map
	muRequests *sync.Mutex

	// Requests mapping ID -> Type
	requests map[string]string
}

// Creates connection handler
func CreateConnectionHandler(conn *websocket.Conn, server *HttpServer, requestController *RequestController) *ConnectionHandler {
	return &ConnectionHandler{
		id:                0,
		connection:        conn,
		server:            server,
		requestController: requestController,
		mu:                &sync.Mutex{},
		lastHeartbeat:     0,
		closed:            false,
		muRequests:        &sync.Mutex{},
		requests:          make(map[string]string),
	}
}

func (ch *ConnectionHandler) LogError(err error, msg string) {
	LogError(err, "[Request: "+fmt.Sprint(ch.id)+"] "+msg)
}

func (ch *ConnectionHandler) LogInfo(msg string) {
	LogInfo("[Request: " + fmt.Sprint(ch.id) + "] " + msg)
}

func (ch *ConnectionHandler) LogDebug(msg string) {
	LogDebug("[Request: " + fmt.Sprint(ch.id) + "] " + msg)
}

func (ch *ConnectionHandler) onClose() {
	ch.mu.Lock()

	ch.closed = true

	ch.mu.Unlock()

	// TODO: Finish all pending requests
}

// Runs connection handler
func (ch *ConnectionHandler) Run() {
	defer func() {
		if err := recover(); err != nil {
			switch x := err.(type) {
			case string:
				ch.LogError(nil, "Error: "+x)
			case error:
				ch.LogError(x, "Connection closed with error")
			default:
				ch.LogError(nil, "Connection Crashed!")
			}
		}
		ch.LogInfo("Connection closed.")
		// Ensure connection is closed
		ch.connection.Close()
		// Release resources
		ch.onClose()
	}()

	// Get a connection ID
	ch.id = ch.server.GetConnectionId()

	c := ch.connection

	ch.LogInfo("Connection established.")

	ch.lastHeartbeat = time.Now().UnixMilli()
	go ch.sendHeartbeatMessages() // Start heartbeat sending

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break // Closed
		}

		if mt != websocket.TextMessage {
			continue
		}

		if log_debug_enabled {
			ch.LogDebug("<<< \n" + string(message))
		}

		msg := simple_rpc_message.ParseRPCMessage(string(message))

		switch msg.Method {
		case "HEARTBEAT":
			ch.receiveHeartbeat()
		case "START-REQUEST":
			ch.receiveStartRequest(&msg)
		case "END-REQUEST":
			ch.receiveEndRequest(&msg)
		case "GET-REQUEST-COUNT":
			ch.receiveGetRequestCount(&msg)
		}
	}
}

// Called when a HEARTBEAT message is received from the client
func (ch *ConnectionHandler) receiveHeartbeat() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.lastHeartbeat = time.Now().UnixMilli()
}

func (ch *ConnectionHandler) AddRequest(requestId string, requestType string) bool {
	ch.muRequests.Lock()
	defer ch.muRequests.Unlock()

	rt := ch.requests[requestId]

	if len(rt) > 0 {
		return false
	}

	ch.requests[requestId] = requestType

	return true
}

func (ch *ConnectionHandler) receiveStartRequest(msg *simple_rpc_message.RPCMessage) {
	requestId := msg.GetParam("Request-ID")

	if len(requestId) == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Missing parameter 'Request-ID' for message 'START-REQUEST'")
		return
	}

	requestType := msg.GetParam("Request-Type")

	if len(requestType) == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Missing parameter 'Request-Type' for message 'START-REQUEST'")
		return
	}

	requestLimitStr := msg.GetParam("Request-Limit")

	requestLimit, err := strconv.ParseUint(requestLimitStr, 10, 32)

	if err != nil {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Parameter 'Request-Limit' for message 'START-REQUEST' must be a valid integer")
		return
	}

	// Checks if id is duplicated

	available := ch.AddRequest(requestId, requestType)

	if !available {
		ch.SendErrorMessage("REQUEST_ID_DUPLICATED", "You sent multiple 'START-REQUEST' messages with the same request id. Only the first one applies. The rest will are dropped.")
		return
	}

	// Checks limit

	canStartRequest := ch.requestController.TryStartRequest(requestType, uint32(requestLimit))

	limited := "FALSE"

	if !canStartRequest {
		limited = "TRUE"
	}

	// Reply

	replyMsg := simple_rpc_message.RPCMessage{
		Method: "START-REQUEST-ACK",
		Params: map[string]string{
			"Request-ID":            requestId,
			"Request-Limit-Reached": limited,
		},
		Body: "",
	}

	ch.Send(&replyMsg)
}

func (ch *ConnectionHandler) RemoveRequest(requestId string) string {
	ch.muRequests.Lock()
	defer ch.muRequests.Unlock()

	rt := ch.requests[requestId]

	if len(rt) == 0 {
		return ""
	}

	delete(ch.requests, requestId)

	return rt
}

func (ch *ConnectionHandler) receiveEndRequest(msg *simple_rpc_message.RPCMessage) {
	requestId := msg.GetParam("Request-ID")

	if len(requestId) == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Missing parameter 'Request-ID' for message 'END-REQUEST'")
		return
	}

	requestType := ch.RemoveRequest(requestId)

	if len(requestType) == 0 {
		return // Multiple end requests ignored
	}

	ch.requestController.EndRequest(requestType)
}

func (ch *ConnectionHandler) receiveGetRequestCount(msg *simple_rpc_message.RPCMessage) {
	requestType := msg.GetParam("Request-Type")

	if len(requestType) == 0 {
		ch.SendErrorMessage("PROTOCOL_ERROR", "Missing parameter 'Request-Type' for message 'GET-REQUEST-COUNT'")
		return
	}

	count := ch.requestController.GetRequestCount(requestType)

	// Reply

	replyMsg := simple_rpc_message.RPCMessage{
		Method: "REQUEST-COUNT",
		Params: map[string]string{
			"Request-Type":  requestType,
			"Request-Count": fmt.Sprint(count),
		},
		Body: "",
	}

	ch.Send(&replyMsg)
}

// Task to send HEARTBEAT periodically
func (ch *ConnectionHandler) sendHeartbeatMessages() {
	for {
		time.Sleep(HEARTBEAT_MSG_PERIOD_SECONDS * time.Second)

		if ch.closed {
			return // Closed
		}

		// Send heartbeat message
		msg := simple_rpc_message.RPCMessage{
			Method: "HEARTBEAT",
			Params: nil,
			Body:   "",
		}

		ch.Send(&msg)

		// Check heartbeat
		ch.checkHeartbeat()
	}
}

// Send error message
func (ch *ConnectionHandler) SendErrorMessage(errorCode string, errorMessage string) {
	msg := simple_rpc_message.RPCMessage{
		Method: "ERROR",
		Params: map[string]string{
			"Error-Code":    errorCode,
			"Error-Message": errorMessage,
		},
		Body: "",
	}

	ch.Send(&msg)

}

// Sends a message to the websocket client
func (ch *ConnectionHandler) Send(msg *simple_rpc_message.RPCMessage) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.closed {
		return
	}

	if log_debug_enabled {
		ch.LogDebug(">>> \n" + msg.Serialize())
	}

	ch.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Checks if the client is sending HEARTBEAT messages
// If not, closes the connection
func (ch *ConnectionHandler) checkHeartbeat() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	now := time.Now().UnixMilli()

	if (now - ch.lastHeartbeat) >= HEARTBEAT_TIMEOUT_MS {
		ch.connection.Close()
	}
}
