// Client

"use strict";

import { DEFAULT_TIMEOUT, PRCClientConfig } from "./config";
import { PRCConnection } from "./connection";
import { StartedRequest } from "./started-request";
import { createPromiseListener, PromiseListener, resolvePromiseListener } from "./utils";

/**
 * Parallel request controller client
 */
export class PRCClient {
    // Configuration
    private config: PRCClientConfig;

    // List of connections
    private connections: PRCConnection[];

    // Index to balance the use of the connections
    private connectionBalancer: number;

    // ID for the next request
    private nextRequestId: number;

    private requestAckListeners: Map<number, PromiseListener<boolean>>;
    private requestCountListeners: Map<string, PromiseListener<number>[]>;

    /**
     * Creates new PRCClient
     * @param config Client configuration
     */
    constructor(config: PRCClientConfig) {
        this.config = config;

        const maxConnections = Math.max(1, Math.floor(config.numberOfConnections || 0));

        this.connections = [];
        this.connectionBalancer = 0;

        for (let i = 0; i < maxConnections; i++) {
            this.connections.push(new PRCConnection(this, config, i));
        }

        this.nextRequestId = 0;

        this.requestAckListeners = new Map();
        this.requestCountListeners = new Map();
    }

    /**
     * Connects to the PRC server
     */
    public connect() {
        this.connections.forEach(c => {
            c.connect();
        });
    }

    /**
     * Closes all the connections
     */
    public close() {
        this.connections.forEach(c => {
            c.close();
        });
    }

    /**
     * Gets a connection from the pool
     * @returns A connection from the pool
     */
    private getConnectionFromPool(): PRCConnection {
        const c = this.connections[this.connectionBalancer];

        this.connectionBalancer++;

        if (this.connectionBalancer >= this.connections.length) {
            this.connectionBalancer = 0;
        }

        return c;
    }

    /**
     * Gets a brand new ID for a request
     * @returns The request ID
     */
    private getNewRequestId(): number {
        const id = this.nextRequestId;

        this.nextRequestId++;

        return id;
    }

    /**
     * Indicates the start of a request
     * @param requestType Arbitrary string to indicate the request type
     * @param limit Máximum number of requests for the specified type allowed to be run in parallel
     * @returns An object containing the information to end the request when necessary
     */
    public async startRequest(requestType: string, limit: number): Promise<StartedRequest> {
        if (limit < 1) {
            return Promise.resolve({
                id: -1,
                limited: true,
            });
        }

        if (!requestType) {
            return Promise.reject(new Error("Invalid request type"));
        }

        const id = this.getNewRequestId();
        const c = this.getConnectionFromPool();

        const listener = createPromiseListener<boolean>(this.config.timeout || DEFAULT_TIMEOUT);

        this.requestAckListeners.set(id, listener);

        c.startRequest(id, requestType, limit);

        let limited: boolean;

        try {
            limited = await listener.promise;
        } catch (ex) {
            this.requestAckListeners.delete(id);
            c.endRequest(id);
            return Promise.reject(ex);
        }

        this.requestAckListeners.delete(id);

        return {
            id: id,
            limited: limited,
            connectionIndex: c.index,
        };
    }

    /**
     * Indicates the ending of a request
     * @param request The started request object
     */
    public endRequest(request: StartedRequest) {
        if (request.limited) {
            return;
        }

        const c = this.connections[request.connectionIndex];

        if (!c) {
            return;
        }

        c.endRequest(request.id);
    }

    /**
     * Counts the current number of parallel requests of a type
     * @param requestType Arbitrary string to indicate the request type
     * @returns The current number of parallel requests for the type
     */
    public async getRequestCount(requestType: string): Promise<number> {
        if (!requestType) {
            return Promise.reject(new Error("Invalid request type"));
        }

        const c = this.getConnectionFromPool();

        const listener = createPromiseListener<number>(this.config.timeout || DEFAULT_TIMEOUT);

        this.addRequestCountListener(requestType, listener);

        c.getRequestCount(requestType);

        let count: number;

        try {
            count = await listener.promise;
        } catch (ex) {
            c.requestCountDone(requestType);
            this.clearRequestCountListener(requestType, listener);
            return Promise.reject(ex);
        }

        return count;
    }


    /**
     * Receives a request ACK
     * @param id The request ID
     * @param limited True if limited
     */
    public receiveRequestAck(id: number, limited: boolean) {
        const listener = this.requestAckListeners.get(id);

        if (!listener) {
            return;
        }

        resolvePromiseListener(listener, limited);
    }

    /**
     * Receives a request count
     * @param rType The request type
     * @param count The request count
     */
    public receiveRequestCount(rType: string, count: number) {
        const listeners = this.requestCountListeners.get(rType);

        if (!listeners) {
            return;
        }

        for (const listener of listeners) {
            resolvePromiseListener(listener, count);
        }

        this.requestCountListeners.delete(rType);
    }

    /**
     * Adds request count listener
     * @param rType Request type
     * @param listener Listener
     */
    private addRequestCountListener(rType: string, listener: PromiseListener<number>) {
        if (!this.requestCountListeners.has(rType)) {
            this.requestCountListeners.set(rType, [listener]);
        } else {
            this.requestCountListeners.get(rType).push(listener);
        }
    }

    /**
     * Removes request count listener
     * @param rType Request type
     * @param listener Listener
     */
    private clearRequestCountListener(rType: string, listener: PromiseListener<number>) {
        if (!this.requestCountListeners.has(rType)) {
            return;
        }

        const listeners = this.requestCountListeners.get(rType);

        for (let i = 0; i < listeners.length; i++) {
            if (listeners[i] === listener) {
                listeners.splice(i, 1);
                return;
            }
        }
    }
}
