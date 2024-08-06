// Config

"use strict";

export const DEFAULT_RETRY_CONNECTION_DELAY = 5 * 1000;

export const DEFAULT_TIMEOUT = 10 * 1000;

/**
 * PRC Client configuration
 */
export interface PRCClientConfig {
    // Parallel request controller base URL. Example: ws://example.com:8080
    url: string;

    // Authentication token
    authToken: string;

    // Number of connections. 1 by default.
    numberOfConnections?: number;

    // Delay to retry the connection after an error happens (milliseconds). 5000 (5 seconds) by default.
    retryConnectionDelay?: number;

    // Function to call in order to log connection errors. The connection will be retried when this happens.
    onConnectionError?: (err: Error) => void;

    onServerError?: (code: string, message: string) => void;

    // Timeout for receiving responses from the server (milliseconds). By default: 10000 (10 seconds)
    timeout?: number;
}

/**
 * Returns URL with path and authentication token
 * @param config The configuration
 * @returns The full connection URL for websocket
 */
export function getFullConnectionUrl(config: PRCClientConfig): string {
    return (new URL("./ws/" + encodeURIComponent(config.authToken), config.url)).toString();
}
