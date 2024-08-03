// Main

package main

import (
	"sync"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // Load env vars

	// Configure logs
	SetDebugLogEnabled(GetEnvBool("LOG_DEBUG", false))
	SetInfoLogEnabled(GetEnvBool("LOG_INFO", true))

	// Setup request controller
	requestController := CreateRequestController()

	// Setup server
	server := CreateHttpServer(HttpServerConfig{
		Port:               GetEnvInt("PORT", 8080),
		BindAddress:        GetEnvString("BIND_ADDRESS", ""),
		TlsEnabled:         GetEnvBool("TLS_ENABLED", false),
		TlsCertificateFile: GetEnvString("TLS_CERTIFICATE", ""),
		TlsPrivateKeyFile:  GetEnvString("TLS_PRIVATE_KEY", ""),
		AuthToken:          GetEnvString("AUTH_TOKEN", ""),
	}, requestController)

	// Run server

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go server.Run(wg)

	// Wait for all threads to finish

	wg.Wait()
}
