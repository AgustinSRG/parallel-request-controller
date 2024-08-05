// Configuration

package prc_client

import (
	"net/url"
	"time"
)

const DEFAULT_RETRY_CONNECTION_DELAY = 5 * time.Second

const DEFAULT_TIMEOUT = 10 * time.Second

// Configuration of the PRC client
type ClientConfig struct {
	// Parallel request controller base URL. Example: ws://example.com:8080
	Url string

	// Number of connections. 1 by default.
	NumberOfConnections int

	// Authentication token
	AuthToken string

	// Delay retry the connection. 5 seconds by default
	RetryConnectionDelay time.Duration

	// Error handler
	ErrorHandler ErrorHandler

	// Timeout for receiving responses from the server. By default: 10 seconds
	Timeout time.Duration
}

// Gets full connection URL (with authentication token)
func (config *ClientConfig) GetFullConnectionUrl() (string, error) {
	return url.JoinPath(config.Url, "./ws/"+url.PathEscape(config.AuthToken))
}
