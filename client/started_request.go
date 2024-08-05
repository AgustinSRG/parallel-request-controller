// Started request

package prc_client

// Started request. Keep it to indicate the ending.
type StartedRequest struct {
	// Request ID
	id uint64

	// Reference to the connection where the start message wa sent
	connection *Connection
}

// Indicates the ending of the request
func (request *StartedRequest) End() {
	request.connection.EndRequest(request.id)
}
