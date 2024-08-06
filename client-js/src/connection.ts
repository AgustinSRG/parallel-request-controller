// Connection

"use strict";

import { PRCClient } from "./client";
import { DEFAULT_RETRY_CONNECTION_DELAY, getFullConnectionUrl, PRCClientConfig } from "./config";
import { RawData, WebSocket } from "ws";
import { PendingRequest } from "./utils";
import { WebsocketMessage, makeMessage, parseMessage, getMessageParam } from "./message";

const HEARTBEAT_MSG_PERIOD_SECONDS = 30;

/**
 * Connection to the PRC server
 */
export class PRCConnection {
    // Connection index
    public index: number;

    // Configuration
    private config: PRCClientConfig;

    // Client reference
    private client: PRCClient;

    // True if closed
    private closed: boolean;

    // Socket
    private socket: WebSocket | null;

    // True if connection is open
    private connected: boolean;

    // Map of pending requests
    private pendingRequests: Map<number, PendingRequest>;

    // Map of pending request counts
    private pendingRequestCounts: Map<string, number>;

    // Interval to send heartbeat messages
    private heartBeatInterval?: NodeJS.Timeout;

    // Timeout to retry the connection
    private retryConnectionTimeout?: NodeJS.Timeout;

    // Timestamp of the last received heartbeat
    private lastReceivedHeartbeat: number;

    /**
     * Connection constructor
     * @param client Client
     * @param config Configuration
     * @param index Connection index
     */
    constructor(client: PRCClient, config: PRCClientConfig, index: number) {
        this.index = index;
        this.client = client;
        this.config = config;
        this.closed = true;
        this.connected = false;
        this.socket = null;
        this.pendingRequests = new Map();
        this.pendingRequestCounts = new Map();
        this.lastReceivedHeartbeat = Date.now();
    }

    /**
     * Starts the connection
     */
    public connect() {
        if (!this.closed) {
            return; // Already connected
        }

        this.closed = false;
        this.createConnection();
    }

    /**
     * Closes the connection
     */
    public close() {
        this.closed = true;

        if (this.heartBeatInterval) {
            clearInterval(this.heartBeatInterval);
            this.heartBeatInterval = undefined;
        }

        if (this.retryConnectionTimeout) {
            clearTimeout(this.retryConnectionTimeout);
            this.retryConnectionTimeout = undefined;
        }

        if (this.socket) {
            this.socket.close();
        }
    }

    /**
     * Creates the websocket connection
     */
    private createConnection() {
        let url: string;

        try {
            url = getFullConnectionUrl(this.config);
        } catch (ex) {
            this.closed = true;
            if (this.config.onConnectionError) {
                this.config.onConnectionError(ex);
            }
            return;
        }

        this.socket = new WebSocket(url);

        this.socket.on("open", this.onOpen.bind(this));
        this.socket.on("close", this.onClose.bind(this));
        this.socket.on("message", this.onMessage.bind(this));

        this.socket.on("error", err => {
            if (this.config.onConnectionError) {
                this.config.onConnectionError(err);
            }
        });
    }

    /**
     * Sends a message
     * @param msg The message to send
     */
    public send(msg: WebsocketMessage) {
        if (this.socket && this.connected) {
            this.socket.send(makeMessage(msg));
        }
    }

    /**
     * Called when the connection is established
     */
    private onOpen() {
        this.connected = true;

        if (this.heartBeatInterval) {
            clearInterval(this.heartBeatInterval);
            this.heartBeatInterval = undefined;
        }

        this.heartBeatInterval = setInterval(this.sendHeartBeat.bind(this), HEARTBEAT_MSG_PERIOD_SECONDS * 1000);

        // Send pending requests

        this.pendingRequests.forEach((pr, rid) => {
            this.send({
                type: "START-REQUEST",
                args: {
                    "Request-ID": rid + "",
                    "Request-Type": pr.requestType,
                    "Request-Limit": pr.limit + "",
                },
            });
        });

        this.pendingRequestCounts.forEach((_count, rType) => {
            this.send({
                type: "GET-REQUEST-COUNT",
                args: {
                    "Request-Type": rType,
                },
            });
        });
    }

    /**
     * Called when the connection is closed
     * @param code Code
     * @param reason Close reason
     */
    private onClose(code: number, reason: Buffer) {
        this.socket = null;
        this.connected = false;

        if (this.heartBeatInterval) {
            clearInterval(this.heartBeatInterval);
            this.heartBeatInterval = undefined;
        }

        if (this.retryConnectionTimeout) {
            clearTimeout(this.retryConnectionTimeout);
            this.retryConnectionTimeout = undefined;
        }

        if (!this.closed) {
            // Retry the connection
            this.retryConnectionTimeout = setTimeout(this.createConnection.bind(this), this.config.retryConnectionDelay || DEFAULT_RETRY_CONNECTION_DELAY);

            // Inform of the connection error
            if (this.config.onConnectionError) {
                this.config.onConnectionError(new Error("Lost connection. Code: " + code + ", Reason: " + reason));
            }
        }
    }

    /**
     * Sends HEARTBEAT message
     * Also checks for the server last HEARTBEAT message
     */
    private sendHeartBeat() {
        this.send({
            type: "HEARTBEAT",
        });

        if (Date.now() - this.lastReceivedHeartbeat >= (HEARTBEAT_MSG_PERIOD_SECONDS * 2 * 1000)) {
            if (this.socket) {
                this.socket.close();
            }
        }
    }

    /**
     * Called on received message
     * @param data Message data
     * @param isBinary True if binary
     */
    private onMessage(data: RawData, isBinary: boolean) {
        if (isBinary) {
            return;
        }

        const msg = parseMessage(data.toString("utf-8"));

        switch (msg.type) {
            case "ERROR":
                if (this.config.onServerError) {
                    this.config.onServerError(getMessageParam(msg, "Error-Code"), getMessageParam(msg, "Error-Message"));
                }
                break;
            case "HEARTBEAT":
                this.lastReceivedHeartbeat = Date.now();
                break;
            case "START-REQUEST-ACK":
                this.receiveStartRequestAck(msg);
                break;
            case "REQUEST-COUNT":
                this.receiveRequestCount(msg);
                break;
        }
    }

    /**
     * Sends the PRC server a message to indicate the start of a request
     * @param id Request ID
     * @param rType The request Type
     * @param limit Max number of allowed parallel requests for the specified request type
     */
    public startRequest(id: number, rType: string, limit: number) {
        this.pendingRequests.set(id, {
            requestType: rType,
            limit: limit,
        });

        this.send({
            type: "START-REQUEST",
            args: {
                "Request-ID": id + "",
                "Request-Type": rType,
                "Request-Limit": limit + "",
            },
        });
    }

    /**
     * Indicates the PRC server the ending of a request
     * @param id Request ID
     */
    public endRequest(id: number) {
        this.pendingRequests.delete(id);

        this.send({
            type: "END-REQUEST",
            args: {
                "Request-ID": id + "",
            },
        });
    }

    /**
     * Receives message: START-REQUEST-ACK
     * @param msg The message
     */
    private receiveStartRequestAck(msg: WebsocketMessage) {
        const id = parseInt(getMessageParam(msg, "Request-ID"), 10);

        if (isNaN(id)) {
            if (this.config.onServerError) {
                this.config.onServerError("PROTOCOL_ERROR", "Server send an invalid Request-Id parameter for message START-REQUEST-ACK");
            }
            return;
        }

        const limited = getMessageParam(msg, "Request-Limit-Reached").toUpperCase() === "TRUE";

        this.client.receiveRequestAck(id, limited);
    }

    /**
     * Sends a GET-REQUEST-COUNT message to the PRC server to count parallel requests
     * @param rType The request type
     */
    public getRequestCount(rType: string) {
        if (this.pendingRequestCounts.has(rType)) {
            const c = this.pendingRequestCounts.get(rType);
            this.pendingRequestCounts.set(rType, c + 1);
        } else {
            this.pendingRequestCounts.set(rType, 1);
        }

        this.send({
            type: "GET-REQUEST-COUNT",
            args: {
                "Request-Type": rType,
            },
        });
    }

    /**
     * Call after a request count is done
     * @param rType The request type
     */
    public requestCountDone(rType: string) {
        if (this.pendingRequestCounts.has(rType)) {
            const c = this.pendingRequestCounts.get(rType);

            if (c === 1) {
                this.pendingRequestCounts.delete(rType);
            } else {
                this.pendingRequestCounts.set(rType, c - 1);
            }
        }
    }

    /**
     * Receives message: REQUEST-COUNT
     * @param msg The message
     */
    private receiveRequestCount(msg: WebsocketMessage) {
        const rType = getMessageParam(msg, "Request-Type");
        const count = parseInt(getMessageParam(msg, "Request-Type"), 10);

        if (isNaN(count)) {
            if (this.config.onServerError) {
                this.config.onServerError("PROTOCOL_ERROR", "Server send an invalid Request-Count parameter for message REQUEST-COUNT");
            }
            return;
        }

        this.client.receiveRequestCount(rType, count);
    }
}

