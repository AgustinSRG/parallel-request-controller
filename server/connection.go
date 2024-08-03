// Connection handler

package main

import (
	"fmt"
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

	// Server ref
	server *HttpServer

	// Mutex for the struct
	mu *sync.Mutex

	// Timestamp: Last time a HEARTBEAT message was received
	lastHeartbeat int64

	// True if closed
	closed bool
}

// Creates connection handler
func CreateConnectionHandler(conn *websocket.Conn, server *HttpServer) *ConnectionHandler {
	return &ConnectionHandler{
		id:            0,
		connection:    conn,
		server:        server,
		mu:            &sync.Mutex{},
		lastHeartbeat: 0,
		closed:        false,
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
		}
	}
}

// Called when a HEARTBEAT message is received from the client
func (ch *ConnectionHandler) receiveHeartbeat() {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.lastHeartbeat = time.Now().UnixMilli()
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

// Sends a message to the websocket client
func (wsh *ConnectionHandler) Send(msg *simple_rpc_message.RPCMessage) {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	if wsh.closed {
		return
	}

	if log_debug_enabled {
		wsh.LogDebug(">>> \n" + msg.Serialize())
	}

	wsh.connection.WriteMessage(websocket.TextMessage, []byte(msg.Serialize()))
}

// Checks if the client is sending HEARTBEAT messages
// If not, closes the connection
func (h *ConnectionHandler) checkHeartbeat() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now().UnixMilli()

	if (now - h.lastHeartbeat) >= HEARTBEAT_TIMEOUT_MS {
		h.connection.Close()
	}
}
