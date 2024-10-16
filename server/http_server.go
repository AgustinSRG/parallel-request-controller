// HTTP server

package main

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

const DEFAULT_HTTP_RESPONSE = "Parallel request controller server."

const WS_PREFIX = "/ws/"

// HTTP server configuration
type HttpServerConfig struct {
	// Server port
	Port int

	// Server bind address
	BindAddress string

	// TLS enabled?
	TlsEnabled bool

	// Certificate file
	TlsCertificateFile string

	// Key file
	TlsPrivateKeyFile string

	// Auth token
	AuthToken string
}

// HTTP websocket server
type HttpServer struct {
	// Server config
	config HttpServerConfig

	// Mutex
	mu *sync.Mutex

	// Next connection ID
	nextConnectionId uint64

	// Websocket connection upgrader
	upgrader *websocket.Upgrader

	// Request controller
	requestController *RequestController
}

// Creates HTTP server
func CreateHttpServer(config HttpServerConfig, requestController *RequestController) *HttpServer {
	if len(config.AuthToken) == 0 {
		LogWarning("The variable AUTH_TOKEN is empty or not set. This variable is required for clients to authenticate. Please, set it before starting the server.")
	}

	return &HttpServer{
		config:            config,
		upgrader:          &websocket.Upgrader{},
		mu:                &sync.Mutex{},
		nextConnectionId:  0,
		requestController: requestController,
	}
}

// Gets an unique ID for a connection
func (server *HttpServer) GetConnectionId() uint64 {
	server.mu.Lock()
	defer server.mu.Unlock()

	id := server.nextConnectionId

	server.nextConnectionId++

	return id
}

// Serves HTTP request
func (server *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ip, _, err := net.SplitHostPort(req.RemoteAddr)

	if err != nil {
		LogError(err, "Error parsing request IP")
		w.WriteHeader(200)
		fmt.Fprint(w, DEFAULT_HTTP_RESPONSE)
		return
	}

	LogInfo("[HTTP] [FROM: " + ip + "] " + req.Method + " " + req.URL.Path)

	if strings.HasPrefix(req.URL.Path, WS_PREFIX) {
		authToken := getAuthTokenFromPath(req.URL.Path)

		// Check auth token
		if subtle.ConstantTimeCompare([]byte(server.config.AuthToken), []byte(authToken)) != 1 {
			w.WriteHeader(403)
			LogDebug("[HTTP] [FROM: " + ip + "] [FORBIDDEN] " + req.Method + " " + req.URL.Path)
			fmt.Fprint(w, "Forbidden.")
			return
		}

		// Upgrade connection

		c, err := server.upgrader.Upgrade(w, req, nil)
		if err != nil {
			LogError(err, "Error upgrading connection")
			return
		}

		// Handle connection
		ch := CreateConnectionHandler(c, server, server.requestController)
		go ch.Run()
	} else {
		w.WriteHeader(200)
		fmt.Fprint(w, DEFAULT_HTTP_RESPONSE)
	}
}

// Runs the server
// wg - Wait group
func (server *HttpServer) Run(wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
	}()

	port := server.config.Port
	bind_addr := server.config.BindAddress

	if server.config.TlsEnabled {
		certFile := server.config.TlsCertificateFile
		keyFile := server.config.TlsPrivateKeyFile

		LogInfo("[HTTPS] Listening on " + bind_addr + ":" + strconv.Itoa(port))
		errSSL := http.ListenAndServeTLS(bind_addr+":"+strconv.Itoa(port), certFile, keyFile, server)

		if errSSL != nil {
			LogError(errSSL, "Error starting HTTPS server")
		}
	} else {
		LogInfo("[HTTP] Listening on " + bind_addr + ":" + strconv.Itoa(port))
		errHTTP := http.ListenAndServe(bind_addr+":"+strconv.Itoa(port), server)

		if errHTTP != nil {
			LogError(errHTTP, "Error starting HTTP server")
		}
	}
}

// Gets authentication token from PATH
func getAuthTokenFromPath(path string) string {
	if len(path) <= len(WS_PREFIX) {
		return ""
	}

	authPart := path[len(WS_PREFIX):]

	if len(authPart) == 0 {
		return ""
	}

	authPartSplit := strings.Split(authPart, "/")

	if len(authPartSplit) == 0 {
		return ""
	}

	token, err := url.PathUnescape(authPartSplit[0])

	if err != nil {
		return ""
	}

	return token
}
