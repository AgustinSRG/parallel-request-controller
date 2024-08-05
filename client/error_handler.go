// Error handler

package prc_client

// Error handler for the PRC client
type ErrorHandler interface {
	// Called on connection error. The client will retry the connection if not manually closed
	OnConnectionError(err error)

	// Called when an ERROR message is received from the server
	OnServerError(code string, message string)
}
